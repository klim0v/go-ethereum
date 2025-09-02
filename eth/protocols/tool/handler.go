package tool

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

type Handler func(peer *Peer) error
type Decoder interface {
	Decode(val interface{}) error
}

type msgHandler func(backend Backend, msg Decoder, peer *Peer) error
type Backend interface {
	Chain() *core.BlockChain
	PeerInfo(id enode.ID) interface{}
	RunPeer(peer *Peer, handler Handler) error
	GetTxs(hashes []common.Hash) []*types.Transaction
	AcceptData() bool
	Handle(peer *Peer, packet Packet) error
}

var handlers1 = map[uint64]msgHandler{
	CommitmentsMsg:         handleCommitments,
	SubSlotsMsg:            handleSubSlots,
	GetToolTransactionsMsg: handleGetToolTransactions,
	ToolTransactionsMsg:    handleToolTransactions,
	ValidatorRegsMsg:       handleValidatorRegs,
}

func MakeProtocols(backend Backend) []p2p.Protocol {
	protocols := make([]p2p.Protocol, 0, len(ProtocolVersions))
	for _, version := range ProtocolVersions {
		protocols = append(protocols, p2p.Protocol{
			Name:    ProtocolName,
			Version: version,
			Length:  protocolLengths[version],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				return backend.RunPeer(NewPeer(version, p, rw), func(peer *Peer) error {
					return Handle(backend, peer)
				})
			},
			NodeInfo: func() interface{} {
				return nodeInfo(backend.Chain())
			},
			PeerInfo: func(id enode.ID) interface{} {
				return backend.PeerInfo(id)
			},
			Attributes: []enr.Entry{&enrEntry{}},
		})
	}
	return protocols
}

func Handle(backend Backend, peer *Peer) error {
	for {
		if err := handleMessage(backend, peer); err != nil {
			peer.Log().Debug("Message handling failed in `eth`", "err", err)
			return err
		}
	}
}

func handleMessage(backend Backend, peer *Peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := peer.rw.ReadMsg()
	if err != nil {
		return err
	}
	//if msg.Size > maxMessageSize {
	//	return fmt.Errorf("%w: %v > %v", errMsgTooLarge, msg.Size, maxMessageSize)
	//}
	defer msg.Discard()

	if !peer.Verified() {
		return nil
	}

	var handlers map[uint64]msgHandler
	switch peer.version {
	case TOOL1:
		handlers = handlers1
	default:
		return fmt.Errorf("unknown tool protocol version: %v", peer.version)
	}

	// Track the amount of time it takes to serve the request and run the handler
	if metrics.Enabled() {
		h := fmt.Sprintf("%s/%s/%d/%#02x", p2p.HandleHistName, ProtocolName, peer.Version(), msg.Code)
		defer func(start time.Time) {
			sampler := func() metrics.Sample {
				return metrics.ResettingSample(
					metrics.NewExpDecaySample(1028, 0.015),
				)
			}
			metrics.GetOrRegisterHistogramLazy(h, nil, sampler).Update(time.Since(start).Microseconds())
		}(time.Now())
	}
	if handler := handlers[msg.Code]; handler != nil {
		return handler(backend, msg, peer)
	}
	return fmt.Errorf("%w: %v", errInvalidMsgCode, msg.Code)
}

// NodeInfo represents a short summary of the `snap` sub-protocol metadata
// known about the host peer.
type NodeInfo struct{}

// nodeInfo retrieves some `snap` protocol metadata about the running host node.
func nodeInfo(chain *core.BlockChain) *NodeInfo {
	return &NodeInfo{}
}
