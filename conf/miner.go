package conf

import "math/big"

type MinerConfig struct {
	GasCeil  uint64   // Target gas ceiling for mined blocks.
	GasPrice *big.Int // Minimum gas price for mining a transaction
}
