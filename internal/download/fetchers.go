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
	"github.com/golang/protobuf/proto"
	"github.com/holiman/uint256"
	"math/rand"
	"time"

	"github.com/amazechain/amc/api/protocol/sync_proto"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/log"
)

// fetchHeaders
func (d *Downloader) fetchHeaders(from uint256.Int, latest uint256.Int) error {

	// 1. create task
	begin := new(uint256.Int).AddUint64(&from, 1)
	difference := new(uint256.Int).Sub(&latest, begin)
	tasks := new(uint256.Int).Add(new(uint256.Int).Div(difference, uint256.NewInt(maxHeaderFetch)), uint256.NewInt(1))
	//
	log.Infof("Starting header downloads from: %v latest: %v difference: %v task: %v", begin.Uint64(), latest.Uint64(), difference.Uint64(), tasks.Uint64())
	defer log.Infof("Header download Finished")

	d.headerTaskLock.Lock()
	for i := 1; i <= int(tasks.Uint64()); i++ {
		taskID := rand.Uint64()
		d.headerTasks = append(d.headerTasks, Task{
			taskID:     taskID,
			IndexBegin: *begin,
			//IndexEnd:   *types.Int256Min(uint256.NewInt(0).Sub(uint256.NewInt(0).Add(&from, uint256.NewInt(maxHeaderFetch)), uint256.NewInt(1)), &latest),
			IsSync: false,
		})
		begin = begin.Add(begin, uint256.NewInt(maxHeaderFetch))
	}
	d.headerTaskLock.Unlock()

	tick := time.NewTicker(syncPeerIntervalRequest)
	defer tick.Stop()

	for {
		log.Tracef("header tasks count is %v, header processing tasks count is: %v", len(d.headerTasks), len(d.headerProcessingTasks))

		if len(d.headerTasks) == 0 && len(d.headerProcessingTasks) == 0 { // break if there are no tasks
			break
		}

		peerSet := d.peersInfo.findPeers(&latest, syncPeerCount)
		d.headerTaskLock.Lock()
		if len(d.headerTasks) > 0 {
			for _, p := range peerSet {
				randIndex := 0
				randTask := d.headerTasks[randIndex]

				//
				var fetchCount uint64
				if latest.Uint64()-randTask.IndexBegin.Uint64() >= maxHeaderFetch-1 {
					fetchCount = maxHeaderFetch
				} else {
					fetchCount = latest.Uint64() - randTask.IndexBegin.Uint64() + 1
				}

				msg := &sync_proto.SyncTask{
					Id:       randTask.taskID,
					SyncType: sync_proto.SyncType_HeaderReq,
					Payload: &sync_proto.SyncTask_SyncHeaderRequest{
						SyncHeaderRequest: &sync_proto.SyncHeaderRequest{
							Number: utils.ConvertUint256IntToH256(&randTask.IndexBegin),
							Amount: utils.ConvertUint256IntToH256(uint256.NewInt(fetchCount)),
						},
					},
				}
				payload, _ := proto.Marshal(msg)
				err := p.WriteMsg(message.MsgDownloader, payload)

				if err == nil {
					randTask.TimeBegin = time.Now()
					d.headerTasks = append(d.headerTasks[:randIndex], d.headerTasks[randIndex+1:]...)
					d.headerProcessingTasks[randTask.taskID] = randTask
					if len(d.headerTasks) == 0 {
						break
					}
				} else {
					log.Errorf("send sync request message to peer %v err is %v", p.ID(), err)
				}
			}
		}

		// If it times out, put the task back
		if len(d.headerProcessingTasks) > 0 {
			for taskID, task := range d.headerProcessingTasks {
				if time.Since(task.TimeBegin) > syncTimeOutPerRequest {
					delete(d.headerProcessingTasks, taskID)
					task.TimeBegin = time.Now()
					d.headerTasks = append(d.headerTasks, task)
				}
			}
		}
		d.headerTaskLock.Unlock()

		select {
		case <-d.ctx.Done():
			return ErrCanceled
		case <-tick.C:
			tick.Reset(syncPeerIntervalRequest)
			continue
		}
	}

	return nil
}

// fetchHeaders
func (d *Downloader) fetchBodies(latest uint256.Int) error {

	defer log.Info("Bodies download Finished")

	tick := time.NewTicker(syncPeerIntervalRequest)
	defer tick.Stop()
	startProcess := false

	for {
		// If there are unprocessed tasks
		peerSet := d.peersInfo.findPeers(&latest, syncPeerCount)

		log.Tracef("downloader body task count is %d, processing task count is %d", len(d.bodyTaskPool), len(d.bodyProcessingTasks))
		if startProcess && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 {
			return nil
		}

		d.once.Do(func() {
			log.Info("Starting body downloads")
			startProcess = true
		})

		d.bodyTaskPoolLock.Lock()
		if len(d.bodyTaskPool) > 0 {
			for _, p := range peerSet {
				// first task
				randIndex := 0
				randTask := d.bodyTaskPool[randIndex]

				msg := &sync_proto.SyncTask{
					Id:       randTask.taskID,
					SyncType: sync_proto.SyncType_BodyReq,
					Payload: &sync_proto.SyncTask_SyncBlockRequest{
						SyncBlockRequest: &sync_proto.SyncBlockRequest{
							Number: utils.Uint256sToH256(randTask.number), //todo task pool
						},
					},
				}
				payload, _ := proto.Marshal(msg)
				err := p.WriteMsg(message.MsgDownloader, payload)
				if err == nil {
					d.bodyTaskPool = append(d.bodyTaskPool[:randIndex], d.bodyTaskPool[randIndex+1:]...)
					d.bodyProcessingTasks[randTask.taskID] = randTask
					// finish
					if len(d.bodyTaskPool) == 0 {
						break
					}
				} else {
					log.Errorf("send sync request message to peer %v err is %v", p.ID(), err)
				}

			}
		}
		d.bodyTaskPoolLock.Unlock()

		select {
		case <-d.ctx.Done():
			return ErrCanceled

		case <-tick.C:
			tick.Reset(syncPeerIntervalRequest)
			continue
		}
	}
}
