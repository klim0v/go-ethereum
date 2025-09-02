package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/protocols/tool"
)

func (ps *peerSet) registerToolPeer(peer *tool.Peer) error {
	// Ensure nobody can double connect
	ps.lock.Lock()
	defer ps.lock.Unlock()

	id := peer.ID()
	if _, ok := ps.toolPeers[id]; ok {
		return errPeerAlreadyRegistered
	}

	ps.toolPeers[id] = &toolPeer{peer}
	return nil
}

func (ps *peerSet) unregisterToolPeer(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	_, ok := ps.toolPeers[id]
	if !ok {
		return errPeerNotRegistered
	}
	delete(ps.toolPeers, id)
	return nil
}

func (ps *peerSet) toolPeer(id string) *toolPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.toolPeers[id]
}

// snapLen returns if the current number of `tool` peers in the set.
func (ps *peerSet) toolLen() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.toolPeers)
}

func (ps *peerSet) toolPeersWithoutTransactions(hash common.Hash) []*toolPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var list []*toolPeer
	for _, p := range ps.toolPeers {
		if p.Verified() && !p.KnownTransaction(hash) {
			list = append(list, p)
		}
	}
	return list
}

func (ps *peerSet) toolPeersWithoutCommitment(hash common.Hash) []*toolPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var list []*toolPeer
	for _, p := range ps.toolPeers {
		if p.Verified() && !p.KnownCommitment(hash) {
			list = append(list, p)
		}
	}
	return list
}

func (ps *peerSet) toolPeersWithoutSubSlot(hash common.Hash) []*toolPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var list []*toolPeer
	for _, p := range ps.toolPeers {
		if p.Verified() && !p.KnownSubSlot(hash) {
			list = append(list, p)
		}
	}
	return list
}

func (ps *peerSet) toolPeersAll() []*toolPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var list []*toolPeer
	for _, p := range ps.toolPeers {
		if p.Verified() {
			list = append(list, p)
		}
	}
	return list
}
