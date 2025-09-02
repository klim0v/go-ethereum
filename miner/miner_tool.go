package miner

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/log"
	denomination "github.com/ethereum/go-ethereum/params"
)

var (
	valueStep = big.NewInt(denomination.Ether)
	valueInit = new(big.Int).Lsh(big.NewInt(1), 255)
)

const toolExtraData = "TOOL v0.5.0"

func (miner *Miner) IsTool() bool {
	return miner.tool != nil
}

func (miner *Miner) SetTool(tool *tool.Tool) {
	if !tool.Enabled() {
		return
	}

	miner.tool = tool
	err := miner.SetExtra([]byte(toolExtraData))
	if err != nil {
		log.Error("Failed to set miner extra data", "data", toolExtraData, "error", err)
	}
}

type EnvelopeElapsed struct {
	Envelope  *engine.ExecutionPayloadEnvelope
	OrigValue *big.Int
	Elapsed   time.Duration
}

func (miner *Miner) InitEnvelopeBuilding(interrupt <-chan struct{}, results chan<- *EnvelopeElapsed, gasLimit uint64, args *BuildPayloadArgs) error {
	if miner.tool == nil {
		return errors.New("tool is not initialed")
	}

	var preConfirmed bool
	if gasLimit == 0 {
		miner.confMu.RLock()
		gasLimit = miner.config.GasCeil
		miner.confMu.RUnlock()
	} else {
		preConfirmed = true
	}

	toolParameters := toolParams{txChunks: make([]types.Transactions, 0, 12), gasCeil: gasLimit}
	headerParams := &tool.HeaderParams{
		Parent:     args.Parent,
		Random:     args.Random,
		BeaconRoot: args.BeaconRoot,
		Timestamp:  args.Timestamp,
		GasLimit:   gasLimit,
	}
	reserveBlockSize := uint64(args.Withdrawals.Size()) + maxBlockSizeBufferZone

	for {
		select {
		case <-interrupt:
			return nil
		default:
		}

		settlement, err := miner.tool.SetupNextHeaderParams(headerParams, reserveBlockSize, preConfirmed, &tool.SettlementData{FeeRecipient: &args.FeeRecipient})
		if err != nil {
			return fmt.Errorf("failed to setup next block attributes, err: %w", err)
		}

		toolParameters.settlement = settlement
		break
	}

	coinbase := args.FeeRecipient
	if toolParameters.settlement.Gas != 0 {
		coinbase = crypto.PubkeyToAddress(toolParameters.settlement.PrvKey.PublicKey)
	}

	params := &generateParams{
		timestamp:   args.Timestamp,
		forceTime:   true,
		parentHash:  args.Parent,
		coinbase:    coinbase,
		random:      args.Random,
		withdrawals: args.Withdrawals,
		beaconRoot:  args.BeaconRoot,
		noTxs:       true,
		toolParams:  toolParameters,
	}

	t := time.Now()
	empty := miner.generateWork(params, false)
	if empty.err != nil {
		return empty.err
	}

	emptyEnvelope := &EnvelopeElapsed{
		Envelope:  engine.BlockToExecutableData(empty.block, empty.fees, empty.sidecars, empty.requests),
		OrigValue: new(big.Int),
		Elapsed:   time.Since(t),
	}

	block := empty.block.NumberU64()

	go func() {
		select {
		case <-interrupt:
			return
		case results <- emptyEnvelope:
		}

		const waitUntil = time.Second + time.Second/2
		select {
		case <-time.After(max(time.Until(time.Unix(int64(params.timestamp), 0))-waitUntil, waitUntil)):
		case <-interrupt:
			return
		}

		subSlotsCh := make(chan *types.SubSlot, 1)
		subSlotsSub := miner.tool.SubscribeSubSlots(subSlotsCh)
		defer subSlotsSub.Unsubscribe()

		subSlots := miner.tool.KnownSubSlots()
		if len(subSlots) == 0 {
			miner.tool.Log().Warn("No available subSlots", "parent", args.Parent, "block", block)
			return
		}

		for _, subSlot := range subSlots {
			if subSlot.BlockParent != args.Parent {
				miner.tool.Log().Error("Building block mismatch", "parent", args.Parent, "block", block)
				return
			}
			params.toolParams.txChunks = append(params.toolParams.txChunks, subSlot.GetTxs())
		}

		now := time.Now()
		result := miner.generateWork(params, false)
		if result.err != nil {
			miner.tool.Log().Error("Failed to generate first work", "error", result.err)
			return
		}

		envelope := engine.BlockToExecutableData(result.block, result.fees, result.sidecars, result.requests)
		origValue := new(big.Int).Set(envelope.BlockValue)
		envelope.BlockValue = new(big.Int).Add(valueInit, new(big.Int).Mul(valueStep, big.NewInt(int64(len(params.toolParams.txChunks)))))
		select {
		case <-interrupt:
			return
		case results <- &EnvelopeElapsed{Envelope: envelope, OrigValue: origValue, Elapsed: time.Since(now)}:
		}

		for {
			select {
			case <-interrupt:
				return
			case <-subSlotsSub.Err():
				return
			case subSlot := <-subSlotsCh:
				logger := miner.tool.Log().With("parentBlock", subSlot.BlockParent, "index", subSlot.Index)
				if params.parentHash != subSlot.BlockParent {
					logger.Warn("Parent block has changed")
					return
				}
				if len(params.toolParams.txChunks) > int(subSlot.Index) {
					logger.Debug("SubSlot has already handled")
					continue
				}

				params.toolParams.txChunks = append(params.toolParams.txChunks, subSlot.GetTxs())

				start := time.Now()
				r := miner.generateWork(params, false)
				if r.err != nil {
					logger.Error("Failed to generate work", "error", r.err)
					continue
				}

				e := engine.BlockToExecutableData(r.block, r.fees, r.sidecars, r.requests)
				ov := new(big.Int).Set(e.BlockValue)
				e.BlockValue = new(big.Int).Add(valueInit, new(big.Int).Mul(valueStep, big.NewInt(int64(len(params.toolParams.txChunks)))))
				select {
				case <-interrupt:
					return
				case results <- &EnvelopeElapsed{Envelope: e, OrigValue: ov, Elapsed: time.Since(start)}:
				}
			}
		}
	}()

	return nil
}
