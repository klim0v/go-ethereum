package ethapi

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
)

type ToolTransactionAPI struct {
	*TransactionAPI
}

func (api *ToolTransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if !api.b.IsToolEnabled() {
		return api.TransactionAPI.GetTransactionCount(ctx, address, blockNrOrHash)
	}

	blockNr, ok := blockNrOrHash.Number()
	if ok {
		var (
			nonce uint64
			err   error
		)
		if nextBlock := api.b.CurrentBlock().Number.Int64() + 1; blockNr.Int64() == nextBlock {
			nonce, err = api.b.GetToolNonce(ctx, address)
		} else if blockNr == rpc.PendingBlockNumber {
			nonce, err = api.b.GetToolNonce(ctx, address)
		} else {
			return api.TransactionAPI.GetTransactionCount(ctx, address, blockNrOrHash)
		}
		if err != nil {
			return nil, err
		}
		return (*hexutil.Uint64)(&nonce), nil
	}

	hash, isHash := blockNrOrHash.Hash()
	if !isHash {
		return nil, errors.New("invalid arguments; neither block nor hash specified")
	}

	if api.b.ToolBlockByHash(ctx, hash) == nil {
		return api.TransactionAPI.GetTransactionCount(ctx, address, blockNrOrHash)
	}

	nonce, err := api.b.GetToolNonce(ctx, address)
	if err != nil {
		return nil, err
	}
	return (*hexutil.Uint64)(&nonce), nil
}

func (api *ToolTransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	if !api.b.IsToolEnabled() {
		return api.TransactionAPI.GetTransactionReceipt(ctx, hash)
	}

	receipt, tx, header, err := api.b.GetToolReceipt(hash)
	if err != nil {
		return api.TransactionAPI.GetTransactionReceipt(ctx, hash)
	}

	signer := types.MakeSigner(api.b.ChainConfig(), header.Number, header.Time)
	return marshalToolReceipt(receipt, header, signer, tx), nil
}

func marshalToolReceipt(receipt *types.Receipt, header *types.Header, signer types.Signer, tx *types.Transaction) map[string]interface{} {
	return marshalReceipt(receipt, header.Hash(), header.Number.Uint64(), signer, tx, int(receipt.TransactionIndex))
}

// SendBundleArgs represents the arguments for a SendBundle call.
type SendBundleArgs struct {
	Txs               []hexutil.Bytes `json:"txs"`
	BlockNumber       rpc.BlockNumber `json:"blockNumber"`
	ReplacementUuid   *uuid.UUID      `json:"replacementUuid"`
	SigningAddress    *common.Address `json:"signingAddress"`
	MinTimestamp      *uint64         `json:"minTimestamp"`
	MaxTimestamp      *uint64         `json:"maxTimestamp"`
	RevertingTxHashes []common.Hash   `json:"revertingTxHashes"`
	Builders          []string        `json:"builders"`
}

// SendBundle will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce and ensuring validity
func (api *ToolTransactionAPI) SendBundle(ctx context.Context, args SendBundleArgs) (map[string]interface{}, error) {
	if !api.b.IsToolEnabled() {
		return nil, errors.New("tool not enabled")
	}

	var txs types.Transactions
	if len(args.Txs) == 0 {
		return nil, errors.New("bundle missing txs")
	}

	for _, encodedTx := range args.Txs {
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(encodedTx); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	hash, err := api.b.SendToolBundle(ctx, txs)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"bundleHash": hash,
	}, nil
}

type ToolBlockChainAPI struct {
	*BlockChainAPI
}

func (api *ToolBlockChainAPI) Call(ctx context.Context, args TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Bytes, error) {
	if !api.b.IsToolEnabled() {
		return api.BlockChainAPI.Call(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}

	if blockNrOrHash == nil {
		latest := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
		blockNrOrHash = &latest
	}

	nextBlock := api.b.CurrentBlock().Number.Int64() + 1
	blockNr, ok := blockNrOrHash.Number()
	if ok {
		var (
			result *core.ExecutionResult
			err    error
		)
		if blockNr.Int64() == nextBlock {
			result, err = api.b.DoToolCall(ctx, &args, uint64(nextBlock), overrides, blockOverrides)
		} else if blockNr == rpc.PendingBlockNumber {
			result, err = api.b.DoToolCall(ctx, &args, uint64(nextBlock), overrides, blockOverrides)
		} else {
			return api.BlockChainAPI.Call(ctx, args, blockNrOrHash, overrides, blockOverrides)
		}
		if err != nil {
			return nil, err
		}
		if errors.Is(result.Err, vm.ErrExecutionReverted) {
			return nil, newRevertError(result.Revert())
		}
		return result.Return(), result.Err
	}

	hash, isHash := blockNrOrHash.Hash()
	if !isHash {
		return nil, errors.New("invalid arguments; neither block nor hash specified")
	}

	if api.b.ToolBlockByHash(ctx, hash) == nil {
		return api.BlockChainAPI.Call(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}

	result, err := api.b.DoToolCall(ctx, &args, uint64(nextBlock), overrides, blockOverrides)
	if err != nil {
		return nil, err
	}
	if errors.Is(result.Err, vm.ErrExecutionReverted) {
		return nil, newRevertError(result.Revert())
	}
	return result.Return(), result.Err
}

func (api *ToolBlockChainAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	if !api.b.IsToolEnabled() {
		return api.BlockChainAPI.GetBlockByHash(ctx, hash, fullTx)
	}

	if block := api.b.ToolBlockByHash(ctx, hash); block != nil {
		return RPCMarshalBlock(block, true, fullTx, api.b.ChainConfig()), nil
	}

	return api.BlockChainAPI.GetBlockByHash(ctx, hash, fullTx)
}
