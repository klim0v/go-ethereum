package core

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

func ApplyToolTransaction(evm *vm.EVM, gp *GasPool, tx *types.Transaction, signer types.Signer, baseFee *big.Int) (*ExecutionResult, error) {
	msg, err := TransactionToMessage(tx, signer, baseFee)
	if err != nil {
		return nil, err
	}
	if msg.From == evm.Context.Coinbase {
		return nil, errors.New("tx from coinbase not allowed")
	}
	for _, address := range tx.SetCodeAuthorities() {
		if address == evm.Context.Coinbase {
			return nil, errors.New("set code for coinbase not allowed")
		}
	}

	if evm.Config.Tracer.OnTxStart != nil {
		evm.Config.Tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
	}

	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	if result.Failed() {
		return nil, result.Unwrap()
	}

	return result, nil
}

// MarshalJSON marshals as JSON.
func (m Message) MarshalJSON() ([]byte, error) {
	type Message struct {
		To                    *common.Address              `json:"to,omitempty"`
		From                  common.Address               `json:"from,omitempty"`
		Nonce                 hexutil.Uint64               `json:"nonce,omitempty"`
		Value                 *hexutil.Big                 `json:"value,omitempty"`
		GasLimit              hexutil.Uint64               `json:"gasLimit,omitempty"`
		GasPrice              *hexutil.Big                 `json:"gasPrice,omitempty"`
		GasFeeCap             *hexutil.Big                 `json:"gasFeeCap,omitempty"`
		GasTipCap             *hexutil.Big                 `json:"gasTipCap,omitempty"`
		Data                  *hexutil.Bytes               `json:"data,omitempty"`
		AccessList            types.AccessList             `json:"accessList,omitempty"`
		BlobGasFeeCap         *hexutil.Big                 `json:"blobGasFeeCap,omitempty"`
		BlobHashes            []common.Hash                `json:"blobHashes,omitempty"`
		SetCodeAuthorizations []types.SetCodeAuthorization `json:"setCodeAuthorizations,omitempty"`
	}
	var enc Message
	enc.To = m.To
	enc.From = m.From
	enc.Nonce = hexutil.Uint64(m.Nonce)
	enc.Value = (*hexutil.Big)(m.Value)
	enc.GasLimit = hexutil.Uint64(m.GasLimit)
	enc.GasPrice = (*hexutil.Big)(m.GasPrice)
	enc.GasFeeCap = (*hexutil.Big)(m.GasFeeCap)
	enc.GasTipCap = (*hexutil.Big)(m.GasTipCap)
	enc.Data = (*hexutil.Bytes)(&m.Data)
	enc.AccessList = m.AccessList
	enc.BlobGasFeeCap = (*hexutil.Big)(m.BlobGasFeeCap)
	enc.BlobHashes = m.BlobHashes
	enc.SetCodeAuthorizations = m.SetCodeAuthorizations
	return json.Marshal(&enc)
}
