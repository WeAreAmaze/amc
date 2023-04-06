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
	"fmt"
	"github.com/amazechain/amc/utils"
	"github.com/golang/protobuf/proto"
	"github.com/holiman/uint256"
	"hash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amazechain/amc/api/protocol/sync_proto"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
)

var (
	ErrBusy          = fmt.Errorf("busy")
	ErrCanceled      = fmt.Errorf("syncing canceled (requested)")
	ErrSyncBlock     = fmt.Errorf("err sync block")
	ErrTimeout       = fmt.Errorf("timeout")
	ErrBadPeer       = fmt.Errorf("bad peer error")
	ErrNoPeers       = fmt.Errorf("no peers to download")
	ErrInvalidPubSub = fmt.Errorf("PubSub is nil")
)

const (
	maxHeaderFetch          = 192             //Get the number of headers at a time
	maxBodiesFetch          = 128             // Get the number of bodies at a time
	maxResultsProcess       = 2048            // Number of content download results to import at once into the chain
	headerDownloadInterval  = 3 * time.Second // header download interval
	syncPeerCount           = 6
	syncTimeTick            = time.Duration(10 * time.Second)
	syncCheckTimes          = 1
	syncTimeOutPerRequest   = time.Duration(1 * time.Minute)
	syncPeerIntervalRequest = time.Duration(3 * time.Second)
	syncPeerInfoTimeTick    = time.Duration(10 * time.Second)
	maxDifferenceNumber     = 2
)

type headerResponse struct {
	taskID  uint64
	ok      bool
	headers []*types_pb.Header
}

type bodyResponse struct {
	taskID uint64
	ok     bool
	bodies []*types_pb.Block
}

type blockTask struct {
	taskID uint64
	ok     bool
	number []uint256.Int
}

type Task struct {
	taskID     uint64
	Id         peer.ID
	H          hash.Hash
	TimeBegin  time.Time
	IsSync     bool
	IndexBegin uint256.Int
	IndexEnd   uint256.Int
}

type Downloader struct {
	mode uint32 // sync mode , use d.getMode() to get the SyncMode

	bc            common.IBlockChain
	network       common.INetwork
	isDownloading int32

	highestNumber uint256.Int

	ctx        context.Context
	cancel     context.CancelFunc
	cancelLock sync.RWMutex
	cancelWg   sync.WaitGroup //
	once       sync.Once

	errorCh chan error

	pubsub    common.IPubSub
	peersInfo *peersInfo

	headerTasks           []Task
	headerProcessingTasks map[uint64]Task
	headerResultStore     map[uint256.Int]*types_pb.Header
	headerTaskLock        sync.Mutex
	//
	headerProcCh chan *headerResponse

	//
	//bodyTaskCh  chan *blockTask
	blockProcCh chan *bodyResponse

	bodyTaskPoolLock    sync.Mutex
	bodyTaskPool        []*blockTask
	bodyProcessingTasks map[uint64]*blockTask
	bodyResultStore     map[uint256.Int]*types_pb.Block
}

func NewDownloader(ctx context.Context, bc common.IBlockChain, network common.INetwork, pubsub common.IPubSub, peers common.PeerMap) common.IDownloader {
	c, cancel := context.WithCancel(ctx)

	highestNumber := bc.CurrentBlock().Number64().Clone()
	for _, peer := range peers {
		if highestNumber.Uint64() < peer.CurrentHeight.Uint64() {
			highestNumber = peer.CurrentHeight.Clone()
		}
	}

	return &Downloader{
		mode:                  uint32(FullSync),
		bc:                    bc,
		network:               network,
		ctx:                   c,
		cancel:                cancel,
		isDownloading:         0,
		pubsub:                pubsub,
		errorCh:               make(chan error, 10),
		headerTasks:           make([]Task, 0),
		headerProcessingTasks: make(map[uint64]Task),
		headerResultStore:     make(map[uint256.Int]*types_pb.Header),
		headerProcCh:          make(chan *headerResponse, 10),
		blockProcCh:           make(chan *bodyResponse, 10),
		bodyTaskPool:          make([]*blockTask, 0),
		bodyProcessingTasks:   make(map[uint64]*blockTask),
		bodyResultStore:       make(map[uint256.Int]*types_pb.Block),
		highestNumber:         *highestNumber,
		peersInfo:             newPeersInfo(c, peers),
	}
}

func (d *Downloader) getMode() SyncMode {
	return SyncMode(atomic.LoadUint32(&d.mode))
}

func (d *Downloader) FindBlock(number uint64, peerID peer.ID) (uint64, error) {
	return 0, nil
}

func (d *Downloader) waitAvailablePeer() {
	timer := time.NewTicker(1 * time.Second)
	defer timer.Stop()

	timeOutTimer := time.NewTicker(60 * time.Second)
	defer timeOutTimer.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-timer.C:
			peers := d.peersInfo.findPeers(new(uint256.Int).AddUint64(d.bc.CurrentBlock().Number64(), 1), 10)
			if len(peers) > 0 {
				return
			}
		case <-timeOutTimer.C:
			log.Warn("Can not find Peers")
		}
	}
}

