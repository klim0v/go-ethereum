package relay

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/attestantio/go-builder-client/api"
	"github.com/attestantio/go-builder-client/api/electra"
	v1 "github.com/attestantio/go-builder-client/api/v1"
	"github.com/attestantio/go-builder-client/spec"
	beaconApi "github.com/attestantio/go-eth2-client/api"
	beaconV1 "github.com/attestantio/go-eth2-client/api/v1"
	beaconSpec "github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/flashbots/go-boost-utils/bls"
	"github.com/flashbots/go-boost-utils/ssz"
	"github.com/flashbots/go-boost-utils/utils"
)

type envelopeBuilder interface {
	InitEnvelopeBuilding(interrupt <-chan struct{}, results chan<- *miner.EnvelopeElapsed, gasLimit uint64, args *miner.BuildPayloadArgs) error
}

// Relay implements MEV-Boost relay that connects slotDuties with TOOL builder
type Relay struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	cfg     Config
	beacon  *Beacon // beacon chain client for consensus layer communication
	builder envelopeBuilder
	log     log.Logger

	nextEpochValidators event.Feed // event feed for next-epoch validator registrations
	valEligCheckCh      chan []*v1.ValidatorRegistration

	validatorsMutex sync.RWMutex
	validatorsCache lru.BasicLRU[phase0.BLSPubKey, *v1.ValidatorRegistration]

	slotDuties  *lru.Cache[phase0.Slot, *beaconV1.ProposerDuty]      // slot-to-proposer duty (including next epoch)
	signedBids  *lru.Cache[SlotKey, *spec.VersionedSignedBuilderBid] // builder bids cache
	bidPayloads *lru.Cache[phase0.Hash32, *BlockAndBid]              // execution payloads by block hash

	deliverMu sync.RWMutex
	delivered lru.BasicLRU[SlotKey, phase0.Hash32] // list of delivered block payloads

	// Cryptographic domains for signature verification
	domainBuilder  phase0.Domain // domain for validator registration signatures and builder bid signatures
	domainProposer phase0.Domain // domain for proposer block signatures

	// Builder identity
	secretKey *bls.SecretKey   // private key for signing relay bids
	publicKey phase0.BLSPubKey // public key identifying this relay as builder
}

// NewRelay creates a new Relay instance with initialized domains.
func NewRelay(cfg Config, builder envelopeBuilder) (*Relay, error) {
	if cfg.ValidatorCache <= 0 {
		log.Warn("Validator cache size forced", "cfg", cfg.ValidatorCache, "default", DefaultConfig.ValidatorCache)
		cfg.ValidatorCache = DefaultConfig.ValidatorCache
	}

	secretKeyHex, err := hexutil.Decode(cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret key, err: %w", err)
	}

	secretKey, err := bls.SecretKeyFromBytes(secretKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse BLS secret key, err: %w", err)
	}

	publicKeyBLS, err := bls.PublicKeyFromSecretKey(secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get BLS public key from private key, err: %w", err)
	}

	publicKey, err := utils.BlsPublicKeyToPublicKey(publicKeyBLS)
	if err != nil {
		return nil, fmt.Errorf("failed to make public key, err: %w", err)
	}

	currentForkVersion, err := findBeaconForkVersion(cfg.BeaconConfig.Forks, "ELECTRA")
	if err != nil {
		return nil, err
	}

	genesisForkVersion, err := findBeaconForkVersion(cfg.BeaconConfig.Forks, "GENESIS")
	if err != nil {
		return nil, err
	}

	validatorsRoot := phase0.Root(cfg.BeaconConfig.GenesisValidatorsRoot)

	r := &Relay{
		cfg:     cfg,
		builder: builder,
		log:     log.New(),

		valEligCheckCh:  make(chan []*v1.ValidatorRegistration),
		validatorsCache: lru.NewBasicLRU[phase0.BLSPubKey, *v1.ValidatorRegistration](cfg.ValidatorCache),

		slotDuties:  lru.NewCache[phase0.Slot, *beaconV1.ProposerDuty](slotsPerEpoch * 4), // 128
		signedBids:  lru.NewCache[SlotKey, *spec.VersionedSignedBuilderBid](32),
		bidPayloads: lru.NewCache[phase0.Hash32, *BlockAndBid](64),

		delivered: lru.NewBasicLRU[SlotKey, phase0.Hash32](12),

		domainBuilder:  ssz.ComputeDomain(ssz.DomainTypeAppBuilder, genesisForkVersion, phase0.Root{}),
		domainProposer: ssz.ComputeDomain(ssz.DomainTypeBeaconProposer, currentForkVersion, validatorsRoot),

		secretKey: secretKey,
		publicKey: publicKey,
	}

	return r, nil

}

