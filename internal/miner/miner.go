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

package miner

import (
	"context"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"golang.org/x/sync/errgroup"
	"time"
)

type Miner struct {
	coinbase types.Address
	engine   consensus.Engine
	worker   *worker
	txsPool  txs_pool.ITxsPool

	startCh chan types.Address
	stopCh  chan struct{}

	ctx context.Context
	//errCtx context.Context
	cancel context.CancelFunc

	group *errgroup.Group
}

func NewMiner(ctx context.Context, cfg *conf.Config, bc common.IBlockChain, engine consensus.Engine, txsPool txs_pool.ITxsPool, isLocalBlock func(header *block.Header) bool) *Miner {
	group, errCtx := errgroup.WithContext(ctx)
	miner := &Miner{
		engine:  engine,
		txsPool: txsPool,
		startCh: make(chan types.Address),
		stopCh:  make(chan struct{}),
		group:   group,
		ctx:     errCtx,
		worker:  newWorker(errCtx, group, cfg.GenesisBlockCfg.Config.Engine, cfg.GenesisBlockCfg.Config, engine, bc, txsPool, isLocalBlock, false, cfg.Miner),
	}

	return miner
}

func (m *Miner) Start() {
	log.Info("start miner", "coinbase", m.coinbase)
	m.group.Go(func() error {
		return m.runLoop()
	})
	m.startCh <- m.coinbase
}

func (m *Miner) runLoop() error {
	defer m.cancel()
	startCh := make(chan common.DownloaderFinishEvent)
	doneCh := make(chan common.DownloaderStartEvent)
	start := event.GlobalEvent.Subscribe(startCh)
	done := event.GlobalEvent.Subscribe(doneCh)

	defer func() {
		start.Unsubscribe()
		done.Unsubscribe()
	}()

	defer func() {
		if m.Mining() {
			m.worker.close()
		}
	}()

	canStart := false
	shouldStart := false

	for {
		select {
		case <-m.ctx.Done():
			return nil
		case _, ok := <-startCh:
			if ok {
				canStart = true
				if !m.Mining() && shouldStart {
					time.Sleep(5 * time.Second)
					m.SetCoinbase(m.coinbase)
					m.worker.start()
				}
			}
		case _, ok := <-doneCh:
			if ok {
				if m.Mining() {
					m.worker.stop()
				}
			}
		case err := <-start.Err():
			return err
		case err := <-done.Err():
			return err
		case addr, ok := <-m.startCh:
			if ok {
				m.SetCoinbase(addr)
				if canStart {
					m.worker.start()
				}
				shouldStart = true
			}
		case <-m.stopCh:
			shouldStart = false
			if m.Mining() {
				m.worker.stop()
			}
		case <-m.ctx.Done():
			return m.ctx.Err()
		}
	}
}

func (m *Miner) Mining() bool {
	return m.worker.isRunning()
}

func (m *Miner) SetCoinbase(addr types.Address) {
	m.coinbase = addr
	m.worker.setCoinbase(addr)
}

func (m *Miner) PendingBlockAndReceipts() (block.IBlock, block.Receipts) {
	return m.worker.pendingBlockAndReceipts()
}
