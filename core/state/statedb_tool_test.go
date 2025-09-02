package state

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
)

func ExampleStateDB_Journal() {
	sdb, _ := New(types.EmptyRootHash, NewDatabaseForTesting())

	addr1 := common.BigToAddress(big.NewInt(11))
	sdb.CreateAccount(addr1)
	sdb.CreateContract(addr1)
	sdb.SetNonce(addr1, 1, tracing.NonceChangeContractCreator)
	sdb.SetState(addr1, common.BigToHash(big.NewInt(11)), common.BigToHash(big.NewInt(11)))
	sdb.SetCode(addr1, []byte{1, 1, 1})

	for _, entry := range sdb.Journal() {
		fmt.Println(entry)
	}

	// Output:
	//state.createObjectChange dirtied 0x000000000000000000000000000000000000000b
	//state.createContractChange dirtied <nil>
	//state.nonceChange dirtied 0x000000000000000000000000000000000000000b
	//state.storageChange dirtied 0x000000000000000000000000000000000000000b
	//state.codeChange dirtied 0x000000000000000000000000000000000000000b
}
