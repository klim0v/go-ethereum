package eth

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
)

func (b *EthAPIBackend) IsToolEnabled() bool {
	return b.eth.tool.Enabled()
}

func (b *EthAPIBackend) GetToolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.tool.GetNonce(addr)
}

func (b *EthAPIBackend) ToolBlockByHash(ctx context.Context, hash common.Hash) *types.Block {
	return b.eth.tool.GetBlock(hash)
}

func (b *EthAPIBackend) GetToolReceipt(tx common.Hash) (*types.Receipt, *types.Transaction, *types.Header, error) {
	return b.eth.tool.GetReceipt(tx)
}

func (b *EthAPIBackend) SendToolBundle(ctx context.Context, txs types.Transactions) (common.Hash, error) {
	commitment, err := b.eth.tool.ProcessBundle(&types.Bundle{NewTxs: txs}, true)
	if err != nil {
		return common.Hash{}, err
	}

	return commitment.ID(), nil
}

func (b *EthAPIBackend) DoToolCall(ctx context.Context, txArgs tool.TransactionArgs, block uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (*core.ExecutionResult, error) {
	var cancel context.CancelFunc
	if timeout := b.RPCEVMTimeout(); timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	return b.eth.tool.DoCall(ctx, txArgs, b.RPCGasCap(), block, stateOverrides, blockOverrides)
}
