package tracing

import (
	"github.com/ethereum/go-ethereum/common"
)

type StateDBTool interface {
	IsNewContract(common.Address) bool
}
