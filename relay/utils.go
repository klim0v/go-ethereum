package relay

import (
	"errors"
	"fmt"
	"time"

	"github.com/attestantio/go-builder-client/api"
	"github.com/attestantio/go-builder-client/spec"
	beaconApi "github.com/attestantio/go-eth2-client/api"
	electra3 "github.com/attestantio/go-eth2-client/api/v1/electra"
	beaconSpec "github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/capella"
	"github.com/attestantio/go-eth2-client/spec/deneb"
	electra2 "github.com/attestantio/go-eth2-client/spec/electra"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	bellatrix2 "github.com/attestantio/go-eth2-client/util/bellatrix"
	capella2 "github.com/attestantio/go-eth2-client/util/capella"
	"github.com/ethereum/go-ethereum/beacon/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/go-boost-utils/bls"
	"github.com/flashbots/go-boost-utils/utils"
)

type BlockAndBid struct {
	Block *api.VersionedSubmitBlindedBlockResponse
	Bid   *spec.VersionedSignedBuilderBid
}

type SlotKey struct {
	ParentHash phase0.Hash32
	Slot       phase0.Slot
}

func newSlotKey(parentHash phase0.Hash32, slot phase0.Slot) SlotKey {
	return SlotKey{ParentHash: parentHash, Slot: slot}
}

type ExecutionData struct {
	transactions []bellatrix.Transaction
	withdrawals  []*capella.Withdrawal
}

type blockObject interface {
	Root() (phase0.Root, error)
	Signature() (phase0.BLSSignature, error)
}

func verifyBlockSignature(block blockObject, domain phase0.Domain, pubKey []byte) (bool, error) {
	root, err := block.Root()
	if err != nil {
		return false, err
	}
	sig, err := block.Signature()
	if err != nil {
		return false, err
	}
	signingData := phase0.SigningData{ObjectRoot: root, Domain: domain}
	msg, err := signingData.HashTreeRoot()
	if err != nil {
		return false, err
	}

	return bls.VerifySignatureBytes(msg[:], sig[:], pubKey[:])
}

const (
	secondsPerSlot  = 12
	durationPerSlot = time.Duration(secondsPerSlot) * time.Second

	slotsPerEpoch    = 32
	durationPerEpoch = time.Duration(slotsPerEpoch) * durationPerSlot
)

func findBeaconForkVersion(forks params.Forks, name string) (phase0.Version, error) {
	for _, fork := range forks {
		if fork.Name == name {
			return getVersion(fork.Version), nil
		}
	}
	return phase0.Version{}, fmt.Errorf("beacon fork %s not found", name)
}

func getVersion(bb []byte) phase0.Version {
	var version phase0.Version
	copy(version[:], bb)
	return version
}

func computeWithdrawalsRoot(withdrawals []*capella.Withdrawal) (phase0.Root, error) {
	if withdrawals == nil {
		return phase0.Root{}, errors.New("no withdrawals")
	}
	w := capella2.ExecutionPayloadWithdrawals{Withdrawals: withdrawals}
	return w.HashTreeRoot()
}

func computeTransactionsRoot(transactions []bellatrix.Transaction) (phase0.Root, error) {
	t := bellatrix2.ExecutionPayloadTransactions{Transactions: transactions}
	return t.HashTreeRoot()
}

func convertWithdrawalsToTypes(withdrawals []*capella.Withdrawal) []*types.Withdrawal {
	ws := make([]*types.Withdrawal, 0, len(withdrawals))
	for _, withdrawal := range withdrawals {
		ws = append(ws, &types.Withdrawal{
			Index:     uint64(withdrawal.Index),
			Validator: uint64(withdrawal.ValidatorIndex),
			Address:   common.Address(withdrawal.Address),
			Amount:    uint64(withdrawal.Amount),
		})
	}
	return ws
}

func convertWithdrawalsFromTypes(withdrawals []*types.Withdrawal) []*capella.Withdrawal {
	result := make([]*capella.Withdrawal, 0, len(withdrawals))
	for _, withdrawal := range withdrawals {
		result = append(result, &capella.Withdrawal{
			Index:          capella.WithdrawalIndex(withdrawal.Index),
			ValidatorIndex: phase0.ValidatorIndex(withdrawal.Validator),
			Address:        bellatrix.ExecutionAddress(withdrawal.Address),
			Amount:         phase0.Gwei(withdrawal.Amount),
		})
	}
	return result
}

