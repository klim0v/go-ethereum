package eth

import (
	"errors"
	"fmt"

	v1 "github.com/attestantio/go-builder-client/api/v1"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/eth/protocols/snap"
	"github.com/ethereum/go-ethereum/eth/protocols/tool"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
)

func (h *handler) StartWithTOOL(maxPeers, teeMaxPeers int, teeOnly bool) {
	if h.tool.Enabled() {
		h.teeMaxPeers = teeMaxPeers
		h.teeOnly = teeOnly

		h.wg.Add(1)
		h.commitmentCh = make(chan *types.Commitment, 123)
		h.commitmentSub = h.tool.SubscribeCommitments(h.commitmentCh)
		go h.commitmentBroadcastLoop()

		h.wg.Add(1)
		h.subSlotCh = make(chan *types.SubSlot, 12)
		h.subSlotSub = h.tool.SubscribeSubSlots(h.subSlotCh)
		go h.subSlotBroadcastLoop()

		h.wg.Add(1)
		h.nextEpochValRegsCh = make(chan []*v1.ValidatorRegistration, 100)
		h.nextEpochValRegsSub = h.relay.SubscribeNextEpochValidators(h.nextEpochValRegsCh)
		go h.valRegBroadcastLoop()
	}

	h.Start(maxPeers)
}

func (h *handler) BroadcastCommitment(cmt *types.Commitment) {
	for _, peer := range h.peers.toolPeersWithoutCommitment(cmt.ID()) {
		go func(p *toolPeer) {
			if err := p.SendCommitments([]*types.Commitment{cmt}); err != nil {
				p.Log().Debug("Failed to send commitment",
					"parentBlock", cmt.BlockParent, "subSlot", cmt.SubSlot, "id", cmt.ID(), "error", err)
				return
			}
			p.Log().Trace("Successfully sent commitment", "parentBlock", cmt.BlockParent, "subSlot", cmt.SubSlot, "id", cmt.ID())
		}(peer)
	}
}

func (h *handler) BroadcastSubSlot(slot *types.SubSlot) {
	for _, peer := range h.peers.toolPeersWithoutSubSlot(slot.ID()) {
		go func(p *toolPeer) {
			if err := p.SendSubSlots([]*types.SubSlot{slot}); err != nil {
				p.Log().Warn("Failed to send subSlot",
					"parentBlock", slot.BlockParent, "index", slot.Index, "id", slot.ID(), "error", err)
				return
			}
			p.Log().Debug("Successfully sent subSlot", "parentBlock", slot.BlockParent, "index", slot.Index, "id", slot.ID())
		}(peer)
	}
}

func (h *handler) subSlotBroadcastLoop() {
	defer h.wg.Done()

	for {
		select {
		case subSlot := <-h.subSlotCh:
			h.BroadcastSubSlot(subSlot)
		case <-h.subSlotSub.Err():
			return
		}
	}
}

func (h *handler) commitmentBroadcastLoop() {
	defer h.wg.Done()

	for {
		select {
		case commitment := <-h.commitmentCh:
			h.BroadcastCommitment(commitment)
		case <-h.commitmentSub.Err():
			return
		}
	}
}

func (h *handler) BroadcastValidatorRegistrations(regs []*v1.ValidatorRegistration) {
	for _, peer := range h.peers.toolPeersAll() {
		go func(p *toolPeer) {
			if err := p.SendValidators(regs); err != nil {
				p.Log().Warn("Failed to send validator registration", "error", err)
				return
			}
			p.Log().Debug("Successfully sent validator registration", "count", len(regs))
		}(peer)
	}
}

func (h *handler) valRegBroadcastLoop() {
	defer h.wg.Done()

	for {
		select {
		case batch := <-h.nextEpochValRegsCh:
			h.BroadcastValidatorRegistrations(batch)
		case <-h.nextEpochValRegsSub.Err():
			return
		}
	}
}

func (h *handler) tryToMakeCommitment(txs types.Transactions) {
	for _, tx := range txs {
		cmt, err := h.tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
		if err != nil {
			h.tool.Log().Debug("Failed to create commitment from tx pool", "error", err)
			continue
		}

		h.tool.Log().Debug("Created commitment from tx pool", "id", cmt.ID, "tx", tx.Hash(), "gasUsed", cmt.GasUsed)
	}

	h.BroadcastTransactions(txs) // TODO
}

