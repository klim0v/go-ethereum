package tool

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

// BlockChain wraps the core.BlockChain to provide specialized functionality
// for SubSlot and Commitment processing. It maintains the state for each
// SubSlot and provides methods to apply SubSlots and create Commitments.
//
// BlockChain keeps track of the current block header and maintains a list of
// processed SubSlots for the current block. It also provides access to the
// current state of the blockchain and methods to query it.
//
// The state maintained by BlockChain allows transactions in Commitments to be
// executed against a consistent state, ensuring deterministic results.
type BlockChain struct {
	BlockChain origBlockChain

	mu           sync.RWMutex
	next         *types.Header   // next is the header of the next block to be created
	parent       *types.Header   // parent is the original header at the last height
	subSlots     []*types.Block  // subSlots contains all processed SubSlots for the current block
	executor     Executor        // executor is responsible for executing transactions
	settlement   *SettlementData // settlement tx data
	reserveSize  uint64          // reserved block size
	isConsistent bool            // isConsistent indicates whether the blockchain is in a consistent state
}

func (b *BlockChain) Next() *types.Header {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.next
}

type origBlockChain interface {
	core.ChainContext
	GetBlockByHash(hash common.Hash) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
}

func newBlockchain(blockChain origBlockChain) *BlockChain {
	return &BlockChain{BlockChain: blockChain}
}

func (b *BlockChain) Close() {
	if b.executor != nil {
		b.executor.Close()
	}
}

func (b *BlockChain) NextSubSlotLimits() (uint64, uint64, uint64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isConsistent {
		return 0, 0, 0, ErrNotConsistent
	}

	leftSubSlots := subSlotsCount - len(b.subSlots)
	if leftSubSlots <= 0 {
		leftSubSlots = 1
	}

	sizeLeft, totalGasLeft, reservedGas := b.executor.SizeAndGasLeft()
	return sizeLeft, (totalGasLeft - reservedGas) / uint64(leftSubSlots), totalGasLeft, nil
}

func (b *BlockChain) SubSlotBlock(hash common.Hash) *types.Block {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subSlot, i := b.subSlotBlock(hash); i != 0 {
		return subSlot
	}

	return nil
}

func (b *BlockChain) prevHash() common.Hash {
	if length := len(b.subSlots); length != 0 {
		return b.subSlots[length-1].Header().Hash()
	}

	return b.parent.Hash()
}

func (b *BlockChain) subSlotBlock(hash common.Hash) (*types.Block, uint8) {
	for i, subSlot := range b.subSlots {
		if subSlot.Hash() == hash {
			return subSlot, uint8(i) + 1
		}
	}

	return nil, 0
}

func (b *BlockChain) updateParentBlock(parent *types.Header) {
	b.next = nil
	b.parent = parent
	b.settlement = nil
	b.subSlots = make([]*types.Block, 0, subSlotsCount)
	b.isConsistent = false
}

