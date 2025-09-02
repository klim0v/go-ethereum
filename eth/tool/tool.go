package tool

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	totalAcceptedTxs = metrics.NewRegisteredCounter("tool/txs/accepted", nil)

	subSlotMgaspsGauge       = metrics.NewRegisteredGauge("tool/subslot/mgasps", nil)
	subSlotNextGauge         = metrics.NewRegisteredGauge("tool/subslot/next", nil)
	subSlotTxsGauge          = metrics.NewRegisteredGauge("tool/subslot/txs", nil)
	subSlotTxsUnclaimedGauge = metrics.NewRegisteredGauge("tool/subslot/txsUnclaimed", nil)

	commitmentLocalCounter  = metrics.NewRegisteredCounter("tool/commitment/local", nil)
	commitmentRemoteCounter = metrics.NewRegisteredCounter("tool/commitment/remote", nil)
	commitmentNewTxsCounter = metrics.NewRegisteredCounter("tool/commitment/newTxs", nil)
	commitmentOwnTxsCounter = metrics.NewRegisteredCounter("tool/commitment/ownTxs", nil)

	ErrToolDisabled   = errors.New("tool disabled")
	ErrStateNotReady  = errors.New("state not ready")
	ErrNotConsistent  = errors.New("not consistent")
	errTxNotResolved  = errors.New("txs not resolved")
	errFailExecution  = errors.New("execution failed")
	errAPIFeedStopped = errors.New("api feed stopped")
)

const subSlotsCount = 12

var DefaultConfig = Config{
	Enabled: false,
	Leader:  false,
	APIFeed: true,
}

type Config struct {
	Enabled bool
	Leader  bool
	APIFeed bool
}

// Tool coordinates all components of the tool system, orchestrating the workflow
// between BlockChain, Pool, and network handlers. It's responsible for processing
// incoming SubSlots and Commitments, and for maintaining synchronization with
// the main blockchain.
//
// The Tool maintains a state machine that tracks the current block and SubSlot
// number, and processes incoming SubSlots in sequence. It also handles the
// lifecycle of Commitments, from creation to acceptance in a SubSlot.
//
// When a new block is detected, Tool resets its state and begins a new cycle
// of SubSlot processing.
type Tool struct {
	Config Config // Config holds configuration options for the Tool
	log    log.Logger
	ready  chan struct{}
	stop   chan struct{}
	wg     sync.WaitGroup

	pool       *Pool // pool manages Transactions, Commitments, SubSlots
	assembler  assembler
	blockChain *BlockChain // blockChain provides access to blockchain state and operations
	isBCSynced func() bool

	apiFeeds   event.SubscriptionScope
	apiFeedOn  bool
	apiFeedMtx sync.RWMutex

	subSlotFeed    event.Feed
	commitmentFeed event.Feed
	nextHeaderFeed event.Feed

	mu            sync.RWMutex
	preConf       bool
	nextTime      uint64
	headBlock     common.Hash
	nextBlock     uint64           // nextBlock is the number of the next block to be processed
	nextSubSlot   uint8            // nextSubSlot is the index of the next SubSlot to be processed
	knownSubSlots []*types.SubSlot // knownSubSlots contains all SubSlots known to the system
	leaderTicker  *time.Ticker
}

func New(config Config, bc origBlockChain, isBCSynced func() bool) *Tool {
	tool := &Tool{
		Config:     config,
		log:        log.New(),
		ready:      make(chan struct{}),
		stop:       make(chan struct{}),
		pool:       newPool(),
		blockChain: newBlockchain(bc),
		isBCSynced: isBCSynced,
		apiFeedOn:  config.APIFeed,
		nextBlock:  math.MaxUint64,
	}

	if config.Enabled && config.Leader {
		tool.wg.Add(1)
		go tool.leaderLoop()
	}

	if !tool.apiFeedOn {
		tool.apiFeeds.Close()
	}

	return tool
}

func (t *Tool) Enabled() bool {
	return t != nil && t.Config.Enabled
}

func (t *Tool) Ready() <-chan struct{} {
	return t.ready
}

