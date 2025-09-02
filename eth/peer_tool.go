package eth

import (
	"github.com/ethereum/go-ethereum/eth/protocols/tool"
)

type toolPeerInfo struct {
	Version uint `json:"version"` // Ethereum protocol version negotiated
}

type toolPeer struct {
	*tool.Peer
}

func (p *toolPeer) info() *ethPeerInfo {
	info := &ethPeerInfo{Version: p.Version()}
	return info
}
