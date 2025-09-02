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

import "github.com/ethereum/go-ethereum/log"

func (f *chainFreezer) SetupBlockHistory(blockHistory uint64) error {
	f.blockHistory.Store(blockHistory)
	return nil
}

// tryPruneHistoryBlock try prune ancient data keep blockHistory
func (f *chainFreezer) tryPruneHistoryBlock(best uint64) {
	blockHistory := f.blockHistory.Load()
	if blockHistory == 0 || best <= blockHistory {
		return
	}

	expectTail := best - blockHistory
	ancientHead, err := f.Ancients()
	if err != nil {
		log.Warn("PruneHistoryBlock query Ancients error", "best", best, "err", err)
		return
	}
	if expectTail > ancientHead {
		expectTail = ancientHead
	}
	old, err := f.TruncateTail(expectTail)
	if err != nil {
		log.Warn("PruneHistoryBlock TruncateTail error", "best", best,
			"expectTail", expectTail, "blockHistory", blockHistory, "err", err)
		return
	}
	log.Debug("Prune block history successful", "oldtail", old, "tail", expectTail, "best", best, "history", blockHistory)
}
