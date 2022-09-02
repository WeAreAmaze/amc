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

type GenesisBlockConfig struct {
	ChainID    uint64          `json:"chainID" yaml:"chainID"`
	Engine     ConsensusConfig `json:"engine" yaml:"engine"`
	Miners     []string        `json:"miners" yaml:"miners"`
	Validators []string        `json:"validators" yaml:"validators"`
	Number     uint64          `json:"number" yaml:"number"`
	Timestamp  int64           `json:"timestamp" yaml:"timestamp"`
	Alloc      []Allocate      `json:"alloc" yaml:"alloc"`
}

type Allocate struct {
	Address string `json:"address" toml:"address"`
	Balance string `json:"balance" toml:"balance"`
}
