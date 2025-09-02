package eth

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/tool"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type toolHandler handler

func (h *toolHandler) RunPeer(peer *tool.Peer, hand tool.Handler) error {
	return (*handler)(h).runToolPeer(peer, hand)
}

func (h *toolHandler) GetTxs(hashes []common.Hash) []*types.Transaction {
	txs, _ := h.tool.Pool().GetTxs(hashes)
	return txs
}

func (h *toolHandler) AcceptData() bool {
	return h.synced.Load()
}

func (h *toolHandler) Chain() *core.BlockChain { return h.chain }

// PeerInfo retrieves all known `snap` information about a peer.
func (h *toolHandler) PeerInfo(id enode.ID) interface{} {
	if p := h.peers.peer(id.String()); p != nil {
		if p.snapExt != nil {
			return p.snapExt.info()
		}
	}
	return nil
}

func (h *toolHandler) Handle(peer *tool.Peer, packet tool.Packet) error {
	switch packet := packet.(type) {

	case *tool.ValidatorRegistrations:
		h.relay.ImportAndSendNextEpochValidators(*packet)
		return nil

	case *tool.ToolTransactionsResponse:
		h.tool.AddTxs(types.Transactions(*packet))
		return nil

	case *tool.SubSlotPacket:
		if h.tool.Config.Leader {
			peer.Log().Debug("Skip handling of received subSlots", "count", len(*(packet)), "reason", "leader mode")
			return nil
		}

		for _, subSlot := range *(packet) {
			unknownTx, old := h.tool.RegisterSubSlotIfNew(subSlot)
			if old {
				continue
			}

			if len(unknownTx) == 0 {
				h.tool.HandleSubSlot(subSlot)
				continue
			}

			go func(ss *types.SubSlot) {
				notFound := h.fetchToolTxs(ss.BlockTime, ss.Index, unknownTx)
				if len(notFound) != 0 {
					h.tool.Log().Error("Failed to resolve subSlot transactions",
						"id", ss.ID(), "parentBlock", ss.BlockParent, "subSlot", ss.Index, "notFound", len(notFound))
					return
				}
				txs, notFound := h.tool.GetTxs(ss.Txs)
				if len(notFound) != 0 {
					h.tool.Log().Error("SubSlot transaction disappeared from pool",
						"id", ss.ID(), "parentBlock", ss.BlockParent, "subSlot", ss.Index, "notFound", len(notFound))
					return
				}
				ss.SetTxs(txs)
				h.tool.HandleSubSlot(ss)
			}(subSlot)
		}

		return nil

	case *tool.CommitmentsPacket:

		for _, commitment := range *(packet) {

			unknownTx, added := h.tool.AddCommitment(commitment)
			if !added {
				return nil
			}

			go func(cmt *types.Commitment) {
				err := peer.RequestToolTxs(unknownTx)
				if err != nil {
					peer.Log().Error("Failed to request commitment transactions",
						"commitment", cmt.ID(), "count", len(unknownTx))
					return
				}
				peer.Log().Debug("Requested commitment transactions",
					"commitment", cmt.ID(), "count", len(unknownTx))
			}(commitment)
		}

		return nil

	default:
		return fmt.Errorf("unexpected eth packet type: %T", packet)
	}
}

func (h *toolHandler) fetchToolTxs(blockTime uint64, expires uint8, unknownTxHashes []common.Hash) (notFound []common.Hash) {
	now := time.Now()

	for _, p := range h.peers.toolPeersAll() { // todo: make smarter choices
		go func(peer *toolPeer) {
			err := peer.RequestToolTxs(unknownTxHashes)
			if err != nil {
				peer.Log().Error("Failed to request tool transactions", "count", len(unknownTxHashes), "error", err)
				return
			}
		}(p)
	}

	newHeaderCh := make(chan *types.Header, 1)
	newHeadSub := h.tool.SubscribeNextHeader(newHeaderCh)
	defer newHeadSub.Unsubscribe()

	newSubSlotCh := make(chan *types.SubSlot, 1)
	newSubSlotSub := h.tool.SubscribeSubSlots(newSubSlotCh)
	defer newSubSlotSub.Unsubscribe()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var foundNumber int
	for {
		select {
		case <-ticker.C:
			for i := foundNumber; i < len(unknownTxHashes); i++ {
				if tx := h.tool.GetTx(unknownTxHashes[i]); tx == nil {
					break
				}
				foundNumber = i + 1
			}

			if foundNumber != len(unknownTxHashes) {
				continue
			}

			h.tool.Log().Info("Successfully fetched tool txs", "elapsed", common.PrettyDuration(time.Since(now)))
			return nil

		case header := <-newHeaderCh:
			if header.Time > blockTime {
				return unknownTxHashes[foundNumber:]
			}

		case subSlot := <-newSubSlotCh:
			if subSlot.Index > expires {
				return unknownTxHashes[foundNumber:]
			}

		}
	}
}