// Start Downloader
func (d *Downloader) Start() error {
	//
	go d.pubSubLoop()
	//
	if d.network.Bootstrapped() {
		//todo
		//log.Debugf("boot node")
		event.GlobalEvent.Send(&common.DownloaderFinishEvent{})
		return nil
	}

	go d.synchronise()
	return nil
}

// Start
func (d *Downloader) doSync(mode SyncMode) error {

	log.Info("do sync", zap.Int("SyncMode", int(mode)))
	if !atomic.CompareAndSwapInt32(&d.isDownloading, 0, 1) {
		return ErrBusy
	}
	defer atomic.StoreInt32(&d.isDownloading, 0)

	// blockChain current block height
	origin, err := d.findAncestor()
	if err != nil {
		return err
	}
	// downloader current height
	latest, err := d.findHead()
	if err != nil {
		return err
	}

	var fetchers []func() error

	switch mode {
	case HeaderSync:
	default:
		fetchers = append(fetchers, func() error { return d.fetchHeaders(origin, latest) })
		fetchers = append(fetchers, func() error { return d.fetchBodies(latest) })
		fetchers = append(fetchers, func() error { return d.processHeaders() })
	}

	// assemble
	fetchers = append(fetchers, func() error { return d.processBodies() })
	fetchers = append(fetchers, func() error { return d.processChain() })

	return d.spawnSync(fetchers)
}

// spawnSync
func (d *Downloader) spawnSync(fetchers []func() error) error {
	errc := make(chan error, len(fetchers))
	d.cancelWg.Add(len(fetchers))
	for _, fn := range fetchers {
		fn := fn
		go func() { defer d.cancelWg.Done(); errc <- fn() }()
	}
	var err error
	for i := 0; i < len(fetchers); i++ {
		if i == len(fetchers)-1 {
		}
		if err = <-errc; err != nil {
			break
		}
	}
	d.Close()
	return err
}

func (d *Downloader) SyncHeader() error {
	return d.doSync(HeaderSync)
}

func (d *Downloader) SyncBody() error {
	return nil
}

func (d *Downloader) SyncTx() error {
	return nil
}

func (d *Downloader) IsDownloading() bool {
	ok := atomic.LoadInt32(&d.isDownloading)
	if ok == 1 {
		return true
	} else if ok == 0 {
		return false
	}
	return true
}

func (d *Downloader) findAncestor() (uint256.Int, error) {
	return *d.bc.CurrentBlock().Number64(), nil
}

func (d *Downloader) findHead() (uint256.Int, error) {
	//if d.highestNumber.IsEmpty {
	//	return d.highestNumber, ErrSyncBlock
	//}
	return d.highestNumber, nil
}

func (d *Downloader) pubSubLoop() {
	defer func() {
		close(d.errorCh)
	}()
	defer d.cancel()

	highestBlockCh := make(chan common.ChainHighestBlock)
	defer close(highestBlockCh)
	highestSub := event.GlobalEvent.Subscribe(highestBlockCh)
	defer highestSub.Unsubscribe()

	for {
		select {
		case <-d.ctx.Done():
			return
		case err := <-highestSub.Err():
			log.Debugf("receive a err from highestSub %v", err)
			return
		case highestBlock, ok := <-highestBlockCh:
			if ok && highestBlock.Block.Number64().Uint64() > d.highestNumber.Uint64() {
				log.Debugf("receive a new highestBlock block number: %d", highestBlock.Block.Number64().Uint64())
				d.highestNumber = *highestBlock.Block.Number64()
				if highestBlock.Inserted {
					d.peersInfo.peerInfoBroadcast(highestBlock.Block.Number64())
				}
				//else {
				//	d.bodyResultStore[*highestBlock.Block.Number64()] = highestBlock.Block.ToProtoMessage().(*types_pb.PBlock)
				//}
			}
		}
	}
}

// runLoop
func (d *Downloader) synchronise() {
	log.Info("start downloader")
	defer log.Info("downloader finished")
	//
	d.waitAvailablePeer()
	//
	event.GlobalEvent.Send(&common.DownloaderStartEvent{})
	defer event.GlobalEvent.Send(&common.DownloaderFinishEvent{})

	defer d.cancel()
	tick := time.NewTicker(syncTimeTick)
	defer tick.Stop()
	// checked := 1

	for {
		select {
		case <-d.ctx.Done():
			return
		case err, ok := <-d.errorCh:
			if ok {
				log.Errorf("failed to running downloader, err:%v", err)
			}
			return
		case <-tick.C:
			difference := new(uint256.Int).Sub(&d.highestNumber, d.bc.CurrentBlock().Number64())
			log.Tracef("highest: %d, current: %d", d.highestNumber.Uint64(), d.bc.CurrentBlock().Number64().Uint64())
			if difference.Uint64() > 1 {
				log.Infof("start downloader Compare Loop remote  highestNumber: %d, current number: %d, difference: %d", d.highestNumber.Uint64(), d.bc.CurrentBlock().Number64().Uint64(), difference.Uint64())
				err := d.doSync(d.getMode())
				if err != nil {
					log.Errorf("failed to running downloader, err:%v", err)
				}
				return
			}
			//if d.highestNumber.Uint64() != 0 && difference.Uint64() ==0 {
			//	return
			//}
			//} else {
			//	if checked >= syncCheckTimes {
			//		return
			//	}
			//	checked++
			//}
			tick.Reset(syncTimeTick)
		}
	}
}

