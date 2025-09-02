package relay

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/attestantio/go-builder-client/api"
	deneb2 "github.com/attestantio/go-builder-client/api/deneb"
	"github.com/attestantio/go-builder-client/api/electra"
	v1 "github.com/attestantio/go-builder-client/api/v1"
	"github.com/attestantio/go-builder-client/spec"
	beaconV1 "github.com/attestantio/go-eth2-client/api/v1"
	beaconSpec "github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/params"
	"github.com/flashbots/go-boost-utils/ssz"
	"github.com/holiman/uint256"
)

// slotPos returns the slot's position in the epoch (1-based, i.e. 1..32)
func slotPos(slot phase0.Slot) uint64 {
	return uint64((slot % slotsPerEpoch) + 1)
}

// startWork initializes and runs the relay's main event processing loop.
// It subscribes to headEvents and payloadAttributesEvents from the beacon node.
//
// On receiving a headEvent, the function updates the relay's understanding of the
// current chain head. It then calls updateNextSlotDuties to fetch proposer duties
// for the next epoch. This is a critical step for maintaining correct duty
// assignments, especially during chain reorganizations. As per the documentation,
// this involves a crucial check:
//
//	"Duties should only need to be checked once per epoch, however a chain
//	 reorganization could occur that results in a change of duties. For full safety,
//	 you should monitor head events and confirm the dependent root in this response matches:
//
//	 event.current_duty_dependent_root when compute_epoch_at_slot(event.slot) == epoch
//	 event.block otherwise"
//
// The updateNextSlotDuties function implements this verification.
//
// On receiving a payloadAttributesEvent, startWork validates the attributes,
// ensures the proposer matches the known duties, triggers block construction
// using the buildPayload function, and then proceeds to create and submit a builder bid.
func (r *Relay) startWork() error {
	headEvents, payloadAttributesEvents, err := r.beacon.Events(r.ctx)
	if err != nil {
		return err
	}

	var head *beaconV1.HeadEvent
	for head == nil {
		select {
		case <-r.ctx.Done():
			return nil
		case _, ok := <-payloadAttributesEvents:
			if !ok {
				return errors.New("payload attribute events channel closed")
			}
			continue
		case headEvent, ok := <-headEvents:
			if !ok {
				return errors.New("head events channel closed")
			}
			r.updateNextSlotDuties(r.ctx, headEvent) // ignore error
			head = headEvent
		}
	}

	type PendingEnvelope struct {
		attributes *beaconV1.PayloadAttributesData
		events     chan *miner.EnvelopeElapsed
		interrupt  chan struct{}
	}

	go func() {
		pendingEnvelope := PendingEnvelope{interrupt: make(chan struct{})}

		var syncedNextDutiesEpoch bool
		for {
			select {
			case <-r.ctx.Done():
				close(pendingEnvelope.interrupt)
				return

			case headEvent := <-headEvents:
				if headEvent.Slot <= head.Slot {
					continue // skip handled head (beacon multi-instance)
				}

				softEpochTransition := headEvent.EpochTransition && head.Block == headEvent.CurrentDutyDependentRoot
				if !syncedNextDutiesEpoch {
					if ok := r.preloadNextEpochDuties(r.ctx, headEvent.Slot); ok {
						syncedNextDutiesEpoch = true
					}
				} else if softEpochTransition {
					syncedNextDutiesEpoch = false
				}

				if headEvent.CurrentDutyDependentRoot != head.CurrentDutyDependentRoot && !softEpochTransition ||
					slotPos(headEvent.Slot+1) == 1 { // last slot of the current epoch

					syncedNextDutiesEpoch = false
					if ok := r.updateNextSlotDuties(r.ctx, headEvent); !ok {
						continue
					}
				}
				head = headEvent

			case attributesEvent := <-payloadAttributesEvents:
				logger := r.log.With("slot", attributesEvent.Data.ProposalSlot,
					"parentHash", common.Hash(attributesEvent.Data.ParentBlockHash),
				)

				switch attributesEvent.Version {
				case beaconSpec.DataVersionElectra:
				default:
					logger.Warn("Unsupported payload attributes version", "version", attributesEvent.Version)
					continue
				}

				logger = logger.With("timestamp", attributesEvent.Data.V4.Timestamp,
					"parentRoot", common.Hash(attributesEvent.Data.ParentBlockRoot),
					"parentV4Root", common.Hash(attributesEvent.Data.V4.ParentBeaconBlockRoot),
				)

				if pendingEnvelope.attributes == nil {
					pendingEnvelope.attributes = attributesEvent.Data
					logger.Debug("Initialized first payload attributes")
					continue
				}

				if attributesEvent.Data.ProposalSlot < pendingEnvelope.attributes.ProposalSlot {
					logger.Warn("Skipping older proposal slot")
					continue
				}

				isSameParent := attributesEvent.Data.ParentBlockHash == pendingEnvelope.attributes.ParentBlockHash
				if isSameParent {
					if attributesEvent.Data.ProposalSlot == pendingEnvelope.attributes.ProposalSlot {
						logger.Warn("Skipping duplicate proposal")
						continue
					}
				}

				close(pendingEnvelope.interrupt)
				pendingEnvelope.interrupt = make(chan struct{})
				pendingEnvelope.attributes = attributesEvent.Data
				pendingEnvelope.events = make(chan *miner.EnvelopeElapsed)

				duty, ok := r.slotDuties.Get(pendingEnvelope.attributes.ProposalSlot)
				if !ok {
					logger.Error("Slot proposer duty not found")
					continue
				}
				if duty.ValidatorIndex != pendingEnvelope.attributes.ProposerIndex {
					logger.Error("Slot proposer duty not match with payload attributes")
					continue
				}

				logger = logger.With("validator", duty.ValidatorIndex)

				go func(pendingEnvelope PendingEnvelope, wait bool) {
					var waitCh chan struct{}
					if wait {
						waitCh = pendingEnvelope.interrupt
					}
					var gasLimit uint64
					feeRecipient := common.Address(pendingEnvelope.attributes.V4.SuggestedFeeRecipient)
					validator, registered := r.getValidator(duty.PubKey, waitCh)
					if registered {
						gasLimit = validator.GasLimit
						feeRecipient = common.Address(validator.FeeRecipient)
						bytes, _ := attributesEvent.MarshalJSON()
						logger = logger.With("feeRecipient", feeRecipient, "attributes", string(bytes))
					} else if wait {
						bidUnknownValidator.Inc(1)
						logger.Warn("Skipping envelope building", "reason", "validator not registered")
						return
					}

					err := r.builder.InitEnvelopeBuilding(pendingEnvelope.interrupt, pendingEnvelope.events, gasLimit,
						&miner.BuildPayloadArgs{
							Parent:       common.Hash(pendingEnvelope.attributes.ParentBlockHash),
							Timestamp:    pendingEnvelope.attributes.V4.Timestamp,
							FeeRecipient: feeRecipient,
							Random:       pendingEnvelope.attributes.V4.PrevRandao,
							Withdrawals:  convertWithdrawalsToTypes(pendingEnvelope.attributes.V4.Withdrawals),
							BeaconRoot:   (*common.Hash)(&pendingEnvelope.attributes.V4.ParentBeaconBlockRoot),
							Version:      engine.PayloadV3,
						})
					if err != nil {
						logger.Error("Failed to build payload", "error", err)
						return
					}

					logger.Info("Envelope building initiated")
				}(pendingEnvelope, r.cfg.WaitValidator)

			case envelopeElapsed, ok := <-pendingEnvelope.events:
				if !ok {
					pendingEnvelope.events = nil
					continue
				}

				logger := r.log.With("number", envelopeElapsed.Envelope.ExecutionPayload.Number,
					"hash", envelopeElapsed.Envelope.ExecutionPayload.BlockHash,
					"slot", pendingEnvelope.attributes.ProposalSlot,
					"txs", len(envelopeElapsed.Envelope.ExecutionPayload.Transactions),
				)

				bidKey := newSlotKey(pendingEnvelope.attributes.ParentBlockHash, pendingEnvelope.attributes.ProposalSlot)
				if len(envelopeElapsed.Envelope.ExecutionPayload.Transactions) == 0 && r.signedBids.Contains(bidKey) {
					logger.Warn("Skipping envelope without txs")
					bidEmpty.Inc(1)
					continue
				}

				if r.isDelivered(bidKey) {
					logger.Warn("Blinded block has already delivered")
					bidUnclaimable.Inc(1)
					continue
				}

				gasUsed := float64(envelopeElapsed.Envelope.ExecutionPayload.GasUsed)
				logger = logger.With("mgas", gasUsed/1_000_000,
					"elapsed", common.PrettyDuration(envelopeElapsed.Elapsed),
					"mgasps", gasUsed*1_000/float64(envelopeElapsed.Elapsed),
					"gwei", new(big.Int).Div(envelopeElapsed.OrigValue, big.NewInt(params.GWei)),
				)

				signedBid, executionData, err := r.makeSignedBid(
					envelopeElapsed.Envelope.ExecutionPayload,
					envelopeElapsed.Envelope.BlobsBundle,
					envelopeElapsed.Envelope.Requests,
					envelopeElapsed.Envelope.BlockValue,
				)
				if err != nil {
					logger.Error("Failed to create relay bid", "error", err)
					continue
				}

				payload := makeExecutionPayload(signedBid.Electra.Message.Header, executionData)
				payloadAndBid := &BlockAndBid{
					Block: &api.VersionedSubmitBlindedBlockResponse{
						Version: beaconSpec.DataVersionElectra,
						Electra: &deneb2.ExecutionPayloadAndBlobsBundle{
							ExecutionPayload: payload.Electra,
							BlobsBundle:      getBlobsBundle(envelopeElapsed.Envelope.BlobsBundle),
						},
					},
					Bid: signedBid,
				}
				r.bidPayloads.Add(signedBid.Electra.Message.Header.BlockHash, payloadAndBid)
				r.signedBids.Add(bidKey, signedBid)

				bidClaimable.Inc(1)
				logger.Info("Relay bid created and cached")
			}
		}
	}()

	return nil
}

