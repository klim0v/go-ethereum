package tool

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/tee"
)

func (p *Peer) VerifyAttestation(chain *core.BlockChain, verifier tee.Verifier) (bool, error) {
	if verifier == nil {
		return false, errors.New("verifier not initialized")
	}
	switch p.version {
	case TOOL1:
		return p.verifyAttestation(chain, verifier)
	default:
		return false, errors.New("unsupported protocol version")
	}
}

func (p *Peer) verifyAttestation(chain *core.BlockChain, verifier tee.Verifier) (bool, error) {
	attested, err := verifier.Verify(chain, common.Hash(p.Peer.ID()))
	if err != nil {
		return false, fmt.Errorf("failed to verify attestation, err: %w", err)
	}
	if !attested {
		return false, nil
	}
	return true, nil
}
