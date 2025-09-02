package utils

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/urfave/cli/v2"
)

// History
var (
	BlockHistoryFlag = &cli.Uint64Flag{
		Name:     "history.blocks", // copied from https://github.com/bnb-chain/bsc/pull/2809/files
		Usage:    "Number of recent blocks to maintain in DB (default = 0, 0 = entire chain)",
		Value:    ethconfig.Defaults.BlockHistory,
		Category: flags.StateCategory,
	}
)

// P2P
var (
	TEEVerifierFlag = &cli.StringFlag{
		Name:     "tee-verifier",
		Usage:    "Address of the TEE attestation contract. Special values: a zero address disables verification (all peers allowed), and '0xff...ff' uses the static list from --tee-passlist instead of a contract.",
		Value:    common.MaxAddress.String(),
		Category: flags.NetworkingCategory,
	}
	TEEPassListFlag = &cli.StringSliceFlag{
		Name:     "tee-passlist",
		Usage:    "List of attested peer node IDs to use as a static allowlist. This flag is active when --tee-verifier is set to its special passlist value (0xff...ff).",
		Category: flags.NetworkingCategory,
	}
	TEEMaxPeersFlag = &cli.IntFlag{
		Name:     "tee-maxpeers",
		Usage:    "",
		Value:    node.DefaultConfig.P2P.TEEMaxPeers,
		Category: flags.NetworkingCategory,
	}
	PublicSyncFlag = &cli.BoolFlag{
		Name:     "public-sync",
		Usage:    "Enables synchronization with all peers regardless of their attestation status; when disabled, node will only sync with peers that have verified TEE attestation",
		Value:    node.DefaultConfig.P2P.PublicSync,
		Category: flags.NetworkingCategory,
	}
	BlobPoolDisableFlag = &cli.BoolFlag{
		Name:     "blobpool.disable",
		Usage:    "Disables the blob transaction pool, the node will not accept or propagate pending blob transactions from the network",
		Value:    ethconfig.Defaults.BlobPool.Disable,
		Category: flags.BlobPoolCategory,
	}
	TxPoolDisableFlag = &cli.BoolFlag{
		Name:     "txpool.disable",
		Usage:    "Disables the legacy transaction pool, the node will not accept or propagate pending legacy transactions from the network",
		Value:    ethconfig.Defaults.TxPool.Disable,
		Category: flags.TxPoolCategory,
	}
)

// Tool
var (
	ToolEnabledFlag = &cli.BoolFlag{
		Name:     "tool",
		Value:    ethconfig.Defaults.Tool.Enabled,
		Category: flags.ToolCategory,
	}
	ToolLeaderFlag = &cli.BoolFlag{
		Name:     "tool.leader",
		Usage:    "Enable leader mode for subSlot generation. When enabled, the node will periodically generate subSlots at regular intervals to facilitate testing",
		Value:    ethconfig.Defaults.Tool.Leader,
		Category: flags.ToolCategory,
	}
	ToolAPIFeedFlag = &cli.BoolFlag{
		Name:     "tool.api-feed",
		Usage:    "Enable API access to private data flow",
		Value:    ethconfig.Defaults.Tool.APIFeed,
		Category: flags.ToolCategory,
	}
)

// Relay
var (
	RelayListenAddrFlag = &cli.StringFlag{
		Name:     "relay.addr",
		Usage:    "HTTP server listening address for the MEV-Boost relay API",
		Value:    ethconfig.Defaults.Relay.ListenAddr,
		Category: flags.RelayCategory,
	}
	RelaySecretKeyFlag = &cli.StringFlag{
		Name:     "relay.secret-key",
		Usage:    "BLS secret key hex string for signing bids",
		Value:    ethconfig.Defaults.Relay.SecretKey,
		Category: flags.RelayCategory,
	}
	RelayBeaconEndpointsFlag = &cli.StringSliceFlag{
		Name:     "relay.beacon-endpoints",
		Usage:    "Comma-separated list of beacon node HTTP endpoints for consensus layer communication",
		Value:    cli.NewStringSlice(ethconfig.Defaults.Relay.BeaconEndpoints...),
		Category: flags.RelayCategory,
	}
	RelayWaitValidatorFlag = &cli.BoolFlag{
		Name:     "relay.wait-validator",
		Usage:    "Wait for validator registration before starting bids building",
		Value:    ethconfig.Defaults.Relay.WaitValidator,
		Category: flags.RelayCategory,
	}
	RelayMaxRequestDelayFlag = &cli.DurationFlag{
		Name:     "relay.max-request-delay",
		Usage:    "Maximum allowed delay after the requested slot time for a relay request to be accepted",
		Value:    ethconfig.Defaults.Relay.MaxRequestDelay,
		Category: flags.RelayCategory,
	}
)