// cacheProposerDuties caches the proposer duties for the next slot.
// and returns the validator pubkeys if registrations are known
func (r *Relay) cacheProposerDuties(duties []*beaconV1.ProposerDuty) []*v1.ValidatorRegistration {
	var pubKeys []phase0.BLSPubKey
	for _, duty := range duties {
		r.slotDuties.Add(duty.Slot, duty)
		pubKeys = append(pubKeys, duty.PubKey)
	}
	return r.getValidators(pubKeys)
}

func (r *Relay) updateNextSlotDuties(ctx context.Context, head *beaconV1.HeadEvent) bool {
	nextSlot := head.Slot + 1
	epoch := nextSlot / slotsPerEpoch
	logger := r.log.With("nextSlot", nextSlot, "slotEpoch", epoch)

	// todo: https://github.com/attestantio/go-eth2-client/pull/235
	duties, err := r.beacon.ProposerDuties(ctx, phase0.Epoch(epoch))
	if err != nil {
		logger.Error("Failed to fetch proposer duties", "error", err)
		return false
	}

	expectedRoot := head.CurrentDutyDependentRoot
	if slotPos(nextSlot) == 1 {
		expectedRoot = head.Block
	}

	unverified, _ := duties.Metadata["execution_optimistic"].(bool)
	dependentRoot, ok := duties.Metadata["dependent_root"].(phase0.Root)
	logger = logger.With("dependentRoot", common.Hash(dependentRoot), "optimistic", unverified)

	if !ok || dependentRoot != expectedRoot { // FIXME
		logger.Warn("Mismatch proposer duties dependent root", "expectedRoot", expectedRoot)
		return false
	}
	if len(duties.Data) == 0 {
		logger.Warn("No proposer duties received for epoch")
		return false
	}

	known := r.cacheProposerDuties(duties.Data)
	if len(known) > 0 {
		r.sendNextEpochValidators(known)
	}

	logger.Info("Proposer duties updated", "duties", len(duties.Data), "known-registrations", len(known))

	return !unverified
}

