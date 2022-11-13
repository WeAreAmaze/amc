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
	"bufio"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	NodeCfg         NodeConfig         `json:"node" yaml:"node"`
	NetworkCfg      NetWorkConfig      `json:"network" yaml:"network"`
	LoggerCfg       LoggerConfig       `json:"logger" yaml:"logger"`
	DatabaseCfg     DatabaseConfig     `json:"database" yaml:"database"`
	PprofCfg        PprofConfig        `json:"pprof" yaml:"pprof"`
	GenesisBlockCfg GenesisBlockConfig `json:"genesis" yaml:"genesis"`
	AccountCfg      AccountConfig      `json:"account" yaml:"account"`
	MetricsCfg      MetricsConfig      `json:"metrics" yaml:"metrics"`
}

func SaveConfigToFile(file string, config Config) error {
	if len(file) == 0 {
		file = "./config2.yaml"
	}

	fd, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		//log.Errorf("filed open file %v, err %v", file, err)
		return err
	}
	defer fd.Close()
	return yaml.NewEncoder(fd).Encode(config)
	//return toml.NewEncoder(fd).Encode(blockchain)
}

func LoadConfigFromFile(file string, config *Config) error {
	if len(file) <= 0 {
		return fmt.Errorf("failed to load blockchain from file, file is nil")
	}
	//_, err := toml.DecodeFile(file, blockchain)
	fd, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fd.Close()
	reader := bufio.NewReader(fd)
	//return toml.NewDecoder(reader).Decode(blockchain)
	return yaml.NewDecoder(reader).Decode(config)
}
