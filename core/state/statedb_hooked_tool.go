package state

import "github.com/ethereum/go-ethereum/common"

func (s *hookedStateDB) IsNewContract(addr common.Address) bool {
	return s.inner.IsNewContract(addr)
}
