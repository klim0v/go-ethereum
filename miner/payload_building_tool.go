package miner

import (
	"github.com/ethereum/go-ethereum/core/types"
)

func (miner *Miner) setupNextToolHeader(args *BuildPayloadArgs) (*Payload, error) {
	payload := &Payload{empty: types.NewBlockWithHeader(&types.Header{}), stop: make(chan struct{})}
	close(payload.stop)
	return payload, nil
}