func (r *Relay) Start() (err error) {
	r.ctx, r.cancel = context.WithCancel(context.Background())

	r.beacon, err = NewBeacon(r.ctx, r.cfg.BeaconEndpoints)
	if err != nil {
		r.cancel()
		return fmt.Errorf("failed to init beacon client, err: %w", err)
	}

	r.log.Info("Starting relay", "publicKey", r.publicKey)
	for !r.beacon.IsSynced() {
		syncing, err := r.beacon.NodeSyncing(r.ctx)
		if err != nil {
			r.log.Warn("Waiting relay beacon node", "error", err)
			time.Sleep(15 * time.Second)
			continue
		}
		if syncing.Data.SyncDistance > 1 {
			r.log.Warn("Waiting relay beacon node", "distance", syncing.Data.SyncDistance)
			time.Sleep(15 * time.Second)
			continue
		}
		r.log.Info("Started relay", "head", syncing.Data.HeadSlot)
		break
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.startValidatorRegistrationChecker()
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.startWork(); err != nil {
			log.Error("Failed to start relay work", "err", err)
		}
	}()

	return nil
}

func (r *Relay) Stop() error {
	r.cancel()
	r.wg.Wait()
	return nil
}

func (r *Relay) ImportAndSendNextEpochValidators(validators []*v1.ValidatorRegistration) {
	var renewedList []*v1.ValidatorRegistration
	for _, registration := range validators {
		renewed := r.setValidator(registration)
		if !renewed {
			registrationNotNewer.Inc(1)
			continue
		}
		renewedList = append(renewedList, registration)
	}
	if count := len(renewedList); count > 0 {
		registrationImported.Inc(int64(count))
		r.sendNextEpochValidators(renewedList)
	}
}

func (r *Relay) RegisterValidators(ctx context.Context, registrations []*v1.SignedValidatorRegistration) error {
	now := time.Now()
	requestedEligibleCheck := map[phase0.BLSPubKey]*v1.ValidatorRegistration{}

	for _, registration := range registrations {
		pubKey := registration.Message.Pubkey
		regTime := registration.Message.Timestamp
		logger := r.log.With("method", "RegisterValidator", "pubKey", pubKey, "regTime", regTime.Format(time.DateTime))

		if regTime.Sub(now) > 10*time.Second || regTime.Unix() < int64(r.cfg.BeaconConfig.GenesisTime) {
			registrationOutdated.Inc(1)
			logger.Debug("Registration is outdated")
			continue
		}

		if existing, ok := r.getValidator(pubKey, nil); ok && !registration.Message.Timestamp.After(existing.Timestamp) {
			registrationNotNewer.Inc(1)
			logger.Debug("Registration not newer than existing")
			continue
		}

		ok, err := ssz.VerifySignature(registration.Message, r.domainBuilder, pubKey[:], registration.Signature[:])
		if err != nil {
			registrationSignatureError.Inc(1)
			logger.Error("Failed to verify registration signature", "error", err)
			continue
		}
		if !ok {
			registrationInvalidSignature.Inc(1)
			logger.Warn("Invalid validator signature")
			continue
		}

		registrationRequested.Inc(1)
		requestedEligibleCheck[pubKey] = registration.Message
		logger.Debug("Validator registration requested successfully")
	}

	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case r.valEligCheckCh <- slices.Collect(maps.Values(requestedEligibleCheck)):
	}

	return nil
}

const (
	tooEarlyThreshold = time.Second
)

