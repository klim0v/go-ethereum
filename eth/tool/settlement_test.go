package tool

import (
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tool/testdata"
	"github.com/ethereum/go-ethereum/params"
)

func TestSettlement_MakeTx(t *testing.T) {
	t.Parallel()

	bc, faucetKey := prepareBlockchain(t)
	tool := New(DefaultConfig, bc, func() bool { return true })
	settlement, err := tool.SetupNextHeaderParams(&HeaderParams{
		Timestamp:  bc.head.Time + 12,
		Parent:     bc.head.Hash(),
		Random:     common.Hash{},
		BeaconRoot: nil,
	}, 0, true, &SettlementData{
		FeeRecipient: nil,
		Data:         slices.Concat(common.FromHex(testdata.MockStateByteCodeHex), common.LeftPadBytes(nil, 32), common.LeftPadBytes(nil, 32), common.LeftPadBytes(nil, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settlement == nil {
		t.Fatal("empty settlement")
	}
	if settlement.TxData == nil {
		t.Fatal("empty settlement data")
	}
	if settlement.Gas == 0 {
		t.Fatal("empty gas")
	}
	_, total, reserved := tool.blockChain.executor.SizeAndGasLeft()
	if reserved != settlement.Gas {
		t.Fatalf("reserved gas is not equal to settlement gas, want %d, got %d", settlement.Gas, reserved)
	}

	select {
	case <-tool.Ready():
	case <-time.After(time.Second):
		t.Fatal("tool not ready")
	}

	signer := tool.blockChain.executor.GetSigner()
	transaction := transferTx(faucetKey, signer, crypto.PubkeyToAddress(settlement.PrvKey.PublicKey), 0, big.NewInt(1e18))

	commitment, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{transaction}}, true)
	if err != nil {
		t.Fatal(err)
	}
	subSlot, err := tool.assembler.newSubSlot(params.MaxBlockSize, total, total, []*types.Commitment{commitment}, tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	tool.RegisterSubSlotIfNew(subSlot)
	processed, err := tool.HandleSubSlot(subSlot)
	if err != nil || !processed {
		t.Fatalf("failed to handle subSlot, err: %v, processed: %v", err, processed)
	}
	cloneExecutor := tool.blockChain.executor.(*executor).Clone()

	if crypto.PubkeyToAddress(settlement.PrvKey.PublicKey) != cloneExecutor.evm.Context.Coinbase {
		t.Fatalf("clone executor is not equal to settlement executor")
	}

	tx, err := settlement.MakeTx(cloneExecutor.signer, cloneExecutor.baseFee, cloneExecutor.statedb)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := core.TransactionToMessage(tx, cloneExecutor.signer, cloneExecutor.baseFee)
	if err != nil {
		t.Fatal(err)
	}
	result, err := core.ApplyMessage(cloneExecutor.evm, msg, &cloneExecutor.gasPool)
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed() {
		t.Fatal(result.Err)
	}
	if result.MaxUsedGas != reserved {
		t.Fatalf("max used gas is not equal to reserved gas, want %d, got %d", reserved, result.MaxUsedGas)
	}
	if cloneExecutor.statedb.GetBalance(cloneExecutor.evm.Context.Coinbase).Sign() != 0 {
		t.Fatalf("coinbase address is not zero")
	}
	t.Log(tx.Size())
}
