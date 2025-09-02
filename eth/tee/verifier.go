package tee

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

type Verifier interface {
	Verify(bc *core.BlockChain, identification common.Hash) (bool, error)
}

type Chain struct {
	abi     abi.ABI
	address common.Address
}

func NewChain(address common.Address) *Chain {
	const attestationABI = `[{"inputs":[{"internalType":"bytes32","name":"_hash","type":"bytes32"}],"name":"verifyNodeIDAttestation","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"}]`
	contractAbi, err := abi.JSON(strings.NewReader(attestationABI))
	if err != nil {
		panic(err)
	}

	return &Chain{
		abi:     contractAbi,
		address: address,
	}
}

func (v *Chain) Verify(bc *core.BlockChain, identification common.Hash) (bool, error) {
	header := bc.CurrentSafeBlock()
	if header == nil {
		return false, errors.New("safe block not found")
	}
	if !bc.HasState(header.Root) {
		return false, errors.New("state not ready")
	}
	stateDB, err := bc.StateAt(header.Root)
	if err != nil {
		return false, err
	}
	const method = "verifyNodeIDAttestation"
	data, err := v.abi.Pack(method, identification)
	if err != nil {
		return false, err
	}
	evm := vm.NewEVM(core.NewEVMBlockContext(header, bc, nil), stateDB, bc.Config(), *bc.GetVMConfig())
	ret, _, err := evm.StaticCall(common.Address{}, v.address, data, params.MaxTxGas)
	if err != nil {
		return false, err
	}
	res, err := v.abi.Unpack(method, ret)
	if err != nil {
		return false, err
	}
	return res[0].(bool), nil
}

type AlwaysTrue struct{}

func (AlwaysTrue) Verify(*core.BlockChain, common.Hash) (bool, error) { return true, nil }

type PassList struct {
	PassList []common.Hash
}

func (v PassList) Verify(_ *core.BlockChain, nodeID common.Hash) (bool, error) {
	for _, id := range v.PassList {
		if nodeID == id {
			return true, nil
		}
	}
	return false, nil
}
