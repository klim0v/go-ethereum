package types

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestAccesses_MarshalJSON(t *testing.T) {
	accesses := &Accesses{
		{
			Address: common.Address{255},
			Meta:    (AccessOpWrite << AccessShiftNonce) | (AccessOpRead << AccessShiftCode) | (AccessOpRW << AccessShiftBalance),
			Storage: []SlotOp{
				{
					Slot: common.Hash{11},
					Ops:  AccessOpRead,
				},
				{
					Slot: common.Hash{22},
					Ops:  AccessOpWrite,
				},
				{
					Slot: common.Hash{33},
					Ops:  AccessOpRW,
				},
			},
		},
	}

	result, err := json.MarshalIndent(accesses, "", "\t")
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{
	"0xff00000000000000000000000000000000000000": {
		"storage": {
			"0x0b00000000000000000000000000000000000000000000000000000000000000": 1,
			"0x1600000000000000000000000000000000000000000000000000000000000000": 2,
			"0x2100000000000000000000000000000000000000000000000000000000000000": 3
		},
		"balance": 3,
		"nonce": 2,
		"code": 1
	}
}`

	if string(result) != expected {
		t.Errorf("want %s, got %s", expected, result)
	}
}

func TestMergeAccesses(t *testing.T) {
	// Common test data
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")

	slot1 := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	slot2 := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")

	t.Run("EmptyAndNilLists", func(t *testing.T) {
		// Test nil and empty lists
		var nilAccesses Accesses = nil
		var emptyAccesses Accesses = Accesses{}

		merged, noConflict := MergeAccesses(nilAccesses, emptyAccesses)
		assert.True(t, noConflict, "Nil and empty lists should not conflict")
		assert.Empty(t, merged, "Result should be empty")

		merged, noConflict = MergeAccesses(emptyAccesses, nilAccesses)
		assert.True(t, noConflict, "Empty and nil lists should not conflict")
		assert.Empty(t, merged, "Result should be empty")

		merged, noConflict = MergeAccesses(nilAccesses, nilAccesses)
		assert.True(t, noConflict, "Nil lists should not conflict")
		assert.Empty(t, merged, "Result should be empty")
	})

	t.Run("DifferentAddresses", func(t *testing.T) {
		// Test lists with different addresses
		list1 := Accesses{
			{Address: addr1, Meta: (AccessOpRead << AccessShiftBalance) | (AccessOpRead << AccessShiftCode)},
		}
		list2 := Accesses{
			{Address: addr2, Meta: (AccessOpWrite << AccessShiftBalance) | (AccessOpWrite << AccessShiftCode)},
		}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Lists with different addresses should not conflict")
		assert.Len(t, merged, 2, "Merged result should contain both addresses")

		// Verify both addresses are in the result with correct operations
		foundAddr1, foundAddr2 := false, false
		for _, access := range merged {
			if access.Address == addr1 {
				foundAddr1 = true
				assert.Equal(t, AccessOpRead, getMetaField(access.Meta, AccessShiftBalance))
				assert.Equal(t, AccessOpRead, getMetaField(access.Meta, AccessShiftCode))
			}
			if access.Address == addr2 {
				foundAddr2 = true
				assert.Equal(t, AccessOpWrite, getMetaField(access.Meta, AccessShiftBalance))
				assert.Equal(t, AccessOpWrite, getMetaField(access.Meta, AccessShiftCode))
			}
		}
		assert.True(t, foundAddr1, "addr1 should be in result")
		assert.True(t, foundAddr2, "addr2 should be in result")
	})

	t.Run("SameAddressNoConflict", func(t *testing.T) {
		// Test read-read operations (no conflict)
		list1 := Accesses{
			{Address: addr1, Meta: (AccessOpRead << AccessShiftBalance) | (AccessOpRead << AccessShiftCode)},
		}
		list2 := Accesses{
			{Address: addr1, Meta: (AccessOpRead << AccessShiftBalance) | (AccessOpRead << AccessShiftCode)},
		}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Same read operations should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, addr1, merged[0].Address)
			assert.Equal(t, AccessOpRead, getMetaField(merged[0].Meta, AccessShiftBalance))
			assert.Equal(t, AccessOpRead, getMetaField(merged[0].Meta, AccessShiftCode))
		}

		// Test read-write operations (no conflict)
		list1 = Accesses{
			{Address: addr1, Meta: AccessOpRead << AccessShiftBalance},
		}
		list2 = Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftBalance},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Read-write on same balance should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, addr1, merged[0].Address)
			assert.Equal(t, AccessOpRW, getMetaField(merged[0].Meta, AccessShiftBalance))
		}

		// Test with all fields read
		allRead := (AccessOpRead << AccessShiftBalance) | (AccessOpRead << AccessShiftCode) | (AccessOpRead << AccessShiftNonce)
		list1 = Accesses{
			{Address: addr1, Meta: allRead},
		}
		list2 = Accesses{
			{Address: addr1, Meta: allRead},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Same read operations should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, allRead, merged[0].Meta, "All Meta bits should be preserved")
		}

		// Test write-write operations (no conflict)
		list1 = Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftCode},
		}
		list2 = Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftCode},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Write-write on same account should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, addr1, merged[0].Address)
			assert.Equal(t, AccessOpWrite, getMetaField(merged[0].Meta, AccessShiftCode))
		}
	})

	t.Run("ConflictCases", func(t *testing.T) {
		// Test write-read conflict (balance)
		list1 := Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftBalance},
		}
		list2 := Accesses{
			{Address: addr1, Meta: AccessOpRead << AccessShiftBalance},
		}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Write-read on same balance should conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		// Test write-read conflict (nonce)
		list1 = Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftNonce},
		}
		list2 = Accesses{
			{Address: addr1, Meta: AccessOpRead << AccessShiftNonce},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Write-read on same nonce should conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		// Test write-read conflict (code)
		list1 = Accesses{
			{Address: addr1, Meta: AccessOpWrite << AccessShiftCode},
		}
		list2 = Accesses{
			{Address: addr1, Meta: AccessOpRead << AccessShiftCode},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Write-read on same code should conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		// Test multiple field conflict
		list1 = Accesses{
			{Address: addr1, Meta: (AccessOpWrite << AccessShiftBalance) |
				(AccessOpRead << AccessShiftCode) |
				(AccessOpWrite << AccessShiftNonce)},
		}
		list2 = Accesses{
			{Address: addr1, Meta: (AccessOpRead << AccessShiftBalance) |
				(AccessOpWrite << AccessShiftCode) |
				(AccessOpRead << AccessShiftNonce)},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Multiple field conflicts should be detected")
		assert.Nil(t, merged, "Result should be nil when conflict detected")
	})

	t.Run("StorageOperations", func(t *testing.T) {
		// Test no storage conflict (read-write)
		list1 := Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRead}}},
		}
		list2 := Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}}},
		}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Read-write on same storage slot should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, addr1, merged[0].Address)
			assert.Len(t, merged[0].Storage, 1, "Should have one storage slot")
			if len(merged[0].Storage) > 0 {
				assert.Equal(t, slot1, merged[0].Storage[0].Slot)
				assert.Equal(t, AccessOpRW, merged[0].Storage[0].Ops)
			}
		}

		// Test storage conflict (write-read)
		list1 = Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}}},
		}
		list2 = Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRead}}},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Write-read on same storage slot should conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		// Test different storage slots (no conflict)
		list1 = Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}}},
		}
		list2 = Accesses{
			{Address: addr1, Storage: []SlotOp{{Slot: slot2, Ops: AccessOpWrite}}},
		}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Different storage slots should not conflict")
		assert.Len(t, merged, 1, "Merged result should contain one address")
		if len(merged) > 0 {
			assert.Equal(t, addr1, merged[0].Address)
			assert.Len(t, merged[0].Storage, 2, "Merged result should contain both slots")

			// Verify both slots are in the result
			foundSlot1, foundSlot2 := false, false
			for _, slot := range merged[0].Storage {
				if slot.Slot == slot1 {
					foundSlot1 = true
					assert.Equal(t, AccessOpWrite, slot.Ops)
				}
				if slot.Slot == slot2 {
					foundSlot2 = true
					assert.Equal(t, AccessOpWrite, slot.Ops)
				}
			}
			assert.True(t, foundSlot1, "slot1 should be in merged result")
			assert.True(t, foundSlot2, "slot2 should be in merged result")
		}
	})

	t.Run("RWOperationInteractions", func(t *testing.T) {
		// Test RW-Read interactions (Meta)
		list1 := Accesses{{
			Address: addr1,
			Meta:    AccessOpRW << AccessShiftBalance,
		}}
		list2 := Accesses{{
			Address: addr1,
			Meta:    AccessOpRead << AccessShiftBalance,
		}}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.False(t, noConflict, "RW + R should conflict (write-then-read)")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		// Test Read-RW interactions (Meta)
		list1 = Accesses{{
			Address: addr1,
			Meta:    AccessOpRead << AccessShiftBalance,
		}}
		list2 = Accesses{{
			Address: addr1,
			Meta:    AccessOpRW << AccessShiftBalance,
		}}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "R + RW should not conflict")
		assert.NotNil(t, merged, "Result should contain merged operations")
		if len(merged) > 0 {
			assert.Equal(t, AccessOpRW, getMetaField(merged[0].Meta, AccessShiftBalance))
		}

		// Test RW-Write interactions (Storage)
		list1 = Accesses{{
			Address: addr1,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRW}},
		}}
		list2 = Accesses{{
			Address: addr1,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}},
		}}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "Storage: RW + W should not conflict")
		assert.NotNil(t, merged, "Result should contain merged operations")
		if len(merged) > 0 && len(merged[0].Storage) > 0 {
			assert.Equal(t, AccessOpRW, merged[0].Storage[0].Ops)
		}

		// Test Write-RW interactions (Storage)
		list1 = Accesses{{
			Address: addr1,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}},
		}}
		list2 = Accesses{{
			Address: addr1,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRW}},
		}}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Storage: W + RW should conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")
	})

	t.Run("ComplexMerging", func(t *testing.T) {
		// Test complex merging scenario with multiple addresses and storage slots
		list1 := Accesses{
			{
				Address: addr1,
				Meta:    (AccessOpRead << AccessShiftBalance) | (AccessOpRead << AccessShiftCode),
				Storage: []SlotOp{
					{Slot: slot1, Ops: AccessOpRead},
				},
			},
			{
				Address: addr2,
				Meta:    AccessOpWrite << AccessShiftBalance,
			},
		}
		list2 := Accesses{
			{
				Address: addr1,
				Storage: []SlotOp{
					{Slot: slot2, Ops: AccessOpWrite},
				},
			},
			{
				Address: addr3,
				Meta:    AccessOpWrite << AccessShiftCode,
			},
		}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.True(t, noConflict, "No overlapping operations should conflict")
		assert.Len(t, merged, 3, "Merged result should contain three addresses")

		// Verify addr1 has correct storage and meta
		for _, access := range merged {
			if access.Address == addr1 {
				assert.Len(t, access.Storage, 2, "Should have both storage slots")
				assert.Equal(t, AccessOpRead, getMetaField(access.Meta, AccessShiftBalance))
				assert.Equal(t, AccessOpRead, getMetaField(access.Meta, AccessShiftCode))

				foundSlot1, foundSlot2 := false, false
				for _, slot := range access.Storage {
					if slot.Slot == slot1 {
						foundSlot1 = true
						assert.Equal(t, AccessOpRead, slot.Ops)
					}
					if slot.Slot == slot2 {
						foundSlot2 = true
						assert.Equal(t, AccessOpWrite, slot.Ops)
					}
				}
				assert.True(t, foundSlot1, "slot1 should be in merged result")
				assert.True(t, foundSlot2, "slot2 should be in merged result")
			}
		}
	})

	t.Run("CombinedMetaAndStorageConflicts", func(t *testing.T) {
		list1 := Accesses{{
			Address: addr1,
			Meta:    AccessOpWrite << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRead}},
		}}
		list2 := Accesses{{
			Address: addr1,
			Meta:    AccessOpRead << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}},
		}}

		merged, noConflict := MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Conflict in Meta should cause overall conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		list1 = Accesses{{
			Address: addr1,
			Meta:    AccessOpRead << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}},
		}}
		list2 = Accesses{{
			Address: addr1,
			Meta:    AccessOpWrite << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRead}},
		}}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.False(t, noConflict, "Conflict in Storage should cause overall conflict")
		assert.Nil(t, merged, "Result should be nil when conflict detected")

		list1 = Accesses{{
			Address: addr1,
			Meta:    AccessOpRead << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpRead}},
		}}
		list2 = Accesses{{
			Address: addr1,
			Meta:    AccessOpWrite << AccessShiftBalance,
			Storage: []SlotOp{{Slot: slot1, Ops: AccessOpWrite}},
		}}

		merged, noConflict = MergeAccesses(list1, list2)
		assert.True(t, noConflict, "No conflicts should be detected")
		assert.NotNil(t, merged, "Result should contain merged operations")
	})
}
