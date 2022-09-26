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

package blockchain

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/api/protocol/msg_proto"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/statedb"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrKnownBlock           = errors.New("block already known")
	ErrUnknownAncestor      = errors.New("unknown ancestor")
	ErrPrunedAncestor       = errors.New("pruned ancestor")
	ErrFutureBlock          = errors.New("block in the future")
	ErrInvalidNumber        = errors.New("invalid block number")
	ErrInvalidTerminalBlock = errors.New("invalid terminal block")
)

type WriteStatus byte

const (
	NonStatTy   WriteStatus = iota //
	CanonStatTy                    //
	SideStatTy                     //
)

const (
	//maxTimeFutureBlocks
	maxTimeFutureBlocks = 10
)

type BlockChain struct {
	ctx          context.Context
	cancel       context.CancelFunc
	genesisBlock block2.IBlock
	blocks       []block2.IBlock
	headers      []block2.IHeader
	currentBlock block2.IBlock
	//state        *statedb.StateDB
	chainDB  db.IDatabase
	changeDB kv.RwDB
	engine   consensus.Engine

	insertLock    chan struct{}
	latestBlockCh chan block2.IBlock
	lock          sync.Mutex

	peers map[peer.ID]bool

	downloader common.IDownloader

	chBlocks chan block2.IBlock

	pubsub common.IPubSub

	errorCh chan error

	process *avm.VMProcessor

	wg sync.WaitGroup //

	procInterrupt int32                        // insert chain
	tdCache       map[types.Hash]types.Int256  // td cache
	futureBlocks  map[types.Hash]block2.IBlock //
	//receiptCache  map[types.Hash][]*block2.Receipt //todo  lru？
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

func NewBlockChain(ctx context.Context, genesisBlock block2.IBlock, engine consensus.Engine, downloader common.IDownloader, database db.IDatabase, changeDB kv.RwDB, pubsub common.IPubSub) (common.IBlockChain, error) {
	c, cancel := context.WithCancel(ctx)
	current, _ := rawdb.GetLatestBlock(database)
	if current == nil {
		current = genesisBlock
	}

	log.Debugf("block chain current %d", current.Number64().Uint64())
	bc := &BlockChain{
		genesisBlock:  genesisBlock,
		blocks:        []block2.IBlock{},
		currentBlock:  current,
		chainDB:       database,
		changeDB:      changeDB,
		ctx:           c,
		cancel:        cancel,
		insertLock:    make(chan struct{}, 1),
		peers:         make(map[peer.ID]bool),
		chBlocks:      make(chan block2.IBlock, 100),
		errorCh:       make(chan error),
		pubsub:        pubsub,
		downloader:    downloader,
		latestBlockCh: make(chan block2.IBlock, 50),
		engine:        engine,
		tdCache:       make(map[types.Hash]types.Int256),
		futureBlocks:  make(map[types.Hash]block2.IBlock),
		//receiptCache:  make(map[types.Hash][]*block2.Receipt),
	}

	bc.process = avm.NewVMProcessor(ctx, bc, engine)

	return bc, nil
}

func (bc *BlockChain) StateAt(root types.Hash) common.IStateDB {
	return statedb.NewStateDB(root, bc.chainDB, bc.changeDB)
}

func (bc *BlockChain) CurrentBlock() block2.IBlock {
	return bc.currentBlock
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
	if bc.pubsub == nil {
		return ErrInvalidPubSub
	}

	bc.wg.Add(3)
	go bc.runLoop()
	go bc.newBlockLoop()
	go bc.updateFutureBlocksLoop()

	return nil
}

func (bc *BlockChain) AddPeer(hash string, remoteBlock uint64, peerID peer.ID) error {
	if bc.genesisBlock.Hash().String() != hash {
		return fmt.Errorf("failed to addPeer, err: genesis block different")
	}
	if _, ok := bc.peers[peerID]; ok {
		return fmt.Errorf("failed to addPeer, err: the peer already exists")
	}

	log.Debugf("local heigth:%d --> remote height: %d", bc.currentBlock.Number64(), remoteBlock)

	bc.peers[peerID] = true
	//if remoteBlock > bc.currentBlock.Number64().Uint64() {
	//	bc.syncChain(remoteBlock, peerID)
	//}

	return nil
}

func (bc *BlockChain) GetReceipts(blockHash types.Hash) (block2.Receipts, error) {
	return rawdb.GetReceipts(bc.chainDB, blockHash)
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

func (bc *BlockChain) InsertBlock(blocks []block2.IBlock, isSync bool) (int, error) {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	runBlock := func(b block2.IBlock) error {
		//todo copy?
		stateDB := statedb.NewStateDB(b.ParentHash(), bc.chainDB, bc.changeDB)

		receipts, logs, err := bc.process.Processor(b, stateDB)
		if err != nil {
			return err
		}

		stateDB.Commit(b.Number64())
		rawdb.StoreReceipts(bc.chainDB, b.Hash(), receipts)

		if len(logs) > 0 {
			event.GlobalEvent.Send(&common.NewLogsEvent{Logs: logs})
		}
		if len(receipts) > 0 {
			log.Infof("Receipt len(%d), receipts: [%v]", len(receipts), receipts)
		}
		return nil
	}

	current := bc.CurrentBlock()
	var insertBlocks []block2.IBlock
	for i, block := range blocks {
		if block.Number64() == current.Number64() && block.Difficulty().Compare(current.Difficulty()) == 1 {
			if err := bc.engine.VerifyHeader(bc, block.Header(), false); err != nil {
				log.Errorf("failed verify block err: %v", err)
				continue
			}
			if err := runBlock(block); err != nil {
				log.Errorf("failed runblock, err:%v", err)
			} else {
				insertBlocks = append(insertBlocks, block)
				current = blocks[i]
			}

		} else if block.Number64().Equal(current.Number64().Add(types.NewInt64(1))) && block.ParentHash().String() == current.Hash().String() {
			if err := bc.engine.VerifyHeader(bc, block.Header(), false); err != nil {
				log.Errorf("failed verify block err: %v", err)
				continue
			}

			if err := runBlock(block); err != nil {
				log.Errorf("failed runblock, err:%v", err)
			} else {
				insertBlocks = append(insertBlocks, block)
				current = blocks[i]
			}
		} else {
			author, _ := bc.engine.Author(block.Header())
			log.Errorf("failed instert mew block, hash: %s, number: %s, diff: %s, miner: %s, txs: %d", block.Hash(), block.Number64().String(), block.Difficulty().String(), author.String(), len(block.Transactions()))
			log.Errorf("failed instert cur block, hash: %s, number: %s, diff: %s, miner: %s, txs: %d", current.Hash(), current.Number64().String(), current.Difficulty().String(), author.String(), len(current.Transactions()))
		}

	}

	if len(insertBlocks) > 0 {
		if _, err := rawdb.SaveBlocks(bc.chainDB, insertBlocks); err != nil {
			log.Errorf("failed to save blocks, err: %v", err)
			return 0, err
		}

		if err := rawdb.SaveLatestBlock(bc.chainDB, current); err != nil {
			log.Errorf("failed to save lates blocks, err: %v", err)
			return 0, err
		}
		bc.currentBlock = current

		if !isSync {
			i := event.GlobalEvent.Send(&current)
			author, _ := bc.engine.Author(current.Header())
			log.Debugf("current number:%d, miner: %s, feed send count: %d", current.Number64().Uint64(), author, i)
		}

		return len(insertBlocks), nil
	}

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
	if bc.pubsub == nil {
		bc.errorCh <- ErrInvalidPubSub
		return
	}

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
				bc.errorCh <- err
				return
			}

			var newBlock types_pb.PBlock
			if err := proto.Unmarshal(msg.Data, &newBlock); err == nil {
				var block block2.Block
				if err := block.FromProtoMessage(&newBlock); err == nil {
					var inserted bool
					log.Infof("Subscribe new block hash: %s", block.Hash())
					if _, err := bc.InsertBlock([]block2.IBlock{&block}, false); err != nil {
						inserted = false
						log.Errorf("failed to inster new block in blockchain, err:%v", err)
					} else {
						inserted = true
						log.Debugf("successfully inserted a new block number is %d", block.Number64().Uint64())
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
			//
			blocks := make([]block2.IBlock, 0, len(bc.futureBlocks))
			for _, block := range bc.futureBlocks {
				blocks = append(blocks, block)
			}
			sort.Slice(blocks, func(i, j int) bool {
				return blocks[i].Number64().Compare(blocks[j].Number64()) < 0
			})
			for i := range blocks {
				bc.InsertChain(blocks[i : i+1])
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
				rawdb.SaveBlocks(bc.chainDB, []block2.IBlock{block})
				rawdb.SaveLatestBlock(bc.chainDB, block)
				if block, err := rawdb.GetLatestBlock(bc.chainDB); err == nil {
					log.Debugf("latest block: %v", block.Header())
				}
			}
		case msg, ok := <-newBlockCh:
			if ok {
				block := block2.Block{}
				if err := block.FromProtoMessage(msg.Block); err == nil {
					rawdb.SaveBlocks(bc.chainDB, []block2.IBlock{&block})
					rawdb.SaveLatestBlock(bc.chainDB, &block)
					if block, err := rawdb.GetLatestBlock(bc.chainDB); err == nil {
						log.Debugf("latest block: %v", block.Header())
					}
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

func (bc *BlockChain) GetHeader(h types.Hash, number types.Int256) block2.IHeader {
	header, err := bc.GetHeaderByHash(h)
	if err != nil {
		return nil
	}
	return header
}

func (bc *BlockChain) GetHeaderByNumber(number types.Int256) (block2.IHeader, error) {
	header, _, err := rawdb.GetHeader(bc.chainDB, number)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func (bc *BlockChain) GetHeaderByHash(h types.Hash) (block2.IHeader, error) {
	header, _, err := rawdb.GetHeaderByHash(bc.chainDB, h)
	return header, err
}
func (bc *BlockChain) GetBlockByHash(h types.Hash) (block2.IBlock, error) {
	number, err := rawdb.GetHashNumber(bc.chainDB, h)
	if err != nil {
		return nil, err
	}
	return bc.GetBlockByNumber(number)
}

func (bc *BlockChain) GetBlockByNumber(number types.Int256) (block2.IBlock, error) {

	header, err := bc.GetHeaderByNumber(number)
	if err != nil {
		return nil, err
	}
	pBody, err := rawdb.GetBody(bc.chainDB, number)
	if err != nil {
		return nil, err
	}

	//var txs []*transaction.Transaction
	//for _, v := range pBody.Txs {
	//	tx, err := transaction.FromProtoMessage(v)
	//	if err == nil {
	//		txs = append(txs, tx)
	//	}
	//}

	block := block2.NewBlock(header, pBody.Transactions())
	return block, err
}

func (bc *BlockChain) NewBlockHandler(payload []byte, peer peer.ID) error {

	var nweBlock msg_proto.NewBlockMessageData
	if err := proto.Unmarshal(payload, &nweBlock); err != nil {
		log.Errorf("failed unmarshal to msg, from peer:%s", peer)
		return err
	} else {
		//log.Debugf("receive new block msg %v", nweBlock.GetBlock())
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
	h := hash
	for i := 0; i < n; i++ {
		block := bc.GetBlock(h)
		if block == nil {
			break
		}

		blocks = append(blocks, block)
		h = block.ParentHash()
	}
	return blocks
}

func (bc *BlockChain) GetBlock(hash types.Hash) block2.IBlock {
	if hash == (types.Hash{}) {
		return nil
	}
	header, h, err := rawdb.GetHeaderByHash(bc.chainDB, hash)
	if err != nil {
		log.Errorf("GetBlock failed to get block, hash: %s, err:%v", hash.String(), err)
		return nil
	}

	if hash.String() != h.String() {
		log.Errorf("GetBlock failed to get block, the hash is differ")
		return nil
	}

	body, err := rawdb.GetBody(bc.chainDB, header.(*block2.Header).Number)
	if err != nil {
		log.Errorf("GetBody failed to get block body, err:%v", err)
		return nil
	}

	return block2.NewBlock(header, body.Transactions())
}

func (bc *BlockChain) SealedBlock(b block2.IBlock) {
	//_, err := bc.InsertBlock([]block2.IBlock{b}, true)
	//if err != nil {
	//	return
	//}

	pbBlock := b.ToProtoMessage()
	log.Infof("sealed block hash: %s, number: %s, used gas: %d", b.Hash(), b.Number64().Uint64(), b.GasUsed())

	_ = bc.pubsub.Publish(message.GossipBlockMessage, pbBlock)
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
func (bc *BlockChain) HasBlockAndState(hash types.Hash) bool {
	block := bc.GetBlock(hash)
	if block == nil {
		return false
	}
	return bc.HasState(block.Hash())
}

// HasState
func (bc *BlockChain) HasState(hash types.Hash) bool {
	return false
}

func (bc *BlockChain) HasBlock(hash types.Hash) bool {
	return true
}

// GetTd
func (bc *BlockChain) GetTd(hash types.Hash) types.Int256 {
	if td, ok := bc.tdCache[hash]; ok {
		return td
	}
	bc.tdCache[hash] = rawdb.ReadTd(bc.chainDB, hash)
	return bc.tdCache[hash]
}

// InsertChain
func (bc *BlockChain) InsertChain(chain []block2.IBlock) (int, error) {
	if len(chain) == 0 {
		return 0, nil
	}
	//
	for i := 1; i < len(chain); i++ {
		block, prev := chain[i], chain[i-1]
		if !block.Number64().Equal(prev.Number64().Add(types.NewInt64(1))) || block.ParentHash() != prev.Hash() {
			log.Error("Non contiguous block insert",
				"number", block.Number64().String(),
				"hash", block.Hash(),
				"parent", block.ParentHash(),
				"prev number", prev.Number64(),
				"prev hash", prev.Hash(),
			)
			return 0, fmt.Errorf("non contiguous insert: item %s is #%d [%x..], item %d is #%d [%x..] (parent [%x..])", i-1, prev.Number64().String(),
				prev.Hash().Bytes()[:4], i, block.Number64().String(), block.Hash().Bytes()[:4], block.ParentHash().Bytes()[:4])
		}
	}
	bc.lock.Lock()
	defer bc.lock.Unlock()
	return bc.insertChain(chain)
}

func (bc *BlockChain) insertChain(chain []block2.IBlock) (int, error) {

	var (
		stats     = insertStats{startTime: time.Now()}
		lastCanon block2.IBlock
	)

	defer func() {
		if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
			// todo
			//event.GlobalEvent.Send(&common.ChainHighestBlock{Block: lastCanon, Inserted: true})
		}
	}()

	for i, block := range chain {
		// 1.
		if bc.insertStopped() {
			return 0, nil
		}
		// 2.
		err := bc.engine.VerifyHeader(bc, block.Header(), false)

		// 3.
		if err == nil {
			err = bc.ValidateBody(block)
		}

		switch {
		case err == ErrKnownBlock:
			if bc.CurrentBlock().Number64().Compare(block.Number64()) >= 0 {
				stats.ignored++
				continue
			}
		case err == ErrFutureBlock:
			max := uint64(time.Now().Unix()) + maxTimeFutureBlocks
			if block.Time() > max {
				return i, fmt.Errorf("future block: %v > %v", block.Time(), max)
			}
			bc.futureBlocks[block.Hash()] = block
			stats.queued++
			continue
		case err == ErrUnknownAncestor:
			if _, ok := bc.futureBlocks[block.ParentHash()]; ok {
				bc.futureBlocks[block.Hash()] = block
				stats.queued++
				continue
			} else {

			}
		case err == ErrPrunedAncestor:
			reorg := bc.ReorgNeeded(bc.CurrentBlock(), block)
			if !reorg {
				if err = bc.WriteBlockWithoutState(block); err != nil {
					return i, err
				}
				continue
			}
			var winner []block2.IBlock
			parent := bc.GetBlock(block.ParentHash())
			for !bc.HasState(parent.Hash()) {
				winner = append(winner, parent)
				parent = bc.GetBlock(parent.ParentHash())
			}
			// reverse
			for j := 0; j < len(winner)/2; j++ {
				winner[j], winner[len(winner)-1-j] = winner[len(winner)-1-j], winner[j]
			}
			return bc.insertChain(winner)
		case err != nil:
			bc.reportBlock(block, nil, err)
			return i, err
		}

		//init stateDB
		//todo:
		stateDB := statedb.NewStateDB(block.ParentHash(), bc.chainDB, bc.changeDB)

		// todo usedGas?
		usedGas := 1000
		receipts, _, err := bc.process.Processor(block, stateDB)

		if err != nil {
			bc.reportBlock(block, receipts, err)
			return i, err
		}

		// verify state
		err = bc.ValidateState(block, receipts, uint64(usedGas))
		if err != nil {
			bc.reportBlock(block, receipts, err)
			return i, err
		}

		state, err := bc.writeBlockWithState(block, receipts)
		if err != nil {
			return i, err
		}

		switch state {
		case CanonStatTy:
			lastCanon = block
			log.Debug("Inserted new block number %s hash %s diff %s", block.Number64().String(), block.Hash().String(), block.Difficulty().String())
		case SideStatTy:
			log.Debug("Inserted forked block number %s hash %s diff %s", block.Number64().String(), block.Hash().String(), block.Difficulty().String())
		}
	}

	return 0, nil
}

// insertSideChain
func (bc *BlockChain) insertSideChain(chain []block2.IBlock) (int, error) {
	return 0, nil
}

// recoverAncestors
func (bc *BlockChain) recoverAncestors(block block2.IBlock) (types.Hash, error) {
	var (
		hashes  []types.Hash
		numbers []types.Int256
		parent  = block
	)
	for parent != nil && !bc.HasState(parent.Hash()) {
		hashes = append(hashes, parent.Hash())
		numbers = append(numbers, parent.Number64())
		parent = bc.GetBlock(parent.ParentHash())

	}
	if parent == nil {
		return types.Hash{}, errors.New("missing parent")
	}
	for i := len(hashes) - 1; i >= 0; i-- {

		var b block2.IBlock
		if i == 0 {
			b = block
		} else {
			b = bc.GetBlock(hashes[i])
		}
		if _, err := bc.insertChain([]block2.IBlock{b}); err != nil {
			return b.ParentHash(), err
		}
	}
	return block.Hash(), nil
}

// ValidateBody verify body
func (bc *BlockChain) ValidateBody(block block2.IBlock) error {
	if bc.HasBlockAndState(block.Hash()) {
		return ErrKnownBlock
	}

	// todo verify Transactions root

	if !bc.HasBlockAndState(block.ParentHash()) {
		if !bc.HasBlock(block.ParentHash()) {
			return ErrUnknownAncestor
		}
		return ErrPrunedAncestor
	}
	return nil
}

// ValidateState verify state
func (v *BlockChain) ValidateState(block block2.IBlock, receipts []*block2.Receipt, usedGas uint64) error {
	return nil
}

// WriteBlockWithoutState without state
func (bc *BlockChain) WriteBlockWithoutState(block block2.IBlock) (err error) {

	//if err := bc.state.WriteTD(block.Hash(), td); err != nil {
	//	return err
	//}
	if _, err := rawdb.SaveBlocks(bc.chainDB, []block2.IBlock{block}); err != nil {
		return err
	}
	return nil
}

// WriteBlockWithState
func (bc *BlockChain) writeBlockWithState(block block2.IBlock, receipts []*block2.Receipt) (status WriteStatus, err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	reorg := bc.ReorgNeeded(bc.currentBlock, block)
	if reorg {
		// 1.
		status = CanonStatTy
	} else {
		status = SideStatTy
	}

	if _, err := rawdb.SaveBlocks(bc.chainDB, []block2.IBlock{block}); err != nil {
		return NonStatTy, err
	}

	// Set new head.
	if status == CanonStatTy {
		if err := rawdb.SaveLatestBlock(bc.chainDB, block); err != nil {
			log.Errorf("failed to save lates blocks, err: %v", err)
			return NonStatTy, err
		}
	}

	//
	if _, ok := bc.futureBlocks[block.Hash()]; ok {
		delete(bc.futureBlocks, block.Hash())
	}

	return status, nil
}

// writeHeadBlock head
func (bc *BlockChain) writeHeadBlock(block block2.IBlock) error {
	if err := rawdb.SaveLatestBlock(bc.chainDB, block); err != nil {
		log.Errorf("failed to save last block, err: %v", err)
		return err
	}
	bc.currentBlock = block
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
	switch current.Number64().Compare(header.Number64()) {
	case 1:
		return false
	case 0:
		return current.Difficulty().Compare(types.NewInt64(2)) != 0
	}
	return true
}

// SetHead set new head
func (bc *BlockChain) SetHead(head uint64) error {
	// todo state rawdb 。。。。。
	newHeadBlock, err := bc.GetBlockByNumber(types.NewInt64(head))
	if err != nil {
		return nil
	}
	err = rawdb.SaveLatestBlock(bc.chainDB, newHeadBlock)
	return err
}
