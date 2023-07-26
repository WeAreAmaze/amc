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

type NetWorkConfig struct {
	ListenersAddress []string `json:"listeners" yaml:"listeners"`
	BootstrapPeers   []string `json:"bootstraps" yaml:"bootstraps"`
	LocalPeerKey     string   `json:"private" yaml:"network_private"`
	Bootstrapped     bool     `json:"discover" yaml:"discover"`
}

type P2PConfig struct {
	NoDiscovery         bool     `json:"no_discovery" yaml:"no_discovery"`
	EnableUPnP          bool     `json:"enable_upnp" yaml:"enable_upnp"`
	StaticPeerID        bool     `json:"static_peer_id" yaml:"static_peer_id"`
	StaticPeers         []string `json:"static_peers" yaml:"static_peers"`
	BootstrapNodeAddr   []string `json:"bootstrap_node_addr" yaml:"bootstrap_node_addr"`
	Discv5BootStrapAddr []string `json:"discv5_bootstrap_addr" yaml:"discv5_bootstrap_addr"`
	RelayNodeAddr       string   `json:"relay_node_addr" yaml:"relay_node_addr"`
	LocalIP             string   `json:"local_ip" yaml:"local_ip"`
	HostAddress         string   `json:"host_address" yaml:"host_address"`
	HostDNS             string   `json:"host_dns" yaml:"host_dns"`
	PrivateKey          string   `json:"private_key" yaml:"private_key"`
	DataDir             string   `json:"data_dir" yaml:"data_dir"`
	MetaDataDir         string   `json:"metadata_dir" yaml:"metadata_dir"`
	TCPPort             int      `json:"tcp_port" yaml:"tcp_port"`
	UDPPort             int      `json:"udp_port" yaml:"udp_port"`
	MaxPeers            int      `json:"max_peers" yaml:"max_peers"`
	AllowListCIDR       string   `json:"allow_list_cidr" yaml:"allow_list_cidr"`
	DenyListCIDR        []string `json:"deny_list_cidr" yaml:"deny_list_cidr"`

	P2PLimit *P2PLimit
}

type P2PLimit struct {
	BlockBatchLimit            int `json:"block_batch_limit" yaml:"block_batch_limit"`
	BlockBatchLimitBurstFactor int `json:"block_batch_limit_burst_factor" yaml:"block_batch_limit_burst_factor"`
	BlockBatchLimiterPeriod    int `json:"block_batch_limiter_period" yaml:"block_batch_limiter_period"`
}
