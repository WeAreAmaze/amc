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
	"github.com/amazechain/amc/log"
	"golang.org/x/sys/unix"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"

	"github.com/amazechain/amc/accounts"

	"github.com/amazechain/amc/accounts/keystore"
	"github.com/amazechain/amc/cmd/utils"

	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/node"
	"github.com/urfave/cli/v2"
)

func appRun(ctx *cli.Context) error {
	if len(cfgFile) > 0 {
		if err := conf.LoadConfigFromFile(cfgFile, &DefaultConfig); err != nil {
			return err
		}
	} else {
		DefaultConfig.NetworkCfg.ListenersAddress = listenAddress.Value()
		DefaultConfig.NetworkCfg.BootstrapPeers = bootstraps.Value()
		if len(privateKey) > 0 {
			DefaultConfig.NetworkCfg.LocalPeerKey = privateKey
		}

		DefaultConfig.P2PCfg.StaticPeers = p2pStaticPeers.Value()
		DefaultConfig.P2PCfg.BootstrapNodeAddr = p2pBootstrapNode.Value()
		DefaultConfig.P2PCfg.DenyListCIDR = p2pDenyList.Value()

		//
		DefaultConfig.P2PCfg.DataDir = DefaultConfig.NodeCfg.DataDir
	}

	log.Init(DefaultConfig.NodeCfg, DefaultConfig.LoggerCfg)
	//log.SetLogger(log.WithContext(c, log.With(zap.NewLogger(zapLog), "caller", log.DefaultCaller)))

	c, cancel := context.WithCancel(context.Background())

	// initializing the node and providing the current git commit there
	//log.WithFields(logrus.Fields{"git_branch": version.GitBranch, "git_tag": version.GitTag, "git_commit": version.GitCommit}).Info("Build info")

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
				log.Error("failed to setup go pprof", "err", err)
				os.Exit(0)
			}
		}()
	}

	n, err := node.NewNode(c, &DefaultConfig)
	if err != nil {
		log.Error("Failed start Node", "err", err)
		return err
	}

	// Unlock any account specifically requested
	unlockAccounts(ctx, n, &DefaultConfig)

	if err := n.Start(); err != nil {
		cancel()
		return err
	}

	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	n.AccountManager().Subscribe(events)

	go func() {
		// Open any wallets already attached
		for _, wallet := range n.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				var derivationPaths []accounts.DerivationPath
				if event.Wallet.URL().Scheme == "ledger" {
					derivationPaths = append(derivationPaths, accounts.LegacyLedgerBaseDerivationPath)
				}
				derivationPaths = append(derivationPaths, accounts.DefaultBaseDerivationPath)

				event.Wallet.SelfDerive(derivationPaths, nil)

			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	//wg := sync.WaitGroup{}
	//wg.Add(1)
	// appWait(cancel, &wg)
	//n.Close()
	//wg.Wait()
	go ListenSignals(cancel, n)
	n.Wait()
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

func ListenSignals(f context.CancelFunc, n *node.Node) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, unix.SIGINT, unix.SIGTERM)
	defer signal.Stop(sigc)

	usr1 := make(chan os.Signal, 1)
	signal.Notify(usr1, unix.SIGUSR1)
	for {
		select {
		case <-sigc:
			log.Info("Got interrupt, shutting down...")
			if n != nil {
				f()
				n.Close()
			}
			for i := 10; i > 0; i-- {
				<-sigc
				if i > 1 {
					log.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
				}
			}
		case <-usr1:
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		}
	}
}

// unlockAccounts unlocks any account specifically requested.
func unlockAccounts(ctx *cli.Context, stack *node.Node, cfg *conf.Config) {
	var unlocks []string
	inputs := strings.Split(ctx.String(UnlockedAccountFlag.Name), ",")
	for _, input := range inputs {
		if trimmed := strings.TrimSpace(input); trimmed != "" {
			unlocks = append(unlocks, trimmed)
		}
	}

	// Short circuit if there is no account to unlock.
	if len(unlocks) == 0 {
		return
	}
	// If insecure account unlocking is not allowed if node's APIs are exposed to external.
	// Print warning log to user and skip unlocking.
	if !cfg.NodeCfg.InsecureUnlockAllowed && cfg.NodeCfg.ExtRPCEnabled() {
		utils.Fatalf("Account unlock with HTTP access is forbidden!")
	}
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	passwords := MakePasswordList(ctx)
	for i, account := range unlocks {
		unlockAccount(ks, account, i, passwords)
	}
}

// MakePasswordList reads password lines from the file specified by the global --password flag.
func MakePasswordList(ctx *cli.Context) []string {
	path := ctx.Path(PasswordFileFlag.Name)
	if path == "" {
		return nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		log.Error("Failed to read password ", "file", err)
	}
	lines := strings.Split(string(text), "\n")
	// Sanitise DOS line endings.
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}
