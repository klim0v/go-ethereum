package tool

import (
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Pool manages the storage and lifecycle of transactions, Commitments, and
// SubSlots. It provides methods to add, retrieve, and query these entities,
// as well as to handle the acceptance of SubSlots and the expiration of
// Commitments.
//
// The Pool keeps track of pending and expired Commitments, as well as
// transactions that have been accepted in SubSlots. It also provides methods
// to check for conflicts between Commitments and SubSlots.
//
// When a SubSlot is accepted, the Pool updates its state by marking transactions
// as accepted and expiring affected Commitments.
type Pool struct {
	mu                  sync.RWMutex
	knownTransactions   map[common.Hash]*types.Transaction // knownTransactions contains transactions from commitments
	acceptedTxReceipts  map[common.Hash]*types.Receipt     // acceptedTxReceipts contains receipts for finalized transactions
	pendingCommitments  map[common.Hash]*types.Commitment  // pendingCommitments contains Commitments waiting to be included in a subSlot
	conflictCommitments map[common.Hash]*types.Commitment  // conflictCommitments contains Commitments that have conflicted or expired

	receiptBlockCache sync.Map
}

func newPool() *Pool {
	pool := &Pool{
		knownTransactions:   make(map[common.Hash]*types.Transaction),
		acceptedTxReceipts:  make(map[common.Hash]*types.Receipt),
		pendingCommitments:  make(map[common.Hash]*types.Commitment),
		conflictCommitments: make(map[common.Hash]*types.Commitment),
	}
	return pool
}

func (p *Pool) AcceptedTx(tx common.Hash) (*types.Receipt, *types.Transaction) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	receipt, ok := p.acceptedTxReceipts[tx]
	if !ok {
		return nil, nil
	}

	return receipt, p.knownTransactions[tx]
}

func (p *Pool) GetTx(hash common.Hash) (tx *types.Transaction) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.knownTransactions[hash]
}

func (p *Pool) GetTxs(hashes []common.Hash) (txs types.Transactions, unknown []common.Hash) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txs = make(types.Transactions, 0, len(hashes))
	for _, hash := range hashes {
		tx := p.knownTransactions[hash]
		if tx == nil {
			unknown = append(unknown, hash)
			continue
		}
		txs = append(txs, tx)
	}

	return txs, unknown
}

func (p *Pool) AddTxs(txs []*types.Transaction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, newTx := range txs {
		tx, ok := p.knownTransactions[newTx.Hash()]
		if !ok || tx != nil {
			continue // not requested or filled
		}
		p.knownTransactions[newTx.Hash()] = newTx
	}

	return
}

// AddCommitment adds a Commitment to the pending Commitments pool. It verifies
// that the Commitment is valid and doesn't conflict with any accepted
// transactions.
//
// Returns a list of transaction hashes that are missing from the pool, or
// an error if the Commitment could not be added.
func (p *Pool) AddCommitment(c *types.Commitment, dirty bool) (unknownTxs []common.Hash, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmtID := c.ID()
	if _, known := p.pendingCommitments[cmtID]; known {
		return nil, errors.New("commitment known")
	}

	if _, expired := p.conflictCommitments[cmtID]; expired {
		return nil, errors.New("commitment expired")
	}

	if p.isAccepted(c.Txs) {
		return nil, errors.New("commitment txs expired")
	}

	if !dirty {
		p.pendingCommitments[cmtID] = c
	} else {
		p.conflictCommitments[cmtID] = c
	}

	return p.markPendingTxs(c.Txs, c.NewTxs), nil
}

// AcceptSubSlot marks a SubSlot as accepted. It updates the state of the Pool
// by marking transactions in the SubSlot as accepted and expiring affected
// Commitments.
func (p *Pool) AcceptSubSlot(subSlot *types.SubSlot) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, receipt := range subSlot.Receipts {
		p.acceptedTxReceipts[receipt.TxHash] = receipt
	}

	for _, c := range p.pendingCommitments {
		if c.ExpiresAtSubSlot <= subSlot.Index || p.isAccepted(c.Txs) {
			p.removePending([]common.Hash{c.ID()})
		}
	}

	p.removePending(p.findPendingConflicts(subSlot))
}

