package miner

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/log"
)

type toolParams struct {
	txChunks   []types.Transactions
	settlement *tool.Settlement
	gasCeil    uint64
}

func (miner *Miner) commitToolTransactions(env *environment, params toolParams) error {
	if miner.tool == nil {
		return errors.New("tool not initialized")
	}

	env.gasPool = new(core.GasPool).AddGas(env.header.GasLimit)
	for subSlot, txs := range params.txChunks {
		for i, tx := range txs {
			env.state.SetTxContext(tx.Hash(), env.tcount)
			if err := miner.commitTransaction(env, tx); err != nil {
				return fmt.Errorf("failed to commit subSlot #%d transaction #%d, err: %w", subSlot, i, err)
			}
		}
	}

	if params.settlement.Gas != 0 && env.tcount != 0 {
		tx, err := params.settlement.MakeTx(env.signer, env.header.BaseFee, env.state)
		if err != nil {
			log.Warn("Skip settlement transaction", "reason", err.Error())
		} else {
			env.state.SetTxContext(tx.Hash(), env.tcount)
			if err := miner.commitTransaction(env, tx); err != nil {
				return fmt.Errorf("failed to commit settlement tx, err: %w", err)
			}
		}
	}

	var failedTxs int
	for i, receipt := range env.receipts {
		if receipt.Status == types.ReceiptStatusFailed {
			failedTxs++
			log.Warn("Failed receipt status", "index", i, "tx", receipt.TxHash.Hex(), "gas", receipt.GasUsed)
		}
	}
	if failedTxs == 0 {
		return nil
	}

	return fmt.Errorf("reverted %d/%d txs, refresh state err: %v", failedTxs, env.tcount, miner.tool.RefreshState())
}
