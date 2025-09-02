package ethapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
)

type BackendTool interface {
	IsToolEnabled() bool
	GetToolNonce(ctx context.Context, addr common.Address) (uint64, error)
	ToolBlockByHash(ctx context.Context, hash common.Hash) *types.Block
	GetToolReceipt(tx common.Hash) (*types.Receipt, *types.Transaction, *types.Header, error)
	SendToolBundle(ctx context.Context, txs types.Transactions) (common.Hash, error)
	DoToolCall(ctx context.Context, txArgs tool.TransactionArgs, block uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (*core.ExecutionResult, error)
}