func (b *BlockChain) SetupNextHeader(attr *HeaderParams, reserveSize uint64, settlement *SettlementData) (*Settlement, *types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.parent != nil && b.parent.Hash() != attr.Parent {
		b.updateParentBlock(nil)
	}
	if b.parent == nil {
		parentBlock := b.BlockChain.GetBlockByHash(attr.Parent)
		if parentBlock == nil {
			return nil, nil, ErrStateNotReady
		}
		b.updateParentBlock(parentBlock.Header())
	}

	// FIXME: b.parent.Hash().Bytes() #0xdD11751cdD3f6EFf01B1f6151B640685bfa5dB4a
	coinbasePrv, err := crypto.ToECDSA(hexutil.MustDecode("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff81"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init coinbase, err: %w", err)
	}

	if b.isConsistent && b.next.ParentHash == attr.Parent && b.next.Time == attr.Timestamp && attr.Random == b.next.MixDigest {
		log.Warn("Next header already up-to-date", "parent", attr.Parent, "timestamp", attr.Timestamp)
		_, _, gasReserved := b.executor.SizeAndGasLeft()
		return &Settlement{
			PrvKey: coinbasePrv,
			Gas:    gasReserved,
			TxData: b.settlement,
		}, nil, nil
	}

	if b.isConsistent {
		b.updateParentBlock(b.parent)
		log.Warn("Next block params changed", "parent", attr.Parent, "timestamp", attr.Timestamp, "random", attr.Random)
	}

	err = b.setupNextHeader(attr, crypto.PubkeyToAddress(coinbasePrv.PublicKey))
	if err != nil {
		log.Error("Failed to setup next header", "parent", attr.Parent, "error", err)
		return nil, nil, ErrNotConsistent
	}

	b.reserveSize = uint64(b.next.Size()) + reserveSize // FIXME: reserve settlement tx size
	b.settlement = settlement

	gasReserved, err := b.initExecutor()
	if err != nil {
		log.Error("Failed to init tool executor,", "parent", attr.Parent, "error", err)
		return nil, nil, ErrNotConsistent
	}

	b.isConsistent = true
	return &Settlement{
		PrvKey: coinbasePrv,
		Gas:    gasReserved,
		TxData: b.settlement,
	}, b.next, nil
}

func (b *BlockChain) setupNextHeader(attr *HeaderParams, coinbase common.Address) (err error) {
	if b.parent == nil {
		return errors.New("parent block is nil")
	}

	excessBlobGas := eip4844.CalcExcessBlobGas(b.BlockChain.Config(), b.parent, attr.Timestamp)
	b.next = &types.Header{
		ParentHash:       b.parent.Hash(),
		UncleHash:        types.EmptyUncleHash,
		Coinbase:         coinbase,
		TxHash:           types.EmptyTxsHash,
		ReceiptHash:      types.EmptyReceiptsHash,
		Difficulty:       common.Big0,
		Number:           new(big.Int).Add(b.parent.Number, common.Big1),
		GasLimit:         core.CalcGasLimit(b.parent.GasLimit, attr.GasLimit),
		GasUsed:          0,
		Time:             attr.Timestamp,
		MixDigest:        attr.Random,
		BaseFee:          eip1559.CalcBaseFee(b.BlockChain.Config(), b.parent),
		WithdrawalsHash:  &types.EmptyWithdrawalsHash,
		BlobGasUsed:      new(uint64),
		ExcessBlobGas:    &excessBlobGas,
		ParentBeaconRoot: attr.BeaconRoot,
		RequestsHash:     &types.EmptyRequestsHash,
	}

	return nil
}

func (b *BlockChain) getNextSignerAndHeader() (types.Signer, *types.Header, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isConsistent {
		return nil, nil, ErrStateNotReady
	}

	return b.executor.GetSigner(), b.next, nil
}

func (b *BlockChain) GetNonce(addr common.Address) (uint64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isConsistent {
		return 0, ErrStateNotReady
	}

	return b.executor.GetNonce(addr), nil
}

func (b *BlockChain) DoCall(ctx context.Context, txArgs TransactionArgs, gasCap uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (result *core.ExecutionResult, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			log.Error("Failed to do call", "error", r)
			if _, ok := r.(runtime.Error); ok {
				debug.PrintStack()
			}

			if _, err := b.initExecutor(); err != nil {
				b.isConsistent = false
				log.Error("Failed to restore TOOL state", "error", err)
			}

			err = errFailExecution
		}
	}()

	return b.executor.DoCall(ctx, txArgs, gasCap, stateOverrides, blockOverrides)
}

// CreateCommitment creates a new Commitment from the given Bundle. It executes
// the transactions in the Bundle to determine their effects and collects
// information about gas used, access list, storage changes, and logs.
//
// Returns the created Commitment or an error if the Bundle could not be processed.
func (b *BlockChain) CreateCommitment(bundle *types.Bundle) (cmt *types.Commitment, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isConsistent {
		return nil, ErrStateNotReady
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error("Failed to create commitment", "error", r)
			if _, ok := r.(runtime.Error); ok {
				debug.PrintStack()
			}

			if _, err := b.initExecutor(); err != nil {
				b.isConsistent = false
				log.Error("Failed to restore TOOL state", "error", err)
			}

			err = errFailExecution
		}
	}()

	return b.executor.CreateCommitment(bundle)
}

