package tool

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const defaultGasCeil = 36_000_000

func init() {
	DefaultConfig.Enabled = true
}

func prepareTool(t interface{ Fatal(args ...any) }) (_ *Tool, faucetKey *ecdsa.PrivateKey) {
	bc, faucetKey := prepareBlockchain(t)
	tool := New(DefaultConfig, bc, func() bool { return true })
	_, err := tool.SetupNextHeaderParams(&HeaderParams{
		Timestamp:  bc.head.Time + 12,
		Parent:     bc.head.Hash(),
		Random:     common.Hash{},
		BeaconRoot: nil,
	}, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-tool.Ready():
	case <-time.After(time.Second):
		t.Fatal("tool not ready")
	}

	return tool, faucetKey
}

func TestTool_ProcessBundle_simple(t *testing.T) {
	t.Parallel()

	tool, faucetKey := prepareTool(t)
	signer := tool.blockChain.executor.GetSigner()

	tx := transferTx(faucetKey, signer, common.Address{123}, 0, new(big.Int))
	commitment, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if commitment == nil {
		t.Fatal("empty commitment")
	}
	if commitment.ExecutedTxsCount() != 1 {
		t.Errorf("executedTxs want 1, got %d", commitment.ExecutedTxsCount())
	}
	txs, unknown := tool.GetTxs(commitment.Txs)
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !commitment.SetTxs(txs) {
		t.Errorf("failed to resolve txs")
	}

	for _, change := range commitment.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}
}

func TestTool_ProcessBundle_basedOn1(t *testing.T) {
	t.Parallel()

	tool, faucetKey := prepareTool(t)
	signer := tool.blockChain.executor.GetSigner()

	senderKey1, err := crypto.ToECDSA(hexutil.MustDecode("0xdc599867fc513f8f5e2c2c9c489cde5e71362d1d9ec6e693e0de063236ed1240"))
	if err != nil {
		t.Fatal(err)
	}
	senderAddr1 := crypto.PubkeyToAddress(senderKey1.PublicKey) // 0xDEa37aE77CefF1F350c25E94c52EB4f65FEdf784

	tx1 := transferTx(faucetKey, signer, senderAddr1, 0, big.NewInt(1e18))
	tx2 := transferTx(faucetKey, signer, crypto.CreateAddress(senderAddr1, 0), 1, big.NewInt(1e18), common.FromHex("d0728f610000000000000000000000000000000000000000000000000000000000000003")...)
	commitment1, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx1, tx2}}, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, change := range commitment1.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}

	bundle2 := &types.Bundle{BasedCommitments: []common.Hash{commitment1.ID()}, NewTxs: storageTxs(senderKey1, signer, 0)}
	commitment2, err := tool.ProcessBundle(bundle2, true)
	if err != nil {
		t.Fatal(err)
	}
	txs, unknown := tool.GetTxs(commitment2.Txs)
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !commitment2.SetTxs(txs) {
		t.Errorf("failed to resolve txs")
	}
	if commitment2.ExecutedTxsCount() != 9 {
		t.Errorf("executedTxs want 9, got %d", commitment2.ExecutedTxsCount())
	}
	t.Log(commitment2.StateDiff.String())

	for _, change := range commitment2.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}
}

