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
	"context"
	"github.com/amazechain/amc/api/protocol/sync_proto"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/golang/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
	"math/rand"
	"sync"
)

// peerInfo
type peerInfo struct {
	ID         peer.ID
	Number     *uint256.Int
	Difficulty *uint256.Int
}

type peersInfo struct {
	ctx    context.Context
	cancel context.CancelFunc
	lock   sync.RWMutex

	peers common.PeerMap
	info  map[peer.ID]peerInfo
}

func (p *peersInfo) findPeers(number *uint256.Int, count int) common.PeerSet {
	p.lock.RLock()
	defer p.lock.RUnlock()
	set := common.PeerSet{}
	ids := make(map[peer.ID]struct{}, count)
	for i := 0; i < len(p.peers) && i < count; i++ {
		for id, _ := range p.peers {
			if _, ok := ids[id]; ok {
				continue
			}
			//p.log.Infof("Compare downloader number : %v, peer id : %v, remote peer number: %v", number.Uint64(), id.String(), p.info[id].Number.Uint64())
			//  Add 2 number as network delay
			if peerInfo, ok := p.info[id]; ok && new(uint256.Int).AddUint64(peerInfo.Number, 2).Cmp(number) >= 0 {
				ids[id] = struct{}{}
				set = append(set, p.peers[id])
			}
		}
	}
	log.Tracef("finded great than number %v peers count: %v, limit: %v", number.Uint64(), len(set), count)
	return set
}

func (p peersInfo) get(id peer.ID) (common.Peer, bool) {
	peer, ok := p.peers[id]
	return peer, ok
}

func (p *peersInfo) update(id peer.ID, Number *uint256.Int, Difficulty *uint256.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if peer, ok := p.info[id]; ok {
		peer.Number = Number
		peer.Difficulty = Difficulty
	} else {
		p.info[id] = peerInfo{
			ID:         id,
			Difficulty: Difficulty,
			Number:     Number,
		}
	}
}

func (p *peersInfo) drop(id peer.ID) {
	p.lock.Lock()
	defer p.lock.Unlock()
	delete(p.info, id)
}

// peerInfoBroadcastLoop
func (p *peersInfo) peerInfoBroadcast(Number *uint256.Int) {
	log.Debugf("start to broadcast peer info , number is :%v , peer count is %v", Number.Uint64(), len(p.peers))
	for _, peer := range p.peers {
		msg := &sync_proto.SyncTask{
			Id:       rand.Uint64(),
			SyncType: sync_proto.SyncType_PeerInfoBroadcast,
			Payload: &sync_proto.SyncTask_SyncPeerInfoBroadcast{
				SyncPeerInfoBroadcast: &sync_proto.SyncPeerInfoBroadcast{
					Number:     utils.ConvertUint256IntToH256(Number),
					Difficulty: utils.ConvertUint256IntToH256(uint256.NewInt(0)),
					//todo
				},
			},
		}

		payload, _ := proto.Marshal(msg)
		err := peer.WriteMsg(message.MsgDownloader, payload)

		if err != nil {
			log.Error("failed to sync peer info", zap.String("peer id", peer.ID().String()), zap.Error(err))
		}
	}
}

func newPeersInfo(ctx context.Context, peers common.PeerMap) *peersInfo {
	c, cancel := context.WithCancel(ctx)
	return &peersInfo{
		ctx:    c,
		cancel: cancel,
		peers:  peers,
		info:   make(map[peer.ID]peerInfo),
	}
}
