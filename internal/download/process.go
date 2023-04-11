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
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"math/rand"
	"time"

	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/log"
)

func (d *Downloader) processHeaders() error {

	defer log.Info("Headers process Finished")
	var (
		headerNumberPool []uint256.Int
		startProcess     = false
	)

	tick := time.NewTicker(time.Duration(1 * time.Second))
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

			d.once.Do(func() {
				startProcess = true
				log.Info("Starting body downloads")
			})

			d.headerTaskLock.Lock()
			if _, ok := d.headerProcessingTasks[task.taskID]; ok {
				delete(d.headerProcessingTasks, task.taskID)
			}
			log.Tracef("received headers from remote peers  , the header counts is %v", len(task.headers))
			for _, header := range task.headers {
				headerNumberPool = append(headerNumberPool, *utils.ConvertH256ToUint256Int(header.Number))
				if len(headerNumberPool) >= maxBodiesFetch {
					d.bodyTaskPoolLock.Lock()
					//
					number := make([]uint256.Int, len(headerNumberPool))
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
				d.headerResultStore[*utils.ConvertH256ToUint256Int(header.Number)] = header
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

// processBodies
func (d *Downloader) processBodies() error {
	tick := time.NewTicker(time.Duration(1 * time.Second))
	defer tick.Stop()
	defer log.Info("Process bodies finished")
	startProcess := false

	for {
		if startProcess && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 {
			break
		}

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case response := <-d.blockProcCh:

			d.once.Do(func() {
				startProcess = true
				log.Info("Starting process bodies")
			})

			//d.log.Infof("processing body task id is %v, response is: %v", response.taskID, response)

			d.bodyTaskPoolLock.Lock()
			if _, ok := d.bodyProcessingTasks[response.taskID]; ok {
				delete(d.bodyProcessingTasks, response.taskID)
			}

			log.Debugf("received block from remote peers  , the block counts is  %v", len(response.bodies))
			for _, body := range response.bodies {
				d.bodyResultStore[*utils.ConvertH256ToUint256Int(body.Header.Number)] = body
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
	defer log.Info("Process chain Finished")

	for {
		if startProcess && len(d.bodyResultStore) == 0 && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 && len(d.headerTasks) == 0 && len(d.headerProcessingTasks) == 0 {
			break
		}
		//
		d.once.Do(func() {
			startProcess = true
			log.Info("Starting process chain")
		})

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case <-tick.C:

			d.bodyTaskPoolLock.Lock()
			wantBlockNumber := new(uint256.Int).AddUint64(d.bc.CurrentBlock().Number64(), 1)
			log.Tracef("want block %d have blocks count is %d", wantBlockNumber.Uint64(), len(d.bodyResultStore))

			blocks := make([]block2.IBlock, 0)
			for i := 0; i < maxResultsProcess; i++ {
				if blockMsg, ok := d.bodyResultStore[*wantBlockNumber]; ok {
					var block block2.Block
					err := block.FromProtoMessage(blockMsg)
					if err != nil {
						//todo
						break
					}
					delete(d.bodyResultStore, *wantBlockNumber)
					blocks = append(blocks, &block)
				}
				wantBlockNumber.AddUint64(wantBlockNumber, 1)
			}

			if len(blocks) == 0 {
				d.bodyTaskPoolLock.Unlock()
				continue
			}

			first, last := blocks[0].Header(), blocks[len(blocks)-1].Header()
			log.Info("Inserting downloaded chain", "items", len(blocks),
				"firstnum", first.Number64().Uint64(), "firsthash", first.Hash(),
				"lastnum", last.Number64().Uint64(), "lasthash", last.Hash(),
			)

			if index, err := d.bc.InsertChain(blocks); err != nil {
				//inserted = false
				if index < len(blocks) {
					log.Errorf("downloader failed to inster new block in blockchain, err:%v", err)
				} else {
				}
				d.bodyTaskPoolLock.Unlock()
				return err

			} else {
				//inserted = true

			}
			d.bodyTaskPoolLock.Unlock()
		}
	}
	return nil
}
