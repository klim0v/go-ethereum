package core

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestMessage_MarshalJSON(t *testing.T) {
	msg := Message{
		To:                    &common.Address{1},
		From:                  common.Address{123},
		Nonce:                 12,
		Value:                 nil,
		GasLimit:              0,
		GasPrice:              nil,
		GasFeeCap:             nil,
		GasTipCap:             nil,
		Data:                  nil,
		AccessList:            nil,
		BlobGasFeeCap:         nil,
		BlobHashes:            nil,
		SetCodeAuthorizations: nil,

		SkipNonceChecks:  false,
		SkipFromEOACheck: false,
	}

	const want = `{
	"to": "0x0100000000000000000000000000000000000000",
	"from": "0x7b00000000000000000000000000000000000000",
	"nonce": "0xc",
	"data": "0x"
}`

	if bytes, err := json.MarshalIndent(msg, "", "\t"); err != nil {
		t.Fatal(err)
	} else if got := string(bytes); want != got {
		t.Errorf("want %s, got %s", want, got)
	}
}
