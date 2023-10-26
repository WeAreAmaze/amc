// Copyright 2023 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package conf

import (
	"math/big"
	"time"
)

type MinerConfig struct {
	Etherbase string        // Public address for block mining rewards
	GasCeil   uint64        // Target gas ceiling for mined blocks.
	GasPrice  *big.Int      // Minimum gas price for mining a transaction
	Recommit  time.Duration // The time interval for miner to re-create mining work
}
