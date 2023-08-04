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

package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/math"
	"github.com/amazechain/amc/contracts/deposit"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/amazechain/amc/api/protocol/msg_proto"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	ErrKnownBlock           = errors.New("block already known")
	ErrUnknownAncestor      = errors.New("unknown ancestor")
	ErrPrunedAncestor       = errors.New("pruned ancestor")
	ErrFutureBlock          = errors.New("block in the future")
	ErrInvalidNumber        = errors.New("invalid block number")
	ErrInvalidTerminalBlock = errors.New("insertion is interrupted")
	errChainStopped         = errors.New("blockchain is stopped")
	errInsertionInterrupted = errors.New("insertion is interrupted")
	errBlockDoesNotExist    = errors.New("block does not exist in blockchain")
)

type WriteStatus byte

const (
	NonStatTy   WriteStatus = iota //
	CanonStatTy                    //
	SideStatTy                     //
)

const (
	//maxTimeFutureBlocks
	blockCacheLimit     = 1024
	receiptsCacheLimit  = 32
	maxFutureBlocks     = 256
	maxTimeFutureBlocks = 5 * 60 // 5 min

	headerCacheLimit = 1024
	tdCacheLimit     = 1024
	numberCacheLimit = 2048
)

type BlockChain struct {
	chainConfig  *params.ChainConfig
	engineConf   *params.ConsensusConfig
	ctx          context.Context
	cancel       context.CancelFunc
	genesisBlock block2.IBlock
	blocks       []block2.IBlock
	headers      []block2.IHeader
	currentBlock atomic.Pointer[block2.Block]
	//state        *statedb.StateDB
	ChainDB kv.RwDB
	engine  consensus.Engine

	insertLock    chan struct{}
	latestBlockCh chan block2.IBlock
	lock          sync.Mutex

	peers map[peer.ID]bool

	downloader common.IDownloader

	chBlocks chan block2.IBlock

	pubsub common.IPubSub
	p2p    p2p.P2P

	errorCh chan error

	process Processor

	wg sync.WaitGroup //

	procInterrupt int32 // insert chain
	futureBlocks  *lru.Cache[types.Hash, *block2.Block]
	receiptCache  *lru.Cache[types.Hash, []*block2.Receipt]
	blockCache    *lru.Cache[types.Hash, *block2.Block]

	headerCache *lru.Cache[types.Hash, *block2.Header]
	numberCache *lru.Cache[types.Hash, uint64]
	tdCache     *lru.Cache[types.Hash, *uint256.Int]

	forker    *ForkChoice
	validator Validator
}

type insertStats struct {
	queued, processed, ignored int
	usedGas                    uint64
	lastIndex                  int
	startTime                  time.Time
}

//func (bc *BlockChain) GetState() *statedb.StateDB {
//	return bc.state
//}

func (bc *BlockChain) Engine() consensus.Engine {
	return bc.engine
}

func NewBlockChain(ctx context.Context, genesisBlock block2.IBlock, engine consensus.Engine, downloader common.IDownloader, db kv.RwDB, p2p p2p.P2P, pConf *params.ChainConfig, eConf *params.ConsensusConfig) (common.IBlockChain, error) {
	c, cancel := context.WithCancel(ctx)
	var current *block2.Block
	_ = db.View(c, func(tx kv.Tx) error {
		current = rawdb.ReadCurrentBlock(tx)
		if current == nil {
			current = genesisBlock.(*block2.Block)
		}
		return nil
	})

	blockCache, _ := lru.New[types.Hash, *block2.Block](blockCacheLimit)
	futureBlocks, _ := lru.New[types.Hash, *block2.Block](maxFutureBlocks)
	receiptsCache, _ := lru.New[types.Hash, []*block2.Receipt](receiptsCacheLimit)
	tdCache, _ := lru.New[types.Hash, *uint256.Int](tdCacheLimit)
	numberCache, _ := lru.New[types.Hash, uint64](numberCacheLimit)
	headerCache, _ := lru.New[types.Hash, *block2.Header](headerCacheLimit)
	bc := &BlockChain{
		chainConfig:  pConf, // Chain & network configuration
		engineConf:   eConf,
		genesisBlock: genesisBlock,
		blocks:       []block2.IBlock{},
		//currentBlock:  current,
		ChainDB:       db,
		ctx:           c,
		cancel:        cancel,
		insertLock:    make(chan struct{}, 1),
		peers:         make(map[peer.ID]bool),
		chBlocks:      make(chan block2.IBlock, 100),
		errorCh:       make(chan error),
		p2p:           p2p,
		downloader:    downloader,
		latestBlockCh: make(chan block2.IBlock, 50),
		engine:        engine,
		blockCache:    blockCache,
		tdCache:       tdCache,
		futureBlocks:  futureBlocks,
		receiptCache:  receiptsCache,

		numberCache: numberCache,
		headerCache: headerCache,
	}

	bc.currentBlock.Store(current)
	bc.forker = NewForkChoice(bc, nil)
	//bc.process = avm.NewVMProcessor(ctx, bc, engine)
	bc.process = NewStateProcessor(pConf, bc, engine)
	bc.validator = NewBlockValidator(pConf, bc, engine)

	return bc, nil
}

func (bc *BlockChain) Config() *params.ChainConfig {
	return bc.chainConfig
}

//func (bc *BlockChain) StateAt(root types.Hash) evmtypes.IntraBlockState {
//	tx, err := bc.ChainDB.BeginRo(bc.ctx)
//	if nil != err {
//		return nil
//	}
//
//	blockNr := rawdb.ReadHeaderNumber(tx, root)
//	if nil == blockNr {
//		return nil
//	}
//
//	stateReader := state.NewStateHistoryReader(tx, tx, *blockNr)
//	return state.New(stateReader)
//	//return statedb.NewStateDB(root, bc.chainDB)
//}

func (bc *BlockChain) CurrentBlock() block2.IBlock {
	return bc.currentBlock.Load()
}

func (bc *BlockChain) Blocks() []block2.IBlock {
	return bc.blocks
}

func (bc *BlockChain) InsertHeader(headers []block2.IHeader) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (bc *BlockChain) GenesisBlock() block2.IBlock {
	return bc.genesisBlock
}

func (bc *BlockChain) Start() error {
	//if bc.pubsub == nil {
	//	return ErrInvalidPubSub
	//}

	bc.wg.Add(3)
	go bc.runLoop()
	//go bc.newBlockLoop()
	go bc.updateFutureBlocksLoop()

	return nil
}

// verifyBody
// Deprecated:
func (bc *BlockChain) verifyBody(block block2.IBlock) error {
	return nil
}

// verifyState
// Deprecated:
func (bc *BlockChain) verifyState(block block2.IBlock, state *state.IntraBlockState, receipts block2.Receipts, usedGas uint64) error {
	return nil
}

func (bc *BlockChain) AddPeer(hash string, remoteBlock uint64, peerID peer.ID) error {
	if bc.genesisBlock.Hash().String() != hash {
		return fmt.Errorf("failed to addPeer, err: genesis block different")
	}
	if _, ok := bc.peers[peerID]; ok {
		return fmt.Errorf("failed to addPeer, err: the peer already exists")
	}

	log.Debugf("local heigth:%d --> remote height: %d", bc.CurrentBlock().Number64(), remoteBlock)

	bc.peers[peerID] = true
	//if remoteBlock > bc.currentBlock.Number64().Uint64() {
	//	bc.syncChain(remoteBlock, peerID)
	//}

	return nil
}

func (bc *BlockChain) GetReceipts(blockHash types.Hash) (block2.Receipts, error) {
	rtx, err := bc.ChainDB.BeginRo(bc.ctx)
	defer rtx.Rollback()
	if nil != err {
		return nil, err
	}
	return rawdb.ReadReceiptsByHash(rtx, blockHash)
}