func convertTransactions(transactions [][]byte) []bellatrix.Transaction {
	result := make([]bellatrix.Transaction, 0, len(transactions))
	for _, transaction := range transactions {
		result = append(result, transaction)
	}
	return result
}

const (
	// Request types
	depositRequestType       = 0x00
	withdrawalRequestType    = 0x01
	consolidationRequestType = 0x02
)

// parseExecutionRequests parses raw request bytes into ExecutionRequests structure
func parseExecutionRequests(requests [][]byte) (*electra2.ExecutionRequests, error) {
	result := &electra2.ExecutionRequests{
		Deposits:       make([]*electra2.DepositRequest, 0),
		Withdrawals:    make([]*electra2.WithdrawalRequest, 0),
		Consolidations: make([]*electra2.ConsolidationRequest, 0),
	}

	for i, requestData := range requests {
		if len(requestData) == 0 {
			return nil, fmt.Errorf("empty request at index %d", i)
		}

		requestType := requestData[0]
		data := requestData[1:]

		switch requestType {
		case depositRequestType:
			deposits, err := parseDepositRequests(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse deposit requests at index %d: %w", i, err)
			}
			result.Deposits = append(result.Deposits, deposits...)

		case withdrawalRequestType:
			withdrawals, err := parseWithdrawalRequests(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse withdrawal requests at index %d: %w", i, err)
			}
			result.Withdrawals = append(result.Withdrawals, withdrawals...)

		case consolidationRequestType:
			consolidations, err := parseConsolidationRequests(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse consolidation requests at index %d: %w", i, err)
			}
			result.Consolidations = append(result.Consolidations, consolidations...)

		default:
			return nil, fmt.Errorf("unknown request type 0x%02x at index %d", requestType, i)
		}
	}

	return result, nil
}

// parseDepositRequests parses raw deposit request bytes into DepositRequest structs
func parseDepositRequests(data []byte) ([]*electra2.DepositRequest, error) {
	depositRequestSize := (&electra2.DepositRequest{}).SizeSSZ()
	if len(data)%depositRequestSize != 0 {
		return nil, fmt.Errorf("invalid deposit requests data length: %d, expected multiple of %d", len(data), depositRequestSize)
	}

	count := len(data) / depositRequestSize
	deposits := make([]*electra2.DepositRequest, 0, count)

	for i := 0; i < count; i++ {
		offset := i * depositRequestSize
		requestData := data[offset : offset+depositRequestSize]

		deposit, err := parseDepositRequest(requestData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse deposit request at index %d: %w", i, err)
		}
		deposits = append(deposits, deposit)
	}

	return deposits, nil
}

// parseDepositRequest parses a single deposit request from raw bytes
func parseDepositRequest(data []byte) (*electra2.DepositRequest, error) {
	deposit := &electra2.DepositRequest{}
	err := deposit.UnmarshalSSZ(data)
	if err != nil {
		return nil, err
	}
	return deposit, nil
}

