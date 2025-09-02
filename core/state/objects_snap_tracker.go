package state

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

type baseline struct {
	// modifications
	deleted    bool // deleted before first tx
	created    bool // created since first tx
	destructed bool // destructed since first tx
	contracted bool // contracted since first tx

	// previous values
	code    []byte
	nonce   *uint64
	balance *uint256.Int
	storage Storage

	origin *types.StateAccount
}

func (bl *baseline) isNew() bool {
	return bl.created
}

func (bl *baseline) copy() *baseline {
	return &baseline{
		created:    bl.created,
		deleted:    bl.deleted,
		destructed: bl.destructed,
		contracted: bl.contracted,
		code:       bl.code,
		nonce:      bl.nonce,
		balance:    bl.balance,
		storage:    bl.storage.Copy(),
		origin:     bl.origin,
	}
}

type objectsSnapshotTracker struct {
	stateDB *StateDB
	txs     []common.Hash

	mutations    map[common.Address]*mutation
	accessList   *accessList
	accessEvents *AccessEvents

	baselines map[common.Address]*baseline
	changes   []journalEntry

	err error
}

func newObjectsSnapshotTracker(state *StateDB) *objectsSnapshotTracker {
	var acl *accessList
	if state.accessList != nil {
		acl = state.accessList.Copy()
	}

	var ace *AccessEvents
	if state.accessEvents != nil {
		ace = state.accessEvents.Copy()
	}

	mutations := make(map[common.Address]*mutation, len(state.mutations))
	for addr, op := range state.mutations {
		mutations[addr] = op.copy()
	}

	return &objectsSnapshotTracker{
		stateDB:      state,
		mutations:    mutations,
		accessList:   acl,
		accessEvents: ace,
		baselines:    make(map[common.Address]*baseline),
	}
}

// copy creates a deep copy of the transactions snapshot stack
// associated with a new stateDB database instance.
func (t *objectsSnapshotTracker) copy(statedb *StateDB) snapshotTracker {
	newStack := newObjectsSnapshotTracker(statedb)

	newStack.baselines = make(map[common.Address]*baseline, len(t.baselines))
	for address, account := range t.baselines {
		newStack.baselines[address] = account.copy()
	}

	newStack.mutations = make(map[common.Address]*mutation, len(t.mutations))
	for addr, op := range t.mutations {
		newStack.mutations[addr] = op.copy()
	}

	if t.accessList != nil {
		newStack.accessList = t.accessList.Copy()
	}
	if t.accessEvents != nil {
		newStack.accessEvents = t.accessEvents.Copy()
	}

	return newStack
}

// restore reverts all transactions in the stack and restores the original stateDB.
func (t *objectsSnapshotTracker) restore() (s *StateDB, err error) {
	defer func() {
		if r := recover(); r != nil || err != nil {
			log.Error(dumpJournal(t.changes))
			if r != nil {
				panic(r)
			}
		}
	}()

	baselines, err := t.baselineAccounts()
	if err != nil {
		return nil, err
	}

	for address, account := range baselines {
		if account.destructed && !account.deleted {
			delete(t.stateDB.stateObjectsDestruct, address)
		}

		if account.created {
			delete(t.stateDB.stateObjects, address)
			continue
		}

		obj := t.stateDB.stateObjects[address]
		if account.contracted || obj == nil { // created contract or emptied
			clearedObj := newObject(t.stateDB, address, account.origin)
			if obj != nil {
				clearedObj.setBalance(obj.Balance())
			}
			obj = clearedObj
		} else if len(account.storage) > 0 { // existed contract
			for key, value := range account.storage {
				obj.dirtyStorage[key] = value
			}
			obj.finalise()
		}

		// EoA or EIP-7702
		if account.nonce != nil && obj.Nonce() != *account.nonce {
			obj.setNonce(*account.nonce)
		}
		if account.code != nil {
			obj.setCode(crypto.Keccak256Hash(account.code), account.code) // todo: set correct dirtyCode flag
		}

		if account.balance != nil && !obj.Balance().Eq(account.balance) {
			obj.setBalance(account.balance)
		}

		if !obj.empty() {
			t.stateDB.stateObjects[address] = obj
		} else {
			delete(t.stateDB.stateObjects, address)
		}
	}

	if err := t.stateDB.Error(); err != nil {
		return nil, err
	}

	for _, txHash := range t.txs {
		if logs := len(t.stateDB.logs[txHash]); logs != 0 {
			t.stateDB.logSize -= uint(logs)
			delete(t.stateDB.logs, txHash)
		}
	}

	t.stateDB.mutations = t.mutations
	t.stateDB.accessList = t.accessList
	t.stateDB.accessEvents = t.accessEvents

	err = t.validateRevert()
	if err != nil {
		return nil, err
	}

	t.baselines = nil
	t.changes = nil
	t.txs = nil
	t.err = nil

	return t.stateDB, nil
}