func setTool(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(BlockHistoryFlag.Name) {
		cfg.BlockHistory = ctx.Uint64(BlockHistoryFlag.Name)

		threshold := cfg.TransactionHistory
		if cfg.BlockHistory != 0 && cfg.BlockHistory < threshold {
			log.Warn("Block history must be at least larger than transaction history or confirmed bloom sections. Forcibly adjusted")
			cfg.BlockHistory = threshold
		}
	}
	if ctx.String(GCModeFlag.Name) == "archive" && cfg.BlockHistory != 0 {
		cfg.BlockHistory = 0
		log.Warn("Disabled partial block reserve for archive node")
	}

	if ctx.IsSet(BlobPoolDisableFlag.Name) {
		cfg.BlobPool.Disable = ctx.Bool(BlobPoolDisableFlag.Name)
	}
	if ctx.IsSet(TxPoolDisableFlag.Name) {
		cfg.TxPool.Disable = ctx.Bool(TxPoolDisableFlag.Name)
	}

	if ctx.IsSet(ToolEnabledFlag.Name) {
		cfg.Tool.Enabled = ctx.Bool(ToolEnabledFlag.Name)
	}
	if !cfg.Tool.Enabled {
		return
	}

	if ctx.IsSet(ToolLeaderFlag.Name) {
		cfg.Tool.Leader = ctx.Bool(ToolLeaderFlag.Name)
	}
	if ctx.IsSet(ToolAPIFeedFlag.Name) {
		cfg.Tool.APIFeed = ctx.Bool(ToolAPIFeedFlag.Name)
	}

	if ctx.IsSet(RelayListenAddrFlag.Name) {
		cfg.Relay.ListenAddr = ctx.String(RelayListenAddrFlag.Name)
	}
	if ctx.IsSet(RelaySecretKeyFlag.Name) {
		cfg.Relay.SecretKey = ctx.String(RelaySecretKeyFlag.Name)
	}
	if ctx.IsSet(RelayBeaconEndpointsFlag.Name) {
		cfg.Relay.BeaconEndpoints = ctx.StringSlice(RelayBeaconEndpointsFlag.Name)
	}
	if ctx.IsSet(RelayWaitValidatorFlag.Name) {
		cfg.Relay.WaitValidator = ctx.Bool(RelayWaitValidatorFlag.Name)
	}
	if ctx.IsSet(RelayMaxRequestDelayFlag.Name) {
		cfg.Relay.MaxRequestDelay = ctx.Duration(RelayMaxRequestDelayFlag.Name)
	}

	beaconConfig := MakeBeaconLightConfig(ctx)
	cfg.Relay.BeaconConfig = beaconConfig.ChainConfig
	if len(cfg.Relay.BeaconEndpoints) == 0 && len(beaconConfig.Apis) != 0 {
		cfg.Relay.BeaconEndpoints = beaconConfig.Apis
	}
	if cfg.Genesis != nil {
		cfg.Relay.ChainConfig = cfg.Genesis.Config
	} else {
		cfg.Relay.ChainConfig = params.MainnetChainConfig
	}
}

func setToolP2P(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.IsSet(TEEVerifierFlag.Name) {
		cfg.TEEVerifier = common.HexToAddress(ctx.String(TEEVerifierFlag.Name))
	}
	if ctx.IsSet(TEEMaxPeersFlag.Name) {
		cfg.TEEMaxPeers = ctx.Int(TEEMaxPeersFlag.Name)
	}
	if ctx.IsSet(TEEPassListFlag.Name) {
		for _, nodeID := range ctx.StringSlice(TEEPassListFlag.Name) {
			cfg.TEEPassList = append(cfg.TEEPassList, common.HexToHash(nodeID))
		}
		if cfg.TEEVerifier == (common.Address{}) {
			cfg.TEEVerifier = common.MaxAddress
		}
		if cfg.TEEVerifier != common.MaxAddress {
			Fatalf("Invalid TEEVerifier address %s and passlist %d", cfg.TEEVerifier, len(cfg.TEEPassList))
		}
	}
	if ctx.IsSet(PublicSyncFlag.Name) {
		cfg.PublicSync = ctx.Bool(PublicSyncFlag.Name) || cfg.TEEVerifier == (common.Address{})
	}
}
