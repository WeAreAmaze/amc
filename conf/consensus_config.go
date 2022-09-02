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

type ConsensusConfig struct {
	EngineName string     `json:"name" yaml:"name"`
	BlsKey     string     `json:"bls" yaml:"bls"`
	MinerKey   string     `json:"miner" yaml:"miner"`
	Period     uint64     `json:"period" yaml:"period"`
	MPoa       MPoaConfig `json:"poa" yaml:"poa"`
	GasFloor   uint64     `json:"gasFloor" yaml:"gasFloor"` // Target gas floor for mined blocks.
	GasCeil    uint64     `json:"gasCeil" yaml:"gasCeil"`   // Target gas ceiling for mined blocks.
}

type MPoaConfig struct {
	Epoch              uint64 `json:"epoch" yaml:"epoch"`
	CheckpointInterval uint64 `json:"checkpointInterval" yaml:"checkpointInterval"`
	InmemorySnapshots  int    `json:"inmemorySnapshots" yaml:"inmemorySnapshots"`
	InmemorySignatures int    `json:"inmemorySignatures" yaml:"inmemorySignatures"`
	InMemory           bool   `json:"inMemory" yaml:"inMemory"`
}
