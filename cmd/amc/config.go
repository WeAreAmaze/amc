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

package main

import (
	"github.com/amazechain/amc/conf"
	"time"
)

const (
	genesisTime  = "2022/01/01 00:00:00"
	timeTemplate = "2006/01/02 15:04:05"
)

var DefaultConfig = conf.Config{
	NodeCfg: conf.NodeConfig{
		NodePrivate: "",
		HTTP:        true,
		HTTPHost:    "127.0.0.1",
		HTTPPort:    "8545",
		IPCPath:     "amc.ipc",
		Miner:       false,
	},
	NetworkCfg: conf.NetWorkConfig{
		Bootstrapped: true,
	},
	LoggerCfg: conf.LoggerConfig{
		LogFile:    "./logger.log",
		Level:      "debug",
		MaxSize:    10,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	},
	PprofCfg: conf.PprofConfig{
		MaxCpu:     0,
		Port:       6060,
		TraceMutex: true,
		TraceBlock: true,
		Pprof:      false,
	},
	DatabaseCfg: conf.DatabaseConfig{
		DBType:     "lmdb",
		DBPath:     "chaindata",
		DBName:     "amc",
		SubDB:      []string{"chain"},
		Debug:      false,
		IsMem:      false,
		MaxDB:      100,
		MaxReaders: 1000,
	},
	GenesisBlockCfg: conf.GenesisBlockConfig{
		ChainID: 0,
		Engine: conf.ConsensusConfig{
			EngineName: "MPoaEngine",
			BlsKey:     "bls",
			MinerKey:   "CAESQFGaSBuerRHhigLhgPWQmd1R+1OB8kmXhd3tMyoMu5YL6KaU6PVjKzzJlkZzuh1TsUyzSqTMYZi6w6hQ2AGp/JU=",
			Period:     1,
			GasCeil:    30000000,
			MPoa: conf.MPoaConfig{
				Epoch:              30000,
				CheckpointInterval: 0,
				InmemorySnapshots:  0,
				InmemorySignatures: 0,
				InMemory:           false,
			},
		},
		Miners:     []string{"CAESIOimlOj1Yys8yZZGc7odU7FMs0qkzGGYusOoUNgBqfyV", "CAESIDgSnhTUd/I2J7aAGJ4+mirj+M4/H7i8Ig5AvtuH7jTt", "CAESIKPHUGuFSB2kiX37o0prc8D/WeKbeLu2+JX6993Xnf9K"},
		Validators: []string{"QmecK5hh6iYLGrdSc8rdMb5bkd9nYVFTx2MkxYcEyQ782r"},
		Number:     0,
		Timestamp:  toLocation(),
		Alloc: []conf.Allocate{
			//conf.Allocate{
			//	Address: "AMCEb8aE6c1A31CeA7B4AfdD247211C863A1e7FB6D6",
			//	Balance: "1000000000000000000",
			//},
		},
	},
}

func toLocation() int64 {
	if stamp, err := time.ParseInLocation(timeTemplate, genesisTime, time.Local); err != nil {
		return time.Now().Unix()
	} else {
		return stamp.Unix()
	}
}