func (bc *BlockChain) GetLogs(blockHash types.Hash) ([][]*block2.Log, error) {
	receipts, err := bc.GetReceipts(blockHash)
	if err != nil {
		return nil, err
	}

	logs := make([][]*block2.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

// InsertBlock
// Deprecated:
func (bc *BlockChain) InsertBlock(blocks []block2.IBlock, isSync bool) (int, error) {
	//bc.lock.Lock()
	//defer bc.lock.Unlock()
	//runBlock := func(b block2.IBlock) error {
	//	//todo copy?
	//	stateDB := statedb.NewStateDB(b.ParentHash(), bc.chainDB, bc.changeDB)
	//
	//	receipts, logs, usedGas, err := bc.process.Processor(b, stateDB)
	//	if err != nil {
	//		return err
	//	}
	//	//verify state
	//	if err = bc.verifyState(b, stateDB, receipts, usedGas); err != nil {
	//		return err
	//	}
	//	_, err = stateDB.Commit(b.Number64())
	//	if err != nil {
	//		return err
	//	}
	//
	//	rawdb.StoreReceipts(bc.chainDB, b.Hash(), receipts)
	//
	//	if len(logs) > 0 {
	//		event.GlobalEvent.Send(&common.NewLogsEvent{Logs: logs})
	//	}
	//	if len(receipts) > 0 {
	//		log.Infof("Receipt len(%d), receipts: [%v]", len(receipts), receipts)
	//	}
	//	return nil
	//}
	//
	//current := bc.CurrentBlock()
	//var insertBlocks []block2.IBlock
	//for i, block := range blocks {
	//	if block.Number64() == current.Number64() && block.Difficulty().Compare(current.Difficulty()) == 1 {
	//		if err := bc.engine.VerifyHeader(bc, block.Header(), false); err != nil {
	//			log.Errorf("failed verify block err: %v", err)
	//			continue
	//		}
	//		//verify body
	//		if err := bc.verifyBody(block); err != nil {
	//			log.Errorf("failed verify block err: %v", err)
	//			continue
	//		}
	//		if err := runBlock(block); err != nil {
	//			log.Errorf("failed runblock, err:%v", err)
	//		}
	//		insertBlocks = append(insertBlocks, block)
	//		current = blocks[i]
	//
	//	} else if block.Number64().Equal(current.Number64().Add(uint256.NewInt(1))) && block.ParentHash().String() == current.Hash().String() {
	//		if err := bc.engine.VerifyHeader(bc, block.Header(), false); err != nil {
	//			log.Errorf("failed verify block err: %v", err)
	//			continue
	//		}
	//
	//		if err := runBlock(block); err != nil {
	//			log.Errorf("failed runblock, err:%v", err)
	//		} else {
	//			insertBlocks = append(insertBlocks, block)
	//			current = blocks[i]
	//		}
	//	} else {
	//		author, _ := bc.engine.Author(block.Header())
	//		log.Errorf("failed instert mew block, hash: %s, number: %s, diff: %s, miner: %s, txs: %d", block.Hash(), block.Number64().String(), block.Difficulty().String(), author.String(), len(block.Transactions()))
	//		log.Errorf("failed instert cur block, hash: %s, number: %s, diff: %s, miner: %s, txs: %d", current.Hash(), current.Number64().String(), current.Difficulty().String(), author.String(), len(current.Transactions()))
	//	}
	//
	//}
	//
	//if len(insertBlocks) > 0 {
	//	if _, err := rawdb.SaveBlocks(bc.chainDB, insertBlocks); err != nil {
	//		log.Errorf("failed to save blocks, err: %v", err)
	//		return 0, err
	//	}
	//
	//	if err := rawdb.SaveLatestBlock(bc.chainDB, current); err != nil {
	//		log.Errorf("failed to save lates blocks, err: %v", err)
	//		return 0, err
	//	}
	//	bc.currentBlock = current
	//
	//	if !isSync {
	//		i := event.GlobalEvent.Send(&current)
	//		author, _ := bc.engine.Author(current.Header())
	//		log.Debugf("current number:%d, miner: %s, feed send count: %d", current.Number64().Uint64(), author, i)
	//	}
	//
	//	return len(insertBlocks), nil
	//}

	return 0, fmt.Errorf("invalid block len(%d)", len(blocks))
}

func (bc *BlockChain) LatestBlockCh() (block2.IBlock, error) {
	select {
	case <-bc.ctx.Done():
		return nil, fmt.Errorf("the main chain is closed")
	case block, ok := <-bc.latestBlockCh:
		if !ok {
			return nil, fmt.Errorf("the main chain is closed")
		}

		return block, nil
	}
}

func (bc *BlockChain) newBlockLoop() {
	bc.wg.Done()
	//if bc.pubsub == nil {
	//	bc.errorCh <- ErrInvalidPubSub
	//	return
	//}

	topic, err := bc.pubsub.JoinTopic(message.GossipBlockMessage)
	if err != nil {
		bc.errorCh <- ErrInvalidPubSub
		return
	}

	sub, err := topic.Subscribe()
	if err != nil {
		bc.errorCh <- ErrInvalidPubSub
		return
	}

	for {
		select {
		case <-bc.ctx.Done():
			log.Infof("block chain quit...")
			return
		default:
			msg, err := sub.Next(bc.ctx)
			if err != nil {
				//todo panic: send on closed channel
				bc.errorCh <- err
				return
			}

			var newBlock types_pb.Block
			if err := proto.Unmarshal(msg.Data, &newBlock); err == nil {
				var block block2.Block
				if err := block.FromProtoMessage(&newBlock); err == nil {
					var inserted bool
					// if future block
					if block.Number64().Uint64() > bc.CurrentBlock().Number64().Uint64()+1 {
						inserted = false
						bc.AddFutureBlock(&block)
					} else {
						if _, err := bc.InsertChain([]block2.IBlock{&block}); err != nil {
							inserted = false
							log.Errorf("failed to inster new block in blockchain, err:%v", err)
						} else {
							log.Info("Imported new chain segment", "hash", block.Hash(), "number", block.Number64().Uint64(), "stateRoot", block.StateRoot(), "txs", len(block.Body().Transactions()))
							inserted = true
						}
					}
					event.GlobalEvent.Send(&common.ChainHighestBlock{Block: block, Inserted: inserted})

				} else {
					log.Errorf("unmarshal err: %v", err)
				}

			} else {
				log.Warnf("failed to unmarshal pubsub message, err:%v", err)
			}
		}
	}

}

func (bc *BlockChain) runLoop() {
	defer func() {
		bc.wg.Done()
		bc.cancel()
		bc.StopInsert()
		close(bc.errorCh)
		bc.wg.Wait()
	}()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case err, ok := <-bc.errorCh:
			if ok {
				log.Errorf("receive error from action, err:%v", err)
				return
			}
		}
	}
}

// updateFutureBlocksLoop
func (bc *BlockChain) updateFutureBlocksLoop() {
	futureTimer := time.NewTicker(2 * time.Second)
	defer futureTimer.Stop()
	defer bc.wg.Done()
	for {
		select {
		case <-futureTimer.C:
			if bc.futureBlocks.Len() > 0 {
				blocks := make([]block2.IBlock, 0, bc.futureBlocks.Len())
				for _, key := range bc.futureBlocks.Keys() {
					if value, ok := bc.futureBlocks.Get(key); ok {
						blocks = append(blocks, value)
					}
				}
				sort.Slice(blocks, func(i, j int) bool {
					return blocks[i].Number64().Cmp(blocks[j].Number64()) < 0
				})

				if blocks[0].Number64().Uint64() > bc.CurrentBlock().Number64().Uint64()+1 {
					continue
				}

				if n, err := bc.InsertChain(blocks); nil != err {
					log.Warn("insert future block failed", err)
				} else {
					for _, k := range bc.futureBlocks.Keys() {
						bc.futureBlocks.Remove(k)
					}
					log.Infof("insert %d future block success, for %d to %d", n, blocks[0].Number64().Uint64(), blocks[n-1].Number64().Uint64())
				}

			}
		case <-bc.ctx.Done():
			return
		}
	}
}

