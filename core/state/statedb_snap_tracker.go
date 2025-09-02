package state

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// SnapshotTrackerVersion defines the available implementations for state tracking and restoration.
// Each implementation offers different trade-offs between performance, memory usage, and capabilities.
type SnapshotTrackerVersion uint8

const (
	NoSnapshotTracker      SnapshotTrackerVersion = iota // No state tracking (default)
	NoSnapDiffTracker                                    // Simple copy-based tracking without diff generation
	ObjectsSnapshotTracker                               // Objects-based tracking with efficient diff generation
)

// snapshotTracker defines the interface for state tracking implementations.
// It provides methods for restoring state, generating diffs, and capturing changes.
// Different implementations can optimize for specific use cases like transaction
// simulation, state reversion, or efficient state propagation.
type snapshotTracker interface {
	copy(*StateDB) snapshotTracker  // Creates a deep copy of the tracker with a new state reference
	restore() (*StateDB, error)     // Reverts state to its original condition
	getStateDiff() *types.StateDiff // Generates a structured representation of state changes
	captureChanges(state *StateDB)  // Records state at transaction boundaries
}

// copierSnapStateTracker implements a simple state tracking mechanism
// that creates a full copy of the state. This approach provides reliable
// state restoration but is memory-intensive and doesn't support diff generation.
// It's mainly used for simple transaction simulations where only the final
// state is needed without intermediate analysis.
type copierSnapStateTracker struct {
	prev *StateDB // Complete copy of the original state
}

// newCopierSnapStateTracker creates a new copy-based tracker using a deep copy
// of the provided state as the baseline.
func newCopierSnapStateTracker(state *StateDB) *copierSnapStateTracker {
	return &copierSnapStateTracker{prev: state.Copy()}
}

// captureChanges is a no-op for copy-based trackers since they only maintain
// the initial state copy without tracking incremental changes.
func (r *copierSnapStateTracker) captureChanges(state *StateDB) {}

// copy creates a new tracker instance with a fresh state copy.
func (r *copierSnapStateTracker) copy(state *StateDB) snapshotTracker {
	return &copierSnapStateTracker{prev: state.Copy()}
}

// restore returns the original state copy.
// This method provides a simple but effective way to revert all changes.
func (r *copierSnapStateTracker) restore() (*StateDB, error) { return r.prev, nil }

// getStateDiff returns nil as this tracker doesn't support diff generation.
func (r *copierSnapStateTracker) getStateDiff() *types.StateDiff { return nil }

// InitRestorer initializes the StateDB with a state tracker according to the specified version.
// This enables capabilities like state restoration and diff generation based on the chosen tracker.
// The txs parameter is used to pre-allocate storage for optimizing transaction tracking.
func (s *StateDB) InitRestorer(restorer SnapshotTrackerVersion) *StateDB {
	switch restorer {
	case NoSnapshotTracker:
		s.snapshotTracker = nil
	case NoSnapDiffTracker:
		s.snapshotTracker = newCopierSnapStateTracker(s)
	case ObjectsSnapshotTracker:
		s.snapshotTracker = newObjectsSnapshotTracker(s)
	default:
		panic("not implemented")
	}
	return s
}

// CopyWithStateRestorer creates a deep copy of the state including its snapshot tracker.
// This allows for forking state execution paths while preserving the ability to track
// and analyze changes in each fork independently.
func (s *StateDB) CopyWithStateRestorer() *StateDB {
	stateDB := s.Copy()
	if s.snapshotTracker != nil {
		stateDB.snapshotTracker = s.snapshotTracker.copy(stateDB)
	}
	return stateDB
}

// Restore reverts all state changes from tracked transactions to return
// to the original state. This is useful for transaction simulation, reverting
// failed transactions, or creating "what-if" scenarios without permanently
// modifying state.
func (s *StateDB) Restore() (*StateDB, error) {
	if s.snapshotTracker == nil {
		return nil, errors.New("snapshotTracker not initialized")
	}

	stateDB, err := s.snapshotTracker.restore()
	if err != nil {
		return nil, err
	}

	stateDB.snapshotTracker = nil
	return stateDB, nil
}

// GetStateDiff generates a structured representation of all state changes
// by comparing the current state against the original state tracked in snapshotTracker.
//
// The diff calculation process:
// 1. For each modified account, compares current vs. original values
// 2. Precisely tracks balance increases/decreases, nonce changes, code updates and storage modifications
// 3. Returns only actual differences, minimizing the diff size
//
// This diff representation can be used for:
// - Transaction simulation and impact analysis
// - "What-if" scenario testing without committing changes
// - Efficient state propagation between nodes
// - Transaction receipt enhancement with detailed state changes
func (s *StateDB) GetStateDiff() *types.StateDiff {
	if s.snapshotTracker == nil {
		return nil
	}
	return s.snapshotTracker.getStateDiff()
}

// AddStateDiff applies a previously extracted state diff to the current state.
// This is the inverse operation of GetStateDiff, allowing state changes to be
// applied without re-executing the transactions that originally created them.
//
// The method performs several validation steps:
// 1. Checks that code isn't being overwritten for existing contracts
// 2. Validates that balance changes won't result in negative balances
// 3. Ensures balance calculations won't cause numeric overflow
//
// After applying all changes, it finalizes the state to handle the journal entries.
func (s *StateDB) AddStateDiff(stateDiff *types.StateDiff) error {
	if stateDiff.IsEmpty() {
		return nil
	}

	for _, data := range stateDiff.Changes {
		obj := s.getOrNewStateObject(data.Address)

		if newCode := slices.Clone(data.NewCode); len(newCode) != 0 {
			addr, delegated := types.ParseDelegation(newCode)
			if !delegated {
				if obj.CodeSize() != 0 {
					return fmt.Errorf("code exists %s", data.Address)
				}
				s.CreateContract(data.Address)
			} else if addr == (common.Address{}) {
				newCode = nil
			}
			obj.SetCode(crypto.Keccak256Hash(newCode), newCode)
		}

		if data.NewNonce != 0 {
			obj.SetNonce(data.NewNonce)
		}

		if balanceDiff := data.BalanceDiff(); balanceDiff.Sign() != 0 {
			if balance := obj.Balance(); balance != nil {
				balanceDiff.Add(balance.ToBig(), balanceDiff)
			}

			if balanceDiff.Sign() == -1 {
				return fmt.Errorf("negative balance %s", data.Address)
			}

			balance, overflow := uint256.FromBig(balanceDiff)
			if overflow {
				return fmt.Errorf("overflow balance %s", data.Address)
			}

			obj.SetBalance(balance)
		}

		for _, kv := range data.Storage {
			obj.SetState(kv.Key, kv.Value)
		}
	}

	s.Finalise(true) // Finalize the state to handle the journal

	return nil
}

// captureNotFinalizedChanges records the current state at transaction boundaries
// for later analysis or reversion. This is called automatically during transaction
// execution to maintain a complete history of state changes.
func (s *StateDB) captureNotFinalizedChanges() {
	if s.snapshotTracker != nil {
		s.snapshotTracker.captureChanges(s)
	}
}

func dumpJournal(changes []journalEntry) string {
	var bb bytes.Buffer
	for i, entry := range changes {
		bb.WriteString(fmt.Sprintf("\t%d %#v:\n", i, entry))
	}
	return bb.String()
}
