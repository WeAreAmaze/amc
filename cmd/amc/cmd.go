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
	"github.com/amazechain/amc/version"
	"github.com/urfave/cli/v2"
)

var (
	privateKey string
	//engine        string
	miner bool
	// todo
	listenAddress = cli.NewStringSlice()
	bootstraps    = cli.NewStringSlice()
	cfgFile       string

	p2pStaticPeers   = cli.NewStringSlice()
	p2pBootstrapNode = cli.NewStringSlice()
	p2pDenyList      = cli.NewStringSlice()
)

var rootCmd = []*cli.Command{
	{
		Name:    "version",
		Aliases: []string{"v"},
		Action: func(context *cli.Context) error {
			version.PrintVersion()
			return nil
		},
	},
}

var networkFlags = []cli.Flag{
	&cli.StringSliceFlag{
		Name:        "p2p.listen",
		Usage:       "p2p listen address",
		Value:       cli.NewStringSlice(),
		Destination: listenAddress,
	},

	&cli.StringSliceFlag{
		Name:        "p2p.bootstrap",
		Usage:       "bootstrap node info",
		Value:       cli.NewStringSlice(),
		Destination: bootstraps,
	},

	&cli.StringFlag{
		Name:        "p2p.key",
		Usage:       "private key of p2p node",
		Value:       "",
		Destination: &DefaultConfig.NetworkCfg.LocalPeerKey,
	},
}

var nodeFlg = []cli.Flag{
	&cli.StringFlag{
		Name:        "node.key",
		Usage:       "node private",
		Value:       "",
		Destination: &DefaultConfig.NodeCfg.NodePrivate,
	},
}

var rpcFlags = []cli.Flag{

	&cli.StringFlag{
		Name:        "ipcpath",
		Usage:       "Filename for IPC socket/pipe within the data dir (explicit paths escape it)",
		Value:       DefaultConfig.NodeCfg.IPCPath,
		Destination: &DefaultConfig.NodeCfg.IPCPath,
	},

	&cli.BoolFlag{
		Name:        "http",
		Usage:       "Enable the HTTP json-rpc server",
		Value:       false,
		Destination: &DefaultConfig.NodeCfg.HTTP,
	},
	&cli.StringFlag{
		Name:        "http.addr",
		Usage:       "HTTP server listening interface",
		Value:       DefaultConfig.NodeCfg.HTTPHost,
		Destination: &DefaultConfig.NodeCfg.HTTPHost,
	},
	&cli.StringFlag{
		Name:        "http.port",
		Usage:       "HTTP server listening port",
		Value:       "20012",
		Destination: &DefaultConfig.NodeCfg.HTTPPort,
	},
	&cli.StringFlag{
		Name:        "http.api",
		Usage:       "API's offered over the HTTP-RPC interface",
		Value:       "",
		Destination: &DefaultConfig.NodeCfg.HTTPApi,
	},

	&cli.BoolFlag{
		Name:        "ws",
		Usage:       "Enable the WS-RPC server",
		Value:       false,
		Destination: &DefaultConfig.NodeCfg.WS,
	},
	&cli.StringFlag{
		Name:        "ws.addr",
		Usage:       "WS-RPC server listening interface",
		Value:       DefaultConfig.NodeCfg.WSHost,
		Destination: &DefaultConfig.NodeCfg.WSHost,
	},
	&cli.StringFlag{
		Name:        "ws.port",
		Usage:       "WS-RPC server listening port",
		Value:       "20013",
		Destination: &DefaultConfig.NodeCfg.WSPort,
	},

	&cli.StringFlag{
		Name:        "ws.api",
		Usage:       "API's offered over the WS-RPC interface",
		Value:       "",
		Destination: &DefaultConfig.NodeCfg.WSApi,
	},
}

var consensusFlag = []cli.Flag{
	&cli.StringFlag{
		Name:        "engine.type",
		Usage:       "consensus engine",
		Value:       "APosEngine", //APoaEngine,APosEngine
		Destination: &DefaultConfig.GenesisBlockCfg.Config.Engine.EngineName,
	},
	&cli.BoolFlag{
		Name:        "engine.miner",
		Usage:       "miner",
		Value:       false,
		Destination: &DefaultConfig.NodeCfg.Miner,
	},
	&cli.StringFlag{
		Name:        "engine.etherbase",
		Usage:       "consensus etherbase",
		Value:       "",
		Destination: &DefaultConfig.GenesisBlockCfg.Config.Engine.Etherbase,
	},
}