func (bc *BlockChain) runNewBlockMessage() {
	newBlockCh := make(chan msg_proto.NewBlockMessageData, 10)
	sub := event.GlobalEvent.Subscribe(newBlockCh)
	defer sub.Unsubscribe()
	db := bc.ChainDB
	for {
		select {
		case <-bc.ctx.Done():
			return
		case err := <-sub.Err():
			log.Errorf("failed subscribe new block at blockchain err :%v", err)
			return
		case block, ok := <-bc.chBlocks:
			if ok {

				//if err := bc.InsertBlock([]*block_proto.Block{block}); err != nil {
				//	log.Errorf("failed insert block into block chain, number:%d, err: %v", block.Header.Number, err)
				//}
				_ = db.Update(bc.ctx, func(tx kv.RwTx) error {
					rawdb.WriteBlock(tx, block.(*block2.Block))
					rawdb.WriteHeadBlockHash(tx, block.Hash())
					_ = rawdb.ReadCurrentBlock(tx)
					return nil
				})
			}
		case msg, ok := <-newBlockCh:
			if ok {
				block := block2.Block{}
				if err := block.FromProtoMessage(msg.Block); err == nil {
					_ = db.Update(bc.ctx, func(tx kv.RwTx) error {
						rawdb.WriteBlock(tx, &block)
						rawdb.WriteHeadBlockHash(tx, block.Hash())
						_ = rawdb.ReadCurrentBlock(tx)
						return nil
					})
				}
			}
		}
	}
}

func (bc *BlockChain) syncChain(remoteBlock uint64, peerID peer.ID) {
	/*sync chain
	 */
	//if remoteBlock < bc.currentBlock.Header.Number {
	//	return
	//}
	//var startNumber uint64
	//if bc.currentBlock.Header.Number == 0 {
	//	startNumber = bc.currentBlock.Header.Number
	//}
	log.Debugf("syncChain.......")
}

func (bc *BlockChain) GetHeader(h types.Hash, number *uint256.Int) block2.IHeader {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := bc.headerCache.Get(h); ok {
		return header
	}

	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		return nil
	}
	defer tx.Rollback()
	header := rawdb.ReadHeader(tx, h, number.Uint64())
	if nil == header {
		return nil
	}

	bc.headerCache.Add(h, header)
	return header
}

func (bc *BlockChain) GetHeaderByNumber(number *uint256.Int) block2.IHeader {
	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		log.Error("cannot open chain db", "err", err)
		return nil
	}
	defer tx.Rollback()

	hash, err := rawdb.ReadCanonicalHash(tx, number.Uint64())
	if nil != err {
		log.Error("cannot open chain db", "err", err)
		return nil
	}
	if hash == (types.Hash{}) {
		return nil
	}

	//return bc.GetHeader(hash, number)
	if header, ok := bc.headerCache.Get(hash); ok {
		return header
	}
	header := rawdb.ReadHeader(tx, hash, number.Uint64())
	if nil == header {
		return nil
	}
	bc.headerCache.Add(hash, header)
	return header
}

func (bc *BlockChain) GetHeaderByHash(h types.Hash) (block2.IHeader, error) {
	number := bc.GetBlockNumber(h)
	if number == nil {
		return nil, nil
	}

	return bc.GetHeader(h, uint256.NewInt(*number)), nil
}

// GetCanonicalHash returns the canonical hash for a given block number
func (bc *BlockChain) GetCanonicalHash(number *uint256.Int) types.Hash {
	//block, err := bc.GetBlockByNumber(number)
	//if nil != err {
	//	return types.Hash{}
	//}
	//
	//return block.Hash()
	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		return types.Hash{}
	}
	defer tx.Rollback()

	hash, err := rawdb.ReadCanonicalHash(tx, number.Uint64())
	if nil != err {
		return types.Hash{}
	}
	return hash
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (bc *BlockChain) GetBlockNumber(hash types.Hash) *uint64 {
	if cached, ok := bc.numberCache.Get(hash); ok {
		return &cached
	}
	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		return nil
	}
	defer tx.Rollback()
	number := rawdb.ReadHeaderNumber(tx, hash)
	if number != nil {
		bc.numberCache.Add(hash, *number)
	}
	return number
}

func (bc *BlockChain) GetBlockByHash(h types.Hash) (block2.IBlock, error) {
	number := bc.GetBlockNumber(h)
	if nil == number {
		return nil, errBlockDoesNotExist
	}
	return bc.GetBlock(h, *number), nil
}

func (bc *BlockChain) GetBlockByNumber(number *uint256.Int) (block2.IBlock, error) {
	var hash types.Hash
	bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {
		hash, _ = rawdb.ReadCanonicalHash(tx, number.Uint64())
		return nil
	})

	if hash == (types.Hash{}) {
		return nil, nil
	}
	return bc.GetBlock(hash, number.Uint64()), nil
}

func (bc *BlockChain) NewBlockHandler(payload []byte, peer peer.ID) error {

	var nweBlock msg_proto.NewBlockMessageData
	if err := proto.Unmarshal(payload, &nweBlock); err != nil {
		log.Errorf("failed unmarshal to msg, from peer:%s", peer)
		return err
	} else {
		var block block2.Block
		if err := block.FromProtoMessage(nweBlock.GetBlock()); err == nil {
			var block block2.Block
			if err := block.FromProtoMessage(nweBlock.GetBlock()); err == nil {
				bc.chBlocks <- &block
			}
		}
	}
	return nil
}

func (bc *BlockChain) SetEngine(engine consensus.Engine) {
	bc.engine = engine
}

func (bc *BlockChain) GetBlocksFromHash(hash types.Hash, n int) (blocks []block2.IBlock) {
	var number *uint64
	if num, ok := bc.numberCache.Get(hash); ok {
		number = &num
	} else {
		bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {
			number = rawdb.ReadHeaderNumber(tx, hash)
			return nil
		})
		if number == nil {
			return nil
		}
		bc.numberCache.Add(hash, *number)
	}

	for i := 0; i < n; i++ {
		block := bc.GetBlock(hash, *number)
		if block == nil {
			break
		}

		blocks = append(blocks, block)
		hash = block.ParentHash()
		*number--
	}
	return blocks
}

func (bc *BlockChain) GetBlock(hash types.Hash, number uint64) block2.IBlock {
	if hash == (types.Hash{}) {
		return nil
	}

	if block, ok := bc.blockCache.Get(hash); ok {
		return block
	}

	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		return nil
	}
	defer tx.Rollback()
	block := rawdb.ReadBlock(tx, hash, number)
	if block == nil {
		return nil
	}
	bc.blockCache.Add(hash, block)
	return block
	//header, err := rawdb.ReadHeaderByHash(tx, hash)
	//if err != nil {
	//	return nil
	//}
	//
	//if hash != header.Hash() {
	//	log.Error("Failed to get block, the hash is differ", "hash", hash.String(), "headerHash", header.Hash().String())
	//	return nil
	//}
	//
	//body, err := rawdb.ReadBlockByHash(tx, header.Hash())
	//if err != nil {
	//	log.Error("Failed to get block body", "err", err)
	//	return nil
	//}

	//return block2.NewBlock(header, body.Transactions())
}

func (bc *BlockChain) SealedBlock(b block2.IBlock) error {
	pbBlock := b.ToProtoMessage()
	//_ = bc.pubsub.Publish(message.GossipBlockMessage, pbBlock)
	return bc.p2p.Broadcast(context.TODO(), pbBlock)
}

// StopInsert stop insert
func (bc *BlockChain) StopInsert() {
	atomic.StoreInt32(&bc.procInterrupt, 1)
}

// insertStopped returns true after StopInsert has been called.
func (bc *BlockChain) insertStopped() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

// HasBlockAndState
func (bc *BlockChain) HasBlockAndState(hash types.Hash, number uint64) bool {
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.Hash())
}

// HasState
func (bc *BlockChain) HasState(hash types.Hash) bool {
	tx, err := bc.ChainDB.BeginRo(bc.ctx)
	if nil != err {
		return false
	}
	defer tx.Rollback()
	is, err := rawdb.IsCanonicalHash(tx, hash)
	if nil != err {
		return false
	}
	return is
}

func (bc *BlockChain) HasBlock(hash types.Hash, number uint64) bool {
	var flag bool
	if bc.blockCache.Contains(hash) {
		return true
	}

	bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {
		flag = rawdb.HasHeader(tx, hash, number)
		return nil
	})

	return flag
}

