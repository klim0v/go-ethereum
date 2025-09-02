package tool

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type SettlementData struct {
	FeeRecipient *common.Address
	Data         []byte
}

type Settlement struct {
	PrvKey *ecdsa.PrivateKey
	Gas    uint64
	TxData *SettlementData
}

func (s *Settlement) MakeTx(signer types.Signer, baseFee *big.Int, stateDB *state.StateDB) (*types.Transaction, error) {
	sender := crypto.PubkeyToAddress(s.PrvKey.PublicKey)
	balance := stateDB.GetBalance(sender).ToBig()
	txFees := new(big.Int).Mul(baseFee, big.NewInt(int64(s.Gas)))
	value := new(big.Int).Sub(balance, txFees)
	if value.Sign() == -1 {
		return nil, fmt.Errorf("not enough balance, %s < %s", balance, txFees)
	}

	if s.TxData == nil {
		return nil, fmt.Errorf("invalid tx data")
	}

	tx, err := types.SignNewTx(s.PrvKey, signer, &types.DynamicFeeTx{
		ChainID:   signer.ChainID(),
		Nonce:     stateDB.GetNonce(sender),
		GasTipCap: common.Big0,
		GasFeeCap: baseFee,
		Gas:       s.Gas,
		To:        s.TxData.FeeRecipient,
		Value:     value,
		Data:      s.TxData.Data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	return tx, nil
}
