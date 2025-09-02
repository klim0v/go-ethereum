package tool

import (
	"errors"

	v1 "github.com/attestantio/go-builder-client/api/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	TOOL1 = 1
)

const ProtocolName = "tool"

// ProtocolVersions are the supported versions of the `eth` protocol (first
// is primary).
var ProtocolVersions = []uint{TOOL1}

// protocolLengths are the number of implemented message corresponding to
// different protocol versions.
var protocolLengths = map[uint]uint64{TOOL1: 5}

var (
	errInvalidMsgCode = errors.New("invalid message code")
)

// Packet represents a p2p message in the `eth` protocol.
type Packet interface {
	Name() string // Name returns a string corresponding to the message type.
	Kind() byte   // Kind returns the message type.
}

const (
	CommitmentsMsg         = 0x00
	SubSlotsMsg            = 0x01
	GetToolTransactionsMsg = 0x02
	ToolTransactionsMsg    = 0x03
	ValidatorRegsMsg       = 0x04
)

type GetToolTransactionsPacket struct {
	RequestId uint64
	GetToolTransactionsRequest
}

type GetToolTransactionsRequest []common.Hash

func (*GetToolTransactionsRequest) Name() string { return "GetToolTransactionsMsg" }
func (*GetToolTransactionsRequest) Kind() byte   { return GetToolTransactionsMsg }

type ToolTransactionsPacket struct {
	RequestId uint64
	ToolTransactionsResponse
}

type ToolTransactionsResponse []*types.Transaction

func (*ToolTransactionsResponse) Name() string { return "ToolTransactions" }
func (*ToolTransactionsResponse) Kind() byte   { return ToolTransactionsMsg }

type CommitmentsPacket []*types.Commitment

func (*CommitmentsPacket) Name() string { return "Commitments" }
func (*CommitmentsPacket) Kind() byte   { return CommitmentsMsg }

type SubSlotPacket []*types.SubSlot

func (*SubSlotPacket) Name() string { return "SubSlots" }
func (*SubSlotPacket) Kind() byte   { return SubSlotsMsg }

type ValidatorRegistrations []*v1.ValidatorRegistration

func (*ValidatorRegistrations) Name() string { return "ValidatorRegs" }
func (*ValidatorRegistrations) Kind() byte   { return ValidatorRegsMsg }