// GetTd
func (bc *BlockChain) GetTd(hash types.Hash, number *uint256.Int) *uint256.Int {

	if td, ok := bc.tdCache.Get(hash); ok {
		return td
	}

	var td *uint256.Int
	if err := bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {

		ptd, err := rawdb.ReadTd(tx, hash, number.Uint64())
		if nil != err {
			return err
		}
		td = ptd
		return nil
	}); nil != err {
		log.Error("get total difficulty failed", "err", err, "hash", hash, "number", number)
	}

	if td != nil {
		bc.tdCache.Add(hash, td)
	}
	return td
}

func (bc *BlockChain) skipBlock(err error) bool {
	if !errors.Is(err, ErrKnownBlock) {
		return false
	}
	return true
}

// InsertChain
func (bc *BlockChain) InsertChain(chain []block2.IBlock) (int, error) {
	if len(chain) == 0 {
		return 0, nil
	}
	//
	for i := 1; i < len(chain); i++ {
		block, prev := chain[i], chain[i-1]
		if block.Number64().Cmp(uint256.NewInt(0).Add(prev.Number64(), uint256.NewInt(1))) != 0 || block.ParentHash() != prev.Hash() {
			log.Error("Non contiguous block insert",
				"number", block.Number64().String(),
				"hash", block.Hash(),
				"parent", block.ParentHash(),
				"prev number", prev.Number64(),
				"prev hash", prev.Hash(),
			)
			return 0, fmt.Errorf("non contiguous insert: item %d is #%s [%x..], item %d is #%s [%x..] (parent [%x..])", i-1, prev.Number64().String(),
				prev.Hash().Bytes()[:4], i, block.Number64().String(), block.Hash().Bytes()[:4], block.ParentHash().Bytes()[:4])
		}
	}
	if !bc.lock.TryLock() {
		return 0, errChainStopped
	}
	defer bc.lock.Unlock()

	return bc.insertChain(chain)
}

func (bc *BlockChain) insertChain(chain []block2.IBlock) (int, error) {
	if bc.insertStopped() {
		return 0, nil
	}

	var (
		stats     = insertStats{startTime: time.Now()}
		lastCanon block2.IBlock
	)

	defer func() {
		if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
			event.GlobalEvent.Send(&common.ChainHighestBlock{Block: *lastCanon.(*block2.Block), Inserted: true})
		}
	}()

	// Start the parallel header verifier
	headers := make([]block2.IHeader, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.Header()
		//seals[i] = true
	}
	abort, results := bc.engine.VerifyHeaders(bc, headers, seals)
	defer close(abort)

	// Peek the error for the first block to decide the directing import logic
	it := newInsertIterator(chain, results, bc.validator)
	block, err := it.next()
	if bc.skipBlock(err) {
		var (
			reorg   bool
			current = bc.CurrentBlock()
		)
		for block != nil && bc.skipBlock(err) {
			reorg, err = bc.forker.ReorgNeeded(current.Header(), block.Header())
			if err != nil {
				return it.index, err
			}
			if reorg {
				// Switch to import mode if the forker says the reorg is necessary
				// and also the block is not on the canonical chain.
				// In eth2 the forker always returns true for reorg decision (blindly trusting
				// the external consensus engine), but in order to prevent the unnecessary
				// reorgs when importing known blocks, the special case is handled here.
				if block.Number64().Uint64() > current.Number64().Uint64() || bc.GetCanonicalHash(block.Number64()) != block.Hash() {
					break
				}
			}
			log.Debug("Ignoring already known block", "number", block.Number64(), "hash", block.Hash())
			stats.ignored++
			block, err = it.next()
		}
		// The remaining blocks are still known blocks, the only scenario here is:
		// During the fast sync, the pivot point is already submitted but rollback
		// happens. Then node resets the head full block to a lower height via `rollback`
		// and leaves a few known blocks in the database.
		//
		// When node runs a fast sync again, it can re-import a batch of known blocks via
		// `insertChain` while a part of them have higher total difficulty than current
		// head full block(new pivot point).
		for block != nil && bc.skipBlock(err) {
			log.Debug("Writing previously known block", "number", block.Number64(), "hash", block.Hash())
			if err := bc.writeKnownBlock(block); err != nil {
				return it.index, err
			}
			lastCanon = block
			block, err = it.next()
		}
	}

	switch {
	// First block is pruned
	case errors.Is(err, ErrPrunedAncestor):
		// First block is pruned, insert as sidechain and reorg only if TD grows enough
		log.Debug("Pruned ancestor, inserting as sidechain", "number", block.Number64(), "hash", block.Hash())
		return bc.insertSideChain(block, it)

	// First block is future, shove it (and all children) to the future queue (unknown ancestor)
	case errors.Is(err, ErrFutureBlock) || (errors.Is(err, ErrUnknownAncestor) && bc.futureBlocks.Contains(it.first().ParentHash())):
		for block != nil && (it.index == 0 || errors.Is(err, ErrUnknownAncestor)) {
			log.Debug("Future block, postponing import", "number", block.Number64(), "hash", block.Hash())
			if err := bc.AddFutureBlock(block); err != nil {
				return it.index, err
			}
			block, err = it.next()
		}
		stats.queued += it.processed()
		stats.ignored += it.remaining()

		// If there are any still remaining, mark as ignored
		return it.index, err

	// Some other error(except ErrKnownBlock) occurred, abort.
	// ErrKnownBlock is allowed here since some known blocks
	// still need re-execution to generate snapshots that are missing
	case err != nil && !errors.Is(err, ErrKnownBlock):
		bc.futureBlocks.Remove(block.Hash())
		stats.ignored += len(it.chain)
		bc.reportBlock(block, nil, err)
		return it.index, err
	}

	//wtx, err := bc.ChainDB.BeginRw(bc.ctx)
	//if nil != err {
	//	return it.index, err
	//}
	//defer wtx.Rollback()

	//var batch ethdb.DbWithPendingMutations

	// state is stored through ethdb batches
	//batch = olddb.NewHashBatch(wtx, bc.Quit(), paths.DefaultDataDir())
	//// avoids stacking defers within the loop
	//defer func() {
	//	batch.Rollback()
	//}()

	evmRecord := func(ctx context.Context, db kv.RwDB, blockNr uint64, f func(tx kv.Tx, ibs *state.IntraBlockState, reader state.StateReader, writer state.WriterWithChangeSets) (map[types.Address]*uint256.Int, error)) (*state.IntraBlockState, map[types.Address]*uint256.Int, error) {
		tx, err := db.BeginRo(ctx)
		if nil != err {
			return nil, nil, err
		}
		defer tx.Rollback()
		//batch := olddb.NewHashBatch(tx, bc.Quit(), paths.DefaultDataDir())
		//defer batch.Rollback()
		//stateReader, stateWriter, err := NewStateReaderWriter(batch, tx, blockNr, true)
		//if nil != err {
		//	return err
		//}

		stateReader := state.NewPlainStateReader(tx)
		ibs := state.New(stateReader)
		//stateWriter := state.NewPlainStateWriter(tx, tx, block.Number64().Uint64())
		stateWriter := state.NewNoopWriter()

		var nopay map[types.Address]*uint256.Int
		nopay, err = f(tx, ibs, stateReader, stateWriter)
		if nil != err {
			return nil, nil, err
		}

		//if err := batch.Commit(); nil != err {
		//	return err
		//}
		return ibs, nopay, nil
		// return tx.Commit()
	}

	for ; block != nil && err == nil || errors.Is(err, ErrKnownBlock); block, err = it.next() {
		// If the chain is terminating, stop processing blocks
		if bc.insertStopped() {
			log.Debug("Abort during block processing")
			break
		}

		log.Tracef("Current block: number=%v, hash=%v, difficult=%v | Insert block block: number=%v, hash=%v, difficult= %v",
			bc.CurrentBlock().Number64(), bc.CurrentBlock().Hash(), bc.CurrentBlock().Difficulty(), block.Number64(), block.Hash(), block.Difficulty())
		// Retrieve the parent block and it's state to execute on top
		start := time.Now()
		// TODO
		//stateDB := statedb.NewStateDB(block.ParentHash(), bc.chainDB, bc.changeDB)
		//stateReader, stateWriter, err := NewStateReaderWriter(batch, wtx, block.Number64().Uint64(), true)
		//if nil != err {
		//	return it.index, err
		//}
		//ibs := state.New(stateReader)

		var receipts block2.Receipts
		var logs []*block2.Log
		var usedGas uint64
		ibs, nopay, err := evmRecord(bc.ctx, bc.ChainDB, block.Number64().Uint64(), func(tx kv.Tx, ibs *state.IntraBlockState, reader state.StateReader, writer state.WriterWithChangeSets) (map[types.Address]*uint256.Int, error) {
			getHeader := func(hash types.Hash, number uint64) *block2.Header {
				return rawdb.ReadHeader(tx, hash, number)
			}
			blockHashFunc := GetHashFn(block.Header().(*block2.Header), getHeader)

			var err error
			var nopay map[types.Address]*uint256.Int
			receipts, nopay, logs, usedGas, err = bc.process.Process(block.(*block2.Block), ibs, reader, writer, blockHashFunc)
			if err != nil {
				bc.reportBlock(block, receipts, err)
				//atomic.StoreUint32(&followupInterrupt, 1)
				return nil, err
			}
			if err := bc.validator.ValidateState(block, ibs, receipts, usedGas); err != nil {
				bc.reportBlock(block, receipts, err)
				//atomic.StoreUint32(&followupInterrupt, 1)
				return nil, err
			}

			return nopay, nil
		})
		if nil != err {
			return it.index, err
		}
		//var followupInterrupt uint32
		//receipts, logs, usedGas, err := bc.process.Process(block.(*block2.Block), ibs, stateReader, stateWriter, blockHashFunc)
		//if err != nil {
		//	bc.reportBlock(block, receipts, err)
		//	//atomic.StoreUint32(&followupInterrupt, 1)
		//	return it.index, err
		//}

		//if err := bc.validator.ValidateState(block, ibs, receipts, usedGas); err != nil {
		//	bc.reportBlock(block, receipts, err)
		//	//atomic.StoreUint32(&followupInterrupt, 1)
		//	return it.index, err
		//}

		// write state
		//if err := bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
		//	stateWrite := state.NewPlainStateWriter(batch, tx, block.Number64().Uint64())
		//	if err := ibs.CommitBlock(params.AmazeChainConfig.Rules(block.Number64().Uint64()), stateWrite); nil != err {
		//		return err
		//	}
		//	return nil
		//}); nil != err {
		//	return it.index, err
		//}

		var status WriteStatus
		status, err = bc.writeBlockWithState(block, receipts, ibs, nopay)
		//atomic.StoreUint32(&followupInterrupt, 1)
		if err != nil {
			return it.index, err
		}

		// Report the import stats before returning the various results
		stats.processed++
		stats.usedGas += usedGas

		switch status {
		case CanonStatTy:
			log.Trace("Inserted new block ", "number ", block.Number64(), "hash", block.Hash(),
				"txs", len(block.Transactions()), "gas", block.GasUsed(),
				"elapsed", time.Since(start).Seconds(),
				"root", block.StateRoot())

			if len(logs) > 0 {
				event.GlobalEvent.Send(&common.NewLogsEvent{Logs: logs})
			}

			lastCanon = block

		case SideStatTy:
			log.Debug("Inserted forked block", "number", block.Number64(), "hash", block.Hash(),
				"diff", block.Difficulty(), "elapsed", time.Since(start).Seconds(),
				"txs", len(block.Transactions()), "gas", block.GasUsed(),
				"root", block.StateRoot())

		default:
			// This in theory is impossible, but lets be nice to our future selves and leave
			// a log, instead of trying to track down blocks imports that don't emit logs.
			log.Warn("Inserted block with unknown status", "number", block.Number64(), "hash", block.Hash(),
				"diff", block.Difficulty(), "elapsed", time.Since(start).Seconds(),
				"txs", len(block.Transactions()), "gas", block.GasUsed(),
				"root", block.StateRoot())
		}
	}

	// Any blocks remaining here? The only ones we care about are the future ones
	if block != nil && errors.Is(err, ErrFutureBlock) {
		if err := bc.AddFutureBlock(block); err != nil {
			return it.index, err
		}
		block, err = it.next()

		for ; block != nil && errors.Is(err, ErrUnknownAncestor); block, err = it.next() {
			if err := bc.AddFutureBlock(block); err != nil {
				return it.index, err
			}
			stats.queued++
		}
	}
	stats.ignored += it.remaining()

	return it.index, err
}