func TestTool_HandleSubSlot(t *testing.T) {
	t.Parallel()

	tool, faucetKey := prepareTool(t)
	signer := tool.blockChain.executor.GetSigner()

	senderKey1, err := crypto.ToECDSA(hexutil.MustDecode("0xdc599867fc513f8f5e2c2c9c489cde5e71362d1d9ec6e693e0de063236ed1240"))
	if err != nil {
		t.Fatal(err)
	}
	senderAddr1 := crypto.PubkeyToAddress(senderKey1.PublicKey) // 0xDEa37aE77CefF1F350c25E94c52EB4f65FEdf784

	senderKey2, err := crypto.ToECDSA(hexutil.MustDecode("0x29738ba0c1a4397d6a65f292eee07f02df8e58d41594ba2be3cf84ce0fc58169"))
	if err != nil {
		t.Fatal(err)
	}
	senderAddr2 := crypto.PubkeyToAddress(senderKey2.PublicKey) // 0x9Cde915E44f74403F4e23b3257f10385597d7873

	tx1 := transferTx(faucetKey, signer, senderAddr1, 0, big.NewInt(1e18))
	tx2 := transferTx(faucetKey, signer, senderAddr2, 1, big.NewInt(1e18))
	commitment0, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx1, tx2}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.GetPendingCommitments()) != 1 {
		t.Fatal("commitment pool is empty")
	}
	for _, change := range commitment0.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}

	subSlotCh := make(chan *types.SubSlot, 1)
	subscribeSubSlots := tool.SubscribeSubSlots(subSlotCh)
	subSlot, err := tool.assembler.newSubSlot(params.MaxBlockSize, defaultGasCeil/2, defaultGasCeil, []*types.Commitment{commitment0}, tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(subSlot.Txs) == 0 {
		t.Fatal("subSlot without txHashes")
	}
	unknown, old := tool.RegisterSubSlotIfNew(subSlot)
	if old {
		t.Error("subSlot got older")
	}
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !subSlot.IsResolved() {
		t.Fatal("failed to resolve txs")
	}
	processed, err := tool.HandleSubSlot(subSlot)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("failed to processed subSlot")
	}
	if len(tool.KnownSubSlots()) != 1 {
		t.Fatal("accepted subSlot not found")
	}
	select {
	case err := <-subscribeSubSlots.Err():
		t.Error(err)
	case handledSubSlot := <-subSlotCh:
		subSlotStateDiff := handledSubSlot.StateDiff.String()
		if commitmentStateDiff := commitment0.StateDiff.String(); commitmentStateDiff != subSlotStateDiff {
			t.Errorf("subSlotStateDiff %s, commitmentStateDiff %s", subSlotStateDiff, commitmentStateDiff)
		}

		subSlotJson, err := json.MarshalIndent(handledSubSlot, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", subSlotJson)
	case <-time.After(time.Second):
		t.Fatal("not received subscribedSubSlots")
	}

	tx11 := storageTxs(senderKey1, signer, 0)
	tx12 := transferTx(senderKey2, signer, crypto.CreateAddress(senderAddr1, 0), 0, big.NewInt(0), common.FromHex("d0728f610000000000000000000000000000000000000000000000000000000000000003")...)

	commitment11, err := tool.ProcessBundle(&types.Bundle{NewTxs: tx11}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(commitment11.Accesses.String())
	for _, change := range commitment11.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}

	commitment12, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx12}}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(commitment12.Accesses.String())
	for _, change := range commitment12.StateDiff.Changes {
		for _, kv := range change.Storage {
			if tool.blockChain.executor.(*executor).statedb.GetState(change.Address, kv.Key) == kv.Value {
				t.Errorf("%s storage changes is not reverted", change.Address)
			}
		}
	}

	bundleBasedOnTwo1 := &types.Bundle{BasedCommitments: []common.Hash{commitment11.ID(), commitment12.ID()}}
	commitmentBasedOnTwo1, err := tool.ProcessBundle(bundleBasedOnTwo1, true)
	if err != nil {
		t.Fatal(err)
	}

	if commitmentBasedOnTwo1.ExecutedTxsCount() != 1 {
		t.Errorf("executedTxs want 1, got %d", commitmentBasedOnTwo1.ExecutedTxsCount())
		t.Log(commitmentBasedOnTwo1.StateDiff.String())
	}

	bundleBasedOnTwo2 := &types.Bundle{BasedCommitments: []common.Hash{commitment12.ID(), commitment11.ID()}}
	commitmentBasedOnTwo2, err := tool.ProcessBundle(bundleBasedOnTwo2, true)
	if err != nil {
		t.Fatal(err)
	}

	if commitmentBasedOnTwo2.ExecutedTxsCount() != 0 {
		t.Errorf("executedTxs want 0, got %d", commitmentBasedOnTwo2.ExecutedTxsCount())
		t.Log(commitmentBasedOnTwo2.StateDiff.String())
	}
}