// parseWithdrawalRequests parses raw withdrawal request bytes into WithdrawalRequest structs
func parseWithdrawalRequests(data []byte) ([]*electra2.WithdrawalRequest, error) {
	withdrawalRequestSize := (&electra2.WithdrawalRequest{}).SizeSSZ()
	if len(data)%withdrawalRequestSize != 0 {
		return nil, fmt.Errorf("invalid withdrawal requests data length: %d, expected multiple of %d", len(data), withdrawalRequestSize)
	}

	count := len(data) / withdrawalRequestSize
	withdrawals := make([]*electra2.WithdrawalRequest, 0, count)

	for i := 0; i < count; i++ {
		offset := i * withdrawalRequestSize
		requestData := data[offset : offset+withdrawalRequestSize]

		withdrawal, err := parseWithdrawalRequest(requestData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse withdrawal request at index %d: %w", i, err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	return withdrawals, nil
}

// parseWithdrawalRequest parses a single withdrawal request from raw bytes
func parseWithdrawalRequest(data []byte) (*electra2.WithdrawalRequest, error) {
	withdrawal := &electra2.WithdrawalRequest{}
	err := withdrawal.UnmarshalSSZ(data)
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

// parseConsolidationRequests parses raw consolidation request bytes into ConsolidationRequest structs
func parseConsolidationRequests(data []byte) ([]*electra2.ConsolidationRequest, error) {
	consolidationRequestSize := (&electra2.ConsolidationRequest{}).SizeSSZ()
	if len(data)%consolidationRequestSize != 0 {
		return nil, fmt.Errorf("invalid consolidation requests data length: %d, expected multiple of %d", len(data), consolidationRequestSize)
	}

	count := len(data) / consolidationRequestSize
	consolidations := make([]*electra2.ConsolidationRequest, 0, count)

	for i := 0; i < count; i++ {
		offset := i * consolidationRequestSize
		requestData := data[offset : offset+consolidationRequestSize]

		consolidation, err := parseConsolidationRequest(requestData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse consolidation request at index %d: %w", i, err)
		}
		consolidations = append(consolidations, consolidation)
	}

	return consolidations, nil
}

// parseConsolidationRequest parses a single consolidation request from raw bytes
func parseConsolidationRequest(data []byte) (*electra2.ConsolidationRequest, error) {
	consolidation := &electra2.ConsolidationRequest{}
	err := consolidation.UnmarshalSSZ(data)
	if err != nil {
		return nil, err
	}
	return consolidation, nil
}

func makeExecutionPayload(header *deneb.ExecutionPayloadHeader, executionData *ExecutionData) *api.VersionedExecutionPayload {
	return &api.VersionedExecutionPayload{
		Version: beaconSpec.DataVersionElectra,
		Electra: &deneb.ExecutionPayload{
			ParentHash:    header.ParentHash,
			FeeRecipient:  header.FeeRecipient,
			StateRoot:     header.StateRoot,
			ReceiptsRoot:  header.ReceiptsRoot,
			LogsBloom:     header.LogsBloom,
			PrevRandao:    header.PrevRandao,
			BlockNumber:   header.BlockNumber,
			GasLimit:      header.GasLimit,
			GasUsed:       header.GasUsed,
			Timestamp:     header.Timestamp,
			ExtraData:     header.ExtraData,
			BaseFeePerGas: header.BaseFeePerGas,
			BlockHash:     header.BlockHash,
			Transactions:  executionData.transactions,
			Withdrawals:   executionData.withdrawals,
			BlobGasUsed:   header.BlobGasUsed,
			ExcessBlobGas: header.ExcessBlobGas,
		},
	}
}

func validateBlindedBlockMatchesBid(signedBlindedBlock *beaconApi.VersionedSignedBlindedBeaconBlock, signedBid *spec.VersionedSignedBuilderBid) error {
	if signedBlindedBlock.Version != signedBid.Version {
		return errors.New("version mismatch between blinded block and bid")
	}

	blockHeaderHTR, err := signedBlindedBlock.Electra.Message.Body.ExecutionPayloadHeader.HashTreeRoot()
	if err != nil {
		return fmt.Errorf("failed to compute block header HTR: %w", err)
	}

	bidHeaderHTR, err := signedBid.Electra.Message.Header.HashTreeRoot()
	if err != nil {
		return fmt.Errorf("failed to compute bid header HTR: %w", err)
	}

	if blockHeaderHTR != bidHeaderHTR {
		return fmt.Errorf("invalid execution payload header")
	}

	if err := validateBlobCommitments(
		signedBlindedBlock.Electra.Message.Body.BlobKZGCommitments,
		signedBid.Electra.Message.BlobKZGCommitments,
	); err != nil {
		return err
	}

	blockExecutionHTR, err := signedBlindedBlock.Electra.Message.Body.ExecutionRequests.HashTreeRoot()
	if err != nil {
		return fmt.Errorf("failed to compute block execution requests HTR: %w", err)
	}

	bidExecutionHTR, err := signedBid.Electra.Message.ExecutionRequests.HashTreeRoot()
	if err != nil {
		return fmt.Errorf("failed to compute bid execution requests HTR: %w", err)
	}

	if blockExecutionHTR != bidExecutionHTR {
		return fmt.Errorf("invalid execution requests header")
	}

	return nil
}

// validateBlobCommitments ensures blob commitments match between block and bid
func validateBlobCommitments(
	blindedCommitments []deneb.KZGCommitment,
	bidCommitments []deneb.KZGCommitment,
) error {
	if len(blindedCommitments) != len(bidCommitments) {
		return fmt.Errorf("commitment count mismatch: block=%d, bid=%d",
			len(blindedCommitments), len(bidCommitments))
	}

	for i, blindedCommitment := range blindedCommitments {
		if blindedCommitment != bidCommitments[i] {
			return fmt.Errorf("commitment mismatch at index %d", i)
		}
	}

	return nil
}

// Convert signed blinded beacon block to full signed beacon block
func convertSignedBlindedBeaconBlockToBeaconBlock(blindedBlock *beaconApi.VersionedSignedBlindedBeaconBlock, blockAndBid *BlockAndBid) (*beaconApi.VersionedSignedProposal, error) {
	if err := validateBlindedBlockMatchesBid(blindedBlock, blockAndBid.Bid); err != nil {
		return nil, fmt.Errorf("mismatch blinded block and bid: %w", err)
	}

	payloadData := blockAndBid.Block
	if blindedBlock.Version != payloadData.Version {
		return nil, errors.New("version mismatch between blinded block and payload")
	}

	switch blindedBlock.Version {
	case beaconSpec.DataVersionElectra:
		bbHeaderHtr, err := blindedBlock.Electra.Message.Body.ExecutionPayloadHeader.HashTreeRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to compute electra blinding header HTR: %w", err)
		}
		payloadHeader, err := utils.PayloadToPayloadHeader(&api.VersionedExecutionPayload{
			Version: beaconSpec.DataVersionElectra,
			Electra: payloadData.Electra.ExecutionPayload,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to compute electra blinding payload header: %w", err)
		}
		payloadHeaderHtr, err := payloadHeader.Electra.HashTreeRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to compute electra blinding header HTR: %w", err)
		}
		if bbHeaderHtr != payloadHeaderHtr {
			return nil, errors.New("beacon-block and payload header mismatch")
		}
		for i, commitment := range blindedBlock.Electra.Message.Body.BlobKZGCommitments {
			if commitment != payloadData.Electra.BlobsBundle.Commitments[i] {
				return nil, fmt.Errorf("mismatched KZG commitment at index %d", i)
			}
		}

		signedBlockContents := &electra3.SignedBlockContents{
			SignedBlock: &electra2.SignedBeaconBlock{
				Message: &electra2.BeaconBlock{
					Slot:          blindedBlock.Electra.Message.Slot,
					ProposerIndex: blindedBlock.Electra.Message.ProposerIndex,
					ParentRoot:    blindedBlock.Electra.Message.ParentRoot,
					StateRoot:     blindedBlock.Electra.Message.StateRoot,
					Body: &electra2.BeaconBlockBody{
						RANDAOReveal:          blindedBlock.Electra.Message.Body.RANDAOReveal,
						ETH1Data:              blindedBlock.Electra.Message.Body.ETH1Data,
						Graffiti:              blindedBlock.Electra.Message.Body.Graffiti,
						ProposerSlashings:     blindedBlock.Electra.Message.Body.ProposerSlashings,
						AttesterSlashings:     blindedBlock.Electra.Message.Body.AttesterSlashings,
						Attestations:          blindedBlock.Electra.Message.Body.Attestations,
						Deposits:              blindedBlock.Electra.Message.Body.Deposits,
						VoluntaryExits:        blindedBlock.Electra.Message.Body.VoluntaryExits,
						SyncAggregate:         blindedBlock.Electra.Message.Body.SyncAggregate,
						ExecutionPayload:      payloadData.Electra.ExecutionPayload,
						BLSToExecutionChanges: blindedBlock.Electra.Message.Body.BLSToExecutionChanges,
						BlobKZGCommitments:    blindedBlock.Electra.Message.Body.BlobKZGCommitments,
						ExecutionRequests:     blindedBlock.Electra.Message.Body.ExecutionRequests,
					},
				},
				Signature: blindedBlock.Electra.Signature,
			},
			KZGProofs: payloadData.Electra.BlobsBundle.Proofs,
			Blobs:     payloadData.Electra.BlobsBundle.Blobs,
		}

		return &beaconApi.VersionedSignedProposal{
			Version: beaconSpec.DataVersionElectra,
			Electra: signedBlockContents,
		}, nil
	default:
		return nil, errors.New("unsupported version")
	}
}
