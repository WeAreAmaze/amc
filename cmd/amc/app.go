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
	"context"
	"fmt"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/node"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/log/zap"
	"github.com/urfave/cli/v2"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

func appRun(ctx *cli.Context) error {

	if len(cfgFile) > 0 {
		if err := conf.LoadConfigFromFile(cfgFile, &DefaultConfig); err != nil {
			return err
		}
	} else {
		lAddrs := listenAddress.Value()
		bns := bootstraps.Value()
		DefaultConfig.NetworkCfg.ListenersAddress = lAddrs

		DefaultConfig.NetworkCfg.BootstrapPeers = bns
		if len(privateKey) > 0 {
			DefaultConfig.NetworkCfg.LocalPeerKey = privateKey
		}
	}
	zapLog, err := Init(&DefaultConfig.NodeCfg, &DefaultConfig.LoggerCfg)
	if err != nil {
		return err
	}

	c, cancel := context.WithCancel(context.Background())
	log.SetLogger(log.WithContext(c, log.With(zap.NewLogger(zapLog), "caller", log.DefaultCaller)))

	//todo
	//log.Infof("blockchain %v", DefaultConfig)

	if DefaultConfig.PprofCfg.Pprof {
		if DefaultConfig.PprofCfg.MaxCpu > 0 {
			runtime.GOMAXPROCS(DefaultConfig.PprofCfg.MaxCpu)
		}
		if DefaultConfig.PprofCfg.TraceMutex {
			runtime.SetMutexProfileFraction(1)
		}
		if DefaultConfig.PprofCfg.TraceBlock {
			runtime.SetBlockProfileRate(1)
		}

		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", DefaultConfig.PprofCfg.Port), nil); err != nil {
				log.Errorf("failed to setup go pprof, err: %v", err)
				os.Exit(0)
			}
		}()
	}

	n, err := node.NewNode(c, &DefaultConfig)

	if err != nil {
		return err
	}

	if err := n.Start(); err != nil {
		cancel()
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	appWait(cancel, &wg)
	n.Close()
	wg.Wait()

	return nil
}

func appWait(cancelFunc context.CancelFunc, group *sync.WaitGroup) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Info(sig.String())
		done <- true
	}()

	log.Info("waiting signal ...")
	<-done
	log.Info("app quit ...")
	cancelFunc()
	group.Done()
}