func (d Downloader) calculateHeight(peer2 common.Peer) error {
	if d.bc.CurrentBlock().Number64().Uint64() == 0 {

	}
	return nil
}

func (d *Downloader) ConnHandler(data []byte, ID peer.ID) error {
	p, ok := d.peersInfo.get(ID)
	if !ok {
		return ErrBadPeer
	}

	syncTask := sync_proto.SyncTask{}
	if err := proto.Unmarshal(data, &syncTask); err != nil {
		log.Errorf("receive sync task(headersResponse) msg err: %v", err)
		return err
	}

	taskID := syncTask.Id
	params := make([]interface{}, 0)
	params = append(params, "peerID", ID, "taskType", syncTask.SyncType, "taskID", taskID, "isOK", syncTask.Ok)

	switch syncTask.SyncType {

	case sync_proto.SyncType_HeaderRes:
		headersResponse := syncTask.Payload.(*sync_proto.SyncTask_SyncHeaderResponse).SyncHeaderResponse
		params = append(params, "headerCount", len(headersResponse.Headers), "headerNumberFrom", utils.ConvertH256ToUint256Int(headersResponse.Headers[0].Number).Uint64(), "headerNumberTo", utils.ConvertH256ToUint256Int(headersResponse.Headers[len(headersResponse.Headers)-1].Number).Uint64())
		d.headerProcCh <- &headerResponse{taskID: taskID, ok: syncTask.Ok, headers: headersResponse.Headers}

	case sync_proto.SyncType_HeaderReq:
		headerRequest := syncTask.Payload.(*sync_proto.SyncTask_SyncHeaderRequest).SyncHeaderRequest
		params = append(params, "Amount", utils.ConvertH256ToUint256Int(headerRequest.Amount).Uint64(), "headerNumberFrom", utils.ConvertH256ToUint256Int(headerRequest.Number).Uint64())
		go d.responseHeaders(taskID, p, headerRequest)

	case sync_proto.SyncType_BodyRes:
		bodiesResponse := syncTask.Payload.(*sync_proto.SyncTask_SyncBlockResponse).SyncBlockResponse
		params = append(params, "blocksCount", len(bodiesResponse.Blocks), "bodyNumberFrom", utils.ConvertH256ToUint256Int(bodiesResponse.Blocks[0].Header.Number).Uint64(), "bodyNumberTo", utils.ConvertH256ToUint256Int(bodiesResponse.Blocks[len(bodiesResponse.Blocks)-1].Header.Number).Uint64())
		d.blockProcCh <- &bodyResponse{taskID: taskID, ok: syncTask.Ok, bodies: bodiesResponse.Blocks}

	case sync_proto.SyncType_BodyReq:
		blockRequest := syncTask.Payload.(*sync_proto.SyncTask_SyncBlockRequest).SyncBlockRequest
		params = append(params, "bodyNumberFrom", utils.ConvertH256ToUint256Int(blockRequest.Number[0]).Uint64(), "bodyNumberTo", utils.ConvertH256ToUint256Int(blockRequest.Number[len(blockRequest.Number)-1]).Uint64())
		go d.responseBlocks(taskID, p, blockRequest)

	case sync_proto.SyncType_PeerInfoBroadcast:
		peerInfoBroadcast := syncTask.Payload.(*sync_proto.SyncTask_SyncPeerInfoBroadcast).SyncPeerInfoBroadcast
		//
		currentNumber := utils.ConvertH256ToUint256Int(peerInfoBroadcast.Number)
		currentDifficulty := utils.ConvertH256ToUint256Int(peerInfoBroadcast.Difficulty)
		params = append(params, "Number", currentNumber, "Difficulty", currentDifficulty)
		//
		if currentNumber.Uint64() > d.highestNumber.Uint64() {
			d.highestNumber.Set(currentNumber.Clone())
		}
		d.peersInfo.update(p.ID(), currentNumber, currentDifficulty)
	}

	log.Info("receive sync task msg", params...)

	return nil
}

func (d *Downloader) Close() error {
	d.cancelLock.Lock()
	defer d.cancelLock.Unlock()
	d.cancel()
	d.cancelWg.Wait()
	return nil
}
