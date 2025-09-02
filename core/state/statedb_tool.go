package state

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

func (s *StateDB) Journal() []string {
	entries := make([]string, 0, s.journal.length())
	for _, entry := range s.journal.entries {
		entries = append(entries, fmt.Sprintf("%T dirtied %s", entry, entry.dirtied()))
	}
	return entries
}

func (s *StateDB) IsNewContract(addr common.Address) bool {
	obj := s.getStateObject(addr)
	if obj == nil {
		return false
	}

	return obj.newContract
}
