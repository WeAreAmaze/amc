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

package download

import (
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"math/rand"
	"time"
)

func (d *Downloader) processHeaders() error {

	var (
		headerNumberPool []types.Int256
		startProcess     = false
	)

	tick := time.NewTicker(time.Duration(3 * time.Second))
	defer tick.Stop()

	for {
		if startProcess && len(d.headerTasks) == 0 && len(d.headerProcessingTasks) == 0 {
			if len(headerNumberPool) > 0 {
				d.bodyTaskPoolLock.Lock()
				d.bodyTaskPool = append(d.bodyTaskPool, &blockTask{
					taskID: rand.Uint64(),
					number: headerNumberPool[:],
				})
				d.bodyTaskPoolLock.Unlock()
			}
			break
		}

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case task := <-d.headerProcCh:
			startProcess = true
			d.headerTaskLock.Lock()
			if _, ok := d.headerProcessingTasks[task.taskID]; ok {
				delete(d.headerProcessingTasks, task.taskID)
			}
			log.Infof("received headers from remote peers  , the header counts is %v", len(task.headers))
			for _, header := range task.headers {
				headerNumberPool = append(headerNumberPool, header.Number)
				if len(headerNumberPool) >= maxBodiesFetch {
					d.bodyTaskPoolLock.Lock()
					//
					number := make([]types.Int256, len(headerNumberPool))
					copy(number, headerNumberPool)
					//d.log.Infof("copy count %d", c)
					headerNumberPool = headerNumberPool[0:0]
					//
					d.bodyTaskPool = append(d.bodyTaskPool, &blockTask{
						taskID: rand.Uint64(),
						number: number,
					})
					d.bodyTaskPoolLock.Unlock()
				}
				d.headerResultStore[header.Number] = header
				//d.log.Infof("process header %v", header)
			}
			d.headerTaskLock.Unlock()
		case <-tick.C:
			tick.Reset(time.Duration(3 * time.Second))
			continue
		}
	}
	return nil
}

// processContent
func (d *Downloader) processContent() error {
	tick := time.NewTicker(time.Duration(3 * time.Second))
	defer tick.Stop()

	startProcess := false

	for {
		if startProcess && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 {
			break
		}

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case response := <-d.blockProcCh:
			startProcess = true
			//d.log.Infof("processing body task id is %v, response is: %v", response.taskID, response)

			d.bodyTaskPoolLock.Lock()
			if _, ok := d.bodyProcessingTasks[response.taskID]; ok {
				delete(d.bodyProcessingTasks, response.taskID)
			}

			log.Debugf("received block from remote peers  , the block counts is  %v", len(response.bodies))
			for _, body := range response.bodies {
				d.bodyResultStore[body.Header.Number] = body
			}
			d.bodyTaskPoolLock.Unlock()
		case <-tick.C:
			tick.Reset(time.Duration(3 * time.Second))
			continue
		}
	}
	return nil
}

func (d *Downloader) processChain() error {
	tick := time.NewTicker(time.Duration(1 * time.Second))
	defer tick.Stop()
	startProcess := false

	for {
		if startProcess && len(d.bodyResultStore) == 0 && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 && len(d.headerTasks) == 0 && len(d.headerProcessingTasks) == 0 {
			break
		}

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case <-tick.C:
			startProcess = true

			d.bodyTaskPoolLock.Lock()
			wantBlockNumber := d.bc.CurrentBlock().Number64().Add(types.NewInt64(1))
			log.Infof("want block %d have blocks count is %d", wantBlockNumber.Uint64(), len(d.bodyResultStore))
			if blockMsg, ok := d.bodyResultStore[wantBlockNumber]; ok {

				var block block2.Block
				//var inserted bool

				block.FromProtoMessage(blockMsg)
				if _, err := d.bc.InsertBlock([]block2.IBlock{&block}, true); err != nil {
					//inserted = false
					log.Errorf("downloader failed to inster new block in blockchain, err:%v", err)
				} else {
					//inserted = true
					log.Infof("downloader successfully inserted a new block number is %d", block.Number64().Uint64())
				}

				delete(d.bodyResultStore, wantBlockNumber)
				//event.GlobalEvent.Send(&common.ChainHighestBlock{Highest: block.Number64(), Inserted: inserted})
			}

			d.bodyTaskPoolLock.Unlock()
		}
	}
	return nil
}