func (b *BlockChain) initExecutor() (uint64, error) {
	if b.executor != nil {
		b.executor.Close()
	}

	statedb, err := b.BlockChain.StateAt(b.parent.Root)
	if err != nil {
		return 0, fmt.Errorf("failed to get stateDB, err: %w", err)
	}

	b.executor = newExecutor(b.next, b.BlockChain, statedb)
	if err = b.executor.SubLeftBlockSize(b.reserveSize); err != nil {
		return 0, err
	}
	var gasReserved uint64
	if b.settlement != nil {
		gasReserved, err = b.executor.ReserveGas(b.settlement.FeeRecipient, b.settlement.Data)
		if err != nil {
			return 0, fmt.Errorf("failed to reserve settlement gas, err: %w", err)
		}
	}

	if len(b.subSlots) == 0 {
		return gasReserved, nil
	}

	// apply historic subSlots when reinitializing executor after failure or new block params
	var txIndex int
	for i, subSlotBlock := range b.subSlots {
		transactions := subSlotBlock.Transactions()
		transactionsCount := transactions.Len()

		txs := make([]common.Hash, 0, transactionsCount)
		for _, transaction := range transactions {
			txs = append(txs, transaction.Hash())
		}
		subSlot := types.NewSubSlot(b.next.ParentHash, uint8(i), b.next.Time, txs...)
		if !subSlot.SetTxs(transactions) {
			return 0, fmt.Errorf("failed to resolve subSlot txs, err: %w", err)
		}

		err := b.executor.ApplySubSlot(txIndex, subSlot)
		if err != nil {
			return 0, fmt.Errorf("%w, err: %w", errFailExecution, err)
		}

		txIndex += transactionsCount
	}

	return gasReserved, nil
}

func (b *BlockChain) ReinitExecutor() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, err := b.initExecutor()
	if err != nil {
		return err
	}

	return nil
}

// ProcessSubSlot applies the given SubSlot to the current state. It executes
// the transactions in the SubSlot and updates the state accordingly.
//
// Returns the transactions and receipts included in the SubSlot, or an error
// if the SubSlot could not be processed.
func (b *BlockChain) ProcessSubSlot(newSubSlot *types.SubSlot) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	err := b.processSubSlot(newSubSlot)
	if err != nil {
		return err
	}

	return nil
}

func (b *BlockChain) processSubSlot(subSlot *types.SubSlot) error {
	if !b.isConsistent {
		return ErrStateNotReady
	}

	if b.parent.Hash() != subSlot.BlockParent || len(b.subSlots) != int(subSlot.Index) {
		return fmt.Errorf("%w: want parent block %s subSlot %d, got parent block %s subSlot %d",
			ErrNotConsistent, b.parent.Hash(), len(b.subSlots), subSlot.BlockParent, subSlot.Index)
	}
	b.isConsistent = false

	err := b.executor.ApplySubSlot(b.acceptedTxsLen(), subSlot)
	if err != nil {
		return fmt.Errorf("%w, err: %w", errFailExecution, err)
	}

	header := types.CopyHeader(b.next)
	header.ReceiptHash = types.DeriveSha(subSlot.Receipts, trie.NewStackTrie(nil))
	header.GasUsed = subSlot.GasUsed        // it's not a total
	header.ParentHash = b.parent.ParentHash // todo: or b.prevHash()?
	header.Root = subSlot.ID()

	b.subSlots = append(b.subSlots, types.NewBlockWithHeader(header).WithBody(types.Body{Transactions: subSlot.GetTxs()}))

	b.isConsistent = true
	return nil
}

func (b *BlockChain) acceptedTxsLen() (count int) {
	for _, subSlot := range b.subSlots {
		count += subSlot.Transactions().Len()
	}
	return count
}