func TestTool_ProcessBundle_multiBased(t *testing.T) {
	t.Parallel()
	const count = 40
	tool, faucetKey := prepareTool(t)
	signer := tool.blockChain.executor.GetSigner()

	var basedOn []common.Hash
	for i := uint64(0); i < count; i++ {
		tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(i+1000)), i, common.Big1)
		commitment, err := tool.ProcessBundle(&types.Bundle{BasedCommitments: basedOn, NewTxs: types.Transactions{tx}}, true)
		if err != nil {
			t.Fatalf("commitment %d, err: %s", i, err)
		}
		basedOn = []common.Hash{commitment.ID()}
	}

	tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(count+1000)), 0, common.Big2)
	_, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
	if err != nil {
		t.Fatalf("commitment err: %s", err)
	}

	subSlotCh := make(chan *types.SubSlot, 1)
	subscribeSubSlots := tool.SubscribeSubSlots(subSlotCh)
	subSlot, err := tool.assembler.newSubSlot(params.MaxBlockSize, defaultGasCeil/2, defaultGasCeil, tool.GetPendingCommitments(), tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(subSlot.Txs) == 0 {
		t.Fatal("subSlot without txHashes")
	}
	unknown, old := tool.RegisterSubSlotIfNew(subSlot)
	if old {
		t.Error("subSlot got older")
	}
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !subSlot.IsResolved() {
		t.Fatal("failed to resolve txs")
	}
	processed, err := tool.HandleSubSlot(subSlot)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("failed to processed subSlot")
	}
	if len(tool.KnownSubSlots()) != 1 {
		t.Fatal("accepted subSlot not found")
	}
	select {
	case err := <-subscribeSubSlots.Err():
		t.Error(err)
	case handledSubSlot := <-subSlotCh:
		if handledSubSlot.Index != 0 {
			t.Errorf("subSlot got %d index", handledSubSlot.Index)
		}
		if len(handledSubSlot.Txs) != count {
			t.Errorf("subSlot got %d txs", len(handledSubSlot.Txs))
		}
		for i, transaction := range handledSubSlot.GetTxs() {
			if transaction.Nonce() != uint64(i) {
				t.Errorf("subSlot tx %d got nonce %d", i, transaction.Nonce())
			}
		}
	case <-time.After(time.Second):
		t.Fatal("not received subscribedSubSlots")
	}

	// test unavailable commitment from previous subSlot
	newTx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(count*2+1000)), count*2, common.Big1)
	_, err = tool.ProcessBundle(&types.Bundle{BasedCommitments: basedOn, NewTxs: types.Transactions{newTx}}, true)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unavailable commitment %s", basedOn[0])) {
		t.Fatalf("commitment err: %s", err)
	}

	basedOn = nil
	for i := uint64(count); i < count*2; i++ {
		tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(i+1000)), i, common.Big1)
		commitment, err := tool.ProcessBundle(&types.Bundle{BasedCommitments: basedOn, NewTxs: types.Transactions{tx}}, true)
		if err != nil {
			t.Fatalf("commitment %d, err: %s", i, err)
		}
		basedOn = []common.Hash{commitment.ID()}
	}

	subSlot, err = tool.assembler.newSubSlot(params.MaxBlockSize, defaultGasCeil/2, defaultGasCeil, tool.GetPendingCommitments(), tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(subSlot.Txs) == 0 {
		t.Fatal("subSlot without txHashes")
	}
	unknown, old = tool.RegisterSubSlotIfNew(subSlot)
	if old {
		t.Error("subSlot got older")
	}
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !subSlot.IsResolved() {
		t.Fatal("failed to resolve txs")
	}
	processed, err = tool.HandleSubSlot(subSlot)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("failed to processed subSlot")
	}
	if len(tool.KnownSubSlots()) != 2 {
		t.Fatal("accepted subSlot not found")
	}
	select {
	case err := <-subscribeSubSlots.Err():
		t.Error(err)
	case handledSubSlot := <-subSlotCh:
		if handledSubSlot.Index != 1 {
			t.Errorf("subSlot got %d index", handledSubSlot.Index)
		}
		if len(handledSubSlot.Txs) != count {
			t.Errorf("subSlot got %d txs", len(handledSubSlot.Txs))
		}
		for i, transaction := range handledSubSlot.GetTxs() {
			if transaction.Nonce() != uint64(i+count) {
				t.Errorf("subSlot tx %d got nonce %d", i, transaction.Nonce())
			}
		}
	case <-time.After(time.Second):
		t.Fatal("not received subscribedSubSlots")
	}

	for i, knownSubSlot := range tool.KnownSubSlots() {
		if int(knownSubSlot.Index) != i {
			t.Fatalf("want %d, got %d", i, knownSubSlot.Index)
		}
	}
}