var configFlag = []cli.Flag{
	&cli.StringFlag{
		Name:        "blockchain",
		Usage:       "Loading a Configuration File",
		Destination: &cfgFile,
	},
}

var pprofCfg = []cli.Flag{
	&cli.BoolFlag{
		Name:        "pprof",
		Usage:       "Enable the pprof HTTP server",
		Value:       false,
		Destination: &DefaultConfig.PprofCfg.Pprof,
	},

	&cli.BoolFlag{
		Name:        "pprof.block",
		Usage:       "Turn on block profiling",
		Value:       false,
		Destination: &DefaultConfig.PprofCfg.TraceBlock,
	},
	&cli.BoolFlag{
		Name:        "pprof.mutex",
		Usage:       "Turn on mutex profiling",
		Value:       false,
		Destination: &DefaultConfig.PprofCfg.TraceMutex,
	},
	&cli.IntFlag{
		Name:        "pprof.maxcpu",
		Usage:       "setup number of cpu",
		Value:       0,
		Destination: &DefaultConfig.PprofCfg.MaxCpu,
	},
	&cli.IntFlag{
		Name:        "pprof.port",
		Usage:       "pprof HTTP server listening port",
		Value:       0,
		Destination: &DefaultConfig.PprofCfg.Port,
	},
}