func (t *Tool) Log() log.Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.log.With("headBlock", t.headBlock)
}

func (t *Tool) RefreshState() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.blockChain.ReinitExecutor()
}

func (t *Tool) GetNonce(addr common.Address) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.isInvalid() {
		return 0, ErrNotConsistent
	}

	return t.blockChain.GetNonce(addr)
}

func (t *Tool) DoCall(ctx context.Context, txArgs TransactionArgs, gasCap uint64, block uint64, stateOverrides *override.StateOverride, blockOverrides *override.BlockOverrides) (*core.ExecutionResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.isInvalid() {
		return nil, ErrNotConsistent
	}

	if t.nextBlock != block {
		return nil, ErrNotConsistent
	}

	return t.blockChain.DoCall(ctx, txArgs, gasCap, stateOverrides, blockOverrides)
}

func (t *Tool) GetReceipt(tx common.Hash) (*types.Receipt, *types.Transaction, *types.Header, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.isInvalid() {
		return nil, nil, nil, ErrNotConsistent
	}

	//if !t.preConfirmed() {
	//	return nil, nil, nil, errors.New("not found")
	//}

	receipt, transaction := t.pool.AcceptedTx(tx)
	if receipt == nil {
		return nil, nil, nil, errors.New("receipt not found")
	}
	if transaction == nil {
		return nil, nil, nil, errors.New("transaction not found")
	}

	signer, header, err := t.blockChain.getNextSignerAndHeader()
	if err != nil {
		return nil, nil, nil, err
	}

	receiptHeader := types.CopyHeader(header)
	receiptHeader.TxHash = types.DeriveSha(types.Transactions{transaction}, trie.NewStackTrie(nil))
	receiptHeader.ReceiptHash = types.DeriveSha(types.Receipts{receipt}, trie.NewStackTrie(nil))
	receiptHeader.GasUsed = receipt.GasUsed

	block := types.NewBlockWithHeader(receiptHeader).WithBody(types.Body{Transactions: types.Transactions{transaction}})
	t.pool.CacheReceiptBlock(block)

	singleBlockReceipt := *receipt
	singleBlockReceipt.DeriveFields(signer, types.DeriveReceiptContext{
		BlockHash:   block.Hash(),
		BlockNumber: block.NumberU64(),
		BlockTime:   block.Time(),
		BaseFee:     block.BaseFee(),
		GasUsed:     receipt.GasUsed,
		Tx:          transaction,
	})

	return &singleBlockReceipt, transaction, block.Header(), nil
}

func (t *Tool) preConfirmed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.preConf
}

func (t *Tool) GetBlock(hash common.Hash) *types.Block {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.isInvalid() {
		return nil
	}

	block := t.pool.GetReceiptBlock(hash)
	if block == nil {
		block = t.blockChain.SubSlotBlock(hash)
	}

	return block
}

func (t *Tool) AddTxs(txs types.Transactions) {
	t.pool.AddTxs(txs)
}

func (t *Tool) GetTx(hash common.Hash) *types.Transaction {
	return t.pool.GetTx(hash)
}

func (t *Tool) GetTxs(hashes []common.Hash) (txs types.Transactions, unknown []common.Hash) {
	return t.pool.GetTxs(hashes)
}

