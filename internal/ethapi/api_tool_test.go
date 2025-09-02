package ethapi

import (
	"context"

	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

func (b testBackend) IsToolEnabled() bool {
	return false
}
func (b *testBackend) GetToolNonce(_ context.Context, _ common.Address) (uint64, error) {
	panic("implement me")
}
func (b *testBackend) ToolBlockByHash(_ context.Context, _ common.Hash) *types.Block {
	panic("implement me")
}
func (b testBackend) GetToolReceipt(_ common.Hash) (*types.Receipt, *types.Transaction, *types.Header, error) {
	panic("implement me")
}
func (b testBackend) DoToolCall(_ context.Context, _ tool.TransactionArgs, _ uint64, _ *override.StateOverride, _ *override.BlockOverrides) (*core.ExecutionResult, error) {
	panic("implement me")
}
func (b testBackend) SendToolBundle(_ context.Context, _ types.Transactions) (common.Hash, error) {
	panic("implement me")
}