// captureChanges captures a new transaction changes from the current journal
// and adds it to the stack. This is called at transaction boundaries to preserve
// the state changes made during that transaction's execution.
func (t *objectsSnapshotTracker) captureChanges(state *StateDB) {
	t.txs = append(t.txs, state.thash)
	t.changes = slices.Grow(t.changes, len(state.journal.entries))
	for _, entry := range state.journal.entries {
		t.changes = append(t.changes, entry.copy())

		switch e := entry.(type) {
		case createContractChange:
			_, exists := t.baselines[e.account]
			if exists {
				continue
			}

			t.baselines[e.account] = &baseline{
				contracted: true,
				origin:     state.getStateObject(e.account).origin,
			}

		case createObjectChange:
			if _, exists := t.baselines[e.account]; exists {
				continue
			}

			t.baselines[e.account] = &baseline{created: true}

		case selfDestructChange:
			existing := t.baselines[e.account]
			if !existing.destructed {
				existing.deleted = state.stateObjectsDestruct[e.account] != nil
			}
			existing.storage = make(Storage)
			existing.destructed = true

		case nonceChange:
			existing, exists := t.baselines[e.account]
			if !exists {
				t.baselines[e.account] = &baseline{nonce: &e.prev}
			} else if existing.nonce == nil {
				existing.nonce = &e.prev
			}

		case balanceChange:
			existing, exists := t.baselines[e.account]
			if !exists {
				t.baselines[e.account] = &baseline{balance: e.prev.Clone()}
			} else if existing.balance == nil {
				existing.balance = e.prev.Clone()
			}

		case storageChange:
			existing, exists := t.baselines[e.account]
			if !exists {
				t.baselines[e.account] = &baseline{storage: Storage{e.key: e.prevvalue}}
			} else if existing.storage == nil {
				existing.storage = Storage{e.key: e.prevvalue}
			} else if _, ok := existing.storage[e.key]; !ok {
				existing.storage[e.key] = e.prevvalue
			}

		case codeChange:
			existing, exists := t.baselines[e.account]
			if !exists {
				t.baselines[e.account] = &baseline{code: append([]byte{}, e.prevCode...)}
			} else if existing.code == nil {
				existing.code = append([]byte{}, e.prevCode...)
			}

		case accessListAddAccountChange, accessListAddSlotChange,
			transientStorageChange, refundChange, addLogChange, touchChange:
			continue

		default:
			t.err = fmt.Errorf("unexpected journal entry: dirtied %s, type %s)", entry.dirtied(), reflect.TypeOf(e))
			return
		}
	}
}

func (t *objectsSnapshotTracker) baselineAccounts() (map[common.Address]*baseline, error) {
	return t.baselines, t.err
}

// validateRevert verifies that the revert operation correctly restored state
// by comparing current account states with their original values.
//
// For each account in the original state map, it checks:
// 1. Deleted accounts: verifies they're properly removed or reset
// 2. Existing accounts: verifies properties match original values
//   - Nonce restored to original value
//   - Balance restored to original value
//   - Code hash restored to original value
func (t *objectsSnapshotTracker) validateRevert() (err error) {
	originalAccounts, err := t.baselineAccounts()
	if err != nil || len(originalAccounts) == 0 {
		return err
	}

	// Check each account against its original state
	for address, original := range originalAccounts {
		obj := t.stateDB.stateObjects[address]
		if obj == nil {
			// If original is nil or marks a new account, deletion is correct
			if !original.isNew() {
				return fmt.Errorf("obj %s not found", address)
			}
			// Check if mutations map correctly tracks this deletion
			op := t.stateDB.mutations[address]
			if op != nil && !op.isDelete() {
				return fmt.Errorf("obj %s not found", address)
			}
			// Verify consistent state between mutations and stateObjectsDestruct
			_, destructed := t.stateDB.stateObjectsDestruct[address]
			if (op != nil && op.isDelete()) != destructed {
				if !destructed {
					return fmt.Errorf("obj %s is destructed", address)
				}
				if original.destructed {
					return fmt.Errorf("obj %s is destructed", address)
				}
				// not original.destructed and obj.empty() -> delete s.stateObject + add s.stateObjectsDestruct
			}
			continue
		}

		if original.destructed {
			// If balance is zero, the account should have been completely removed
			if obj.Balance().IsZero() || original.isNew() {
				return fmt.Errorf("obj %s not deleted", address)
			}
			// If any account fields are non-zero, the account wasn't properly reset
			if obj.Nonce() != 0 || obj.CodeSize() != 0 || len(obj.originStorage) != 0 || len(obj.dirtyStorage) != 0 {
				return fmt.Errorf("obj %s not reset", address)
			}
			continue
		}

		if obj.newContract || obj.selfDestructed || len(obj.dirtyStorage) != 0 {
			return fmt.Errorf("obj %s not finalized: newContract %v, selfDestructed %v, dirtyStorage %d",
				address, obj.newContract, obj.selfDestructed, len(obj.dirtyStorage))
		}
		// Verify nonce matches original value
		if original.nonce != nil && obj.Nonce() != *original.nonce {
			return fmt.Errorf("obj %s nonce not reverted: want %d, got %d", address, *original.nonce, obj.Nonce())
		}
		// Verify balance matches original value
		if original.balance != nil && !obj.Balance().Eq(original.balance) {
			return fmt.Errorf("obj %s balance not reverted: want %s, got %s", address, original.balance, obj.Balance())
		}
		// Verify code hash matches original value
		if original.code != nil && !bytes.Equal(obj.Code(), original.code) {
			return fmt.Errorf("obj %s code not reverted: want %x, got %x", address, crypto.Keccak256Hash(original.code), obj.CodeHash())
		}
		for key, orig := range original.storage {
			if current := obj.GetState(key); current != orig {
				return fmt.Errorf("obj %s committed storage slot %s not reverted: want %s, got %s", address, key, orig, current)
			}
		}
	}

	return nil
}

