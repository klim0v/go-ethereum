package eth

import (
	"github.com/ethereum/go-ethereum/eth/protocols/tool"
)

func (h *handler) syncCommitments(p *tool.Peer) {
	commitments := h.tool.GetPendingCommitments()
	if len(commitments) == 0 {
		return
	}
	err := p.SendCommitments(commitments)
	if err != nil {
		p.Log().Error("Failed to sync pending commitments", "count", len(commitments), "error", err)
		return
	}
	p.Log().Info("Synced pending commitments", "count", len(commitments))
}

func (h *handler) syncSubSlots(p *tool.Peer) {
	subSlots := h.tool.KnownSubSlots()
	if len(subSlots) == 0 {
		return
	}
	err := p.SendSubSlots(subSlots)
	if err != nil {
		p.Log().Error("Failed to sync known subSlots", "count", len(subSlots), "error", err)
		return
	}
	p.Log().Info("Synced known subSlots", "count", len(subSlots))
}