var loggerFlag = []cli.Flag{
	&cli.StringFlag{
		Name:        "log.name",
		Usage:       "logger file name and path",
		Value:       "amc.log",
		Destination: &DefaultConfig.LoggerCfg.LogFile,
	},

	&cli.StringFlag{
		Name:        "log.level",
		Usage:       "logger output level (value:[debug,info,warn,error,dpanic,panic,fatal])",
		Value:       "debug",
		Destination: &DefaultConfig.LoggerCfg.Level,
	},

	&cli.IntFlag{
		Name:        "log.maxSize",
		Usage:       "logger file max size M",
		Value:       10,
		Destination: &DefaultConfig.LoggerCfg.MaxSize,
	},
	&cli.IntFlag{
		Name:        "log.maxBackups",
		Usage:       "logger file max backups",
		Value:       10,
		Destination: &DefaultConfig.LoggerCfg.MaxBackups,
	},
	&cli.IntFlag{
		Name:        "log.maxAge",
		Usage:       "logger file max age",
		Value:       30,
		Destination: &DefaultConfig.LoggerCfg.MaxAge,
	},
	&cli.BoolFlag{
		Name:        "log.compress",
		Usage:       "logger file compress",
		Value:       false,
		Destination: &DefaultConfig.LoggerCfg.Compress,
	},
}
var (
	// P2PNoDiscovery specifies whether we are running a local network and have no need for connecting
	// to the bootstrap nodes in the cloud
	P2PNoDiscovery = &cli.BoolFlag{
		Name:        "p2p.no-discovery",
		Usage:       "Enable only local network p2p and do not connect to cloud bootstrap nodes.",
		Destination: &DefaultConfig.P2PCfg.NoDiscovery,
	}
	// P2PStaticPeers specifies a set of peers to connect to explicitly.
	P2PStaticPeers = &cli.StringSliceFlag{
		Name:        "p2p.peer",
		Usage:       "Connect with this peer. This flag may be used multiple times.",
		Destination: p2pStaticPeers,
	}
	// P2PBootstrapNode tells the beacon node which bootstrap node to connect to
	P2PBootstrapNode = &cli.StringSliceFlag{
		Name:        "p2p.bootstrap-node",
		Usage:       "The address of bootstrap node. Beacon node will connect for peer discovery via DHT.  Multiple nodes can be passed by using the flag multiple times but not comma-separated. You can also pass YAML files containing multiple nodes.",
		Destination: p2pBootstrapNode,
	}
	// P2PRelayNode tells the beacon node which relay node to connect to.
	P2PRelayNode = &cli.StringFlag{
		Name: "p2p.relay-node",
		Usage: "The address of relay node. The beacon node will connect to the " +
			"relay node and advertise their address via the relay node to other peers",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.RelayNodeAddr,
	}
	// P2PUDPPort defines the port to be used by discv5.
	P2PUDPPort = &cli.IntFlag{
		Name:        "p2p.udp-port",
		Usage:       "The port used by discv5.",
		Value:       61015,
		Destination: &DefaultConfig.P2PCfg.UDPPort,
	}
	// P2PTCPPort defines the port to be used by libp2p.
	P2PTCPPort = &cli.IntFlag{
		Name:        "p2p.tcp-port",
		Usage:       "The port used by libp2p.",
		Value:       61016,
		Destination: &DefaultConfig.P2PCfg.TCPPort,
	}
	// P2PIP defines the local IP to be used by libp2p.
	P2PIP = &cli.StringFlag{
		Name:        "p2p.local-ip",
		Usage:       "The local ip address to listen for incoming data.",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.LocalIP,
	}
	// P2PHost defines the host IP to be used by libp2p.
	P2PHost = &cli.StringFlag{
		Name:        "p2p.host-ip",
		Usage:       "The IP address advertised by libp2p. This may be used to advertise an external IP.",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.HostAddress,
	}
	// P2PHostDNS defines the host DNS to be used by libp2p.
	P2PHostDNS = &cli.StringFlag{
		Name:        "p2p.host-dns",
		Usage:       "The DNS address advertised by libp2p. This may be used to advertise an external DNS.",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.HostDNS,
	}
	// P2PPrivKey defines a flag to specify the location of the private key file for libp2p.
	P2PPrivKey = &cli.StringFlag{
		Name:        "p2p.priv-key",
		Usage:       "The file containing the private key to use in communications with other peers.",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.PrivateKey,
	}
	P2PStaticID = &cli.BoolFlag{
		Name:        "p2p.static-id",
		Usage:       "Enables the peer id of the node to be fixed by saving the generated network key to the default key path.",
		Value:       true,
		Destination: &DefaultConfig.P2PCfg.StaticPeerID,
	}
	// P2PMetadata defines a flag to specify the location of the peer metadata file.
	P2PMetadata = &cli.StringFlag{
		Name:        "p2p.metadata",
		Usage:       "The file containing the metadata to communicate with other peers.",
		Value:       "",
		Destination: &DefaultConfig.P2PCfg.MetaDataDir,
	}
	// P2PMaxPeers defines a flag to specify the max number of peers in libp2p.
	P2PMaxPeers = &cli.IntFlag{
		Name:        "p2p.max-peers",
		Usage:       "The max number of p2p peers to maintain.",
		Value:       20,
		Destination: &DefaultConfig.P2PCfg.MaxPeers,
	}
	// P2PAllowList defines a CIDR subnet to exclusively allow connections.
	P2PAllowList = &cli.StringFlag{
		Name: "p2p.allowlist",
		Usage: "The CIDR subnet for allowing only certain peer connections. " +
			"Using \"public\" would allow only public subnets. Example: " +
			"192.168.0.0/16 would permit connections to peers on your local network only. The " +
			"default is to accept all connections.",
		Destination: &DefaultConfig.P2PCfg.AllowListCIDR,
	}
	// P2PDenyList defines a list of CIDR subnets to disallow connections from them.
	P2PDenyList = &cli.StringSliceFlag{
		Name: "p2p.denylist",
		Usage: "The CIDR subnets for denying certainty peer connections. " +
			"Using \"private\" would deny all private subnets. Example: " +
			"192.168.0.0/16 would deny connections from peers on your local network only. The " +
			"default is to accept all connections.",
		Destination: p2pDenyList,
	}

	// P2PBlockBatchLimit specifies the requested block batch size.
	P2PBlockBatchLimit = &cli.IntFlag{
		Name:        "p2p.limit.block-batch",
		Usage:       "The amount of blocks the local peer is bounded to request and respond to in a batch.",
		Value:       64,
		Destination: &DefaultConfig.P2PCfg.P2PLimit.BlockBatchLimit,
	}
	// P2PBlockBatchLimitBurstFactor specifies the factor by which block batch size may increase.
	P2PBlockBatchLimitBurstFactor = &cli.IntFlag{
		Name:        "p2p.limit.block-burst-factor",
		Usage:       "The factor by which block batch limit may increase on burst.",
		Value:       2,
		Destination: &DefaultConfig.P2PCfg.P2PLimit.BlockBatchLimitBurstFactor,
	}
)

var (
	DataDirFlag = &cli.StringFlag{
		Name:        "data.dir",
		Usage:       "data save dir",
		Value:       "./amc/",
		Destination: &DefaultConfig.NodeCfg.DataDir,
	}

	FromDataDirFlag = &cli.StringFlag{
		Name:  "chaindata.from",
		Usage: "source data  dir",
	}
	ToDataDirFlag = &cli.StringFlag{
		Name:  "chaindata.to",
		Usage: "to data  dir",
	}
)