func (r *Relay) preloadNextEpochDuties(ctx context.Context, slot phase0.Slot) bool {
	epoch := slot/slotsPerEpoch + 1
	logger := r.log.With("headSlot", slot, "nextEpoch", epoch)

	duties, err := r.beacon.ProposerDuties(ctx, phase0.Epoch(epoch))
	if err != nil {
		logger.Error("Failed to fetch proposer duties", "error", err)
		return false
	}

	unverified, _ := duties.Metadata["execution_optimistic"].(bool)
	dependentRoot, _ := duties.Metadata["dependent_root"].(phase0.Root)
	logger = logger.With("dependentRoot", common.Hash(dependentRoot), "optimistic", unverified)

	if len(duties.Data) == 0 {
		logger.Warn("No proposer duties received for epoch")
		return false
	}

	known := r.cacheProposerDuties(duties.Data)
	if len(known) > 0 {
		r.sendNextEpochValidators(known)
	}

	logger.Info("Proposer duties preloaded", "duties", len(duties.Data), "known-registrations", len(known))

	return !unverified
}

func (r *Relay) makeSignedBid(executableData *engine.ExecutableData, blobsBundle *engine.BlobsBundleV1, requests [][]byte, value *big.Int) (*spec.VersionedSignedBuilderBid, *ExecutionData, error) {
	if executableData.GasUsed > executableData.GasLimit {
		return nil, nil, errors.New("gas used exceeds limit")
	}
	if value.Sign() < 0 {
		return nil, nil, errors.New("block value must be positive")
	}
	if len(executableData.Transactions) == 0 {
		//return nil, nil, errors.New("block must contain transactions")
	}

	var blobGasUsed uint64
	if executableData.BlobGasUsed != nil {
		blobGasUsed = *executableData.BlobGasUsed
	}

	var excessBlobGas uint64
	if executableData.ExcessBlobGas != nil {
		excessBlobGas = *executableData.ExcessBlobGas
	}

	commitments := make([]deneb.KZGCommitment, 0, len(blobsBundle.Commitments))
	for _, commitment := range blobsBundle.Commitments {
		commitments = append(commitments, deneb.KZGCommitment(commitment))
	}

	transactions := convertTransactions(executableData.Transactions)
	transactionsRoot, err := computeTransactionsRoot(transactions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute transactions root, err: %w", err)
	}

	withdrawals := convertWithdrawalsFromTypes(executableData.Withdrawals)
	withdrawalsRoot, err := computeWithdrawalsRoot(withdrawals)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute withdrawals root, err: %w", err)
	}

	executionRequests, err := parseExecutionRequests(requests)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert execution requests: %w", err)
	}

	message := &electra.BuilderBid{
		Header: &deneb.ExecutionPayloadHeader{
			ParentHash:       phase0.Hash32(executableData.ParentHash),
			FeeRecipient:     bellatrix.ExecutionAddress(executableData.FeeRecipient),
			StateRoot:        phase0.Root(executableData.StateRoot),
			ReceiptsRoot:     phase0.Root(executableData.ReceiptsRoot),
			LogsBloom:        types.BytesToBloom(executableData.LogsBloom),
			PrevRandao:       executableData.Random,
			BlockNumber:      executableData.Number,
			GasLimit:         executableData.GasLimit,
			GasUsed:          executableData.GasUsed,
			Timestamp:        executableData.Timestamp,
			ExtraData:        executableData.ExtraData,
			BaseFeePerGas:    uint256.MustFromBig(executableData.BaseFeePerGas),
			BlockHash:        phase0.Hash32(executableData.BlockHash),
			TransactionsRoot: transactionsRoot,
			WithdrawalsRoot:  withdrawalsRoot,
			BlobGasUsed:      blobGasUsed,
			ExcessBlobGas:    excessBlobGas,
		},
		BlobKZGCommitments: commitments,
		ExecutionRequests:  executionRequests,
		Value:              uint256.MustFromBig(value),
		Pubkey:             r.publicKey,
	}

	signature, err := ssz.SignMessage(message, r.domainBuilder, r.secretKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign bid message, err: %w", err)
	}

	return &spec.VersionedSignedBuilderBid{
		Version: beaconSpec.DataVersionElectra,
		Electra: &electra.SignedBuilderBid{
			Message:   message,
			Signature: signature,
		},
	}, &ExecutionData{transactions: transactions, withdrawals: withdrawals}, nil
}

func getBlobsBundle(blobsBundle *engine.BlobsBundleV1) *deneb2.BlobsBundle {
	commitments := make([]deneb.KZGCommitment, len(blobsBundle.Commitments))
	proofs := make([]deneb.KZGProof, len(blobsBundle.Proofs))
	blobs := make([]deneb.Blob, len(blobsBundle.Blobs))

	// we assume the lengths for blobs bundle is validated beforehand to be the same
	for i := range blobsBundle.Blobs {
		var commitment deneb.KZGCommitment
		copy(commitment[:], blobsBundle.Commitments[i][:])
		commitments[i] = commitment

		var proof deneb.KZGProof
		copy(proof[:], blobsBundle.Proofs[i][:])
		proofs[i] = proof

		var blob deneb.Blob
		copy(blob[:], blobsBundle.Blobs[i][:])
		blobs[i] = blob
	}
	return &deneb2.BlobsBundle{
		Commitments: commitments,
		Proofs:      proofs,
		Blobs:       blobs,
	}
}