func TestTool_ProcessBundle_duplicateNonce(t *testing.T) {
	t.Parallel()
	const count = 40
	tool, faucetKey := prepareTool(t)
	signer := tool.blockChain.executor.GetSigner()

	for i := uint64(0); i < count; i++ {
		tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(i+1000)), 0, common.Big1)
		_, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
		if err != nil {
			t.Fatalf("commitment %d, err: %s", i, err)
		}
	}

	tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(count+1000)), 0, common.Big2)
	_, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
	if err != nil {
		t.Fatalf("commitment err: %s", err)
	}

	subSlotCh := make(chan *types.SubSlot, 1)
	subscribeSubSlots := tool.SubscribeSubSlots(subSlotCh)
	subSlot, err := tool.assembler.newSubSlot(params.MaxBlockSize, defaultGasCeil/2, defaultGasCeil, tool.GetPendingCommitments(), tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(subSlot.Txs) == 0 {
		t.Fatal("subSlot without txHashes")
	}
	unknown, old := tool.RegisterSubSlotIfNew(subSlot)
	if old {
		t.Error("subSlot got older")
	}
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !subSlot.IsResolved() {
		t.Fatal("failed to resolve txs")
	}
	processed, err := tool.HandleSubSlot(subSlot)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("failed to processed subSlot")
	}
	if len(tool.KnownSubSlots()) != 1 {
		t.Fatal("accepted subSlot not found")
	}
	select {
	case err := <-subscribeSubSlots.Err():
		t.Error(err)
	case handledSubSlot := <-subSlotCh:
		if handledSubSlot.Index != 0 {
			t.Errorf("subSlot got %d index", handledSubSlot.Index)
		}
		if len(handledSubSlot.Txs) != 1 {
			t.Errorf("subSlot got %d txs", len(handledSubSlot.Txs))
		}
		for i, transaction := range handledSubSlot.GetTxs() {
			if transaction.Nonce() != 0 {
				t.Errorf("subSlot tx %d got nonce %d", i, transaction.Nonce())
			}
		}
	case <-time.After(time.Second):
		t.Fatal("not received subscribedSubSlots")
	}

	for i := uint64(0); i < count; i++ {
		tx := transferTx(faucetKey, signer, common.BigToAddress(new(big.Int).SetUint64(i+1000)), 1, common.Big1)
		_, err := tool.ProcessBundle(&types.Bundle{NewTxs: types.Transactions{tx}}, true)
		if err != nil {
			t.Fatalf("commitment %d, err: %s", i, err)
		}
	}

	subSlot, err = tool.assembler.newSubSlot(params.MaxBlockSize, defaultGasCeil/2, defaultGasCeil, tool.GetPendingCommitments(), tool.headBlock, tool.nextSubSlot, tool.nextTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(subSlot.Txs) == 0 {
		t.Fatal("subSlot without txHashes")
	}
	unknown, old = tool.RegisterSubSlotIfNew(subSlot)
	if old {
		t.Error("subSlot got older")
	}
	if len(unknown) != 0 {
		t.Errorf("%d txs not found", len(unknown))
	}
	if !subSlot.IsResolved() {
		t.Fatal("failed to resolve txs")
	}
	processed, err = tool.HandleSubSlot(subSlot)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("failed to processed subSlot")
	}
	if len(tool.KnownSubSlots()) != 2 {
		t.Fatal("accepted subSlot not found")
	}
	select {
	case err := <-subscribeSubSlots.Err():
		t.Error(err)
	case handledSubSlot := <-subSlotCh:
		if handledSubSlot.Index != 1 {
			t.Errorf("subSlot got %d index", handledSubSlot.Index)
		}
		if len(handledSubSlot.Txs) != 1 {
			t.Errorf("subSlot got %d txs", len(handledSubSlot.Txs))
		}
		for i, transaction := range handledSubSlot.GetTxs() {
			if transaction.Nonce() != 1 {
				t.Errorf("subSlot tx %d got nonce %d", i, transaction.Nonce())
			}
		}
	case <-time.After(time.Second):
		t.Fatal("not received subscribedSubSlots")
	}

}
