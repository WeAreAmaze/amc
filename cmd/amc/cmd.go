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
	cli "github.com/urfave/cli/v2"
)

var (
	privateKey string
	//engine        string
	miner bool
	// todo
	listenAddress = cli.NewStringSlice()
	bootstraps    = cli.NewStringSlice()
	cfgFile       string
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
}

var consensusFlag = []cli.Flag{
	&cli.StringFlag{
		Name:        "engine.type",
		Usage:       "consensus engine",
		Value:       "APoaEngine",
		Destination: &DefaultConfig.GenesisBlockCfg.Engine.EngineName,
	},
	&cli.BoolFlag{
		Name:        "engine.miner",
		Usage:       "miner",
		Value:       false,
		Destination: &DefaultConfig.NodeCfg.Miner,
	},
	&cli.StringFlag{
		Name:        "engine.key",
		Usage:       "consensus private key",
		Value:       "CAESQFGaSBuerRHhigLhgPWQmd1R+1OB8kmXhd3tMyoMu5YL6KaU6PVjKzzJlkZzuh1TsUyzSqTMYZi6w6hQ2AGp/JU=",
		Destination: &DefaultConfig.GenesisBlockCfg.Engine.MinerKey,
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

var settingFlag = []cli.Flag{
	&cli.StringFlag{
		Name:        "data.dir",
		Usage:       "data save dir",
		Value:       "./amc/",
		Destination: &DefaultConfig.NodeCfg.DataDir,
	},
}
