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
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

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

	stack, err := node.NewNode(ctx, &DefaultConfig)
	if err != nil {
		log.Error("Failed start Node", "err", err)
		return err
	}

	StartNode(ctx, stack, false)

	// Unlock any account specifically requested
	unlockAccounts(ctx, stack, &DefaultConfig)

	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	go func() {
		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
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

	stack.Wait()

	return nil
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

func StartNode(ctx *cli.Context, stack *node.Node, isConsole bool) {
	if err := stack.Start(); err != nil {
		log.Critf("Error starting protocol stack: %v", err)
	}
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)

		if ctx.IsSet(MinFreeDiskSpaceFlag.Name) {
			minFreeDiskSpace := ctx.Int(MinFreeDiskSpaceFlag.Name)
			go monitorFreeDiskSpace(sigc, stack.InstanceDir(), uint64(minFreeDiskSpace)*1024*1024*1024)
		}

		shutdown := func() {
			log.Info("Got interrupt, shutting down...")
			go stack.Close()
			for i := 10; i > 0; i-- {
				<-sigc
				if i > 1 {
					log.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
				}
			}
			panic("Panic closing the amc node")
		}

		if isConsole {
			// In JS console mode, SIGINT is ignored because it's handled by the console.
			// However, SIGTERM still shuts down the node.
			for {
				sig := <-sigc
				if sig == syscall.SIGTERM {
					shutdown()
					return
				}
			}
		} else {
			<-sigc
			shutdown()
		}
	}()
}

func monitorFreeDiskSpace(sigc chan os.Signal, path string, freeDiskSpaceCritical uint64) {
	if path == "" {
		return
	}
	for {
		freeSpace, err := getFreeDiskSpace(path)
		if err != nil {
			log.Warn("Failed to get free disk space", "path", path, "err", err)
			break
		}
		if freeSpace < freeDiskSpaceCritical {
			log.Error("Low disk space. Gracefully shutting down Geth to prevent database corruption.", "available", types.StorageSize(freeSpace), "path", path)
			sigc <- syscall.SIGTERM
			break
		} else if freeSpace < 2*freeDiskSpaceCritical {
			log.Warn("Disk space is running low. Geth will shutdown if disk space runs below critical level.", "available", types.StorageSize(freeSpace), "critical_level", types.StorageSize(freeDiskSpaceCritical), "path", path)
		}
		time.Sleep(30 * time.Second)
	}
}
