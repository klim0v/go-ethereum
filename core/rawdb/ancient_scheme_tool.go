// Copyright 2022 The go-ethereum Authors
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

package rawdb

import (
	"fmt"
)

func init() {
	//err := SetPrunableChainFreezerTableConfigs(ChainFreezerHeaderTable, ChainFreezerHashTable)
	//if err != nil {
	//	panic(err)
	//}
}

func SetPrunableChainFreezerTableConfigs(chainFreezerTableNames ...string) error {
	updates := make(map[string]freezerTableConfig, len(chainFreezerTableNames))
	for _, chainFreezerTableName := range chainFreezerTableNames {
		tableConfig, ok := chainFreezerTableConfigs[chainFreezerTableName]
		if !ok {
			return fmt.Errorf("chainFreezerTableConfigs not found by %s name", chainFreezerTableName)
		}

		updates[chainFreezerTableName] = freezerTableConfig{
			noSnappy: tableConfig.noSnappy,
			prunable: true,
		}
	}

	for name, tableConfig := range updates {
		chainFreezerTableConfigs[name] = tableConfig
	}

	return nil
}
