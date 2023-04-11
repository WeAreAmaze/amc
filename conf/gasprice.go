package conf

import (
	"github.com/amazechain/amc/params"
	"math/big"
)

var (
	DefaultMaxPrice    = big.NewInt(500 * params.GWei)
	DefaultIgnorePrice = big.NewInt(2 * params.Wei)
)

type GpoConfig struct {
	Blocks           int
	Percentile       int
	MaxHeaderHistory int
	MaxBlockHistory  int
	Default          *big.Int `toml:",omitempty"`
	MaxPrice         *big.Int `toml:",omitempty"`
	IgnorePrice      *big.Int `toml:",omitempty"`
}

// FullNodeGPO contains default gasprice oracle settings for full node.
var FullNodeGPO = GpoConfig{
	Blocks:           20,
	Percentile:       60,
	MaxHeaderHistory: 1024,
	MaxBlockHistory:  1024,
	MaxPrice:         DefaultMaxPrice,
	IgnorePrice:      DefaultIgnorePrice,
}

// LightClientGPO contains default gasprice oracle settings for light client.
var LightClientGPO = GpoConfig{
	Blocks:           2,
	Percentile:       60,
	MaxHeaderHistory: 300,
	MaxBlockHistory:  5,
	MaxPrice:         DefaultMaxPrice,
	IgnorePrice:      DefaultIgnorePrice,
}