// insertSideChain
func (bc *BlockChain) insertSideChain(block block2.IBlock, it *insertIterator) (int, error) {
	var (
		externTd  *uint256.Int
		lastBlock = block
		current   = bc.CurrentBlock()
	)
	err := ErrPrunedAncestor
	for ; block != nil && errors.Is(err, ErrPrunedAncestor); block, err = it.next() {
		// Check the canonical state root for that number
		if number := block.Number64(); current.Number64().Cmp(number) >= 0 {
			canonical, err := bc.GetBlockByNumber(number)
			if nil != err {
				return 0, err
			}

			if canonical != nil && canonical.Hash() == block.Hash() {
				// Not a sidechain block, this is a re-import of a canon block which has it's state pruned

				// Collect the TD of the block. Since we know it's a canon one,
				// we can get it directly, and not (like further below) use
				// the parent and then add the block on top
				pt := bc.GetTd(block.Hash(), block.Number64())
				externTd = pt
				continue
			}
			if canonical != nil && canonical.StateRoot() == block.StateRoot() {
				// This is most likely a shadow-state attack. When a fork is imported into the
				// database, and it eventually reaches a block height which is not pruned, we
				// just found that the state already exist! This means that the sidechain block
				// refers to a state which already exists in our canon chain.
				//
				// If left unchecked, we would now proceed importing the blocks, without actually
				// having verified the state of the previous blocks.
				log.Warn("Sidechain ghost-state attack detected", "number", block.Number64(), "sideroot", block.StateRoot(), "canonroot", canonical.StateRoot())

				// If someone legitimately side-mines blocks, they would still be imported as usual. However,
				// we cannot risk writing unverified blocks to disk when they obviously target the pruning
				// mechanism.
				return it.index, errors.New("sidechain ghost-state attack")
			}
		}
		if externTd == nil {
			externTd = bc.GetTd(block.ParentHash(), uint256.NewInt(0).Sub(block.Number64(), uint256.NewInt(1)))
		}
		externTd = externTd.Add(externTd, block.Difficulty())

		if !bc.HasBlock(block.Hash(), block.Number64().Uint64()) {
			start := time.Now()
			if err := bc.WriteBlockWithoutState(block, externTd); err != nil {
				return it.index, err
			}
			log.Debug("Injected sidechain block", "number", block.Number64(), "hash", block.Hash(),
				"diff", block.Difficulty(), "elapsed", time.Since(start).Seconds(),
				"txs", len(block.Transactions()), "gas", block.GasUsed(),
				"root", block.StateRoot())
		}
		lastBlock = block
	}

	reorg, err := bc.forker.ReorgNeeded(current.Header(), lastBlock.Header())
	if err != nil {
		return it.index, err
	}

	if !reorg {
		localTd := bc.GetTd(current.Hash(), current.Number64())
		log.Info("Sidechain written to disk", "start", it.first().Number64(), "end", it.previous().Number64(), "sidetd", externTd, "localtd", localTd)
		return it.index, err
	}
	var (
		hashes  []types.Hash
		numbers []uint64
	)
	parent := it.previous()
	for parent != nil && !bc.HasState(parent.Hash()) {
		hashes = append(hashes, parent.Hash())
		numbers = append(numbers, parent.Number64().Uint64())

		parent = bc.GetHeader(parent.(*block2.Header).ParentHash, uint256.NewInt(0).Sub(parent.Number64(), uint256.NewInt(1)))
	}
	if parent == nil {
		return it.index, errors.New("missing parent")
	}
	// Import all the pruned blocks to make the state available
	var (
		blocks []block2.IBlock
	)
	for i := len(hashes) - 1; i >= 0; i-- {
		// Append the next block to our batch
		block := bc.GetBlock(hashes[i], numbers[i])

		blocks = append(blocks, block)

		// If memory use grew too large, import and continue. Sadly we need to discard
		// all raised events and logs from notifications since we're too heavy on the
		// memory here.
		if len(blocks) >= 2048 {
			log.Info("Importing heavy sidechain segment", "blocks", len(blocks), "start", blocks[0].Number64(), "end", block.Number64())
			if _, err := bc.insertChain(blocks); err != nil {
				return 0, err
			}
			blocks = blocks[:0]
			// If the chain is terminating, stop processing blocks
			if bc.insertStopped() {
				log.Debug("Abort during blocks processing")
				return 0, nil
			}
		}
	}
	if len(blocks) > 0 {
		log.Info("Importing sidechain segment", "start", blocks[0].Number64(), "end", blocks[len(blocks)-1].Number64())
		return bc.insertChain(blocks)
	}
	return 0, nil
}

