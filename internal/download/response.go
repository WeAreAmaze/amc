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
	"github.com/amazechain/amc/api/protocol/sync_proto"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"google.golang.org/protobuf/proto"

	"github.com/holiman/uint256"
)

// responseHeaders
func (d *Downloader) responseHeaders(taskID uint64, p common.Peer, task *sync_proto.SyncHeaderRequest) {

	headers := make([]*types_pb.Header, 0, utils.ConvertH256ToUint256Int(task.Amount).Uint64())
	ok := true

	origin := task.Number
	for i := 0; i < int(utils.ConvertH256ToUint256Int(task.Amount).Uint64()); i++ {
		fullHeader := d.bc.GetHeaderByNumber(utils.ConvertH256ToUint256Int(origin))
		var header *types_pb.Header
		if fullHeader == nil {
			log.Warnf("cannot fetch header from db the number is:%v", utils.ConvertH256ToUint256Int(origin).Uint64())
			headers = headers[0:0]
			ok = false
			break
		} else {
			header = fullHeader.ToProtoMessage().(*types_pb.Header)
			log.Tracef("fetch header from db the number is:%v", utils.ConvertH256ToUint256Int(header.Number).Uint64())
		}

		headers = append(headers, header)
		origin = utils.ConvertUint256IntToH256(uint256.NewInt(0).Add(utils.ConvertH256ToUint256Int(origin), uint256.NewInt(1)))
	}

	log.Tracef("fetch all header from db the count is:%v", len(headers))

	msg := &sync_proto.SyncTask{
		Id:       taskID,
		Ok:       ok,
		SyncType: sync_proto.SyncType_HeaderRes,
		Payload: &sync_proto.SyncTask_SyncHeaderResponse{
			SyncHeaderResponse: &sync_proto.SyncHeaderResponse{
				Headers: headers,
			},
		},
	}
	payload, err := proto.Marshal(msg)

	if err != nil {
		log.Errorf("proto Marshal err: %v", err)
		return
	}

	p.WriteMsg(message.MsgDownloader, payload)
	log.Debugf("response sync task(headersRequest) ok: %v , taskID: %v, header count: %v", ok, taskID, len(headers))
}

// responseHeaders body
func (d *Downloader) responseBlocks(taskID uint64, p common.Peer, task *sync_proto.SyncBlockRequest) {

	blocks := make([]*types_pb.Block, 0, len(task.Number))

	ok := true

	for _, number := range task.Number {
		block, err := d.bc.GetBlockByNumber(utils.ConvertH256ToUint256Int(number))
		if err != nil {
			log.Infof("cannot fetch block from db the number is:%d, err: %v", utils.ConvertH256ToUint256Int(number).Uint64(), err)
			blocks = blocks[0:0]
			ok = false
			break
		}
		if PBlock, ok := block.ToProtoMessage().(*types_pb.Block); ok {
			blocks = append(blocks, PBlock)
			log.Tracef("fetch body from db the number is:%v", utils.ConvertH256ToUint256Int(number).Uint64())
		}
	}

	log.Tracef("fetch all blocks from db the count is:%d", len(blocks))

	msg := &sync_proto.SyncTask{
		Id:       taskID,
		Ok:       ok,
		SyncType: sync_proto.SyncType_BodyRes,
		Payload: &sync_proto.SyncTask_SyncBlockResponse{
			SyncBlockResponse: &sync_proto.SyncBlockResponse{
				Blocks: blocks,
			},
		},
	}
	payload, err := proto.Marshal(msg)

	if err != nil {
		log.Errorf("proto Marshal err: %v", err)
		return
	}

	p.WriteMsg(message.MsgDownloader, payload)
	log.Debugf("response sync task(blockRequest) ok: %v , taskID: %v, block count: %v", ok, taskID, len(task.Number))
}
