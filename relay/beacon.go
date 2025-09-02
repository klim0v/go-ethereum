package relay

import (
	"context"

	"github.com/attestantio/go-eth2-client/api"
	v1 "github.com/attestantio/go-eth2-client/api/v1"
	v2 "github.com/attestantio/go-eth2-client/api/v2"
	"github.com/attestantio/go-eth2-client/multi"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/rs/zerolog"
)

type Beacon struct {
	clients *multi.Service

	opts api.CommonOpts
}

func NewBeacon(ctx context.Context, endpoints []string) (*Beacon, error) {
	clients, err := multi.New(ctx,
		multi.WithAddresses(endpoints),
		multi.WithTimeout(durationPerEpoch),
		multi.WithLogLevel(zerolog.WarnLevel),
		multi.WithAllowDelayedStart(true),
		//multi.WithMonitor(Presenter{}),
	)
	if err != nil {
		return nil, err
	}

	return &Beacon{clients: clients.(*multi.Service), opts: api.CommonOpts{Timeout: durationPerSlot / 2}}, nil
}

func (b *Beacon) Validator(ctx context.Context, pubKeys []phase0.BLSPubKey) (map[phase0.ValidatorIndex]*v1.Validator, error) {
	validators, err := b.clients.Validators(ctx, &api.ValidatorsOpts{
		Common:  b.opts,
		State:   "head",
		PubKeys: pubKeys,
		ValidatorStates: []v1.ValidatorState{
			v1.ValidatorStatePendingInitialized,
			v1.ValidatorStatePendingQueued,
			v1.ValidatorStateActiveOngoing,
			v1.ValidatorStateActiveExiting,
		},
	})
	if err != nil {
		return nil, err
	}

	return validators.Data, nil
}

func (b *Beacon) ProposerDuties(ctx context.Context, epoch phase0.Epoch) (*api.Response[[]*v1.ProposerDuty], error) {
	duties, err := b.clients.ProposerDuties(ctx, &api.ProposerDutiesOpts{
		Common:  b.opts,
		Epoch:   epoch,
		Indices: nil,
	})
	if err != nil {
		return nil, err
	}

	return duties, nil
}

func (b *Beacon) SubmitProposal(ctx context.Context, block *api.VersionedSignedProposal) error {
	broadcastValidation := v2.BroadcastValidationConsensusAndEquivocation
	err := b.clients.SubmitProposal(ctx, &api.SubmitProposalOpts{
		Common:              b.opts,
		Proposal:            block,
		BroadcastValidation: &broadcastValidation,
	})
	if err != nil {
		return err
	}

	return nil
}

func (b *Beacon) Events(ctx context.Context) (<-chan *v1.HeadEvent, <-chan *v1.PayloadAttributesEvent, error) {
	headEvents := make(chan *v1.HeadEvent)
	payloadAttributesEvents := make(chan *v1.PayloadAttributesEvent)
	err := b.clients.Events(ctx, &api.EventsOpts{
		Common: b.opts,
		Topics: []string{"head", "payload_attributes"},
		HeadHandler: func(ctx context.Context, event *v1.HeadEvent) {
			headEvents <- event
		},
		PayloadAttributesHandler: func(ctx context.Context, event *v1.PayloadAttributesEvent) {
			payloadAttributesEvents <- event
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return headEvents, payloadAttributesEvents, nil
}

func (b *Beacon) NodeSyncing(ctx context.Context) (*api.Response[*v1.SyncState], error) {
	res, err := b.clients.NodeSyncing(ctx, &api.NodeSyncingOpts{Common: b.opts})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (b *Beacon) IsSynced() bool {
	return b.clients.IsSynced()
}
