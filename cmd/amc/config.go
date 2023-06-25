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
	"embed"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/params"
	"math/big"
	"time"

	"github.com/amazechain/amc/conf"
)

//go:embed allocs
var allocs embed.FS

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
	MetricsCfg: conf.MetricsConfig{
		InfluxDBEndpoint:     "",
		InfluxDBToken:        "",
		InfluxDBBucket:       "",
		InfluxDBOrganization: "",
	},

	P2PCfg: &conf.P2PConfig{},

	GenesisBlockCfg: ReadGenesis("allocs/genesis.json"),
	GPO:             conf.FullNodeGPO,
	Miner: conf.MinerConfig{
		GasCeil:  30000000,
		GasPrice: big.NewInt(params.GWei),
		Recommit: 4 * time.Second,
	},
}

func ReadGenesis(filename string) *conf.GenesisBlockConfig {
	f, err := allocs.Open(filename)
	defer f.Close()

	if err != nil {
		panic(fmt.Sprintf("%s not found, use default genesis", filename))
	}

	decoder := json.NewDecoder(f)
	gc := new(conf.GenesisBlockConfig)
	err = decoder.Decode(gc)
	if err != nil {
		panic(fmt.Sprintf("Could not parse genesis preallocation for %s: %v", filename, err))
	}
	return gc
}