// GetHeader retrieves a bid by parent hash if the validator is validators.
func (r *Relay) GetHeader(ctx context.Context, slot phase0.Slot, parentHash phase0.Hash32, pubKey phase0.BLSPubKey) (*spec.VersionedSignedBuilderBid, error) {
	logger := r.log.With("method", "GetHeader", "slot", slot, "parentHash", parentHash, "proposer", pubKey)

	if _, ok := r.getValidator(pubKey, nil); !ok {
		getHeaderProposerNotRegistered.Inc(1)
		logger.Warn("Proposer not registered")
		return nil, errors.New("proposer not registered")
	}

	duty, ok := r.slotDuties.Get(slot)
	if !ok {
		getHeaderProposerNotFound.Inc(1)
		logger.Warn("Slot proposer unknown")
		return nil, errors.New("slot proposer unknown")
	}
	if duty.PubKey != pubKey {
		getHeaderProposerNotAssigned.Inc(1)
		logger.Warn("Proposer not assigned to slot")
		return nil, errors.New("proposer not assigned to slot")
	}

	signedBidKey := newSlotKey(parentHash, slot)
	signedBid, ok := r.signedBids.Get(signedBidKey)
	if !ok {
		getHeaderBidNotFound.Inc(1)
		logger.Warn("Bid not found for parent hash and slot")
		return nil, errors.New("bid not found for parent hash and slot")
	}

	until := time.Until(time.Unix(int64(signedBid.Electra.Message.Header.Timestamp), 0))
	getHeaderAfterSlotHit.Update(-until.Milliseconds())
	if time.Second-until > r.cfg.MaxRequestDelay { // reserve 1s buffer for getPayload.
		getHeaderRequestTooLate.Inc(1)
		logger.Warn("Request received too late", "until", until)
		return nil, errors.New("too late request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(until - tooEarlyThreshold):
	}

	newerSignedBid, ok := r.signedBids.Get(signedBidKey)
	if ok {
		signedBid = newerSignedBid
	}

	getHeaderSuccess.Inc(1)
	logger.Info("Successfully provided signed builder bid", "blockHash", signedBid.Electra.Message.Header.BlockHash)
	return &spec.VersionedSignedBuilderBid{
		Version: beaconSpec.DataVersionElectra,
		Electra: &electra.SignedBuilderBid{
			Message:   signedBid.Electra.Message,
			Signature: signedBid.Electra.Signature,
		},
	}, nil
}

// GetPayload exchanges signed blinded beacon block for execution payload and submits block to beacon chain
func (r *Relay) GetPayload(ctx context.Context, signedBlindedBlock *beaconApi.VersionedSignedBlindedBeaconBlock) (*api.VersionedSubmitBlindedBlockResponse, error) {
	if signedBlindedBlock == nil {
		return nil, errors.New("signed blinded block is nil")
	}

	logger := r.log.With("method", "GetPayload")

	switch signedBlindedBlock.Version {
	case beaconSpec.DataVersionElectra:
	default:
		logger.Warn("Unsupported version", "version", signedBlindedBlock.Version)
		return nil, errors.New("unsupported version")
	}

	blockHash, err := signedBlindedBlock.ExecutionBlockHash()
	if err != nil {
		return nil, err
	}
	logger = logger.With("blockHash", common.Hash(blockHash))

	payloadData, ok := r.bidPayloads.Get(blockHash)
	if !ok {
		logger.Warn("Payload not found")
		getPayloadNotFound.Inc(1)
		return nil, errors.New("payload not found")
	}

	until := time.Until(time.Unix(int64(payloadData.Block.Electra.ExecutionPayload.Timestamp), 0))
	getPayloadAfterSlotHit.Update(-until.Milliseconds())
	if -until > r.cfg.MaxRequestDelay {
		getPayloadRequestTooLate.Inc(1)
		logger.Warn("Request received too late", "until", until)
		return nil, errors.New("too late request")
	}

	slot, err := signedBlindedBlock.Slot()
	if err != nil {
		return nil, err
	}
	logger = logger.With("slot", slot)

	parentHash, err := signedBlindedBlock.ExecutionParentHash()
	if err != nil {
		return nil, err
	}
	logger = logger.With("parentHash", common.Hash(parentHash))

	duty, ok := r.slotDuties.Get(slot)
	if !ok {
		getPayloadProposerNotFound.Inc(1)
		logger.Warn("Proposer not found for slot")
		return nil, errors.New("proposer not found")
	}
	proposerIndex, err := signedBlindedBlock.ProposerIndex()
	if err != nil {
		return nil, err
	}
	if duty.ValidatorIndex != proposerIndex {
		getPayloadProposerMismatch.Inc(1)
		logger.Warn("Proposer index mismatch")
		return nil, errors.New("proposer index mismatch")
	}
	if _, ok = r.getValidator(duty.PubKey, nil); !ok {
		getPayloadProposerNotRegistered.Inc(1)
		logger.Warn("Validator not registered")
		return nil, errors.New("validator not registered")
	}
	logger = logger.With("proposer", duty.PubKey)

	ok, err = verifyBlockSignature(signedBlindedBlock, r.domainProposer, duty.PubKey[:])
	if err != nil {
		getPayloadSignatureError.Inc(1)
		logger.Error("Failed to verify block signature", "error", err)
		return nil, errors.New("failed to verify block signature")
	}
	if !ok {
		getPayloadInvalidSignature.Inc(1)
		logger.Warn("Invalid blinded block signature")
		return nil, errors.New("invalid blinded block signature")
	}

	proposal, err := convertSignedBlindedBeaconBlockToBeaconBlock(signedBlindedBlock, payloadData)
	if err != nil {
		getPayloadValidationFailed.Inc(1)
		logger.Warn("Failed to validate and convert beacon block", "error", err)
		return nil, errors.New("failed to validate and convert beacon block")
	}

	bidKey := newSlotKey(parentHash, slot)
	if !r.addIfNotDelivered(bidKey, blockHash) {
		getPayloadAlreadyDelivered.Inc(1)
		logger.Warn("Block has already been delivered")
		return nil, errors.New("block has already been delivered")
	}

	bytes, _ := proposal.Electra.MarshalJSON()
	logger = logger.With("proposal", string(bytes))
	getPayloadLatest.Update(int64(proposal.Electra.SignedBlock.Message.Body.ExecutionPayload.BlockNumber))

	if err := r.beacon.SubmitProposal(ctx, proposal); err != nil {
		getPayloadSubmitError.Inc(1)
		logger.Error("Failed to submit block proposal", "error", err)
		return nil, errors.New("failed to submit block proposal")
	}

	getPayloadSuccess.Inc(1)
	logger.Info("Successfully submitted blinded proposal")
	return &api.VersionedSubmitBlindedBlockResponse{
		Version: beaconSpec.DataVersionElectra,
		Electra: payloadData.Block.Electra,
	}, nil
}

func (r *Relay) addIfNotDelivered(bidKey SlotKey, blockHash phase0.Hash32) (added bool) {
	r.deliverMu.Lock()
	defer r.deliverMu.Unlock()

	if r.delivered.Contains(bidKey) {
		return false
	}

	r.delivered.Add(bidKey, blockHash)
	return true
}

func (r *Relay) isDelivered(bidKey SlotKey) (added bool) {
	r.deliverMu.RLock()
	defer r.deliverMu.RUnlock()

	return r.delivered.Contains(bidKey)
}

const validatorRegistrationCheckCount = 1_000

func (r *Relay) startValidatorRegistrationChecker() {
	registrationRequests := make(map[phase0.BLSPubKey]*v1.ValidatorRegistration, 2*validatorRegistrationCheckCount)
	registrationQueue := r.valEligCheckCh
	for {
		select {
		case <-r.ctx.Done():
			return
		case validatorRegistrations := <-registrationQueue:
			for _, validatorRegistration := range validatorRegistrations {
				registrationRequests[validatorRegistration.Pubkey] = validatorRegistration
			}
			if len(registrationRequests) >= validatorRegistrationCheckCount {
				registrationQueue = nil
			}
		case <-time.After(10 * time.Millisecond):
			if len(registrationRequests) == 0 {
				continue
			}
			registrationQueue = r.valEligCheckCh
			pubKeys := slices.Collect(maps.Keys(registrationRequests))

			start := time.Now()
			validators, err := r.beacon.Validator(r.ctx, pubKeys)
			logger := r.log.With("duration", common.PrettyDuration(time.Since(start)), "requested", len(pubKeys))
			if err != nil {
				clear(registrationRequests)
				registrationFailedCheck.Inc(1)
				logger.Error("Failed to get validator states", "error", err)
				continue
			}

			eligible := make([]*v1.ValidatorRegistration, 0, len(validators))
			for _, validator := range validators {
				publicKey := validator.Validator.PublicKey
				registration := registrationRequests[publicKey]
				eligible = append(eligible, registration)
				delete(registrationRequests, publicKey)
			}
			if len(eligible) > 0 {
				r.setValidators(eligible)
				registrationSuccess.Inc(int64(len(eligible)))
			}

			// for the remaining registrations delete from cache validator registrations
			if count := len(registrationRequests); count > 0 {
				r.delValidators(slices.Collect(maps.Keys(registrationRequests)))
				registrationNotEligible.Inc(int64(count))
				clear(registrationRequests)
			}

			logger.Info("Validator registration check completed", "eligible", len(eligible), "total", r.cntValidators())
		}
	}
}

func (r *Relay) SubscribeNextEpochValidators(ch chan<- []*v1.ValidatorRegistration) event.Subscription {
	return r.nextEpochValidators.Subscribe(ch)
}

func (r *Relay) sendNextEpochValidators(batch []*v1.ValidatorRegistration) {
	r.nextEpochValidators.Send(batch)
}

func (r *Relay) delValidators(pubkeys []phase0.BLSPubKey) {
	r.validatorsMutex.Lock()
	defer r.validatorsMutex.Unlock()

	for _, pubkey := range pubkeys {
		r.validatorsCache.Remove(pubkey)
	}
}

func (r *Relay) cntValidators() int {
	r.validatorsMutex.RLock()
	defer r.validatorsMutex.RUnlock()

	return r.validatorsCache.Len()
}

func (r *Relay) setValidators(validators []*v1.ValidatorRegistration) {
	r.validatorsMutex.Lock()
	defer r.validatorsMutex.Unlock()

	for _, validator := range validators {
		r.validatorsCache.Add(validator.Pubkey, validator)
	}
}

func (r *Relay) setValidator(validator *v1.ValidatorRegistration) (renewed bool) {
	r.validatorsMutex.Lock()
	defer r.validatorsMutex.Unlock()

	existing, exists := r.validatorsCache.Get(validator.Pubkey)
	if exists && !validator.Timestamp.After(existing.Timestamp) {
		return false
	}

	r.validatorsCache.Add(validator.Pubkey, validator)
	return true
}

func (r *Relay) getValidators(pubKeys []phase0.BLSPubKey) []*v1.ValidatorRegistration {
	r.validatorsMutex.RLock()
	defer r.validatorsMutex.RUnlock()

	validators := make([]*v1.ValidatorRegistration, 0, len(pubKeys)/2)
	for _, pubKey := range pubKeys {
		if validator, ok := r.validatorsCache.Get(pubKey); ok {
			validators = append(validators, validator)
		}
	}
	return validators
}

func (r *Relay) getValidator(pubKey phase0.BLSPubKey, wait chan struct{}) (*v1.ValidatorRegistration, bool) {
	r.validatorsMutex.RLock()

	validator, ok := r.validatorsCache.Get(pubKey)
	if ok || wait == nil {
		r.validatorsMutex.RUnlock()

		return validator, ok
	}

	news := make(chan []*v1.ValidatorRegistration, 1)
	subscribe := r.nextEpochValidators.Subscribe(news)
	defer subscribe.Unsubscribe()

	r.validatorsMutex.RUnlock()

	for {
		select {
		case validators, more := <-news:
			if !more {
				return nil, false
			}
			for _, registration := range validators {
				if registration.Pubkey == pubKey {
					return registration, true
				}
			}

		case <-wait:
			return nil, false
		case <-r.ctx.Done():
			return nil, false
		case <-subscribe.Err():
			return nil, false
		case <-time.After(10 * time.Millisecond):
			registration, ok := r.getValidator(pubKey, nil)
			if !ok {
				return nil, false
			}
			return registration, true
		}
	}
}
