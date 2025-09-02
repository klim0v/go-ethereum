package state

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

func TestObjectsSnapshotTracker_createEmptyObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)

	sdb.Finalise(true)

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker_createContractObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SetCode(addr1, []byte{1, 1, 1})

	sdb.Finalise(true)

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker_contractExistingObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)

	sdb.Finalise(true)

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SetCode(addr1, []byte{1, 1, 1})

	sdb.Finalise(true)

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker_emptiedObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)

	sdb.Finalise(true)

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	sdb.SubBalance(addr1, uint256.NewInt(11), 0)

	sdb.Finalise(true)

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker_createDestructedObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SelfDestruct6780(addr1)
	sdb.SetCode(addr1, nil)

	sdb.Finalise(true)

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker_destructExistingObject(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)

	sdb.Finalise(true)

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	objectsSnapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = objectsSnapshotTracker

	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SelfDestruct6780(addr1)
	sdb.SetCode(addr1, nil)

	sdb.Finalise(true)

	sdb.CreateAccount(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(22), 0)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	newStorageKey := common.BigToHash(big.NewInt(2))
	newStorageValue := common.BigToHash(big.NewInt(3))
	sdb.SetState(addr1, newStorageKey, newStorageValue)
	sdb.SetCode(addr1, []byte{1, 1, 1})

	sdb.Finalise(true)

	stateDiff := sdb.snapshotTracker.getStateDiff()
	if stateDiff.IsEmpty() || len(stateDiff.Changes) != 1 {
		t.Errorf("stateDiff is empty, want 1 change")
	} else if change := stateDiff.Changes[0]; change != nil {
		if addr := change.Address; addr != addr1 {
			t.Errorf("stateDiff change address want %s, got %s", addr1, addr)
		} else if l := len(change.Storage); l != 1 {
			t.Errorf("stateDiff storage want 1, got %d", l)
		} else if storageKeyValue := change.Storage[0]; storageKeyValue.Key != newStorageKey {
			t.Errorf("stateDiff storage key want %s, got %s", newStorageKey, storageKeyValue.Key)
		} else if newStorageValue != storageKeyValue.Value {
			t.Errorf("stateDiff storage value want %s, got %s", newStorageValue, storageKeyValue.Value)
		}
	} else {
		t.Errorf("stateDiff change is nil")
	}

	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		t.Fatalf("want %s, got %s", root, r)
	}
}

func TestObjectsSnapshotTracker(t *testing.T) {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	addr1 := common.BigToAddress(big.NewInt(11)) // 0x000000000000000000000000000000000000000b
	sdb.CreateAccount(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)

	sdb.Finalise(true)

	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, 0)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SelfDestruct6780(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)
	sdb.SetCode(addr1, nil)

	addr2 := common.BigToAddress(big.NewInt(22)) // 0x0000000000000000000000000000000000000016
	sdb.CreateAccount(addr2)
	sdb.AddBalance(addr2, uint256.NewInt(22), 0)

	sdb.Finalise(true)

	sdbCopy := sdb.Copy()
	root := sdbCopy.IntermediateRoot(true)
	snapshotTracker := newObjectsSnapshotTracker(sdb)
	sdb.snapshotTracker = snapshotTracker

	sdb.CreateAccount(addr1)
	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, 0)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SelfDestruct6780(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)
	sdb.SetCode(addr1, nil)

	sdb.CreateContract(addr2)
	sdb.SetNonce(addr2, 1, 0)
	sdb.AddBalance(addr2, uint256.NewInt(22), 0)
	sdb.SelfDestruct6780(addr2)
	sdb.SetCode(addr2, nil)

	sdb.Finalise(true)

	addr3 := common.BigToAddress(big.NewInt(33)) // 0x0000000000000000000000000000000000000021
	sdb.CreateAccount(addr3)
	sdb.CreateContract(addr3)
	sdb.SetNonce(addr3, 1, 0)
	sdb.AddBalance(addr3, uint256.NewInt(33), 0)
	sdb.SetState(addr3, common.BigToHash(big.NewInt(33)), common.BigToHash(big.NewInt(33)))
	sdb.SetCode(addr3, []byte{3, 3, 3})
	sdb.SubBalance(addr3, uint256.NewInt(3), 0)

	sdb.Finalise(true)

	sdb.CreateAccount(addr1)
	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, 0)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SelfDestruct6780(addr1)
	sdb.AddBalance(addr1, uint256.NewInt(11), 0)
	sdb.SetCode(addr1, []byte{1, 2, 3})

	//sdb.CreateAccount(addr2)
	//sdb.AddBalance(addr2, uint256.NewInt(22), 0)

	sdb.CreateAccount(common.BigToAddress(big.NewInt(44))) // 0x000000000000000000000000000000000000002c

	sdb.Finalise(true)

	t.Log(dumpJournal(snapshotTracker.changes))
	t.Log(snapshotTracker.getStateDiff().String())
	_, err := sdb.Restore()
	if err != nil {
		t.Fatal(err)
	}

	if r := sdb.IntermediateRoot(true); r != root {
		b1 := &bytes.Buffer{}
		sdbCopy.IterativeDump(nil, json.NewEncoder(b1))

		b2 := &bytes.Buffer{}
		sdb.IterativeDump(nil, json.NewEncoder(b2))

		t.Logf("%s vs %s", b1.String(), b2.String())
		t.Fatalf("want %s, got %s", root, r)
	}
}
