package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/bits"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

//go:generate go run ../../rlp/rlpgen -type Access -decoder=true -out gen_access_rlp.go
//go:generate go run ../../rlp/rlpgen -type SlotOp -decoder=true -out gen_slot_op_rlp.go
//go:generate go run github.com/fjl/gencodec -type Access -field-override metaMarshaling -out gen_access_json.go

// Operation masks for state access
const (
	AccessOpRead  uint8 = 1 << iota // 01 - Represents read operations on state
	AccessOpWrite                   // 10 - Represents write operations on state

	AccessOpRW = AccessOpRead | AccessOpWrite // 11 - Represents both read and write operations

	// Meta field shifts and masks (2 bits per category)
	AccessShiftNonce   = 4
	AccessShiftCode    = 2
	AccessShiftBalance = 0

	balanceMaskWrite = AccessOpWrite << AccessShiftBalance
	codeMaskWrite    = AccessOpWrite << AccessShiftCode
	nonceMaskWrite   = AccessOpWrite << AccessShiftNonce

	allWriteMask = balanceMaskWrite | codeMaskWrite | nonceMaskWrite
	allReadMask  = (balanceMaskWrite >> 1) | (codeMaskWrite >> 1) | (nonceMaskWrite >> 1)
)

func getMetaField(meta uint8, shift uint8) uint8 {
	return (meta >> shift) & AccessOpRW
}

// Accesses is a collection of state access records for accounts, providing
// a complete picture of which accounts and storage slots were accessed and how.
type Accesses []Access

func (aa Accesses) MarshalJSON() ([]byte, error) {
	length := len(aa)
	if length == 0 {
		return nil, nil
	}

	data := make(map[common.Address]Access, length)
	for _, value := range aa {
		data[value.Address] = value
	}
	return json.Marshal(data)
}

type StorageOps []SlotOp

func (s StorageOps) MarshalJSON() ([]byte, error) {
	length := len(s)
	if length == 0 {
		return nil, nil
	}

	data := make(map[common.Hash]uint8, length)
	for _, kv := range s {
		data[kv.Slot] = kv.Ops
	}
	return json.Marshal(data)
}

// SlotOp records operations performed on a specific storage slot.
// It contains the slot hash and a bitmask representing performed operations.
type SlotOp struct {
	Slot common.Hash `json:"slot"` // Hash of the storage slot
	Ops  uint8       `json:"ops"`  // Bitmask of operations (read, write, or both)
}

// Access records operations performed on a specific account during execution.
// It tracks balance operations, account state operations (like nonce/code changes),
// and storage operations using bitmasks to efficiently represent read/write patterns.
type Access struct {
	Address common.Address `json:"-"`                                // Ethereum address of the accessed account
	Meta    uint8          `json:"-"`                                // Bitmask of account state operations (nonce, code, balance)
	Storage StorageOps     `json:"storage,omitempty" rlp:"optional"` // Operations performed on storage slots
}

func (a *Access) Balance() uint8 {
	return a.Meta >> AccessShiftBalance & AccessOpRW
}

func (a *Access) Nonce() uint8 {
	return a.Meta >> AccessShiftNonce & AccessOpRW
}

func (a *Access) Code() uint8 {
	return a.Meta >> AccessShiftCode & AccessOpRW
}

// metaMarshaling provides custom marshaling rules for Access
type metaMarshaling struct {
	Balance uint8 `json:"balance,omitempty"`
	Nonce   uint8 `json:"nonce,omitempty"`
	Code    uint8 `json:"code,omitempty"`
}

// OpsCount returns the total value of operations accessed by all accounts.
// This is useful for calculating gas costs or estimating state access footprint.
func (a Accesses) OpsCount() (total int) {
	for _, access := range a {
		total += bits.OnesCount8(access.Meta)
		for _, slotOp := range access.Storage {
			total += bits.OnesCount8(slotOp.Ops)
		}
	}
	return total
}

// hasMetaConflict checks for a write-then-read conflict between two operations.
// It only flags a conflict if the first operation writes and the second reads,
// which would create a data dependency that prevents parallel execution.
func hasMetaConflict(meta1, meta2 uint8) bool {
	// 1. (meta1 & allWriteMask): Isolates bits in meta1 that represent WRITE operations (or RW, since 0b11 & 0b10 = 0b10).
	//    Result: NN_w CC_w BB_w 00 (where _w is 1 if a write occurred, 0 otherwise).
	// 2. (meta2 & allReadMask): Isolates bits in meta2 that represent READ operations (or RW, since 0b11 & 0b01 = 0b01).
	//    Result: NN_r CC_r BB_r 00 (where _r is 1 if a read occurred, 0 otherwise).
	// 3. ((meta2 & allReadMask) << 1): Shifts the isolated read bits from meta2 to align them with the write bit positions.
	//    If meta2 had a read (e.g., 0b01 for a field), after the <<1 shift, this becomes 0b10, matching the write bit pattern.
	//    Result: NN_r' CC_r' BB_r' 00 (where _r' is 1 if a read occurred, 0 otherwise, but now at the 'write' bit's position).
	// 4. Intersection (AND) of the results from step 1 and step 3:
	//    This checks if, for any field (Nonce, Code, Balance), meta1 has a write operation (its bit is set in the result of step 1)
	//    AND, for the same field, meta2 has a read operation (its corresponding bit, after shifting, is set in the result of step 3).
	//    If both conditions are true for a field, the bitwise AND will set the bit for that field in the final result.
	// 5. != 0: If at least one bit is set in the final ANDed result, it signifies a conflict.
	return (meta1&allWriteMask)&((meta2&allReadMask)<<1) != 0
}

