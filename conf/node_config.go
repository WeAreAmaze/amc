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

type NodeConfig struct {
	NodePrivate string `json:"private" yaml:"private"`
	HTTP        bool   `json:"http" yaml:"http" `
	HTTPHost    string `json:"http_host" yaml:"http_host" `
	HTTPPort    string `json:"http_port" yaml:"http_port"`
	WS          bool   `json:"ws" yaml:"ws" `
	WSHost      string `json:"ws_host" yaml:"ws_host" `
	WSPort      string `json:"ws_port" yaml:"ws_port"`
	IPCPath     string `json:"ipc_path" yaml:"ipc_path"`
	DataDir     string `json:"data_dir" yaml:"data_dir"`
	Miner       bool   `json:"miner" yaml:"miner"`
}
