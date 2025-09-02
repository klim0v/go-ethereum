// Copyright 2025 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package history

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	err := UpdateHistoryPrunePoint(params.MainnetGenesisHash, 22026000, common.HexToHash("0xd12dd2c1f714638f074bde0f51c10ddbfdcd18cc8b06fa50176ad81f7380a880"))
	if err != nil {
		panic(err)
	}
}

func UpdateHistoryPrunePoint(genesisHash common.Hash, blockNumber uint64, blockHash common.Hash) error {
	historyPrunePoint, ok := PrunePoints[genesisHash]
	if !ok {
		return errors.New("unknown network")
	}

	historyPrunePoint.BlockNumber = blockNumber
	historyPrunePoint.BlockHash = blockHash
	return nil
}