// recoverAncestors
func (bc *BlockChain) recoverAncestors(block block2.IBlock) (types.Hash, error) {
	var (
		hashes  []types.Hash
		numbers []uint256.Int
		parent  = block
	)
	for parent != nil && !bc.HasState(parent.Hash()) {
		hashes = append(hashes, parent.Hash())
		numbers = append(numbers, *parent.Number64())
		parent = bc.GetBlock(parent.ParentHash(), parent.Number64().Uint64()-1)

	}
	if parent == nil {
		return types.Hash{}, errors.New("missing parent")
	}
	for i := len(hashes) - 1; i >= 0; i-- {
		var b block2.IBlock
		if i == 0 {
			b = block
		} else {
			b = bc.GetBlock(hashes[i], numbers[i].Uint64())
		}
		if _, err := bc.insertChain([]block2.IBlock{b}); err != nil {
			return b.ParentHash(), err
		}
	}
	return block.Hash(), nil
}

// WriteBlockWithoutState without state
func (bc *BlockChain) WriteBlockWithoutState(block block2.IBlock, td *uint256.Int) (err error) {
	if bc.insertStopped() {
		return errInsertionInterrupted
	}
	//if err := bc.state.WriteTD(block.Hash(), td); err != nil {
	//	return err
	//}
	return bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
		rawdb.WriteTd(tx, block.Hash(), block.Number64().Uint64(), td)
		if err := rawdb.WriteBlock(tx, block.(*block2.Block)); err != nil {
			return err
		}
		return nil
	})
}

func (bc *BlockChain) WriteBlockWithState(block block2.IBlock, receipts []*block2.Receipt, ibs *state.IntraBlockState, nopay map[types.Address]*uint256.Int) error {
	if !bc.lock.TryLock() {
		return errChainStopped
	}
	defer bc.lock.Unlock()

	_, err := bc.writeBlockWithState(block, receipts, ibs, nopay)
	return err
}

// writeBlockWithState
func (bc *BlockChain) writeBlockWithState(block block2.IBlock, receipts []*block2.Receipt, ibs *state.IntraBlockState, nopay map[types.Address]*uint256.Int) (status WriteStatus, err error) {
	if block.Number64().Uint64() == 20775 {
		fmt.Println("start")
	}
	if err := bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
		//ptd := bc.GetTd(block.ParentHash(), block.Number64().Sub(uint256.NewInt(1)))
		ptd, err := rawdb.ReadTd(tx, block.ParentHash(), uint256.NewInt(0).Sub(block.Number64(), uint256.NewInt(1)).Uint64())
		if nil != err {
			log.Errorf("ReadTd failed err: %v", err)
		}
		if ptd == nil {
			return consensus.ErrUnknownAncestor
		}

		//if err := bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
		externTd := uint256.NewInt(0).Add(ptd, block.Difficulty())
		if err := rawdb.WriteTd(tx, block.Hash(), block.Number64().Uint64(), externTd); nil != err {
			return err
		}
		log.Trace("writeTd:", "number", block.Number64().Uint64(), "hash", block.Hash(), "td", externTd.Uint64())
		if len(receipts) > 0 {
			//if err := bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
			if err := rawdb.AppendReceipts(tx, block.Number64().Uint64(), receipts); nil != err {
				log.Errorf("rawdb.AppendReceipts failed err= %v", err)
				return err
			}
		}
		if err := rawdb.WriteBlock(tx, block.(*block2.Block)); err != nil {
			return err
		}

		stateWriter := state.NewPlainStateWriter(tx, tx, block.Number64().Uint64())
		if err := ibs.CommitBlock(bc.chainConfig.Rules(block.Number64().Uint64()), stateWriter); nil != err {
			return err
		}

		if err := stateWriter.WriteChangeSets(); err != nil {
			return fmt.Errorf("writing changesets for block %d failed: %w", block.Number64().Uint64(), err)
		}

		if err := stateWriter.WriteHistory(); err != nil {
			return fmt.Errorf("writing history for block %d failed: %w", block.Number64().Uint64(), err)
		}

		if nil != nopay {
			for addr, v := range nopay {
				rawdb.PutAccountReward(tx, addr, v)
			}
		}

		return nil
	}); nil != err {
		return NonStatTy, err
	}
	currentBlock := bc.CurrentBlock()
	reorg, err := bc.forker.ReorgNeeded(currentBlock.Header(), block.Header())
	if nil != err {
		return NonStatTy, err
	}
	if reorg {
		// Reorganise the chain if the parent is not the head block
		if block.ParentHash() != currentBlock.Hash() {
			if err := bc.reorg(currentBlock, block); err != nil {
				return NonStatTy, err
			}
		}
		status = CanonStatTy
	} else {
		status = SideStatTy
	}
	// Set new head.
	if status == CanonStatTy {
		if err := bc.writeHeadBlock(nil, block); nil != err {
			log.Errorf("failed to save lates blocks, err: %v", err)
			return NonStatTy, err
		}
	}
	//
	if _, ok := bc.futureBlocks.Get(block.Hash()); ok {
		bc.futureBlocks.Remove(block.Hash())
	}
	return status, nil
}

// writeHeadBlock head
func (bc *BlockChain) writeHeadBlock(tx kv.RwTx, block block2.IBlock) error {
	var err error
	var notExternalTx bool
	if nil == tx {
		tx, err = bc.ChainDB.BeginRw(bc.ctx)
		if nil != err {
			return err
		}
		defer tx.Rollback()
		notExternalTx = true
	}

	//if err := rawdb.WriteBlock(tx, block.(*block2.Block)); nil != err {
	//	log.Errorf("failed to save last block, err: %v", err)
	//	return err
	//}
	rawdb.WriteHeadBlockHash(tx, block.Hash())
	rawdb.WriteTxLookupEntries(tx, block.(*block2.Block))

	if err = rawdb.WriteCanonicalHash(tx, block.Hash(), block.Number64().Uint64()); nil != err {
		return err
	}

	bc.currentBlock.Store(block.(*block2.Block))
	if notExternalTx {
		if err = tx.Commit(); nil != err {
			return err
		}
	}
	return nil
}

