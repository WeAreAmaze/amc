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
	"fmt"
	"os"

	"github.com/amazechain/amc/params"
	"github.com/urfave/cli/v2"

	// Force-load the tracer engines to trigger registration
	_ "github.com/amazechain/amc/internal/tracers/js"
	_ "github.com/amazechain/amc/internal/tracers/native"
)

func main() {
	flags := append(networkFlags, consensusFlag...)
	flags = append(flags, loggerFlag...)
	flags = append(flags, pprofCfg...)
	flags = append(flags, nodeFlg...)
	flags = append(flags, rpcFlags...)
	flags = append(flags, authRPCFlag...)
	flags = append(flags, configFlag...)
	flags = append(flags, settingFlag...)
	flags = append(flags, accountFlag...)
	flags = append(flags, metricsFlags...)

	rootCmd = append(rootCmd, walletCommand, accountCommand, exportCommand)
	commands := rootCmd

	app := &cli.App{
		Name:     "amc",
		Usage:    "AmazeChain system",
		Flags:    flags,
		Commands: commands,
		//Version:                version.FormatVersion(),
		Version:                params.VersionWithCommit(params.GitCommit, ""),
		UseShortOptionHandling: true,
		Action:                 appRun,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("failed amc system setup %v", err)
	}
}
