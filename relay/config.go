package relay

import (
	"time"

	bparams "github.com/ethereum/go-ethereum/beacon/params"
	"github.com/ethereum/go-ethereum/params"
)

var DefaultConfig = Config{
	SecretKey:       "0x28c3cd61b687fdd03488e167a5d84f50269df2a4c29a2cfb1390903aa775c5d0",
	ListenAddr:      "0.0.0.0:9060",
	WaitValidator:   false,
	ValidatorCache:  1_000_000,
	BeaconEndpoints: []string{"http://eth_lighthouse_container:3500"},
	MaxRequestDelay: time.Second * 5,
}

type Config struct {
	SecretKey       string
	ListenAddr      string
	WaitValidator   bool
	ValidatorCache  int
	CheckValidators bool
	BeaconEndpoints []string
	MaxRequestDelay time.Duration

	ChainConfig  *params.ChainConfig `toml:"-"`
	BeaconConfig bparams.ChainConfig `toml:"-"`
}
