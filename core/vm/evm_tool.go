package vm

import "github.com/ethereum/go-ethereum/params"

func (evm *EVM) GetPrecompiles() PrecompiledContracts {
	return evm.precompiles
}

func (evm *EVM) GetRules() params.Rules {
	return evm.chainRules
}
