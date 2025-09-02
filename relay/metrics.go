package relay

import (
	consensusMetrics "github.com/attestantio/go-eth2-client/metrics"
	"github.com/ethereum/go-ethereum/metrics"
)

var _ consensusMetrics.Service = Presenter{}

type Presenter struct{}

func (p Presenter) Presenter() string { return "prometheus" }

var (
	registrationImported         = metrics.NewRegisteredCounter("relay/registration/imported", nil)
	registrationSuccess          = metrics.NewRegisteredCounter("relay/registration/success", nil)
	registrationOutdated         = metrics.NewRegisteredCounter("relay/registration/outdated", nil)
	registrationNotNewer         = metrics.NewRegisteredCounter("relay/registration/not_newer", nil)
	registrationRequested        = metrics.NewRegisteredCounter("relay/registration/requested", nil)
	registrationNotEligible      = metrics.NewRegisteredCounter("relay/registration/not_eligible", nil)
	registrationFailedCheck      = metrics.NewRegisteredCounter("relay/registration/failed_check", nil)
	registrationSignatureError   = metrics.NewRegisteredCounter("relay/registration/signature_error", nil)
	registrationInvalidSignature = metrics.NewRegisteredCounter("relay/registration/invalid_signature", nil)

	bidEmpty            = metrics.NewRegisteredCounter("relay/bid/empty", nil)
	bidClaimable        = metrics.NewRegisteredCounter("relay/bid/claimable", nil)
	bidUnclaimable      = metrics.NewRegisteredCounter("relay/bid/unclaimable", nil)
	bidUnknownValidator = metrics.NewRegisteredCounter("relay/bid/unknown_validator", nil)

	getHeaderAfterSlotHit          = metrics.NewRegisteredHistogram("relay/get_header/after", nil, metrics.NewExpDecaySample(1028, 0.015))
	getHeaderSuccess               = metrics.NewRegisteredCounter("relay/get_header/success", nil)
	getHeaderBidNotFound           = metrics.NewRegisteredCounter("relay/get_header/bid_not_found", nil)
	getHeaderRequestTooLate        = metrics.NewRegisteredCounter("relay/get_header/request_too_late", nil)
	getHeaderProposerNotFound      = metrics.NewRegisteredCounter("relay/get_header/proposer_not_found", nil)
	getHeaderProposerNotAssigned   = metrics.NewRegisteredCounter("relay/get_header/proposer_not_assigned", nil)
	getHeaderProposerNotRegistered = metrics.NewRegisteredCounter("relay/get_header/proposer_not_registered", nil)

	getPayloadAfterSlotHit          = metrics.NewRegisteredHistogram("relay/get_payload/after", nil, metrics.NewExpDecaySample(1028, 0.015))
	getPayloadSuccess               = metrics.NewRegisteredCounter("relay/get_payload/success", nil)
	getPayloadNotFound              = metrics.NewRegisteredCounter("relay/get_payload/not_found", nil)
	getPayloadSubmitError           = metrics.NewRegisteredCounter("relay/get_payload/submit_error", nil)
	getPayloadRequestTooLate        = metrics.NewRegisteredCounter("relay/get_payload/request_too_late", nil)
	getPayloadSignatureError        = metrics.NewRegisteredCounter("relay/get_payload/signature_error", nil)
	getPayloadValidationFailed      = metrics.NewRegisteredCounter("relay/get_payload/validation_failed", nil)
	getPayloadAlreadyDelivered      = metrics.NewRegisteredCounter("relay/get_payload/already_delivered", nil)
	getPayloadInvalidSignature      = metrics.NewRegisteredCounter("relay/get_payload/invalid_signature", nil)
	getPayloadProposerMismatch      = metrics.NewRegisteredCounter("relay/get_payload/proposer_mismatch", nil)
	getPayloadProposerNotFound      = metrics.NewRegisteredCounter("relay/get_payload/proposer_not_found", nil)
	getPayloadProposerNotRegistered = metrics.NewRegisteredCounter("relay/get_payload/proposer_not_registered", nil)

	getPayloadLatest = metrics.NewRegisteredGauge("relay/get_payload/latest", nil)
)
