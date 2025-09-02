package eth

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/blobpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/tee"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/relay"
	"github.com/ethereum/go-ethereum/rpc"
)

func toolAPIs(s *Ethereum) []rpc.API {
	if !s.tool.Enabled() {
		return nil
	}

	return []rpc.API{
		{
			Namespace:     "maintenance",
			Service:       NewMaintenanceAPI(s),
			Authenticated: true,
		},
		{
			Namespace: "tool",
			Service:   NewToolAPI(s),
		},
	}
}

func registerRelay(stack *node.Node, r *relay.Relay) {
	if r == nil {
		log.Warn("Relay is disabled")
		return
	}

	stack.RegisterLifecycle(r)
	stack.RegisterLifecycle(relay.NewServer(r))
}

func newTEEVerifier(eth *Ethereum) tee.Verifier {
	logger := log.New("publicSync", eth.p2pServer.PublicSync)
	if eth.p2pServer.TEEVerifier == (common.Address{}) {
		logger.Error("TEE peer chain verification disabled", "attested", "ALL")
		return tee.AlwaysTrue{}
	} else if eth.p2pServer.TEEVerifier == common.MaxAddress {
		logger.Warn("TEE peer chain verification disabled", "attested", len(eth.p2pServer.TEEPassList))
		return tee.PassList{PassList: eth.p2pServer.TEEPassList}
	} else {
		logger.Info("TEE peer chain verification is on", "contract", eth.p2pServer.TEEVerifier)
		return tee.NewChain(eth.p2pServer.TEEVerifier)
	}
}

func filterSubPools(config *ethconfig.Config, legacyPool *legacypool.LegacyPool, blobTxPool *blobpool.BlobPool) []txpool.SubPool {
	var subPools []txpool.SubPool
	if !config.TxPool.Disable {
		subPools = append(subPools, legacyPool)
	}
	if !config.BlobPool.Disable {
		subPools = append(subPools, blobTxPool)
	}
	return subPools
}