// getStateDiff implements the state diff generation.
// It compares the current state against the original account states captured during
// transaction execution to produce a minimal but complete representation of all changes.
//
// The algorithm:
// 1. Retrieves original account states from tracked journals
// 2. Compares each account's current state with its original values
// 3. Records only the actual differences (balance deltas, nonce changes, code updates, storage changes)
// 4. Optimizes the output by omitting unchanged values and using deltas instead of absolute values
//
// The resulting diff can be used to analyze transaction effects, propagate state changes
// efficiently across nodes, or enhance transaction receipts with detailed change information.
func (t *objectsSnapshotTracker) getStateDiff() *types.StateDiff {
	// Get baseline account states from the snapshot
	originalAccounts, err := t.baselineAccounts()
	if err != nil || len(originalAccounts) == 0 {
		return nil
	}

	stateDiff := new(types.StateDiff)

	// Compare current vs. original state for each affected account
	for addr, account := range originalAccounts {
		obj := t.stateDB.stateObjects[addr]
		if obj == nil {
			obj = newObject(t.stateDB, addr, account.origin)
		}

		// Calculate balance changes - we track increases and decreases separately
		// because RLP doesn't support negative values, deltas are more compact than
		// full balances, and it allows efficient merging of multiple changes
		var balanceInc, balanceDec *uint256.Int
		if account.balance == nil {
			// No record of original balance - was never modified in captured transactions
		} else if cmp := obj.Balance().Cmp(account.balance); cmp > 0 {
			balanceInc = new(uint256.Int).Sub(obj.Balance(), account.balance)
		} else if cmp < 0 {
			balanceDec = new(uint256.Int).Sub(account.balance, obj.Balance())
		}

		var newNonce uint64
		if account.nonce != nil && obj.Nonce() != *account.nonce {
			newNonce = obj.Nonce()
		}

		var newCode []byte
		if (account.code != nil) && (len(account.code) != obj.CodeSize() || !bytes.Equal(account.code, obj.Code())) {
			newCode = obj.Code()
			if len(newCode) == 0 {
				newCode = types.AddressToDelegation(common.Address{})
			}
		}

		var newStorage []*types.KeyValue
		if len(account.storage) > 0 {
			newStorage = make([]*types.KeyValue, 0, len(account.storage))
			for key, prevValue := range account.storage {
				newValue := obj.GetState(key)
				if newValue == prevValue {
					continue
				}
				newStorage = append(newStorage, &types.KeyValue{
					Key:   key,
					Value: newValue,
				})
			}
		}

		// Skip accounts with no actual changes to keep the diff minimal
		if newNonce == 0 && balanceInc == nil && balanceDec == nil && len(newCode) == 0 && len(newStorage) == 0 {
			continue
		}

		stateDiff.Changes = append(stateDiff.Changes, &types.AccountChange{
			Address:    addr,
			BalanceInc: balanceInc,
			BalanceDec: balanceDec,
			Storage:    newStorage,
			NewNonce:   newNonce,
			NewCode:    newCode,
		})
	}

	if stateDiff.IsEmpty() {
		return nil
	}

	return stateDiff
}
