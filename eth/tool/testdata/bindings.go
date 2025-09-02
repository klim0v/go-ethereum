// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package testdata

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = bytes.Equal
	_ = errors.New
	_ = big.NewInt
	_ = common.Big1
	_ = types.BloomLookup
	_ = abi.ConvertType
)

// MockAttestationMetaData contains all meta data concerning the MockAttestation contract.
var MockAttestationMetaData = bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_hash\",\"type\":\"bytes32\"}],\"name\":\"publishNodeIDAttestation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_hash\",\"type\":\"bytes32\"}],\"name\":\"recallNodeIDAttestation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_hash\",\"type\":\"bytes32\"}],\"name\":\"verifyNodeIDAttestation\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	ID:  "040fae46d2c312b60c1f1091fc62951337",
}

// MockAttestation is an auto generated Go binding around an Ethereum contract.
type MockAttestation struct {
	abi abi.ABI
}

// NewMockAttestation creates a new instance of MockAttestation.
func NewMockAttestation() *MockAttestation {
	parsed, err := MockAttestationMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &MockAttestation{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *MockAttestation) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackPublishNodeIDAttestation is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd0728f61.
//
// Solidity: function publishNodeIDAttestation(bytes32 _hash) returns()
func (mockAttestation *MockAttestation) PackPublishNodeIDAttestation(hash [32]byte) []byte {
	enc, err := mockAttestation.abi.Pack("publishNodeIDAttestation", hash)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackRecallNodeIDAttestation is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xefa05c84.
//
// Solidity: function recallNodeIDAttestation(bytes32 _hash) returns()
func (mockAttestation *MockAttestation) PackRecallNodeIDAttestation(hash [32]byte) []byte {
	enc, err := mockAttestation.abi.Pack("recallNodeIDAttestation", hash)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackVerifyNodeIDAttestation is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa9cf4e45.
//
// Solidity: function verifyNodeIDAttestation(bytes32 _hash) view returns(bool)
func (mockAttestation *MockAttestation) PackVerifyNodeIDAttestation(hash [32]byte) []byte {
	enc, err := mockAttestation.abi.Pack("verifyNodeIDAttestation", hash)
	if err != nil {
		panic(err)
	}
	return enc
}

// UnpackVerifyNodeIDAttestation is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa9cf4e45.
//
// Solidity: function verifyNodeIDAttestation(bytes32 _hash) view returns(bool)
func (mockAttestation *MockAttestation) UnpackVerifyNodeIDAttestation(data []byte) (bool, error) {
	out, err := mockAttestation.abi.Unpack("verifyNodeIDAttestation", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, err
}

// MockStateMetaData contains all meta data concerning the MockState contract.
var MockStateMetaData = bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_stateVarA\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"_stateVarB\",\"type\":\"bytes32\"},{\"internalType\":\"addresspayable\",\"name\":\"destroyBeneficiary\",\"type\":\"address\"}],\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"Create2EmptyBytecode\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"EthSendFailed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FailedDeployment\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"res\",\"type\":\"bytes\"}],\"name\":\"ForwardCallFailed\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"balance\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"needed\",\"type\":\"uint256\"}],\"name\":\"InsufficientBalance\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"val\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"EthSent\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Fallback\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Received\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"val\",\"type\":\"bytes32\"}],\"name\":\"StateVarAChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"val\",\"type\":\"bytes32\"}],\"name\":\"StateVarBChanged\",\"type\":\"event\"},{\"stateMutability\":\"nonpayable\",\"type\":\"fallback\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"salt\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"bytecode\",\"type\":\"bytes\"}],\"name\":\"deploy\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"destroySelf\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"target\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"val\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"forwardCall\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"addresspayable\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"sendEth\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"val\",\"type\":\"bytes32\"}],\"name\":\"writeStateVarA\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"val\",\"type\":\"bytes32\"}],\"name\":\"writeStateVarB\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
	ID:  "eae9944a8573fa8427bf3bdcb491b4db25",
}

// MockState is an auto generated Go binding around an Ethereum contract.
type MockState struct {
	abi abi.ABI
}

// NewMockState creates a new instance of MockState.
func NewMockState() *MockState {
	parsed, err := MockStateMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &MockState{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *MockState) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackConstructor is the Go binding used to pack the parameters required for
// contract deployment.
//
// Solidity: constructor(bytes32 _stateVarA, bytes32 _stateVarB, address destroyBeneficiary) payable returns()
func (mockState *MockState) PackConstructor(_stateVarA [32]byte, _stateVarB [32]byte, destroyBeneficiary common.Address) []byte {
	enc, err := mockState.abi.Pack("", _stateVarA, _stateVarB, destroyBeneficiary)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackDeploy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x66cfa057.
//
// Solidity: function deploy(uint256 amount, bytes32 salt, bytes bytecode) returns(address)
func (mockState *MockState) PackDeploy(amount *big.Int, salt [32]byte, bytecode []byte) []byte {
	enc, err := mockState.abi.Pack("deploy", amount, salt, bytecode)
	if err != nil {
		panic(err)
	}
	return enc
}

// UnpackDeploy is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x66cfa057.
//
// Solidity: function deploy(uint256 amount, bytes32 salt, bytes bytecode) returns(address)
func (mockState *MockState) UnpackDeploy(data []byte) (common.Address, error) {
	out, err := mockState.abi.Unpack("deploy", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, err
}

// PackDestroySelf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x4de48708.
//
// Solidity: function destroySelf(address addr) returns()
func (mockState *MockState) PackDestroySelf(addr common.Address) []byte {
	enc, err := mockState.abi.Pack("destroySelf", addr)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackForwardCall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6effec50.
//
// Solidity: function forwardCall(address target, uint256 val, bytes data) returns(bytes)
func (mockState *MockState) PackForwardCall(target common.Address, val *big.Int, data []byte) []byte {
	enc, err := mockState.abi.Pack("forwardCall", target, val, data)
	if err != nil {
		panic(err)
	}
	return enc
}

// UnpackForwardCall is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6effec50.
//
// Solidity: function forwardCall(address target, uint256 val, bytes data) returns(bytes)
func (mockState *MockState) UnpackForwardCall(data []byte) ([]byte, error) {
	out, err := mockState.abi.Unpack("forwardCall", data)
	if err != nil {
		return *new([]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)
	return out0, err
}

// PackSendEth is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x34368602.
//
// Solidity: function sendEth(uint256 value, address to) payable returns()
func (mockState *MockState) PackSendEth(value *big.Int, to common.Address) []byte {
	enc, err := mockState.abi.Pack("sendEth", value, to)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackWriteStateVarA is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc773413d.
//
// Solidity: function writeStateVarA(bytes32 val) returns()
func (mockState *MockState) PackWriteStateVarA(val [32]byte) []byte {
	enc, err := mockState.abi.Pack("writeStateVarA", val)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackWriteStateVarB is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xac84bb09.
//
// Solidity: function writeStateVarB(bytes32 val) returns()
func (mockState *MockState) PackWriteStateVarB(val [32]byte) []byte {
	enc, err := mockState.abi.Pack("writeStateVarB", val)
	if err != nil {
		panic(err)
	}
	return enc
}

// MockStateEthSent represents a EthSent event raised by the MockState contract.
type MockStateEthSent struct {
	Val *big.Int
	To  common.Address
	Raw *types.Log // Blockchain specific contextual infos
}

const MockStateEthSentEventName = "EthSent"

// ContractEventName returns the user-defined event name.
func (MockStateEthSent) ContractEventName() string {
	return MockStateEthSentEventName
}

// UnpackEthSentEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event EthSent(uint256 val, address to)
func (mockState *MockState) UnpackEthSentEvent(log *types.Log) (*MockStateEthSent, error) {
	event := "EthSent"
	if log.Topics[0] != mockState.abi.Events[event].ID {
		return nil, errors.New("event signature mismatch")
	}
	out := new(MockStateEthSent)
	if len(log.Data) > 0 {
		if err := mockState.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range mockState.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}

// MockStateFallback represents a Fallback event raised by the MockState contract.
type MockStateFallback struct {
	Raw *types.Log // Blockchain specific contextual infos
}

const MockStateFallbackEventName = "Fallback"

// ContractEventName returns the user-defined event name.
func (MockStateFallback) ContractEventName() string {
	return MockStateFallbackEventName
}

// UnpackFallbackEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Fallback()
func (mockState *MockState) UnpackFallbackEvent(log *types.Log) (*MockStateFallback, error) {
	event := "Fallback"
	if log.Topics[0] != mockState.abi.Events[event].ID {
		return nil, errors.New("event signature mismatch")
	}
	out := new(MockStateFallback)
	if len(log.Data) > 0 {
		if err := mockState.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range mockState.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}

// MockStateReceived represents a Received event raised by the MockState contract.
type MockStateReceived struct {
	Raw *types.Log // Blockchain specific contextual infos
}

const MockStateReceivedEventName = "Received"

// ContractEventName returns the user-defined event name.
func (MockStateReceived) ContractEventName() string {
	return MockStateReceivedEventName
}

// UnpackReceivedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Received()
func (mockState *MockState) UnpackReceivedEvent(log *types.Log) (*MockStateReceived, error) {
	event := "Received"
	if log.Topics[0] != mockState.abi.Events[event].ID {
		return nil, errors.New("event signature mismatch")
	}
	out := new(MockStateReceived)
	if len(log.Data) > 0 {
		if err := mockState.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range mockState.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}

// MockStateStateVarAChanged represents a StateVarAChanged event raised by the MockState contract.
type MockStateStateVarAChanged struct {
	Val [32]byte
	Raw *types.Log // Blockchain specific contextual infos
}

const MockStateStateVarAChangedEventName = "StateVarAChanged"

// ContractEventName returns the user-defined event name.
func (MockStateStateVarAChanged) ContractEventName() string {
	return MockStateStateVarAChangedEventName
}

// UnpackStateVarAChangedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event StateVarAChanged(bytes32 val)
func (mockState *MockState) UnpackStateVarAChangedEvent(log *types.Log) (*MockStateStateVarAChanged, error) {
	event := "StateVarAChanged"
	if log.Topics[0] != mockState.abi.Events[event].ID {
		return nil, errors.New("event signature mismatch")
	}
	out := new(MockStateStateVarAChanged)
	if len(log.Data) > 0 {
		if err := mockState.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range mockState.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}

// MockStateStateVarBChanged represents a StateVarBChanged event raised by the MockState contract.
type MockStateStateVarBChanged struct {
	Val [32]byte
	Raw *types.Log // Blockchain specific contextual infos
}

const MockStateStateVarBChangedEventName = "StateVarBChanged"

// ContractEventName returns the user-defined event name.
func (MockStateStateVarBChanged) ContractEventName() string {
	return MockStateStateVarBChangedEventName
}

// UnpackStateVarBChangedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event StateVarBChanged(bytes32 val)
func (mockState *MockState) UnpackStateVarBChangedEvent(log *types.Log) (*MockStateStateVarBChanged, error) {
	event := "StateVarBChanged"
	if log.Topics[0] != mockState.abi.Events[event].ID {
		return nil, errors.New("event signature mismatch")
	}
	out := new(MockStateStateVarBChanged)
	if len(log.Data) > 0 {
		if err := mockState.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range mockState.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}

// UnpackError attempts to decode the provided error data using user-defined
// error definitions.
func (mockState *MockState) UnpackError(raw []byte) (any, error) {
	if bytes.Equal(raw[:4], mockState.abi.Errors["Create2EmptyBytecode"].ID.Bytes()[:4]) {
		return mockState.UnpackCreate2EmptyBytecodeError(raw[4:])
	}
	if bytes.Equal(raw[:4], mockState.abi.Errors["EthSendFailed"].ID.Bytes()[:4]) {
		return mockState.UnpackEthSendFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], mockState.abi.Errors["FailedDeployment"].ID.Bytes()[:4]) {
		return mockState.UnpackFailedDeploymentError(raw[4:])
	}
	if bytes.Equal(raw[:4], mockState.abi.Errors["ForwardCallFailed"].ID.Bytes()[:4]) {
		return mockState.UnpackForwardCallFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], mockState.abi.Errors["InsufficientBalance"].ID.Bytes()[:4]) {
		return mockState.UnpackInsufficientBalanceError(raw[4:])
	}
	return nil, errors.New("Unknown error")
}

// MockStateCreate2EmptyBytecode represents a Create2EmptyBytecode error raised by the MockState contract.
type MockStateCreate2EmptyBytecode struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error Create2EmptyBytecode()
func MockStateCreate2EmptyBytecodeErrorID() common.Hash {
	return common.HexToHash("0x4ca249dcffe41558ef8b961d71c905e4fa4317a1663f377b9610642e4e0abdb6")
}

// UnpackCreate2EmptyBytecodeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error Create2EmptyBytecode()
func (mockState *MockState) UnpackCreate2EmptyBytecodeError(raw []byte) (*MockStateCreate2EmptyBytecode, error) {
	out := new(MockStateCreate2EmptyBytecode)
	if err := mockState.abi.UnpackIntoInterface(out, "Create2EmptyBytecode", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MockStateEthSendFailed represents a EthSendFailed error raised by the MockState contract.
type MockStateEthSendFailed struct {
	To    common.Address
	Value *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error EthSendFailed(address to, uint256 value)
func MockStateEthSendFailedErrorID() common.Hash {
	return common.HexToHash("0xdfb217179b9539bf85a4ee2de3b9d2c7272c622a02a71eb2ae63480b5a8b9b96")
}

// UnpackEthSendFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error EthSendFailed(address to, uint256 value)
func (mockState *MockState) UnpackEthSendFailedError(raw []byte) (*MockStateEthSendFailed, error) {
	out := new(MockStateEthSendFailed)
	if err := mockState.abi.UnpackIntoInterface(out, "EthSendFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MockStateFailedDeployment represents a FailedDeployment error raised by the MockState contract.
type MockStateFailedDeployment struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error FailedDeployment()
func MockStateFailedDeploymentErrorID() common.Hash {
	return common.HexToHash("0xb06ebf3d5067824a3fe5d5ba19471e035a7de6c88dac362c77b162830a5b9093")
}

// UnpackFailedDeploymentError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error FailedDeployment()
func (mockState *MockState) UnpackFailedDeploymentError(raw []byte) (*MockStateFailedDeployment, error) {
	out := new(MockStateFailedDeployment)
	if err := mockState.abi.UnpackIntoInterface(out, "FailedDeployment", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MockStateForwardCallFailed represents a ForwardCallFailed error raised by the MockState contract.
type MockStateForwardCallFailed struct {
	Res []byte
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ForwardCallFailed(bytes res)
func MockStateForwardCallFailedErrorID() common.Hash {
	return common.HexToHash("0x151cef583148a43b3eb83ebc059e6e1a970fa8382b2da68c318ae1e0ada6c67a")
}

// UnpackForwardCallFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ForwardCallFailed(bytes res)
func (mockState *MockState) UnpackForwardCallFailedError(raw []byte) (*MockStateForwardCallFailed, error) {
	out := new(MockStateForwardCallFailed)
	if err := mockState.abi.UnpackIntoInterface(out, "ForwardCallFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MockStateInsufficientBalance represents a InsufficientBalance error raised by the MockState contract.
type MockStateInsufficientBalance struct {
	Balance *big.Int
	Needed  *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func MockStateInsufficientBalanceErrorID() common.Hash {
	return common.HexToHash("0xcf4791818fba6e019216eb4864093b4947f674afada5d305e57d598b641dad1d")
}

// UnpackInsufficientBalanceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func (mockState *MockState) UnpackInsufficientBalanceError(raw []byte) (*MockStateInsufficientBalance, error) {
	out := new(MockStateInsufficientBalance)
	if err := mockState.abi.UnpackIntoInterface(out, "InsufficientBalance", raw); err != nil {
		return nil, err
	}
	return out, nil
}
