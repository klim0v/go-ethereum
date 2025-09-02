package tool

import (
	"errors"
	"math/rand"
	"sync/atomic"

	v1 "github.com/attestantio/go-builder-client/api/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
)

// Peer is a collection of relevant information we have about a `tool` peer.
type Peer struct {
	verified atomic.Bool

	id string // Unique ID for the peer, cached

	*p2p.Peer                   // The embedded P2P package peer
	rw        p2p.MsgReadWriter // Input/output streams for tool
	version   uint              // Protocol version negotiated

	logger log.Logger // Contextual logger with the peer id injected

	knownTxs  *knownCache
	knownCmts *knownCache
	knownSlts *knownCache
	knownVals *knownCache
}

// NewPeer creates a wrapper for a network connection and negotiated  protocol
// version.
func NewPeer(version uint, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	id := p.ID().String()
	return &Peer{
		id:      id,
		Peer:    p,
		rw:      rw,
		version: version,
		logger:  log.New("peer", id[:8]),

		knownTxs:  newKnownCache(65536),
		knownCmts: newKnownCache(4096),
		knownSlts: newKnownCache(32),
		knownVals: newKnownCache(256),
	}
}

// ID retrieves the peer's unique identifier.
func (p *Peer) ID() string {
	return p.id
}

// Version retrieves the peer's negotiated `tool` protocol version.
func (p *Peer) Version() uint {
	return p.version
}

// Log overrides the P2P logger with the higher level one containing only the id.
func (p *Peer) Log() log.Logger {
	return p.logger
}

func (p *Peer) Verified() bool {
	return p.verified.Load()
}

func (p *Peer) MarkVerified() {
	p.verified.Store(true)
}

func (p *Peer) KnownTransaction(hash common.Hash) bool {
	return p.knownTxs.Contains(hash)
}

func (p *Peer) KnownCommitment(hash common.Hash) bool {
	return p.knownCmts.Contains(hash)
}

func (p *Peer) KnownSubSlot(hash common.Hash) bool {
	return p.knownSlts.Contains(hash)
}

func (p *Peer) markTransaction(hash common.Hash) {
	p.knownTxs.Add(hash)
}

func (p *Peer) markCommitment(hash common.Hash) {
	p.knownCmts.Add(hash)
}

func (p *Peer) markSubSlot(hash common.Hash) {
	p.knownSlts.Add(hash)
}

func (p *Peer) markValidator(hash common.Hash) {
	p.knownVals.Add(hash)
}

func (p *Peer) RequestToolTxs(hashes []common.Hash) error {
	if !p.verified.Load() {
		return errors.New("peer not verified")
	}

	p.Log().Debug("Fetching batch of tool transactions", "count", len(hashes))
	id := rand.Uint64()

	requestTracker.Track(p.id, p.version, GetToolTransactionsMsg, ToolTransactionsMsg, id)
	return p2p.Send(p.rw, GetToolTransactionsMsg, &GetToolTransactionsPacket{
		RequestId:                  id,
		GetToolTransactionsRequest: hashes,
	})
}

func (p *Peer) ReplyToolTransactions(id uint64, txs []*types.Transaction) error {
	if !p.verified.Load() {
		return errors.New("peer not verified")
	}

	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}

	return p2p.Send(p.rw, ToolTransactionsMsg, &ToolTransactionsPacket{
		RequestId:                id,
		ToolTransactionsResponse: txs,
	})
}

func (p *Peer) SendCommitments(commitments []*types.Commitment) error {
	if !p.verified.Load() {
		return errors.New("peer not verified")
	}

	for _, commitment := range commitments {
		p.knownCmts.Add(commitment.ID())
		for _, tx := range commitment.NewTxs {
			p.knownTxs.Add(tx.Hash())
		}
	}

	return p2p.Send(p.rw, CommitmentsMsg, commitments)
}

func (p *Peer) SendSubSlots(subSlots []*types.SubSlot) error {
	if !p.verified.Load() {
		return errors.New("peer not verified")
	}

	for _, slot := range subSlots {
		p.knownSlts.Add(slot.ID())
	}

	return p2p.Send(p.rw, SubSlotsMsg, subSlots)
}

func (p *Peer) SendValidators(regs []*v1.ValidatorRegistration) error {
	if !p.verified.Load() {
		return errors.New("peer not verified")
	}

	data := make([]*v1.ValidatorRegistration, 0, len(regs))
	for _, reg := range regs {
		hash, err := reg.HashTreeRoot()
		if err != nil {
			return err
		}
		if p.knownVals.Contains(hash) {
			continue
		}
		p.knownTxs.Add(hash)
		data = append(data, reg)
	}
	if len(data) == 0 {
		return nil
	}

	return p2p.Send(p.rw, ValidatorRegsMsg, ValidatorRegistrations(data))
}
