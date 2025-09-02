package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tool"
	"github.com/ethereum/go-ethereum/eth/tool/testdata"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"testing"
)

func TestEthAPIBackend_DoToolCall(t *testing.T) {
	b := initBackend(false)
	b.eth.tool = tool.New(tool.Config{Enabled: true}, b.eth.blockchain, func() bool { return true })

	parent := b.eth.blockchain.CurrentBlock()
	_, err := b.eth.tool.SetupNextHeaderParams(&tool.HeaderParams{
		Timestamp:  parent.Time + 12,
		Parent:     parent.Hash(),
		Random:     common.Hash{},
		BeaconRoot: &common.MaxHash,
		GasLimit:   45_000_000,
	}, 0, true, &tool.SettlementData{FeeRecipient: &common.MaxAddress})
	if err != nil {
		t.Fatal(err)
	}

	contractAddress := crypto.CreateAddress(address, 0)
	tx0 := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce:    0,
		Value:    big.NewInt(0),
		Gas:      500_000,
		GasPrice: big.NewInt(params.InitialBaseFee),
		Data:     common.FromHex(testdata.MockAttestationByteCodeHex),
	})
	commitment0, err := b.eth.tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx0}}, true)
	if err != nil {
		t.Fatal(err)
	}
	subSlot0 := &types.SubSlot{
		Txs:         commitment0.Txs,
		BlockParent: commitment0.BlockParent,
		BlockTime:   commitment0.BlockTime,
		Index:       0,
	}
	unknown, old := b.eth.tool.RegisterSubSlotIfNew(subSlot0)
	if len(unknown) != 0 || old {
		t.Error(unknown, old)
	}
	processed, err := b.eth.tool.HandleSubSlot(subSlot0)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Error("SubSlot0 not processed")
	}

	maxGasLimit := hexutil.Uint64(params.MaxTxGas)
	{
		dryWrite0 := testdata.NewMockAttestation().PackPublishNodeIDAttestation(common.Hash{111})
		executionResult0, err := b.eth.tool.DoCall(t.Context(), &ethapi.TransactionArgs{
			From: &address,
			To:   &contractAddress,
			Data: (*hexutil.Bytes)(&dryWrite0),
			Gas:  &maxGasLimit,
		}, 0, 1, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if executionResult0.Failed() {
			t.Error(executionResult0.Err)
		}

		checkData0 := testdata.NewMockAttestation().PackVerifyNodeIDAttestation(common.Hash{111})
		executionResult, err := b.eth.tool.DoCall(t.Context(), &ethapi.TransactionArgs{
			From: &address,
			To:   &contractAddress,
			Data: (*hexutil.Bytes)(&checkData0),
			Gas:  &maxGasLimit,
		}, 0, 1, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if executionResult.Failed() {
			t.Error(executionResult.Err)
		}
		result, err := testdata.NewMockAttestation().UnpackVerifyNodeIDAttestation(executionResult.ReturnData)
		if err != nil {
			t.Fatal(err)
		}
		if result {
			t.Error("Expected false, got true")
		}
	}

	write1 := testdata.NewMockAttestation().PackPublishNodeIDAttestation(common.Hash{111})
	tx1 := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce:    1,
		Value:    big.NewInt(0),
		Gas:      500_000,
		GasPrice: big.NewInt(params.InitialBaseFee),
		Data:     write1,
		To:       &contractAddress,
	})
	commitment1, err := b.eth.tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx1}}, true)
	if err != nil {
		t.Fatal(err)
	}
	subSlot1 := &types.SubSlot{
		Txs:         commitment1.Txs,
		BlockParent: commitment1.BlockParent,
		BlockTime:   commitment1.BlockTime,
		Index:       1,
	}
	unknown, old = b.eth.tool.RegisterSubSlotIfNew(subSlot1)
	if len(unknown) != 0 || old {
		t.Error(unknown, old)
	}
	processed, err = b.eth.tool.HandleSubSlot(subSlot1)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Error("SubSlot1 not processed")
	}

	checkData := testdata.NewMockAttestation().PackVerifyNodeIDAttestation(common.Hash{111})
	executionResult, err := b.eth.tool.DoCall(t.Context(), &ethapi.TransactionArgs{
		From: &address,
		To:   &contractAddress,
		Data: (*hexutil.Bytes)(&checkData),
		Gas:  &maxGasLimit,
	}, 0, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if executionResult.Failed() {
		t.Error(executionResult.Err)
	}
	result, err := testdata.NewMockAttestation().UnpackVerifyNodeIDAttestation(executionResult.ReturnData)
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("Expected true, got false")
	}
}