func (h *handler) runToolPeer(peer *tool.Peer, handler tool.Handler) error {
	if !h.incHandlers() {
		return p2p.DiscQuitting
	}
	defer h.decHandlers()

	if !peer.RunningCap(eth.ProtocolName, eth.ProtocolVersions) {
		return fmt.Errorf("peer connected on tool without compatible eth support: have %v", peer.Caps())
	}

	if h.synced.Load() {
		attested, err := peer.VerifyAttestation(h.chain, h.teeVerifier)
		if err != nil {
			peer.Log().Warn("TOOL verification failed", "err", err)
		}
		if !attested {
			return p2p.DiscSubprotocolError
		}
		peer.MarkVerified()
	} else if h.snapSync.Load() && !peer.RunningCap(snap.ProtocolName, snap.ProtocolVersions) {
		return p2p.DiscUselessPeer
	}

	// Ignore teeMaxPeers if this is a trusted peer
	if !peer.Peer.Info().Network.Trusted {
		if h.peers.toolLen() >= h.teeMaxPeers {
			return p2p.DiscTooManyPeers
		}
	}
	peer.Log().Debug("TOOL peer connected", "name", peer.Name())

	if err := h.peers.registerToolPeer(peer); err != nil {
		if metrics.Enabled() {
			if peer.Inbound() {
				tool.IngressRegistrationErrorMeter.Mark(1)
			} else {
				tool.EgressRegistrationErrorMeter.Mark(1)
			}
		}
		peer.Log().Debug("TOOL extension registration failed", "err", err)
		return err
	}
	defer h.unregisterToolPeer(peer.ID())

	if h.peers.toolPeer(peer.ID()) == nil {
		return errors.New("peer dropped during handling")
	}

	if peer.Verified() {
		h.syncCommitments(peer)
		h.syncSubSlots(peer)
	}

	return handler(peer)
}

// unregisterPeer removes a peer from tool peer set.
func (h *handler) unregisterToolPeer(id string) {
	// Create a custom logger to avoid printing the entire id
	var logger log.Logger
	if len(id) < 16 {
		// Tests use short IDs, don't choke on them
		logger = log.New("peer", id)
	} else {
		logger = log.New("peer", id[:8])
	}
	// Abort if the peer does not exist
	peer := h.peers.toolPeer(id)
	if peer == nil {
		logger.Warn("TOOL peer removal failed", "err", errPeerNotRegistered)
		return
	}
	// Remove the `eth` peer if it exists
	logger.Debug("Removing TOOL peer", "verified", peer.Verified())

	if err := h.peers.unregisterToolPeer(id); err != nil {
		logger.Error("TOOL peer removal failed", "err", err)
	}
}

func (h *handler) enforceTEEPolicy() {

	h.peers.lock.Lock()
	defer h.peers.lock.Unlock()

	var toolConnected int
	peersToDisconnect := make(map[string]*p2p.Peer)
	for id, peer := range h.peers.toolPeers {
		if peer.Verified() {
			continue
		}
		attested, err := peer.VerifyAttestation(h.chain, h.teeVerifier)
		if err != nil {
			peer.Log().Warn("TOOL reverification failed", "err", err)
		}
		if !attested {
			peersToDisconnect[id] = peer.Peer.Peer
			continue
		}
		peer.MarkVerified()
		toolConnected++
	}
	if h.teeOnly {
		for id, peer := range h.peers.peers {
			if !peer.RunningCap(tool.ProtocolName, tool.ProtocolVersions) {
				peersToDisconnect[id] = peer.Peer.Peer
			}
		}
	}
	for _, peer := range peersToDisconnect {
		peer.Disconnect(p2p.DiscUselessPeer)
	}
	log.Info("Successfully enforced TEE policy", "disconnected", len(peersToDisconnect), "toolConnected", toolConnected)
}

func (h *handler) maxVanillaPeers() int {
	return h.maxPeers - h.teeMaxPeers
}
