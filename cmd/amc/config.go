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
	"github.com/amazechain/amc/conf"
	"time"
)

//go:embed allocs
var allocs embed.FS

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
	MetricsCfg: conf.MetricsConfig{
		InfluxDBEndpoint:     "",
		InfluxDBToken:        "",
		InfluxDBBucket:       "",
		InfluxDBOrganization: "",
	},

	GenesisBlockCfg: conf.GenesisBlockConfig{
		ChainID: 0,
		Engine: conf.ConsensusConfig{
			EngineName: "APoaEngine",
			BlsKey:     "bls",
			Period:     8,
			GasCeil:    30000000,
			APoa: conf.APoaConfig{
				Epoch:              30000,
				CheckpointInterval: 0,
				InmemorySnapshots:  0,
				InmemorySignatures: 0,
				InMemory:           false,
			},
		},
		Miners:     []string{"AMCA2142AB3F25EAA9985F22C3F5B1FF9FA378DAC21"},
		Validators: []string{"AMCA2142AB3F25EAA9985F22C3F5B1FF9FA378DAC21", "AMC3CA698823AE0474EE80D2F4BF29EC649474F4040", "AMC781ACBE8BECB693098D36875D48E967C92DB3A4E"},
		Number:     0,
		Timestamp:  toLocation(),
		Alloc:      readPrealloc("allocs/amc.json"),
	},
}

func toLocation() int64 {
	if stamp, err := time.ParseInLocation(timeTemplate, genesisTime, time.Local); err != nil {
		return time.Now().Unix()
	} else {
		return stamp.Unix()
	}
}

func readPrealloc(filename string) []conf.Allocate {
	f, err := allocs.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("Could not open genesis preallocation for %s: %v", filename, err))
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	ga := []conf.Allocate{}
	err = decoder.Decode(&ga)
	if err != nil {
		panic(fmt.Sprintf("Could not parse genesis preallocation for %s: %v", filename, err))
	}
	return ga
}