func hasSlotConflict(ops1, ops2 uint8) bool {
	// Conflict only if ops1 is Write and ops2 is Read
	return (ops1&AccessOpWrite != 0) && (ops2&AccessOpRead != 0)
}

// HasAccessesConflicts combines two access lists, checking for conflicts in the process.
// Returns a merged map if there are no conflicts, or nil and true if conflicts exist.
// A conflict occurs when one access list writes to a state element that the other reads.
func HasAccessesConflicts(accesses1, accesses2 Accesses) (map[common.Address]Access, bool) {
	// Create a map for the result
	result := make(map[common.Address]Access, len(accesses1))

	// Process the first access list
	for _, access := range accesses1 {
		result[access.Address] = access
	}

	// Process and check the second access list
	for _, access2 := range accesses2 {
		access1, found := result[access2.Address]
		if !found {
			result[access2.Address] = access2
			continue
		}

		// Meta operation conflicts
		if hasMetaConflict(access1.Meta, access2.Meta) {
			return nil, true
		}

		// Check storage slot conflicts and prepare for merge
		slotMap := make(map[common.Hash]uint8, len(access1.Storage))

		// Build map of slots from first address
		for _, slot := range access1.Storage {
			slotMap[slot.Slot] = slot.Ops
		}

		// Check for conflicts with slots from second address
		for _, slot2 := range access2.Storage {
			if ops1, ok := slotMap[slot2.Slot]; ok {
				if hasSlotConflict(ops1, slot2.Ops) {
					return nil, true
				}
			}
			slotMap[slot2.Slot] |= slot2.Ops
		}

		// No conflicts found, create merged entry
		mergedAccess := Access{
			Address: access1.Address,
			Meta:    access1.Meta | access2.Meta,
		}

		// Convert merged slot map back to slice
		if l := len(slotMap); l > 0 {
			mergedAccess.Storage = make([]SlotOp, 0, l)
			for slot, ops := range slotMap {
				mergedAccess.Storage = append(mergedAccess.Storage, SlotOp{
					Slot: slot,
					Ops:  ops,
				})
			}
		}

		// Update the entry in the result map
		result[access1.Address] = mergedAccess
	}

	// If we got here, no conflicts were found
	return result, false
}

// toAccesses converts a map of address accesses back to an Accesses slice.
// This is primarily used as a helper for MergeAccesses to maintain the slice-based API.
func toAccesses(accessMap map[common.Address]Access) Accesses {
	result := make(Accesses, 0, len(accessMap))
	for _, access := range accessMap {
		result = append(result, access)
	}
	return result
}

// MergeAccesses checks for conflicts between two access lists and returns
// a merged list if there are none. The merging combines Read/Write operations
// while maintaining their semantics. If any conflicts are detected, it returns
// nil and false.
func MergeAccesses(accesses1, accesses2 Accesses) (Accesses, bool) {
	mergedMap, conflicts := HasAccessesConflicts(accesses1, accesses2)

	if conflicts {
		return nil, false
	}

	// Convert the map back to a slice
	return toAccesses(mergedMap), true
}

func (a Accesses) String() string {
	if len(a) == 0 {
		return ""
	}

	var sb strings.Builder

	sort.Slice(a, func(i, j int) bool { return bytes.Compare(a[i].Address[:], a[j].Address[:]) < 0 })

	sb.WriteString("Accesses:\n")
	for _, access := range a {
		sb.WriteString(fmt.Sprintf("Address: %s\n", access.Address.Hex()))
		sb.WriteString(fmt.Sprintf("\tbalance: %02b\n", getMetaField(access.Meta, AccessShiftBalance)))
		sb.WriteString(fmt.Sprintf("\tnonce: %02b\n", getMetaField(access.Meta, AccessShiftNonce)))
		sb.WriteString(fmt.Sprintf("\tcode: %02b\n", getMetaField(access.Meta, AccessShiftCode)))

		sort.Slice(access.Storage, func(i, j int) bool { return bytes.Compare(access.Storage[i].Slot[:], access.Storage[j].Slot[:]) < 0 })

		for _, slot := range access.Storage {
			sb.WriteString(fmt.Sprintf("\t\tslot: %s, ops: %b\n", slot.Slot.Hex(), slot.Ops))
		}
	}

	return sb.String()
}
