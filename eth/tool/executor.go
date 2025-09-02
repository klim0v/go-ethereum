package tool

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// Executor is responsible for executing transactions in the context of a
// Commitment or SubSlot. It provides methods to create a Commitment from a
// Bundle and to apply a SubSlot to the current state.
//
// The Executor maintains an EVM instance and a state database, and uses them
// to execute transactions and collect information about their effects.
//
// The Executor ensures that transactions are executed in a deterministic way,
// producing consistent results across different nodes in the network.
type Executor interface {
	CreateCommitment(bundle *types.Bundle) (commitment *types.Commitment, err error)
	ApplySubSlot(txIndex int, subSlot *types.SubSlot) error
	GetSigner() types.Signer
	Close()
	SizeAndGasLeft() (size uint64, totalGas uint64, reservedGas uint64)
	ReserveGas(receiver *common.Address, data []byte) (reserved uint64, err error)
	SubLeftBlockSize(size uint64) error
	DoCall(ctx context.Context, txArgs TransactionArgs, gasCap uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (*core.ExecutionResult, error)
	GetNonce(addr common.Address) uint64
}

type TransactionArgs interface {
	CallDefaults(globalGasCap uint64, baseFee *big.Int, chainID *big.Int) error
	ToMessage(baseFee *big.Int, skipNonceCheck, skipEoACheck bool) *core.Message
}

type executor struct {
	evm         *vm.EVM
	signer      types.Signer
	baseFee     *big.Int
	gasPool     core.GasPool
	leftSize    uint64
	usedGas     uint64
	gasReserved uint64
	blockNumber *big.Int
	blockTime   uint64
	statedb     *state.StateDB
	restorer    state.SnapshotTrackerVersion
}

func newExecutor(header *types.Header, bc origBlockChain, statedb *state.StateDB) *executor {
	evm := vm.NewEVM(core.NewEVMBlockContext(header, bc, &header.Coinbase), statedb, bc.Config(), vm.Config{})
	if header.ParentBeaconRoot != nil {
		core.ProcessBeaconBlockRoot(*header.ParentBeaconRoot, evm)
	}
	core.ProcessParentBlockHash(header.ParentHash, evm)
	return &executor{
		evm:         evm,
		signer:      types.MakeSigner(bc.Config(), header.Number, header.Time),
		baseFee:     header.BaseFee,
		leftSize:    params.MaxBlockSize,
		gasPool:     core.GasPool(header.GasLimit),
		blockNumber: header.Number,
		blockTime:   header.Time,
		statedb:     statedb,
		restorer:    state.ObjectsSnapshotTracker,
	}
}

func (e *executor) Close() {
}

func (e *executor) Clone() *executor {
	stateDB := e.statedb.Copy()
	return &executor{
		evm:         vm.NewEVM(e.evm.Context, stateDB, e.evm.ChainConfig(), e.evm.Config),
		signer:      e.signer,
		baseFee:     e.baseFee,
		gasPool:     e.gasPool,
		leftSize:    e.leftSize,
		usedGas:     e.usedGas,
		gasReserved: e.gasReserved,
		blockNumber: new(big.Int).Set(e.blockNumber),
		blockTime:   e.blockTime,
		statedb:     stateDB,
		restorer:    e.restorer,
	}
}

func (e *executor) SizeAndGasLeft() (uint64, uint64, uint64) {
	return e.leftSize, e.gasPool.Gas(), e.gasReserved
}

func (e *executor) resetStateRestorer() {
	e.statedb.InitRestorer(e.restorer)
}

func (e *executor) SubLeftBlockSize(size uint64) error {
	if e.leftSize < size {
		return core.ErrBlockOversized
	}
	e.leftSize -= size
	return nil
}

func (e *executor) ReserveGas(receiver *common.Address, data []byte) (reserved uint64, err error) {
	if receiver != nil && e.statedb.GetCodeSize(*receiver) == 0 {
		if len(data) != 0 {
			return 0, fmt.Errorf("expected empty data for simple transfer tx")
		}
		e.gasReserved = params.TxGas
		return e.gasReserved, nil
	}

	evm := vm.NewEVM(e.evm.Context, e.statedb, e.evm.ChainConfig(), vm.Config{NoBaseFee: true})
	gasPool := new(core.GasPool).AddGas(e.gasPool.Gas())
	msg := &core.Message{
		To:        receiver,
		From:      evm.Context.Coinbase,
		Nonce:     e.statedb.GetNonce(evm.Context.Coinbase),
		Value:     new(big.Int).Lsh(big.NewInt(1), 255),
		GasLimit:  gasPool.Gas(),
		GasPrice:  common.Big0,
		GasFeeCap: common.Big0,
		GasTipCap: common.Big0,
		Data:      data,
	}

	snapshot := e.statedb.Snapshot()
	defer e.statedb.RevertToSnapshot(snapshot)

	e.statedb.SetBalance(msg.From, new(uint256.Int).SetAllOne(), 0)
	result, err := core.ApplyMessage(evm, msg, gasPool)
	if err != nil {
		return 0, fmt.Errorf("failed settelment message to %s, err: %w", receiver, err)
	}
	if result.Failed() {
		return 0, fmt.Errorf("failed settelment execution to %s, err: %w", receiver, result.Err)
	}

	e.gasReserved = result.MaxUsedGas
	return e.gasReserved, nil
}

func (e *executor) restoreStateDB() (err error) {
	if e.restorer == state.NoSnapshotTracker {
		return nil
	}
	e.statedb, err = e.statedb.Restore()
	if err != nil {
		return err
	}
	return nil
}

func (e *executor) GetSigner() types.Signer {
	return e.signer
}

// CreateCommitment simulates the execution of transactions from a bundle to create a new commitment.
// This method does not permanently modify the state, but only simulates the effect of transactions
// on top of the current state formed by previous subSlots.
//
// Workflow:
//  1. Creates a temporary copy of state
//  2. Reproduces the base state from already accepted transactions on this copy
//  3. Simulates new transactions from bundle
//  4. Collects information about results (logs, gas, storage, accesses)
//  5. Forms the commitment structure
func (e *executor) CreateCommitment(bundle *types.Bundle) (commitment *types.Commitment, err error) {
	if block := e.blockNumber.Uint64(); bundle.Block == 0 {
		bundle.Block = block
	} else if bundle.Block != block {
		return nil, fmt.Errorf("bundle block mismatch with execution state")
	}

	commitment = &types.Commitment{
		BlockParent:      bundle.BlockParent,
		Block:            e.blockNumber.Uint64(),
		SubSlot:          bundle.SubSlot,
		ExpiresAtSubSlot: bundle.ExpiresAtSubSlot,
		BlockTime:        e.blockTime,
		Txs:              bundle.BasedTxs(),
		Logs:             bundle.BasedLogs(),
		TxsSize:          bundle.BasedTxsSize(),
		GasUsed:          bundle.BasedGasUsed(),
		MaxGasLimit:      bundle.BasedMaxGasLimit(),
	}

	txIndex := len(commitment.Txs)
	for _, tx := range bundle.ExecutingTxs() {
		commitment.Txs = append(commitment.Txs, tx.Hash())
	}

	oldBalance := e.statedb.GetBalance(e.evm.Context.Coinbase)
	e.resetStateRestorer()
	defer func() {
		if err := e.restoreStateDB(); err != nil {
			panic(err) // recover state by existing subSlots
		}
	}()

	snapshot := e.statedb.Snapshot()
	if err = e.statedb.AddStateDiff(bundle.BasedStateDiff()); err != nil {
		e.statedb.RevertToSnapshot(snapshot)
		log.Error("Failed to setup based state diff", "txs", bundle.BasedTxs(), "error", err)
		return nil, fmt.Errorf("failed to setup based state diff, err: %w", err)
	}

	tracer := logger.NewAccessesTracer(bundle.BasedAccesses())
	evm := vm.NewEVM(e.evm.Context, state.NewHookedState(e.statedb, tracer.Hooks()), e.evm.ChainConfig(), vm.Config{Tracer: tracer.Hooks()})

	gasPool := new(core.GasPool).AddGas(e.gasPool.Gas())
	if err = gasPool.SubGas(bundle.BasedGasUsed()); err != nil {
		return nil, fmt.Errorf("failed to setup based gas pool, err: %w", err)
	}

	cmtID := commitment.ID()
	for i, tx := range bundle.ExecutingTxs() {
		txHash := tx.Hash() // FIXME: re-executed tx hashes leak from logs
		cmtTxIndex := i + txIndex
		e.statedb.SetTxContext(txHash, cmtTxIndex)

		snap := e.statedb.Snapshot()
		result, err := core.ApplyToolTransaction(evm, gasPool, tx, e.signer, e.baseFee)
		if err != nil {
			e.statedb.RevertToSnapshot(snap)
			return nil, fmt.Errorf("failed to apply tx %d, err: %w", cmtTxIndex, err)
		}

		e.statedb.Finalise(true)

		commitment.TxsSize += tx.WithoutBlobTxSidecar().Size()
		commitment.GasUsed += result.UsedGas
		commitment.MaxGasLimit = max(commitment.MaxGasLimit, commitment.GasUsed+tx.Gas())
		commitment.Logs = slices.Concat(commitment.Logs, e.statedb.GetLogs(txHash, e.blockNumber.Uint64(), cmtID, e.blockTime))
	}

	commitment.Fees = new(uint256.Int).Sub(e.statedb.GetBalance(e.evm.Context.Coinbase), oldBalance)
	commitment.Accesses = tracer.Accesses()
	commitment.StateDiff = e.statedb.GetStateDiff()
	commitment.SetExecutedTxsCount(bundle.ExecutingTxs().Len())

	return commitment, nil
}

// ApplySubSlot applies transactions from a subSlot to the current blockchain state.
// This method changes the state, creating the foundation for further simulation of new transactions.
//
// Workflow:
//  1. Applies each transaction from the subSlot to the current state
//  2. Updates the main StateDB and EVM
//  3. Saves execution results (receipts, logs, gas, storage, accesses)
func (e *executor) ApplySubSlot(txIndex int, subSlot *types.SubSlot) error {
	if block := e.blockNumber.Uint64(); subSlot.Block == 0 {
		subSlot.Block = block
	} else if subSlot.Block != block {
		return fmt.Errorf("subSlot block mismatch with execution state")
	}

	e.resetStateRestorer()

	tracer := logger.NewAccessesTracer(nil)
	e.evm.Config.Tracer = tracer.Hooks()
	e.evm.StateDB = state.NewHookedState(e.statedb, tracer.Hooks())

	var usedGas uint64
	txReceipts := make(types.Receipts, 0, subSlot.GetTxs().Len())
	for i, tx := range subSlot.GetTxs() {
		e.statedb.SetTxContext(tx.Hash(), txIndex+i)

		msg, err := core.TransactionToMessage(tx, e.signer, e.baseFee)
		if err != nil {
			return err
		}

		receipt, err := core.ApplyTransactionWithEVM(msg, &e.gasPool, e.statedb, e.blockNumber, common.Hash{}, e.blockTime, tx, &usedGas, e.evm)
		if err != nil {
			return fmt.Errorf("failed to apply tx %d, err: %w", i, err)
		}

		if receipt.Status == types.ReceiptStatusFailed {
			return fmt.Errorf("failed tx receipt %s", tx.Hash())
		}

		if err = e.SubLeftBlockSize(tx.WithoutBlobTxSidecar().Size()); err != nil {
			return err
		}
		txReceipts = append(txReceipts, receipt)
	}

	e.usedGas += usedGas
	subSlot.GasUsed = usedGas
	subSlot.Receipts = txReceipts
	subSlot.Accesses = tracer.Accesses()
	subSlot.StateDiff = e.statedb.GetStateDiff()

	return nil
}

func (e *executor) DoCall(ctx context.Context, txArgs TransactionArgs, gasCap uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (*core.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing TOOL call finished", "runtime", time.Since(start)) }(time.Now())

	gp := new(core.GasPool)
	if gasCap == 0 {
		gp.AddGas(math.MaxUint64)
	} else {
		gp.AddGas(gasCap)
	}
	if err := txArgs.CallDefaults(gp.Gas(), e.evm.Context.BaseFee, e.signer.ChainID()); err != nil {
		return nil, err
	}
	msg := txArgs.ToMessage(e.baseFee, true, true)

	blockCtx := e.evm.Context
	if blockOverrides != nil {
		if err := blockOverrides.Apply(&blockCtx); err != nil {
			return nil, err
		}
	}
	if msg.GasPrice.Sign() == 0 {
		blockCtx.BaseFee = new(big.Int)
	}
	if msg.BlobGasFeeCap != nil && msg.BlobGasFeeCap.BitLen() == 0 {
		blockCtx.BlobBaseFee = new(big.Int)
	}

	evm := vm.NewEVM(blockCtx, e.statedb, e.evm.ChainConfig(), vm.Config{NoBaseFee: true})
	if stateOverrides != nil {
		e.resetStateRestorer()
		defer func() {
			if err := e.restoreStateDB(); err != nil {
				panic(err) // recover state by existing subSlots
			}
		}()

		snapshot := e.statedb.Snapshot()
		precompiles := evm.GetPrecompiles()
		if err := stateOverrides.Apply(e.statedb, precompiles); err != nil {
			e.statedb.RevertToSnapshot(snapshot)
			log.Error("Failed to override state", "error", err)
			return nil, fmt.Errorf("failed to override state, err: %w", err)
		}
		evm.SetPrecompiles(precompiles)
	}

	snapshot := e.statedb.Snapshot()
	defer e.statedb.RevertToSnapshot(snapshot)

	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	result, err := core.ApplyMessage(evm, msg, gp)

	if evm.Cancelled() {
		return nil, fmt.Errorf("execution aborted by timeout")
	}
	if err != nil {
		return result, fmt.Errorf("err: %w (supplied gas %d)", err, msg.GasLimit)
	}

	return result, nil
}

func (e *executor) GetNonce(addr common.Address) uint64 {
	return e.statedb.GetNonce(addr)
}
