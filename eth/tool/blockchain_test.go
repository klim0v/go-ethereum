package tool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type blockchain struct {
	head    *types.Header
	statedb *state.StateDB
	genesis *core.Genesis
}

func (b *blockchain) Engine() consensus.Engine {
	return nil
}

func (b *blockchain) GetHeader(hash common.Hash, u uint64) *types.Header {
	return b.head
}

func (b *blockchain) Config() *params.ChainConfig {
	return b.genesis.Config
}

func (b *blockchain) GetBlockByHash(hash common.Hash) *types.Block {
	if hash != b.head.Hash() {
		return nil
	}
	return types.NewBlockWithHeader(b.head)
}

func (b *blockchain) StateAt(root common.Hash) (*state.StateDB, error) {
	return b.statedb, nil
}
