package eth

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// ToolAPI provides an API to interact with the tool subsystem, allowing clients
// to submit bundles, retrieve commitments, and subscribe to relevant events.
type ToolAPI struct {
	eth *Ethereum
}

// NewToolAPI creates a new instance of the ToolAPI with a reference to the Ethereum service.
func NewToolAPI(eth *Ethereum) *ToolAPI {
	return &ToolAPI{eth: eth}
}

// SendBundle processes a transaction bundle and creates a new commitment.
// The bundle must contain either new transactions, based commitments, or both.
// Returns the created commitment or an error if processing fails.
// Autoajustment is disabled, meaning bundle's block/subslot parameters must be valid.
func (api *ToolAPI) SendBundle(bundle *types.Bundle) (*types.Commitment, error) {
	if bundle.NewTxs.Len()+len(bundle.BasedCommitments) == 0 {
		return nil, errors.New("empty transactions list")
	}

	commitment, err := api.eth.tool.ProcessBundle(bundle, false)
	if err != nil {
		return nil, err
	}

	return commitment, nil
}

// GetPoolCommitment retrieves a commitment from the pool by its hash.
// Returns the commitment and a boolean indicating whether it's still pending.
// If the commitment is not found, both return values will be nil/false.
func (api *ToolAPI) GetPoolCommitment(hash common.Hash) (commitment *types.Commitment, dirty bool, err error) {
	commitment, _, dirty = api.eth.tool.GetCommitment(hash)
	if commitment == nil {
		return nil, false, errors.New("not found")
	}

	return commitment, dirty, nil
}

// GetCommitmentTransactions retrieves the transaction messages associated with a commitment.
// Returns the list of transaction messages, a boolean indicating whether the commitment
// is still pending, and an error if the commitment is not found or if retrieving the
// transaction messages fails.
func (api *ToolAPI) GetCommitmentTransactions(hash common.Hash) (txs []*core.Message, err error) {
	commitment, _, _ := api.eth.tool.GetCommitment(hash)
	if commitment == nil {
		return nil, errors.New("commitment not found")
	}
	massages, err := api.eth.tool.GetTxMassages(commitment.Txs)
	if err != nil {
		return nil, err
	}
	return massages, nil
}

// Commitments creates a subscription for new commitments.
// If addresses are provided, only commitments affecting the specified addresses will be sent.
// Returns a subscription that can be used to receive commitment events or an error if
// notifications are not supported by the client.
func (api *ToolAPI) Commitments(ctx context.Context, addresses map[common.Address]bool) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	commitments := make(chan *types.Commitment, 1)
	sub, err := api.eth.tool.APISubscribeCommitments(commitments)
	if err != nil {
		return nil, err
	}

	go func() {
		defer sub.Unsubscribe()

		for {
			select {
			case commitment := <-commitments:

				if len(addresses) != 0 {
					var interested bool
					for _, change := range commitment.StateDiff.Changes {
						if addresses[change.Address] {
							interested = true
							break
						}
					}
					if !interested {
						continue
					}
				}

				if err := notifier.Notify(rpcSub.ID, commitment); err != nil {
					log.Error("Failed to notify commitment", "err", err)
				}

			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// SubSlots creates a subscription for new subslots.
// Returns a subscription that can be used to receive subslot events or an error if
// notifications are not supported by the client.
func (api *ToolAPI) SubSlots(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	subSlots := make(chan *types.SubSlot, 1)
	sub, err := api.eth.tool.APISubscribeSubSlots(subSlots)
	if err != nil {
		return nil, err
	}

	go func() {
		defer sub.Unsubscribe()

		for {
			select {
			case subSlot := <-subSlots:
				if err := notifier.Notify(rpcSub.ID, subSlot); err != nil {
					log.Error("Failed to notify subSlot", "err", err)
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

func (api *ToolAPI) NextHeaders(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		nextHeaders := make(chan *types.Header, 1)
		sub := api.eth.tool.SubscribeNextHeader(nextHeaders)
		defer sub.Unsubscribe()

		for {
			select {
			case nextHeader := <-nextHeaders:
				if err := notifier.Notify(rpcSub.ID, nextHeader); err != nil {
					log.Error("Failed to notify next header", "err", err)
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// VerifyPeerNodeIDAttestation verifies the TEE attestation for the specified node IDs.
// If no IDs are provided, the current node's ID is used.
// Returns a map of node IDs to attestation status (true if attested, false otherwise)
// or an error if verification fails.
func (api *ToolAPI) VerifyPeerNodeIDAttestation(ids []common.Hash) (map[common.Hash]bool, error) {
	if len(ids) == 0 {
		ids = append(ids, common.Hash(api.eth.p2pServer.Self().ID()))
	}
	result := make(map[common.Hash]bool, len(ids))
	for _, id := range ids {
		attested, err := api.eth.handler.teeVerifier.Verify(api.eth.blockchain, id)
		if err != nil {
			return nil, err
		}
		result[id] = attested
	}
	return result, nil
}

// GetAttestedPeers returns a list of attested peer IDs.
// These are peers that have successfully completed the attestation process
// and are trusted for tool operations.
func (api *ToolAPI) GetAttestedPeers() (ids []string) {
	for _, peer := range api.eth.handler.peers.toolPeersAll() {
		ids = append(ids, peer.ID())
	}
	return ids
}