func (t *Tool) GetTxMassages(hashes []common.Hash) ([]*core.Message, error) {
	txs, unknown := t.pool.GetTxs(hashes)
	if len(unknown) != 0 {
		return nil, fmt.Errorf("not found %d txs", len(unknown))
	}
	signer, header, err := t.blockChain.getNextSignerAndHeader()
	if err != nil {
		return nil, err
	}
	messages := make([]*core.Message, 0, len(txs))
	for _, tx := range txs {
		message, err := core.TransactionToMessage(tx, signer, header.BaseFee)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (t *Tool) GetCommitment(hash common.Hash) (_ *types.Commitment, available, valid bool) {
	return t.pool.GetCommitment(hash)
}

func (t *Tool) Pool() *Pool {
	return t.pool
}

func (t *Tool) Blockchain() *BlockChain {
	return t.blockChain
}

func (t *Tool) SubscribeNextHeader(ch chan<- *types.Header) event.Subscription {
	return t.nextHeaderFeed.Subscribe(ch)
}

func (t *Tool) SubscribeSubSlots(ch chan<- *types.SubSlot) event.Subscription {
	return t.subSlotFeed.Subscribe(ch)
}

func (t *Tool) SubscribeCommitments(ch chan<- *types.Commitment) event.Subscription {
	return t.commitmentFeed.Subscribe(ch)
}

func (t *Tool) APIFeedSwitch(enable bool) (success bool) {
	t.apiFeedMtx.Lock()
	defer t.apiFeedMtx.Unlock()

	if t.apiFeedOn == enable {
		return true
	}

	if enable {
		t.apiFeeds = event.SubscriptionScope{}
	} else {
		t.apiFeeds.Close()
	}
	t.apiFeedOn = enable

	return true
}

func (t *Tool) sendCommitmentFeed(commitment *types.Commitment) {
	t.commitmentFeed.Send(commitment)
}

func (t *Tool) sendSubSlotFeed(subSlot *types.SubSlot) {
	t.subSlotFeed.Send(subSlot)
}

func (t *Tool) APISubscribeSubSlots(ch chan<- *types.SubSlot) (event.Subscription, error) {
	t.apiFeedMtx.RLock()
	defer t.apiFeedMtx.RUnlock()

	if !t.apiFeedOn {
		return nil, errAPIFeedStopped
	}

	return t.apiFeeds.Track(t.SubscribeSubSlots(ch)), nil
}

func (t *Tool) APISubscribeCommitments(ch chan<- *types.Commitment) (event.Subscription, error) {
	t.apiFeedMtx.RLock()
	defer t.apiFeedMtx.RUnlock()

	if !t.apiFeedOn {
		return nil, errAPIFeedStopped
	}

	return t.apiFeeds.Track(t.SubscribeCommitments(ch)), nil
}

func (t *Tool) GetPendingCommitments() []*types.Commitment {
	return t.pool.GetPendingCommitments()
}

func (t *Tool) AddCommitment(commitment *types.Commitment) (unknownTxs []common.Hash, added bool) {
	dirty, invalid := t.checkCommitment(commitment)
	if invalid {
		return nil, false
	}

	unknownTxs, err := t.pool.AddCommitment(commitment, dirty)
	if err != nil {
		return nil, false
	}

	t.sendCommitmentFeed(commitment.CloneWithoutNewTxs())
	commitmentRemoteCounter.Inc(1)
	commitmentNewTxsCounter.Inc(int64(len(unknownTxs)))

	return unknownTxs, true
}

func (t *Tool) checkCommitment(commitment *types.Commitment) (dirty, invalid bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if commitment.BlockParent != t.headBlock || commitment.BlockTime != t.nextTime {
		return false, true
	}

	if commitment.SubSlot == t.nextSubSlot {
		return false, false
	}

	if t.nextSubSlot < commitment.SubSlot || t.nextSubSlot > commitment.ExpiresAtSubSlot {
		return false, true
	}

	for i, subSlot := range t.knownSubSlots[commitment.SubSlot+1 : t.nextSubSlot] {
		if commitment.SubSlot+uint8(i+1) != subSlot.Index {
			return false, true
		}

		_, conflicts := types.HasAccessesConflicts(subSlot.Accesses, commitment.Accesses)
		if conflicts {
			return true, false
		}
	}

	return false, false
}

// ProcessBundle creates a new Commitment from the given Bundle. It verifies
// that all referenced Commitments exist and are compatible, and executes the
// new transactions to determine their effects. The resulting Commitment is
// added to the pending Commitments pool.
//
// If autoAdjust is true, the Block, SubSlot, and ExpiresAtSubSlot fields of
// the Bundle will be set to appropriate values based on the current state.
//
// When base commitments have conflicting access patterns (e.g., they modify
// the same storage slots in different ways), their transactions need to be
// re-executed to ensure correctness. The function handles two scenarios:
//  1. No conflicts: effects from base commitments are directly combined
//  2. Conflicts detected:
//     - The function identifies non-conflicting sequential commitments
//     - Uses their effects directly (gas, logs, storage changes)
//     - Re-executes only transactions from conflicting commitments
//
// Returns the created Commitment or an error if the Bundle could not be processed.
func (t *Tool) ProcessBundle(bundle *types.Bundle, autoAdjust bool) (*types.Commitment, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, tx := range bundle.NewTxs {
		if tx.Type() == types.BlobTxType {
			return nil, fmt.Errorf("bundle contains blob tx %s", tx.Hash().Hex())
		}
	}

	sizeLeft, gasLimit, totalGasLeft, err := t.blockChain.NextSubSlotLimits()
	if err != nil {
		return nil, err
	}

	if autoAdjust && !t.isInvalid() {
		bundle.BlockParent = t.headBlock
		bundle.Block = t.nextBlock
		bundle.SubSlot = t.nextSubSlot
		bundle.ExpiresAtSubSlot = math.MaxUint8
	} else {
		if bundle.Block != t.nextBlock {
			return nil, errors.New("bundle block invalid")
		}
		if bundle.BlockParent != t.headBlock {
			return nil, errors.New("invalid bundle parent block")
		}
		if bundle.SubSlot != t.nextSubSlot {
			return nil, errors.New("invalid bundle subSlot")
		}
		if bundle.ExpiresAtSubSlot < t.nextSubSlot {
			return nil, errors.New("invalid bundle expiration")
		}
	}

	firstConflictIndex := len(bundle.BasedCommitments)
	commitments := make([]*types.Commitment, 0, len(bundle.BasedCommitments))
	for i, hash := range bundle.BasedCommitments {
		commitment, available, valid := t.pool.GetCommitment(hash)
		if commitment == nil {
			return nil, fmt.Errorf("not found commitment %s", hash)
		}
		if !available {
			return nil, fmt.Errorf("unavailable commitment %s", hash)
		}
		if commitment.BlockParent != bundle.BlockParent {
			return nil, fmt.Errorf("invalid commitment %s block", hash)
		}
		if commitment.ExpiresAtSubSlot < bundle.ExpiresAtSubSlot {
			return nil, fmt.Errorf("invalid commitment %s experation", hash)
		}
		if commitment.MaxGasLimit > totalGasLeft {
			return nil, fmt.Errorf("invalid commitment %s max gas limit", hash)
		}

		if !valid && firstConflictIndex > i {
			firstConflictIndex = i // required to re-execute
		}
		commitments = append(commitments, commitment)
	}

	// Find first conflict and merge access lists
	var accesses types.Accesses
	for i, commitment := range commitments[:firstConflictIndex] {
		merged, ok := types.MergeAccesses(accesses, commitment.Accesses)
		if !ok {
			firstConflictIndex = i
			break
		}
		accesses = merged
	}

	// Get transactions to re-execute from conflicting commitments
	var reExecuteTxs types.Transactions
	var conflictingTxHashes []common.Hash
	for _, commitment := range commitments[firstConflictIndex:] {
		if commitment.IsResolved() {
			reExecuteTxs = append(reExecuteTxs, commitment.GetTxs()...)
			continue
		}
		conflictingTxHashes = append(conflictingTxHashes, commitment.Txs...)
	}
	if len(conflictingTxHashes) > 0 {
		txs, notFound := t.pool.GetTxs(conflictingTxHashes)
		if len(notFound) > 0 {
			return nil, fmt.Errorf("access lists are not compatible and %d based txs not found", len(notFound))
		}
		reExecuteTxs = append(reExecuteTxs, txs...)
	}

	// Combine effects from non-conflicting commitments
	var basedTxsSize uint64
	var basedGasUsed uint64
	var basedMaxGasLimit uint64
	var basedOnTxs []common.Hash
	var basedOnLogs []*types.Log
	stateDiffs := make([]*types.StateDiff, 0, firstConflictIndex)
	for _, commitment := range commitments[:firstConflictIndex] {
		basedOnTxs = append(basedOnTxs, commitment.Txs...)
		basedOnLogs = append(basedOnLogs, commitment.Logs...)
		basedTxsSize += commitment.TxsSize
		basedGasUsed += commitment.GasUsed
		basedMaxGasLimit = max(basedMaxGasLimit, commitment.MaxGasLimit)
		stateDiffs = append(stateDiffs, commitment.StateDiff)
	}

	if basedTxsSize > sizeLeft {
		return nil, errors.New("commitments size is too big")
	}
	if basedGasUsed > gasLimit {
		return nil, errors.New("commitments gas is too big")
	}

	stateDiff, err := types.MergeStateDiffs(stateDiffs...)
	if err != nil {
		return nil, fmt.Errorf("stateDiffs are not compatible, reason: %s", err)
	}

	bundle.SetBasedTxs(basedOnTxs)
	bundle.SetBasedLogs(basedOnLogs)
	bundle.SetBasedTxsSize(basedGasUsed)
	bundle.SetBasedGasUsed(basedGasUsed)
	bundle.SetBasedMaxGasLimit(basedMaxGasLimit)
	bundle.SetBasedAccesses(accesses)
	bundle.SetBasedStateDiff(stateDiff)
	bundle.SetExecutingTxs(slices.Concat(reExecuteTxs, bundle.NewTxs))

	commitment, err := t.blockChain.CreateCommitment(bundle)
	if err != nil {
		return nil, err
	}

	commitment.NewTxs = bundle.NewTxs
	_, err = t.pool.AddCommitment(commitment, false)
	if err != nil {
		return nil, err
	}

	// first propagation will be with NewTxs
	t.sendCommitmentFeed(commitment)
	commitmentLocalCounter.Inc(1)
	commitmentOwnTxsCounter.Inc(int64(len(commitment.NewTxs)))

	return commitment, err
}

// HandleSubSlot processes the given SubSlot. It verifies that the SubSlot is
// valid and applies it to the current state. If the SubSlot is accepted, it
// updates the state of the Pool accordingly.
//
// Returns whether the SubSlot was processed successfully.
func (t *Tool) HandleSubSlot(newSubSlot *types.SubSlot) (processed bool, err error) {
	if !newSubSlot.IsResolved() {
		t.log.Error("SubSlot transactions not resolved yet",
			"parentBlock", newSubSlot.BlockParent, "subSlot", newSubSlot.Index, "hash", newSubSlot.ID())
		return false, errTxNotResolved
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if newSubSlot.BlockParent != t.headBlock {
		t.log.Warn("Received parent block mismatch", "headBlock", t.headBlock, "parentBlock", newSubSlot.BlockParent)
		return false, ErrNotConsistent
	}

	// make sure the newSubSlot is not from the past
	if t.nextSubSlot > newSubSlot.Index {
		return false, nil
	}

	// in case we missed some before the newSubSlot, let's cache it
	if newSubSlot.Index > t.nextSubSlot {
		t.addSubSlot(newSubSlot)
		t.log.Warn("Received ahead-of-the-curve subSlot, added and waiting for the missed ones",
			"expectedParentBlock", t.headBlock, "expectedSubSlot", t.nextSubSlot,
			"receivedParentBlock", newSubSlot.BlockParent, "receivedSubSlot", newSubSlot.Index)
		return false, nil
	}

	// try to execute all pending subSlots
	for i, subSlot := range t.pendingSubSlots() {
		// if some missed in cache order, skip them for next time
		if newSubSlot.Index+uint8(i) != subSlot.Index || !subSlot.IsResolved() {
			return i != 0, nil // false - if skip new/first one
		}

		logger := t.log.With("parentBlock", subSlot.BlockParent, "id", subSlot.ID(), "index", subSlot.Index)

		startTime := time.Now()
		if err := t.blockChain.ProcessSubSlot(subSlot); err != nil {
			logger.Error("Failed to process subSlot", "txs", subSlot.Txs, "error", err)
			t.setInvalid()
			return false, err
		}

		elapsed := time.Since(startTime)
		mgasps := float64(subSlot.GasUsed) * 1_000 / float64(elapsed)
		logger.Info("Imported new TOOL subSlot",
			"txs", len(subSlot.Txs),
			"mgas", float64(subSlot.GasUsed)/1_000_000,
			"elapsed", common.PrettyDuration(elapsed),
			"mgasps", mgasps,
			"ops", subSlot.Accesses.OpsCount(),
		)

		t.updateSubSlot(subSlot)

		all, claimed := t.pool.GetTxsCount()
		subSlotTxsUnclaimedGauge.Update(int64(all - claimed))
		subSlotMgaspsGauge.Update(int64(mgasps))
		subSlotNextGauge.Update(int64(subSlot.Index + 1))
		subSlotTxsGauge.Update(int64(len(subSlot.Txs)))
		totalAcceptedTxs.Inc(int64(len(subSlot.Txs)))
	}

	return true, nil
}

func (t *Tool) addSubSlot(subSlot *types.SubSlot) (ok bool) {
	if t.headBlock != subSlot.BlockParent {
		return false
	}

	if t.nextTime != subSlot.BlockTime {
		return false
	}

	if t.nextSubSlot > subSlot.Index {
		return false
	}

	subSlotID := subSlot.ID()
	for i, known := range t.knownSubSlots {
		if known.ID() == subSlotID {
			t.knownSubSlots[i] = subSlot
			return true
		}
	}

	t.knownSubSlots = append(t.knownSubSlots, subSlot)
	sort.Slice(t.knownSubSlots, func(i, j int) bool {
		return t.knownSubSlots[i].Index < t.knownSubSlots[j].Index
	})

	return true
}

func (t *Tool) KnownSubSlots() []*types.SubSlot {
	t.mu.Lock()
	defer t.mu.Unlock()

	subSlots := make([]*types.SubSlot, 0, len(t.knownSubSlots))
	for i, subSlot := range t.knownSubSlots {
		if uint8(i) != subSlot.Index || !subSlot.IsResolved() {
			return subSlots
		}
		subSlots = append(subSlots, subSlot)
	}

	return subSlots
}

func (t *Tool) pendingSubSlots() []*types.SubSlot {
	return t.knownSubSlots[t.nextSubSlot:]
}

func (t *Tool) RegisterSubSlotIfNew(subSlot *types.SubSlot) (unknown []common.Hash, old bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.headBlock != subSlot.BlockParent {
		return nil, true
	}

	if t.nextTime != subSlot.BlockTime {
		return nil, true
	}

	if t.nextSubSlot > subSlot.Index {
		return nil, true
	}

	for _, known := range t.knownSubSlots {
		if known.Index == subSlot.Index {
			return nil, known.ID() == subSlot.ID()
		}
	}

	t.knownSubSlots = append(t.knownSubSlots, subSlot)
	sort.Slice(t.knownSubSlots, func(i, j int) bool {
		return t.knownSubSlots[i].Index < t.knownSubSlots[j].Index
	})

	return t.pool.loadSubSlotTxs(subSlot), false
}

type HeaderParams struct {
	Timestamp  uint64
	Parent     common.Hash
	Random     common.Hash
	BeaconRoot *common.Hash
	GasLimit   uint64
}

func (t *Tool) SetupNextHeaderParams(params *HeaderParams, reserveSize uint64, preConfirmed bool, settlementData *SettlementData) (*Settlement, error) {
	if !t.isBCSynced() {
		return nil, errors.New("chain not synced yet")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	settlement, newNextHeader, err := t.blockChain.SetupNextHeader(params, reserveSize, settlementData)
	if err != nil {
		t.setInvalid()
		return nil, err
	}

	if newNextHeader != nil {
		t.updateNext(newNextHeader, preConfirmed)
	}

	if newNextHeader == nil || !t.Config.Leader || t.isInvalid() {
		return settlement, nil
	}

	if t.leaderTicker != nil {
		t.leaderTicker.Reset(time.Second)
	}

	return settlement, nil
}

func (t *Tool) LeaderTick() (ch <-chan time.Time) {
	t.mu.RLock()
	if t.leaderTicker != nil {
		ch = t.leaderTicker.C
	}
	t.mu.RUnlock()

	return ch
}

func (t *Tool) updateSubSlot(subSlot *types.SubSlot) {
	t.nextSubSlot = subSlot.Index + 1
	t.pool.AcceptSubSlot(subSlot)
	t.sendSubSlotFeed(subSlot)
	return
}

func (t *Tool) updateNext(next *types.Header, preConf bool) {
	if next == nil {
		t.setInvalid()
		return
	}

	subSlotTxsUnclaimedGauge.Update(0)
	subSlotNextGauge.Update(0)
	subSlotTxsGauge.Update(0)

	t.preConf = preConf
	t.nextTime = next.Time
	t.headBlock = next.ParentHash
	t.nextBlock = next.Number.Uint64()
	t.nextSubSlot = 0
	t.knownSubSlots = make([]*types.SubSlot, 0, subSlotsCount)

	t.stopLeaderTicker()
	select {
	case <-t.ready:
	default:
		close(t.ready)
	}

	t.pool.Reset() // todo: keep some cache?
	t.nextHeaderFeed.Send(next)
}

func (t *Tool) setInvalid() {
	t.nextBlock = math.MaxUint64
	t.stopLeaderTicker()
}

func (t *Tool) stopLeaderTicker() {
	if t.leaderTicker == nil {
		t.leaderTicker = time.NewTicker(time.Second)
	}
	t.leaderTicker.Stop()
}

func (t *Tool) isInvalid() bool {
	return t.nextBlock == math.MaxUint64
}

func (t *Tool) Close() {
	close(t.stop)
	t.wg.Wait()
	t.close()
}

func (t *Tool) close() {
	t.blockChain.Close()
}

func (t *Tool) leaderLoop() {
	defer t.wg.Done()

	select {
	case <-t.ready:
	case <-t.stop:
		return
	}

	for {
		select {
		case <-t.stop:
			return
		case <-t.LeaderTick(): // we don't really need mutex, single leader case
			sizeLeft, gasLimit, totalGasLeft, err := t.blockChain.NextSubSlotLimits()
			if err != nil {
				t.setInvalid()
				t.log.Warn("Skip new subSlot assembly", "headBlock", t.headBlock,
					"block", t.nextBlock, "index", t.nextSubSlot, "reason", err)
				continue
			}
			if gasLimit < params.TxGas {
				t.setInvalid()
				t.log.Warn("Skip new subSlot assembly", "headBlock", t.headBlock,
					"block", t.nextBlock, "index", t.nextSubSlot, "reason", "gas limit exceeded")
				continue
			}
			newSubSlot, err := t.assembler.newSubSlot(sizeLeft, gasLimit, totalGasLeft, t.GetPendingCommitments(), t.headBlock, t.nextSubSlot, t.nextTime)
			if err != nil {
				t.setInvalid()
				t.log.Error("Failed to assemble new subSlot", "headBlock", t.headBlock,
					"block", t.nextBlock, "index", t.nextSubSlot, "reason", err.Error())
				continue
			}
			unknownTxs, old := t.RegisterSubSlotIfNew(newSubSlot)
			if old {
				// t.setInvalid()
				t.log.Warn("Failed to register new subSlot", "headBlock", t.headBlock,
					"parent", newSubSlot.BlockParent, "index", newSubSlot.Index, "reason", "too late")
				continue
			}
			if len(unknownTxs) != 0 || !newSubSlot.IsResolved() {
				// t.setInvalid()
				t.log.Error("Failed to resolve new subSlot", "headBlock", t.headBlock,
					"parent", newSubSlot.BlockParent, "index", newSubSlot.Index, "unknownTxs", len(unknownTxs))
				continue
			}
			if processed, _ := t.HandleSubSlot(newSubSlot); !processed {
				// t.setInvalid()
				continue
			}
		}
	}
}
