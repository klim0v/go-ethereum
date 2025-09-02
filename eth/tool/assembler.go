package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// assembler constructs optimized SubSlots from pending Commitments. It intelligently selects and
// combines compatible Commitments (those without conflicting state accesses and changes) into a new SubSlot.
type assembler struct{}

// newSubSlot creates a new SubSlot by selecting compatible Commitments from the provided list.
func (g *assembler) newSubSlot(sizeLeft, gasLimit, totalGasLeft uint64, pendingCommitments []*types.Commitment, parent common.Hash, subSlot uint8, blockTime uint64) (*types.SubSlot, error) {
	result := types.NewSubSlot(parent, subSlot, blockTime)

	slices.SortFunc(pendingCommitments, func(i, j *types.Commitment) int {
		return j.Fees.Cmp(i.Fees)
	})

	var stateDiffs []*types.StateDiff
	var acceptedCommitments []*types.Commitment
	for _, commitment := range pendingCommitments {
		if commitment.TxsSize > sizeLeft {
			continue
		}
		if result.GasUsed+commitment.GasUsed > gasLimit {
			continue
		}
		if commitment.MaxGasLimit > totalGasLeft {
			continue
		}
		if commitment.BlockTime != blockTime {
			continue
		}

		mergedAccesses, ok := types.MergeAccesses(result.Accesses, commitment.Accesses)
		if !ok {
			continue
		}

		sizeLeft -= commitment.TxsSize
		totalGasLeft -= commitment.GasUsed
		result.GasUsed += commitment.GasUsed
		result.Accesses = mergedAccesses
		result.Txs = slices.Concat(result.Txs, commitment.Txs)
		stateDiffs = append(stateDiffs, commitment.StateDiff)
		acceptedCommitments = append(acceptedCommitments, commitment)
	}

	stateDiff, err := types.MergeStateDiffs(stateDiffs...)
	if err != nil {
		if mErr := (&types.MergeError{}); errors.As(err, &mErr) {
			first, second := mErr.Conflicts()
			acceptedCommitmentsFirst, _ := json.Marshal(acceptedCommitments[first])
			acceptedCommitmentsSecond, _ := json.Marshal(acceptedCommitments[second])
			fmt.Printf("first (%d): %v\n%s\n",
				acceptedCommitments[first].ExecutedTxsCount(), acceptedCommitments[first].Txs, acceptedCommitmentsFirst)
			fmt.Printf("second(%d): %v\n%s\n",
				acceptedCommitments[second].ExecutedTxsCount(), acceptedCommitments[second].Txs, acceptedCommitmentsSecond)
			return nil, fmt.Errorf("%s: conflict %s vs %s", mErr.Error(), acceptedCommitments[first].ID(), acceptedCommitments[second].ID())
		}
		return nil, err
	}

	result.StateDiff = stateDiff

	return result, nil
}
