package types

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

func TestStateDiff_MarshalJSON(t *testing.T) {
	stateDiff := &StateDiff{Changes: []*AccountChange{
		{
			Address:    common.Address{12},
			BalanceInc: uint256.NewInt(123456789),
			BalanceDec: nil,
			Storage: StorageValues{
				{
					Key:   common.Hash{23},
					Value: common.Hash{34},
				},
				{
					Key:   common.Hash{45},
					Value: common.Hash{56},
				},
			},
			NewNonce: 1,
			NewCode:  []byte{6, 7, 8, 9},
		},
		{
			Address:    common.Address{255},
			BalanceInc: nil,
			BalanceDec: uint256.NewInt(987654321),
			Storage:    nil,
			NewNonce:   123,
			NewCode:    AddressToDelegation(common.Address{127}),
		},
	}}

	result, err := json.MarshalIndent(stateDiff, "", "\t")
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{
	"changes": {
		"0x0c00000000000000000000000000000000000000": {
			"storage": {
				"0x1700000000000000000000000000000000000000000000000000000000000000": "0x2200000000000000000000000000000000000000000000000000000000000000",
				"0x2d00000000000000000000000000000000000000000000000000000000000000": "0x3800000000000000000000000000000000000000000000000000000000000000"
			},
			"nonce": 1,
			"code": "0x06070809",
			"balance": "123456789"
		},
		"0xff00000000000000000000000000000000000000": {
			"nonce": 123,
			"code": "0xef01007f00000000000000000000000000000000000000",
			"balance": "-987654321"
		}
	}
}`

	if string(result) != expected {
		t.Errorf("want %s, got %s", expected, result)
	}
}

// --- Helper functions for test data ---

func cloneAccountChange(au *AccountChange) *AccountChange {
	if au == nil {
		return nil
	}
	clone := &AccountChange{
		Address:  au.Address,
		NewNonce: au.NewNonce,
		NewCode:  slices.Clone(au.NewCode),
		Storage:  slices.Clone(au.Storage),
	}
	if au.BalanceInc != nil {
		clone.BalanceInc = au.BalanceInc.Clone()
	}
	if au.BalanceDec != nil {
		clone.BalanceDec = au.BalanceDec.Clone()
	}
	return clone
}

func cloneStateDiff(ds *StateDiff) *StateDiff {
	if ds == nil {
		return nil
	}
	clone := &StateDiff{
		Changes: make([]*AccountChange, len(ds.Changes)),
	}
	for i, upd := range ds.Changes {
		clone.Changes[i] = cloneAccountChange(upd)
	}
	return clone
}

// TestMergeStateDiffs provides comprehensive testing for MergeStateDiffs.
func TestMergeStateDiffs(t *testing.T) {
	addr1 := common.BytesToAddress([]byte{1})
	addr2 := common.BytesToAddress([]byte{2})
	//addr3 := common.BytesToAddress([]byte{3})

	key1 := common.BytesToHash([]byte{1})
	key2 := common.BytesToHash([]byte{2})
	key3 := common.BytesToHash([]byte{3})

	val1 := common.BytesToHash([]byte{101})
	val2 := common.BytesToHash([]byte{102})
	val3 := common.BytesToHash([]byte{103})
	//val4 := common.BytesToHash([]byte{104})

	code1 := []byte{0x60, 0x01}
	code2 := []byte{0x60, 0x02}

	u256Max := new(uint256.Int).SetAllOne() // (2^256 - 1)

	testCases := []struct {
		name               string
		diffs              []*StateDiff
		want               *StateDiff
		wantErr            error
		wantErrMsgContains string // For more specific error checking, e.g., address in MergeError
	}{
		// --- Empty and Single Diff Cases ---
		{
			name:  "no diffs",
			diffs: []*StateDiff{},
			want:  nil, // As per implementation: if len(diffs) == 0 { return nil, nil }
		},
		{
			name:  "nil diffs slice",
			diffs: nil, // Equivalent to no diffs
			want:  nil,
		},
		{
			name:  "one empty diff",
			diffs: []*StateDiff{{}},
			want:  nil, // Returns an empty, non-nil StateDiff
		},
		{
			name:  "one nil diff in slice",
			diffs: []*StateDiff{nil},
			want:  nil, // Handled by loops, results in empty
		},
		{
			name:  "multiple empty/nil diffs",
			diffs: []*StateDiff{{}, nil, {}},
			want:  nil,
		},
		{
			name: "one non-empty diff",
			diffs: []*StateDiff{
				{
					Changes: []*AccountChange{
						{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10), NewNonce: 1},
					},
				},
			},
			want: &StateDiff{
				Changes: []*AccountChange{
					{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10), NewNonce: 1},
				},
			},
		},

		// --- Successful Merges (No Conflicts) ---
		{
			name: "merge balance updates for same account",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(5)}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceDec: new(uint256.Int).SetUint64(3)}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(12)}},
			},
		},
		{
			name: "merge balance updates to zero",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceDec: new(uint256.Int).SetUint64(10)}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, BalanceInc: nil, BalanceDec: nil}}, // Results in an update with no balance change
			},
		},
		{
			name: "merge storage updates for same account, different keys",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}}}}},
				{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key2, Value: val2}}}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{
					{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}, {Key: key2, Value: val2}}},
				},
			},
		},
		{
			name: "merge nonce/code from one, balance/storage from another (same account)",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 1, NewCode: code1}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(100), Storage: []*KeyValue{{Key: key1, Value: val1}}}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{
					{Address: addr1, NewNonce: 1, NewCode: code1, BalanceInc: new(uint256.Int).SetUint64(100), Storage: []*KeyValue{{Key: key1, Value: val1}}},
				},
			},
		},
		{
			name: "merge updates for different accounts",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{Changes: []*AccountChange{{Address: addr2, NewNonce: 1}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{
					{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)},
					{Address: addr2, NewNonce: 1},
				},
			},
		},
		{
			name: "order independence (d1, d2)",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 1}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, NewNonce: 1, BalanceInc: new(uint256.Int).SetUint64(10)}},
			},
		},
		{
			name: "order independence (d2, d1)",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 1}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, NewNonce: 1, BalanceInc: new(uint256.Int).SetUint64(10)}},
			},
		},
		{
			name: "merge with empty diff in between",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{}, // Empty diff
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(5)}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(15)}},
			},
		},
		{
			name: "balance max large decrement",
			diffs: []*StateDiff{
				// Start with a positive balance, then decrement massively
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceDec: u256Max}}}, // Net effect: 10 - u256Max (very negative)
			},
			// This results in a negative balance, which becomes a BalanceDec. If BalanceDec itself overflows, it's an issue.
			// 10 - Max => Negative. Abs(Negative) => Max - 10. This should be fine.
			want: &StateDiff{
				Changes: []*AccountChange{{Address: addr1, BalanceDec: uint256.MustFromDecimal("115792089237316195423570985008687907853269984665640564039457584007913129639925")}},
			},
		},

		// --- Conflict Scenarios (Expecting Errors) ---
		{
			name: "conflict: complex merge",
			diffs: []*StateDiff{
				{ // Diff 1
					Changes: []*AccountChange{
						{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10), Storage: []*KeyValue{{Key: key1, Value: val1}}},
						{Address: addr2, NewNonce: 1},
					},
				},
				{ // Diff 2
					Changes: []*AccountChange{
						{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(5), Storage: []*KeyValue{{Key: key2, Value: val2}}}, // More for addr1
						{Address: addr2, NewCode: code1}, // More for addr2
					},
				},
			},
			wantErr:            errAccountStateUpdate,
			wantErrMsgContains: addr2.String(),
		},
		{
			name: "conflict: nonce updated in two diffs",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 1}}},
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 2}}},
			},
			wantErr:            errAccountStateUpdate,
			wantErrMsgContains: addr1.String(),
		},
		{
			name: "conflict: code updated in two diffs",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, NewCode: code1}}},
				{Changes: []*AccountChange{{Address: addr1, NewCode: code2}}},
			},
			wantErr:            errAccountStateUpdate,
			wantErrMsgContains: addr1.String(),
		},
		{
			name: "conflict: nonce in one, code in another for same account",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, NewNonce: 1}}},
				{Changes: []*AccountChange{{Address: addr1, NewCode: code1}}},
			},
			wantErr:            errAccountStateUpdate,
			wantErrMsgContains: addr1.String(),
		},
		//{
		//	name: "conflict: storage slot updated in two diffs",
		//	diffs: []*StateDiff{
		//		{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}}}}},
		//		{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val2}}}}}, // Same key
		//	},
		//	wantErr:            errStorageSlotUpdate,
		//	wantErrMsgContains: fmt.Sprintf("addr %s slot %s", addr1.String(), key1.String()),
		//},
		// errAccountBalanceOverflow Cases
		{
			name: "conflict: balance increment overflow",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: u256Max}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(1)}}},
			},
			wantErr:            errAccountBalanceOverflow,
			wantErrMsgContains: addr1.String(),
		},
		{
			name: "conflict: balance decrement overflow (Dec + Dec)",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, BalanceDec: u256Max}}},
				{Changes: []*AccountChange{{Address: addr1, BalanceDec: new(uint256.Int).SetUint64(1)}}}, // Effectively -(u256Max + 1)
			},
			// BalanceDiff = -u256Max - 1. Abs = u256Max + 1. This should overflow FromBig for BalanceDec.
			wantErr:            errAccountBalanceOverflow,
			wantErrMsgContains: addr1.String(),
		},
		//{
		//	name: "complex conflict scenario (storage conflict deep in diffs)",
		//	diffs: []*StateDiff{
		//		{Changes: []*AccountChange{{Address: addr1, BalanceInc: new(uint256.Int).SetUint64(10)}}},
		//		{Changes: []*AccountChange{{Address: addr2, NewNonce: 1}}},
		//		{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}}}}}, // addr1, key1 in diff 2
		//		{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val2}}}}}, // addr1, key1 again in diff 4
		//	},
		//	wantErr:            errStorageSlotUpdate,
		//	wantErrMsgContains: fmt.Sprintf("addr %s slot %s", addr1.String(), key1.String()),
		//},
		{
			name: "merge storage for same account from multiple diffs (different keys)",
			diffs: []*StateDiff{
				{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}}}}},
				{Changes: []*AccountChange{{Address: addr2, BalanceInc: new(uint256.Int).SetUint64(5)}}}, // Intermediary diff
				{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key2, Value: val2}}}}},
				{Changes: []*AccountChange{{Address: addr1, Storage: []*KeyValue{{Key: key3, Value: val3}}}}},
			},
			want: &StateDiff{
				Changes: []*AccountChange{
					{Address: addr1, Storage: []*KeyValue{{Key: key1, Value: val1}, {Key: key2, Value: val2}, {Key: key3, Value: val3}}},
					{Address: addr2, BalanceInc: new(uint256.Int).SetUint64(5)},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clone input diffs to prevent modification by MergeStateDiffs if it ever does that (good practice)
			inputDiffs := make([]*StateDiff, len(tc.diffs))
			for i, d := range tc.diffs {
				inputDiffs[i] = cloneStateDiff(d)
			}

			got, err := MergeStateDiffs(inputDiffs...)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("MergeStateDiffs() error = nil, wantErr %v", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					// Try to unwrap MergeError
					var mergeErr *MergeError
					if errors.As(err, &mergeErr) {
						if !errors.Is(mergeErr.Unwrap(), tc.wantErr) {
							t.Fatalf("MergeStateDiffs() error = %v, wantErr %v (checked via unwrapped MergeError)", err, tc.wantErr)
						}
					} else {
						t.Fatalf("MergeStateDiffs() error = %v, wantErr %v", err, tc.wantErr)
					}
				}
				if tc.wantErrMsgContains != "" && !strings.Contains(err.Error(), tc.wantErrMsgContains) {
					t.Errorf("MergeStateDiffs() error message = %q, want %q", err.Error(), tc.wantErrMsgContains)
				}
				if got != nil {
					t.Errorf("MergeStateDiffs() got = %v, want nil when error is expected", got)
				}
			} else { // No error expected
				if err != nil {
					t.Fatalf("MergeStateDiffs() unexpected error = %v", err)
				}

				if tc.want == nil { // Expecting a truly nil result (e.g., no input diffs)
					if got != nil {
						t.Errorf("MergeStateDiffs() got = %s, want nil", got.String())
					}
				} else if got == nil { // Expected non-nil, got nil
					t.Errorf("MergeStateDiffs() got = nil, want %s", tc.want.String())
				} else {
					// Compare string representations (after sorting within String())
					wantStr := tc.want.String()
					gotStr := got.String()
					if wantStr != gotStr {
						t.Errorf("MergeStateDiffs() mismatch:\nGOT:\n%s\nWANT:\n%s", gotStr, wantStr)
					}
					// Additionally, check IsEmpty for cases where an empty StateDiff is expected
					if tc.want.IsEmpty() && !got.IsEmpty() {
						t.Errorf("MergeStateDiffs() got non-empty, want empty. GOT: %s", gotStr)
					}
					if !tc.want.IsEmpty() && got.IsEmpty() {
						t.Errorf("MergeStateDiffs() got empty, want non-empty. WANT: %s", wantStr)
					}
				}
			}
		})
	}
}
