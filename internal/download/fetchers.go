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
	"math/rand"
	"time"

	"github.com/amazechain/amc/api/protocol/sync_proto"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/gogo/protobuf/proto"
)

// fetchHeaders
func (d *Downloader) fetchHeaders(from types.Int256, latest types.Int256) error {

	// 1. create task
	difference := latest.Sub(from)
	tasks := difference.Div(difference, types.NewInt64(maxHeaderFetch))
	tasks = tasks.Add(types.NewInt64(1))
	//
	log.Infof("Starting header downloads from: %v latest: %v difference: %v task: %v", from.Uint64(), latest.Uint64(), difference.Uint64(), tasks.Uint64())
	defer log.Infof("Header download terminated")

	d.headerTaskLock.Lock()
	for i := 1; i <= int(tasks.Uint64()); i++ {
		taskID := rand.Uint64()
		d.headerTasks = append(d.headerTasks, Task{
			taskID:     taskID,
			IndexBegin: from,
			IndexEnd:   types.Int256Min(from.Add(types.NewInt64(maxHeaderFetch)).Sub(types.NewInt64(1)), latest),
			IsSync:     false,
		})
		from = from.Add(types.NewInt64(maxHeaderFetch)).Add(types.NewInt64(1))
	}
	d.headerTaskLock.Unlock()

	tick := time.NewTicker(syncPeerIntervalRequest)
	defer tick.Stop()

	for {
		log.Infof("header tasks count is %v, header processing tasks count is: %v", len(d.headerTasks), len(d.headerProcessingTasks))

		if len(d.headerTasks) == 0 && len(d.headerProcessingTasks) == 0 { // break if there are no tasks
			break
		}

		peerSet := d.peersInfo.findPeers(latest, syncPeerCount)
		d.headerTaskLock.Lock()
		if len(d.headerTasks) > 0 {
			for _, p := range peerSet {
				randIndex := 0
				randTask := d.headerTasks[randIndex]

				msg := &sync_proto.SyncTask{
					Id:       randTask.taskID,
					SyncType: sync_proto.SyncType_HeaderReq,
					Payload: &sync_proto.SyncTask_SyncHeaderRequest{
						SyncHeaderRequest: &sync_proto.SyncHeaderRequest{
							Number: randTask.IndexBegin,
							Amount: randTask.IndexEnd.Sub(randTask.IndexBegin),
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
func (d *Downloader) fetchBodies(latest types.Int256) error {

	log.Debug("Starting body downloads")
	defer log.Debug("Bodies download terminated")

	tick := time.NewTicker(syncPeerIntervalRequest)
	defer tick.Stop()

	startProcess := false

	for {
		// If there are unprocessed tasks
		peerSet := d.peersInfo.findPeers(latest, syncPeerCount)

		log.Infof("downloader body task count is %d, processing task count is %d", len(d.bodyTaskPool), len(d.bodyProcessingTasks))
		if startProcess && len(d.bodyTaskPool) == 0 && len(d.bodyProcessingTasks) == 0 {
			break
		}

		d.bodyTaskPoolLock.Lock()
		if len(d.bodyTaskPool) > 0 {
			startProcess = true
			for _, p := range peerSet {
				// first task
				randIndex := 0
				randTask := d.bodyTaskPool[randIndex]

				msg := &sync_proto.SyncTask{
					Id:       randTask.taskID,
					SyncType: sync_proto.SyncType_BodyReq,
					Payload: &sync_proto.SyncTask_SyncBlockRequest{
						SyncBlockRequest: &sync_proto.SyncBlockRequest{
							Number: randTask.number, //todo task pool
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
	return nil
}