var (
	// Account settings
	UnlockedAccountFlag = &cli.StringFlag{
		Name:  "account.unlock",
		Usage: "Comma separated list of accounts to unlock",
		Value: "",
	}
	PasswordFileFlag = &cli.PathFlag{
		Name:        "account.password",
		Usage:       "Password file to use for non-interactive password input",
		Destination: &DefaultConfig.NodeCfg.PasswordFile,
	}
	LightKDFFlag = &cli.BoolFlag{
		Name:  "account.lightkdf",
		Usage: "Reduce key-derivation RAM & CPU usage at some expense of KDF strength",
	}
	KeyStoreDirFlag = &cli.PathFlag{
		Name:        "account.keystore",
		Usage:       "Directory for the keystore (default = inside the datadir)",
		TakesFile:   true,
		Destination: &DefaultConfig.NodeCfg.KeyStoreDir,
	}
	InsecureUnlockAllowedFlag = &cli.BoolFlag{
		Name:        "account.allow.insecure.unlock",
		Usage:       "Allow insecure account unlocking when account-related RPCs are exposed by http",
		Value:       false,
		Destination: &DefaultConfig.NodeCfg.InsecureUnlockAllowed,
	}

	// MetricsEnabledFlag Metrics flags
	MetricsEnabledFlag = &cli.BoolFlag{
		Name:  "metrics",
		Usage: "Enable metrics collection and reporting",
	}

	MetricsEnableInfluxDBFlag = &cli.BoolFlag{
		Name:        "metrics.influxdb",
		Usage:       "Enable metrics export/push to an external InfluxDB database",
		Value:       false,
		Destination: &DefaultConfig.MetricsCfg.EnableInfluxDB,
	}
	MetricsInfluxDBEndpointFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.endpoint",
		Usage:       "InfluxDB API endpoint to report metrics to",
		Value:       DefaultConfig.MetricsCfg.InfluxDBEndpoint,
		Destination: &DefaultConfig.MetricsCfg.InfluxDBEndpoint,
	}

	MetricsInfluxDBDatabaseFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.database",
		Usage:       "InfluxDB database name to push reported metrics to",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBDatabase,
	}
	MetricsInfluxDBUsernameFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.username",
		Usage:       "Username to authorize access to the database",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBUsername,
	}
	MetricsInfluxDBPasswordFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.password",
		Usage:       "Password to authorize access to the database",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBPassword,
	}

	MetricsInfluxDBTagsFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.tags",
		Usage:       "Comma-separated InfluxDB tags (key/values) attached to all measurements",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBTags,
	}
	//
	//MetricsEnableInfluxDBV2Flag = &cli.BoolFlag{
	//	Name:  "metrics.influxdbv2",
	//	Usage: "Enable metrics export/push to an external InfluxDB v2 database",
	//}

	MetricsInfluxDBTokenFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.token",
		Usage:       "Token to authorize access to the database (v2 only)",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBToken,
	}

	MetricsInfluxDBBucketFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.bucket",
		Usage:       "InfluxDB bucket name to push reported metrics to (v2 only)",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBBucket,
	}

	MetricsInfluxDBOrganizationFlag = &cli.StringFlag{
		Name:        "metrics.influxdb.organization",
		Usage:       "InfluxDB organization name (v2 only)",
		Destination: &DefaultConfig.MetricsCfg.InfluxDBOrganization,
	}
)

var (
	settingFlag = []cli.Flag{
		DataDirFlag,
	}
	accountFlag = []cli.Flag{
		PasswordFileFlag,
		KeyStoreDirFlag,
		LightKDFFlag,
		InsecureUnlockAllowedFlag,
		UnlockedAccountFlag,
	}

	metricsFlags = []cli.Flag{
		MetricsEnabledFlag,
		MetricsEnableInfluxDBFlag,
		MetricsInfluxDBEndpointFlag,
		MetricsInfluxDBTokenFlag,
		MetricsInfluxDBBucketFlag,
		MetricsInfluxDBOrganizationFlag,
		MetricsInfluxDBTagsFlag,

		MetricsInfluxDBPasswordFlag,
		MetricsInfluxDBUsernameFlag,
		MetricsInfluxDBDatabaseFlag,
	}

	p2pFlags = []cli.Flag{
		P2PNoDiscovery,
		P2PAllowList,
		P2PBootstrapNode,
		P2PDenyList,
		P2PIP,
		P2PHost,
		P2PMaxPeers,
		P2PMetadata,
		P2PStaticID,
		P2PPrivKey,
		P2PHostDNS,
		P2PRelayNode,
		P2PStaticPeers,
		P2PUDPPort,
		P2PTCPPort,
	}

	p2pLimitFlags = []cli.Flag{
		P2PBlockBatchLimit,
		P2PBlockBatchLimitBurstFactor,
	}
)