func (p *Pool) removePending(expired []common.Hash) {
	for _, hash := range expired {
		p.conflictCommitments[hash] = p.pendingCommitments[hash]
		delete(p.pendingCommitments, hash)
	}
}

func (p *Pool) findPendingConflicts(subSlot *types.SubSlot) []common.Hash {
	var conflicted []common.Hash
	for hash, commitment := range p.pendingCommitments {
		_, conflicts := types.HasAccessesConflicts(subSlot.Accesses, commitment.Accesses)
		if conflicts {
			conflicted = append(conflicted, hash)
		}
	}
	return conflicted
}

func (p *Pool) loadSubSlotTxs(subSlot *types.SubSlot) (unknown []common.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()

	known, unknown := p.checkPendingTxs(subSlot.Txs)
	if len(unknown) == 0 {
		subSlot.SetTxs(known)
	}

	return unknown
}

func (p *Pool) checkPendingTxs(hashes []common.Hash) (known types.Transactions, unknown []common.Hash) {
	for _, hash := range hashes {
		tx := p.knownTransactions[hash]
		if tx != nil {
			known = append(known, tx)
			continue
		}
		p.knownTransactions[hash] = nil
		unknown = append(unknown, hash)
	}
	return known, unknown
}

func (p *Pool) markPendingTxs(hashes []common.Hash, txs types.Transactions) []common.Hash {
	for _, tx := range txs {
		p.knownTransactions[tx.Hash()] = tx
	}

	if len(hashes) == txs.Len() {
		return nil
	}

	return p.markNewPending(hashes[:len(hashes)-txs.Len()])
}

func (p *Pool) markNewPending(hashes []common.Hash) []common.Hash {
	var news []common.Hash
	for _, hash := range hashes {
		_, ok := p.knownTransactions[hash]
		if ok {
			continue
		}
		p.knownTransactions[hash] = nil
		news = append(news, hash)
	}
	return news
}

func (p *Pool) isAccepted(txs []common.Hash) bool {
	for _, tx := range txs {
		if _, ok := p.acceptedTxReceipts[tx]; ok {
			return true
		}
	}
	return false
}

func (p *Pool) GetCommitment(hash common.Hash) (_ *types.Commitment, available, valid bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	commitment, ok := p.pendingCommitments[hash]
	if ok {
		return commitment, true, true
	}

	commitment, ok = p.conflictCommitments[hash]
	if !ok {
		return nil, false, false
	}

	for _, tx := range commitment.Txs {
		if _, accepted := p.acceptedTxReceipts[tx]; accepted {
			return commitment, false, false
		}
	}

	return commitment, true, false

}

func (p *Pool) GetPendingCommitments() []*types.Commitment {
	p.mu.RLock()
	defer p.mu.RUnlock()

	commitments := make([]*types.Commitment, 0, len(p.pendingCommitments))
	for _, commitment := range p.pendingCommitments {
		commitments = append(commitments, commitment)
	}

	return commitments
}

func (p *Pool) GetTxsCount() (all, accepted int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.knownTransactions), len(p.acceptedTxReceipts)
}

func (p *Pool) GetReceiptBlock(hash common.Hash) *types.Block {
	block, ok := p.receiptBlockCache.Load(hash)
	if !ok {
		return nil
	}
	return block.(*types.Block)
}

func (p *Pool) CacheReceiptBlock(block *types.Block) {
	p.receiptBlockCache.Store(block.Hash(), block)
}

func (p *Pool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	clear(p.knownTransactions)
	clear(p.acceptedTxReceipts)
	clear(p.pendingCommitments)
	clear(p.conflictCommitments)
	p.receiptBlockCache.Clear()
}