// reportBlock logs a bad block error.
func (bc *BlockChain) reportBlock(block block2.IBlock, receipts []*block2.Receipt, err error) {

	var receiptString string
	for i, receipt := range receipts {
		receiptString += fmt.Sprintf("\t %d: cumulative: %v gas: %v contract: %v status: %v tx: %v logs: %v bloom: %x state: %x\n",
			i, receipt.CumulativeGasUsed, receipt.GasUsed, receipt.ContractAddress.String(),
			receipt.Status, receipt.TxHash.String(), "Logs", receipt.Bloom, receipt.PostState)
	}
	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########

Number: %v
Hash: %#x
%v

Error: %v
##############################
`, block.Number64().String(), block.Hash(), receiptString, err))
}

// ReorgNeeded
func (bc *BlockChain) ReorgNeeded(current block2.IBlock, header block2.IBlock) bool {
	switch current.Number64().Cmp(header.Number64()) {
	case 1:
		return false
	case 0:
		return current.Difficulty().Cmp(uint256.NewInt(2)) != 0
	}
	return true
}

// SetHead set new head
func (bc *BlockChain) SetHead(head uint64) error {
	if !bc.lock.TryLock() {
		return errChainStopped
	}
	defer bc.lock.Unlock()

	current := bc.CurrentBlock()
	newHeadBlock, err := bc.GetBlockByNumber(uint256.NewInt(head))
	if err != nil {
		return err
	}
	if current.Number64().Cmp(newHeadBlock.Number64()) == 0 {
		return nil
	}

	if current.Number64().Cmp(newHeadBlock.Number64()) < 0 {
		return errors.New("unbelievable chain")
	}
	// 1. collect txs for delete
	// 2. send remove log event
	// 3. rewind auth reward
	// 4. rewind  state, changeSet
	// 5. delete block, canonical header, receipts, td, transaction
	// 6. send event remove logs, blocks
	// pure cache

	var (
		deletedLogs []*block2.Log
		deleteTxs   []types.Hash
		rmBlocks    []block2.IBlock
	)
	recoverMap := make(map[types.Address]*uint256.Int)
	for point := current; point.Hash() != newHeadBlock.Hash(); point = bc.GetBlock(point.ParentHash(), point.Number64().Uint64()-1) {
		rmBlocks = append(rmBlocks, point)
		for _, tx := range point.Transactions() {
			deleteTxs = append(deleteTxs, tx.Hash())
		}
		if logs := bc.collectLogs(point.(*block2.Block), true); len(logs) > 0 {
			deletedLogs = append(deletedLogs, logs...)
		}
		if bc.chainConfig.IsBeijing(point.Number64().Uint64()) {
			beijing, _ := uint256.FromBig(bc.chainConfig.BeijingBlock)
			if new(uint256.Int).Mod(new(uint256.Int).Sub(point.Number64(), beijing), uint256.NewInt(bc.engineConf.APos.RewardEpoch)).
				Cmp(uint256.NewInt(0)) == 0 {
				last := new(uint256.Int).Sub(point.Number64(), uint256.NewInt(bc.engineConf.APos.RewardEpoch))
				rewardMap, err := bc.RewardsOfEpoch(point.Number64(), last)
				if nil != err {
					return err
				}

				for _, r := range point.Body().Reward() {
					if _, ok := recoverMap[r.Address]; !ok {
						recoverMap[r.Address], err = bc.GetAccountRewardUnpaid(r.Address)
						if nil != err {
							return err
						}
					}

					high := new(uint256.Int).Add(recoverMap[r.Address], r.Amount)
					if _, ok := rewardMap[r.Address]; ok {
						recoverMap[r.Address] = high.Sub(high, rewardMap[r.Address])
					}
				}
			}
		}
	}

	if err := bc.ChainDB.Update(bc.ctx, func(tx kv.RwTx) error {
		if err := state.UnwindState(context.Background(), tx, current.Number64().Uint64(), newHeadBlock.Number64().Uint64()); nil != err {
			return fmt.Errorf("uwind state failed, start: %d, end: %d,  %v", current.Number64().Uint64(), newHeadBlock.Number64().Uint64(), err)
		}
		if err := bc.writeHeadBlock(tx, newHeadBlock); nil != err {
			return err
		}

		for _, t := range deleteTxs {
			if err := rawdb.DeleteTxLookupEntry(tx, t); nil != err {
				return err
			}
		}

		for i := newHeadBlock.Number64().Uint64() + 1; ; i++ {
			hash, _ := rawdb.ReadCanonicalHash(tx, i)
			if hash == (types.Hash{}) {
				break
			}
			rawdb.TruncateCanonicalHash(tx, i, false)
			rawdb.TruncateTd(tx, i)
		}
		for k, v := range recoverMap {
			if err := rawdb.PutAccountReward(tx, k, v); nil != err {
				return err
			}
		}
		return nil
	}); nil != err {
		return err
	}

	bc.blockCache.Purge()
	bc.numberCache.Purge()
	bc.headerCache.Purge()
	bc.futureBlocks.Purge()
	bc.tdCache.Purge()
	bc.receiptCache.Purge()

	for i := len(rmBlocks) - 1; i >= 0; i-- {
		// Also send event for blocks removed from the canon chain.
		event.GlobalEvent.Send(&common.ChainSideEvent{Block: rmBlocks[i].(*block2.Block)})
	}
	if len(deletedLogs) > 0 {
		event.GlobalEvent.Send(&common.RemovedLogsEvent{Logs: deletedLogs})
	}
	return nil
}

// AddFutureBlock checks if the block is within the max allowed window to get
// accepted for future processing, and returns an error if the block is too far
// ahead and was not added.
//
// TODO after the transition, the future block shouldn't be kept. Because
// it's not checked in the Geth side anymore.
func (bc *BlockChain) AddFutureBlock(block block2.IBlock) error {
	max := uint64(time.Now().Unix() + maxTimeFutureBlocks)
	if block.Time() > max {
		return fmt.Errorf("future block timestamp %v > allowed %v", block.Time(), max)
	}
	if block.Difficulty().Uint64() == 0 {
		// Never add PoS blocks into the future queue
		return nil
	}

	log.Info("add future block", "hash", block.Hash(), "number", block.Number64().Uint64(), "stateRoot", block.StateRoot(), "txs", len(block.Body().Transactions()))
	bc.futureBlocks.Add(block.Hash(), block.(*block2.Block))
	return nil
}

// writeKnownBlock updates the head block flag with a known block
// and introduces chain reorg if necessary.
func (bc *BlockChain) writeKnownBlock(block block2.IBlock) error {
	var err error

	current := bc.CurrentBlock()
	if block.ParentHash() != current.Hash() {
		if err := bc.reorg(current, block); err != nil {
			return err
		}
	}
	bc.DB().Update(bc.ctx, func(tx kv.RwTx) error {
		err = bc.writeHeadBlock(tx, block)
		return nil
	})
	return err
}

// reorg takes two blocks, an old chain and a new chain and will reconstruct the
// blocks and inserts them to be part of the new canonical chain and accumulates
// potential missing transactions and post an event about them.
// Note the new head block won't be processed here, callers need to handle it
// externally.
func (bc *BlockChain) reorg(oldBlock, newBlock block2.IBlock) error {
	log.Debug("reorg", "oldBlock", oldBlock.Number64(), "newBlock", newBlock.Number64())
	var (
		newChain    block2.Blocks
		oldChain    block2.Blocks
		commonBlock block2.IBlock

		deletedTxs []types.Hash
		addedTxs   []types.Hash
	)
	// Reduce the longer chain to the same number as the shorter one
	if oldBlock.Number64().Uint64() > newBlock.Number64().Uint64() {
		// Old chain is longer, gather all transactions and logs as deleted ones
		for ; oldBlock != nil && oldBlock.Number64().Uint64() != newBlock.Number64().Uint64(); oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.Number64().Uint64()-1) {
			oldChain = append(oldChain, oldBlock)
			for _, tx := range oldBlock.Transactions() {
				hash := tx.Hash()
				deletedTxs = append(deletedTxs, hash)
			}
		}
	} else {
		// New chain is longer, stash all blocks away for subsequent insertion
		for ; newBlock != nil && newBlock.Number64().Uint64() != oldBlock.Number64().Uint64(); newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.Number64().Uint64()-1) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("invalid new chain")
	}
	log.Debug("reorg to the same height", "height", oldBlock.Number64())
	var err error
	// Both sides of the reorg are at the same number, reduce both until the common
	// ancestor is found
	for {
		// If the common ancestor was found, bail out
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}
		// Remove an old block as well as stash away a new block
		oldChain = append(oldChain, oldBlock)
		for _, t := range oldBlock.Transactions() {
			h := t.Hash()
			deletedTxs = append(deletedTxs, h)
		}
		newChain = append(newChain, newBlock)

		// Step back with both chains
		oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.Number64().Uint64()-1)
		if oldBlock == nil {
			return fmt.Errorf("invalid old chain")
		}
		newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.Number64().Uint64()-1)
		if newBlock == nil {
			return fmt.Errorf("invalid new chain")
		}
	}
	log.Debug("reorg find common ancestor", "height", oldBlock.Number64())
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Info
		msg := "Chain reorg detected"
		if len(oldChain) > 63 {
			msg = "Large chain reorg detected"
			logFn = log.Warn
		}
		logFn(msg, "number", commonBlock.Number64(), "hash", commonBlock.Hash(),
			"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
	} else if len(newChain) > 0 {
		// Special case happens in the post merge stage that current head is
		// the ancestor of new head while these two blocks are not consecutive
		log.Info("Extend chain", "add", len(newChain), "number", newChain[0].Number64(), "hash", newChain[0].Hash())
	} else {
		// len(newChain) == 0 && len(oldChain) > 0
		// rewind the canonical chain to a lower point.
		log.Error("Impossible reorg, please file an issue", "oldnum", oldBlock.Number64(), "oldhash", oldBlock.Hash(), "oldblocks", len(oldChain), "newnum", newBlock.Number64(), "newhash", newBlock.Hash(), "newblocks", len(newChain))
	}
	// Insert the new chain(except the head block(reverse order)),
	// taking care of the proper incremental order.

	// collect oldblock event
	// Deleted logs + blocks:
	var deletedLogs []*block2.Log
	recoverMap := make(map[types.Address]*uint256.Int)
	for i := len(oldChain) - 1; i >= 0; i-- {
		// Collect deleted logs for notification
		if logs := bc.collectLogs(oldChain[i].(*block2.Block), true); len(logs) > 0 {
			deletedLogs = append(deletedLogs, logs...)
		}

		if bc.chainConfig.IsBeijing(oldChain[i].Number64().Uint64()) {
			beijing, _ := uint256.FromBig(bc.chainConfig.BeijingBlock)
			if new(uint256.Int).Mod(new(uint256.Int).Sub(oldChain[i].Number64(), beijing), uint256.NewInt(bc.engineConf.APos.RewardEpoch)).
				Cmp(uint256.NewInt(0)) == 0 {
				last := new(uint256.Int).Sub(oldChain[i].Number64(), uint256.NewInt(bc.engineConf.APos.RewardEpoch))
				rewardMap, err := bc.RewardsOfEpoch(oldChain[i].Number64(), last)
				if nil != err {
					return err
				}

				for _, r := range oldChain[i].Body().Reward() {
					if _, ok := recoverMap[r.Address]; !ok {
						recoverMap[r.Address], err = bc.GetAccountRewardUnpaid(r.Address)
						if nil != err {
							return err
						}
					}

					high := new(uint256.Int).Add(recoverMap[r.Address], r.Amount)
					if _, ok := rewardMap[r.Address]; ok {
						recoverMap[r.Address] = high.Sub(high, rewardMap[r.Address])
					}
				}
			}
		}
	}
	reInsert := make([]block2.IBlock, 0, len(newChain))
	err = bc.DB().Update(bc.ctx, func(tx kv.RwTx) error {
		if err := state.UnwindState(context.Background(), tx, bc.CurrentBlock().Number64().Uint64(), newChain[len(newChain)-1].Number64().Uint64()); nil != err {
			return fmt.Errorf("uwind state failed, start: %d, end: %d,  %v", newChain[len(newChain)-1].Number64().Uint64(), bc.CurrentBlock().Number64().Uint64(), err)
		}
		if err := bc.writeHeadBlock(tx, commonBlock); nil != err {
			return err
		}

		for i := len(newChain) - 1; i >= 0; i-- {
			// Collect the new added transactions.
			for _, t := range newChain[i].Transactions() {
				h := t.Hash()
				addedTxs = append(addedTxs, h)
			}
			reInsert = append(reInsert, newChain[i])
		}

		//return bc.ChainDB.Update(bc.ctx, func(txw kv.RwTx) error {
		// Delete useless indexes right now which includes the non-canonical
		// transaction indexes, canonical chain indexes which above the head.
		for _, t := range types.HashDifference(deletedTxs, addedTxs) {
			rawdb.DeleteTxLookupEntry(tx, t)
		}

		// Delete all hash markers that are not part of the new canonical chain.
		// Because the reorg function does not handle new chain head, all hash
		// markers greater than or equal to new chain head should be deleted.
		number := commonBlock.Number64().Uint64()
		//if len(newChain) > 1 {
		//	number = newChain[1].Number64().Uint64()
		//}
		for i := number + 1; ; i++ {
			hash, _ := rawdb.ReadCanonicalHash(tx, i)
			if hash == (types.Hash{}) {
				break
			}
			rawdb.TruncateCanonicalHash(tx, i, false)
			rawdb.TruncateTd(tx, i)
		}

		// unwind reward
		for k, v := range recoverMap {
			rawdb.PutAccountReward(tx, k, v)
		}
		bc.tdCache.Purge()
		bc.blockCache.Purge()
		bc.headerCache.Purge()
		bc.numberCache.Purge()
		return nil
	})
	if nil != err {
		return err
	}
	for i := len(oldChain) - 1; i >= 0; i-- {
		// Also send event for blocks removed from the canon chain.
		event.GlobalEvent.Send(&common.ChainSideEvent{Block: oldChain[i].(*block2.Block)})
	}
	if len(deletedLogs) > 0 {
		event.GlobalEvent.Send(&common.RemovedLogsEvent{Logs: deletedLogs})
	}

	if _, err := bc.insertChain(reInsert); nil != err {
		return err
	}
	return nil
}
func (bc *BlockChain) Quit() <-chan struct{} {
	return bc.ctx.Done()
}

func (bc *BlockChain) DB() kv.RwDB {
	return bc.ChainDB
}

func (bc *BlockChain) StateAt(tx kv.Tx, blockNr uint64) *state.IntraBlockState {
	reader := state.NewPlainState(tx, blockNr+1)
	return state.New(reader)
}

func (bc *BlockChain) GetDepositInfo(address types.Address) (*uint256.Int, *uint256.Int) {
	var info *deposit.Info
	bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {
		info = deposit.GetDepositInfo(tx, address)
		return nil
	})
	if nil == info {
		return nil, nil
	}
	return info.RewardPerBlock, info.MaxRewardPerEpoch
}

func (bc *BlockChain) GetAccountRewardUnpaid(account types.Address) (*uint256.Int, error) {
	var value *uint256.Int
	var err error
	bc.ChainDB.View(bc.ctx, func(tx kv.Tx) error {
		value, err = rawdb.GetAccountReward(tx, account)
		return nil
	})
	return value, err
}

func (bc *BlockChain) RewardsOfEpoch(number *uint256.Int, lastEpoch *uint256.Int) (map[types.Address]*uint256.Int, error) {
	// total reword for address this epoch
	rewardMap := make(map[types.Address]*uint256.Int, 0)
	depositeMap := map[types.Address]struct {
		reward    *uint256.Int
		maxReward *uint256.Int
	}{}

	currentNr := number.Clone()
	currentNr = currentNr.Sub(currentNr, uint256.NewInt(1))
	endNumber := lastEpoch.Clone()
	for currentNr.Cmp(endNumber) >= 0 {
		block, err := bc.GetBlockByNumber(currentNr)
		if nil != err {
			return nil, err
		}

		verifiers := block.Body().Verifier()
		for _, verifier := range verifiers {
			_, ok := depositeMap[verifier.Address]
			if !ok {
				low, max := bc.GetDepositInfo(verifier.Address)
				if low == nil || max == nil {
					continue
				}
				depositeMap[verifier.Address] = struct {
					reward    *uint256.Int
					maxReward *uint256.Int
				}{reward: low, maxReward: max}

				log.Debug("account deposite infos", "addr", verifier.Address, "perblock", low, "perepoch", max)
			}

			addrReward, ok := rewardMap[verifier.Address]
			if !ok {
				addrReward = uint256.NewInt(0)
			}

			rewardMap[verifier.Address] = math.Min256(addrReward.Add(addrReward, depositeMap[verifier.Address].reward), depositeMap[verifier.Address].maxReward.Clone())
		}

		currentNr.SubUint64(currentNr, 1)
	}
	return rewardMap, nil
}

// collectLogs collects the logs that were generated or removed during
// the processing of a block. These logs are later announced as deleted or reborn.
func (bc *BlockChain) collectLogs(b *block2.Block, removed bool) []*block2.Log {
	var receipts block2.Receipts
	bc.DB().View(context.Background(), func(tx kv.Tx) error {
		receipts = rawdb.ReadRawReceipts(tx, b.Number64().Uint64())
		return nil
	})

	receipts.DeriveFields(bc.chainConfig, b.Hash(), b.Number64().Uint64(), b.Transactions())

	var logs []*block2.Log
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			l := *log
			if removed {
				l.Removed = true
			}
			logs = append(logs, &l)
		}
	}
	return logs
}
