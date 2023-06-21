// Copyright 2022 The AmazeChain Authors
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
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"

	"github.com/amazechain/amc/params"
)

type GenesisBlockConfig struct {
	Config *params.ChainConfig `json:"config" yaml:"config"`

	//	ChainID    uint64             `json:"chainID" yaml:"chainID"`
	Nonce      uint64        `json:"nonce"`
	Timestamp  uint64        `json:"timestamp"`
	ExtraData  []byte        `json:"extraData"`
	GasLimit   uint64        `json:"gasLimit"   gencodec:"required"`
	Difficulty *uint256.Int  `json:"difficulty" gencodec:"required"`
	Mixhash    types.Hash    `json:"mixHash"`
	Coinbase   types.Address `json:"coinbase"`

	//Engine *ConsensusConfig `json:"engine" yaml:"engine"`
	Miners []string   `json:"miners" yaml:"miners"`
	Alloc  []Allocate `json:"alloc" yaml:"alloc"  gencodec:"required"`

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64       `json:"number"`
	GasUsed    uint64       `json:"gasUsed"`
	ParentHash types.Hash   `json:"parentHash"`
	BaseFee    *uint256.Int `json:"baseFeePerGas"`

	RewardEpoch uint64 `json:"reward_epoch" yaml:"rewardEpoch"`
	RewardLimit uint64 `json:"reward_limit" yaml:"rewardLimit"`
}

type Allocate struct {
	Address string                    `json:"address" toml:"address"`
	Balance string                    `json:"balance" toml:"balance"`
	Code    []byte                    `json:"code,omitempty"`
	Storage map[types.Hash]types.Hash `json:"storage,omitempty"`
	Nonce   uint64                    `json:"nonce,omitempty"`
}
