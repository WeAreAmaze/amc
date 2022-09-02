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

package poa

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/consensus"
	log "github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/utils"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/crypto"
	rand2 "math/rand"
	"sort"
	"sync"
	"time"
)

const (
	name       = "MPoaEngine"
	epoch      = uint64(30000)
	wiggleTime = 500 * time.Millisecond
)

var (
	diffInTurn = types.NewInt64(2) // Block difficulty for in-turn signatures
	diffNoTurn = types.NewInt64(1) // Block difficulty for out-of-turn signatures
)

type SignerFn func(signer types.Address, mimeType string, message []byte) ([]byte, error)

type ILatestBlock interface {
	LatestBlockCh() (block.IBlock, error)
}

type MPoa struct {
	config *conf.ConsensusConfig
	db     db.IDatabase

	miner crypto.PrivKey

	recentBlocks *lru.ARCCache
	signatures   *lru.ARCCache

	proposals     map[types.Address]bool
	singers       map[types.Address]crypto.PubKey
	recentSigners map[types.Int256]types.Address
	txsPool       txs_pool.ITxsPool
	signer        types.Address
	signFn        SignerFn
	lock          sync.RWMutex

	chain common.IBlockChain

	ctx    context.Context
	cancel context.CancelFunc

	exitCh chan struct{}
	stopCh chan struct{}
	pubsub common.IPubSub
}

func (M *MPoa) GetMiner(iBlock block.IBlock) types.Address {
	//return M.getMiner(iBlock.Number64().Add(types.NewInt64(1)))
	if M.checkMiner(iBlock.Number64().Add(types.NewInt64(1)), M.signer) {
		return M.signer
	}

	return types.Address{}
}

func (M *MPoa) TxPool() txs_pool.ITxsPool {
	return M.txsPool
}

func (M *MPoa) Author(header *block.Header) types.Address {
	return header.Coinbase
}

func (M *MPoa) VerifyHeader(header block.IHeader) error {
	log.Debugf("verify header hash:%s", header.Hash())
	if h, ok := header.(*block.Header); ok {
		var minerPublic crypto.PubKey
		_, err := M.chain.GetHeaderByHash(h.ParentHash)
		if err != nil {
			return fmt.Errorf("get Header by hash err: %v", err)
		}

		if h.Time > uint64(time.Now().Unix()) {
			return errors.New(fmt.Sprintf("block in the future, block number: %d, block time: %d, current time: %d", h.Number.Uint64(), h.Time, time.Now().Unix()))
		}

		if minerPublic, ok = M.singers[h.Coinbase]; !ok {
			return fmt.Errorf("invalid coinbase: %s", h.Coinbase)
		}

		for _, recent := range M.recentSigners {
			if recent == h.Coinbase {
				return errors.New("recently signed")
			}
		}

		if !h.Difficulty.Equal(diffNoTurn) && !h.Difficulty.Equal(diffInTurn) {
			return errors.New(fmt.Sprintf("invalid block difficulty, %s", h.Difficulty.String()))
		}

		//if !M.checkMiner(h.Number, h.Coinbase) {
		//	minerAddr := M.getMiner(h.Number)
		//	return errors.New(fmt.Sprintf("failed to check miner, got: %s, want: %s ", h.Coinbase, minerAddr))
		//}

		headerHash := h.Hash()
		ok, err := minerPublic.Verify(headerHash[:], h.Extra)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("invalid sign, hash: %s", headerHash)
		}

		return nil
	}
	return fmt.Errorf("invalid block message")
}

func (M *MPoa) VerifyBlock(block block.IBlock) error {
	if err := M.VerifyHeader(block.Header()); err != nil {
		return err
	}

	M.updateContinuity(block.Coinbase(), block.Number64())

	//TODO verify Hash root
	return nil
}

func (M *MPoa) GenerateBlock(parentBlock block.IBlock, txs []*transaction.Transaction) (block.IBlock, error) {
	//TODO implement me
	panic("implement me")
}

func (M *MPoa) EngineName() string {
	return name
}

func (M *MPoa) Start() error {
	minerAddr := types.PrivateToAddress(M.miner)
	if _, ok := M.singers[minerAddr]; !ok {
		log.Warnf("the current address cannot generate blocks %s, %v", minerAddr.String(), M.singers)
		return nil
	}

	go M.runLoop()
	go M.workLoop()

	return nil
}

func (M *MPoa) runLoop() {
	defer M.cancel()

	stopCh := make(chan common.DownloaderStartEvent)
	defer close(stopCh)
	stopSub := event.GlobalEvent.Subscribe(stopCh)
	defer stopSub.Unsubscribe()

	startCh := make(chan common.DownloaderFinishEvent)
	defer close(stopCh)
	startSub := event.GlobalEvent.Subscribe(startCh)
	defer startSub.Unsubscribe()

	for {
		select {
		case <-M.ctx.Done():
			return
		case <-stopSub.Err():
			return
		case <-startSub.Err():
			return
		case <-stopCh:
			M.stopCh <- struct{}{}
		case <-startCh:
			M.stopCh <- struct{}{}
		}
	}
}

func (M *MPoa) Prepare(header *block.Header) error {
	if M.checkMiner(header.Number, M.signer) {
		header.Difficulty = diffInTurn
	} else {
		header.Difficulty = diffNoTurn
	}

	parent, err := M.chain.GetHeaderByNumber(header.Number.Sub(types.NewInt64(1)))
	if err != nil {
		return err
	}

	header.Time = parent.(*block.Header).Time + M.config.Period

	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}

	return nil
}

func (M *MPoa) workLoop() {
	defer M.cancel()
	defer func() {
		if err := M.saveRecent(); err != nil {
			log.Errorf("failed to save recent signers, err: %v", err)
		}
	}()
	minerAddr := types.PrivateToAddress(M.miner)
	log.Infof("mpoa running...")

	newBlockCh := make(chan block.IBlock, 50)
	defer close(newBlockCh)

	newBlockSub := event.GlobalEvent.Subscribe(newBlockCh)
	defer newBlockSub.Unsubscribe()
	//time.Sleep(15 * time.Second)
loop:
	for {
		log.Infof("start mpoa")
		currentBlock := M.chain.CurrentBlock()
		currentHeader := currentBlock.Header().(*block.Header)

		select {
		case <-M.ctx.Done():
			return
		case <-newBlockSub.Err():
			return
		case _, ok := <-M.stopCh:
			if ok {
				<-M.stopCh
				goto loop
			} else {
				return
			}
		case <-time.After(10 * time.Second):
			newBlock := M.chain.CurrentBlock()

			currentBlock = newBlock
			currentHeader = newBlock.Header().(*block.Header)
		case newBlock, ok := <-newBlockCh:
			if ok {
				if newBlock.Coinbase() == minerAddr {
					goto loop
				}

				currentBlock = newBlock
				currentHeader = newBlock.Header().(*block.Header)
			} else {
				return
			}
		}

		// check block
		if ok := M.checkContinuity(minerAddr, currentHeader); ok {
			log.Infof("continuous block output node")
			goto loop
		}

		var (
			errCh       = make(chan error)
			sealBlockCh = make(chan block.IBlock)
		)

		go func() {
			err := M.seal(currentBlock, minerAddr, sealBlockCh, M.stopCh)
			errCh <- err
		}()

		select {
		case <-M.ctx.Done():
			return
		case err, ok := <-errCh:
			if ok {
				log.Errorf("failed to seal, err: %v", err)
			}
		case b, ok := <-sealBlockCh:
			if ok {
				log.Infof("seal new block %d", b.Number64().Uint64())
				if currentBlock.Number64().Uint64() == 0 {
					currentBlock = b
					currentHeader = currentBlock.Header().(*block.Header)
				}
				pbBlock := b.ToProtoMessage()
				//buf, err := proto.Marshal(pbBlock)
				//if err == nil {
				//	sBuf := hex.EncodeToString(buf)
				//	M.log.Infof("new block proto buf: %s", sBuf)
				//}
				if err := M.pubsub.Publish(message.GossipBlockMessage, pbBlock); err != nil {
					log.Errorf("failed to gossip block(number:%d), err:%v", b.Number64().Uint64(), err)
				}
				//_ = M.chain.InsertBlock([]block.IBlock{b})

			}
		}
		close(sealBlockCh)

	}

}

func (M *MPoa) checkContinuity(minerAddr types.Address, header *block.Header) bool {
	number := header.Number
	limit := types.NewInt64(uint64(len(M.singers)/2 + 1))

	//
	M.lock.RLock()
	defer M.lock.RUnlock()
	for seen, addr := range M.recentSigners {
		if addr == minerAddr {
			if number.Slt(limit) || number.Sub(limit).Slt(seen) {
				log.Warnf("signer recently")
				return true
			}
			return false
		}
	}

	return false
}

func (M *MPoa) updateContinuity(minerAddr types.Address, number types.Int256) {
	M.lock.Lock()
	defer M.lock.Unlock()
	limit := types.NewInt64(uint64(len(M.singers)/2 + 1))
	if limit.Slt(number) || limit.Equal(number) {
		delete(M.recentSigners, number.Sub(limit))
	}

	M.recentSigners[number] = minerAddr
}

func (M *MPoa) checkMiner(number types.Int256, singer types.Address) bool {
	sigs := make([]types.Address, 0, len(M.singers))
	for sig := range M.singers {
		sigs = append(sigs, sig)
	}

	sort.Sort(types.Addresses(sigs))
	var offset = 0

	for offset < len(sigs) && sigs[offset] != singer {
		offset++
	}

	//log.Debugf("check miner singer count: %d, offset: %d", len(sigs), offset)

	z := number.Mod(types.NewInt64(uint64(len(sigs))))
	other := types.NewInt64(uint64(offset))
	return z.Equal(other)
}

func (M *MPoa) getMiner(number types.Int256) types.Address {
	sigs := make([]types.Address, 0, len(M.singers))
	for sig := range M.singers {
		sigs = append(sigs, sig)
	}

	sort.Sort(types.Addresses(sigs))

	z := number.Mod(types.NewInt64(uint64(len(sigs))))
	return sigs[z.Uint64()]
}

// todo only to test
func (M *MPoa) getHash() types.Hash {
	_, pub, _ := crypto.GenerateEd25519Key(rand.Reader)
	bPub, _ := crypto.MarshalPublicKey(pub)
	return types.BytesToHash(bPub)
}

func (M *MPoa) Seal(b block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error {
	header := b.Header().(*block.Header)
	if header.Number.Equal(types.NewInt64(0)) {
		return errors.New("unknown block")
	}

	minerAddr := types.PrivateToAddress(M.miner)
	if _, ok := M.singers[minerAddr]; !ok {
		log.Warnf("the current address cannot generate blocks %s, %v", minerAddr.String(), M.singers)
		return nil
	}

	if M.checkContinuity(minerAddr, header) {
		return errors.New("signed recently, must wait for others")
	}

	//if M.getMiner(b.Number64()) != minerAddr {
	//	return errors.New(fmt.Sprintf("not the signer of the current height[%v]", b.Number64().String()))
	//}

	// todo  only sign hash？
	headerHash := header.Hash()
	log.Debugf("newBlock hash: %s", headerHash)
	s, err := M.miner.Sign(headerHash[:])
	if err != nil {
		return err
	}

	header.Extra = s

reTimer:
	delay := time.Unix(int64(header.Time), 0).Sub(time.Now())
	//
	if header.Difficulty.Equal(diffNoTurn) {
		wiggle := time.Duration(len(M.singers)/2+1) * wiggleTime
		delay += time.Duration(rand2.Int63n(int64(wiggle)))
	}

	select {
	case <-M.ctx.Done():
		M.exitCh <- struct{}{}
		return fmt.Errorf("mpa quit")
	case <-M.stopCh:
		return fmt.Errorf("take the initiative to stop")
	case <-time.After(delay):
		//todo  will remove
		log.Debugf("time after delay %d", delay)
		if header.Time > uint64(time.Now().Unix()) {
			goto reTimer
		}
	}

	//newBlock := block.NewBlock(header, b.Body().Transactions())

	results <- b
	log.Debugf("newblock hash: %s, number: %d, diff: %v, phash: %s", b.Hash(), b.Number64().Uint64(), b.Difficulty().String(), b.ParentHash())

	return nil
}

func (M *MPoa) seal(currentBlock block.IBlock, minerAddr types.Address, results chan<- block.IBlock, stop <-chan struct{}) error {
	pHeader := currentBlock.Header().(*block.Header)
	header := block.Header{
		ParentHash:  currentBlock.Hash(),
		Coinbase:    minerAddr,
		Root:        M.getHash(),
		TxHash:      M.getHash(), //types.Hash{},
		ReceiptHash: M.getHash(), //types.Hash{},
		Difficulty:  diffInTurn,
		Number:      currentBlock.Number64().Add(types.NewInt64(1)),
		GasLimit:    0,
		GasUsed:     0,
		Time:        pHeader.Time + M.config.Period,
		MixDigest:   types.Hash{},
		Nonce:       block.BlockNonce{},
		BaseFee:     types.Int256{},
		Extra:       []byte{},
	}

	log.Infof("current number: %d, new number: %d", currentBlock.Number64().Uint64(), header.Number.Uint64())

	txs, err := M.txsPool.GetTransaction()
	if err != nil {
		return err
	}

	// checkMiner
	nowTime := time.Now()

	delay := time.Unix(int64(header.Time), 0).Sub(nowTime)

	if !M.checkMiner(header.Number, minerAddr) {
		wiggle := time.Duration(len(M.singers)/2+1) * wiggleTime
		delay += time.Duration(rand2.Int63n(int64(wiggle)))
		header.Difficulty = diffNoTurn
	}

	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}

	log.Infof("delay: %d", delay)

	// todo sign header
	headerHash := header.Hash()
	s, err := M.miner.Sign(headerHash[:])
	if err != nil {
		return err
	}

	header.Extra = s

	select {
	case <-M.ctx.Done():
		M.exitCh <- struct{}{}
		return fmt.Errorf("mpa quit")
	case <-M.stopCh:
		//<-M.stopCh
		return fmt.Errorf("take the initiative to stop")
	case <-time.After(delay):
		log.Infof("time after delay %d", delay)
	}

	newBlock := block.NewBlock(&header, txs)
	results <- newBlock
	log.Infof("newblock %v", newBlock)

	return nil
}

func (M *MPoa) Stop() error {
	//TODO implement me
	panic("implement me")
}

func NewMPoa(ctx context.Context, config *conf.ConsensusConfig, db db.IDatabase, chain common.IBlockChain, txsPool txs_pool.ITxsPool, pubsub common.IPubSub) (consensus.IEngine, error) {
	if config.MPoa.Epoch == 0 {
		config.MPoa.Epoch = epoch
	}

	resents, _ := lru.NewARC(config.MPoa.InmemorySnapshots)
	signatures, _ := lru.NewARC(config.MPoa.InmemorySignatures)

	signers, err := rawdb.Signers(db)

	if err != nil {
		log.Errorf("failed get signers, err: %v", err)
		return nil, err
	}

	var (
		minerKey crypto.PrivKey
	)

	if len(config.MinerKey) > 0 {
		minerKey, err = utils.StringToPrivate(config.MinerKey)
		if err != nil {
			log.Errorf("failed resolver miner key, err: %v", err)
			return nil, err
		}
	} else {
		minerKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			log.Errorf("failed generate private, err: %v", err)
			return nil, err
		}
	}

	recentSigners := make(map[types.Int256]types.Address)
	recentSignBytes, err := rawdb.RecentSigners(db)
	if err == nil {
		log.Infof("loading recentSignBytes start")
		var temp map[string]types.Address
		if err := json.Unmarshal(recentSignBytes, &temp); err != nil {
			return nil, err
		}

		for k, v := range temp {
			number, err := types.FromHex(k)
			if err != nil {
				return nil, err
			}

			recentSigners[number] = v
		}
		log.Infof("loading recentSignBytes end %v", recentSigners)
	}

	c, cancel := context.WithCancel(ctx)
	log.Infof("miner address: %s", types.PrivateToAddress(minerKey).String())

	engine := &MPoa{
		config:        config,
		db:            db,
		recentBlocks:  resents,
		signatures:    signatures,
		recentSigners: recentSigners,
		proposals:     make(map[types.Address]bool),
		exitCh:        make(chan struct{}),
		stopCh:        make(chan struct{}),
		singers:       signers,
		miner:         minerKey,
		ctx:           c,
		cancel:        cancel,
		chain:         chain,
		txsPool:       txsPool,
		pubsub:        pubsub,
		signer:        types.PrivateToAddress(minerKey),
	}

	return engine, nil
}

func (M *MPoa) saveRecent() error {
	M.lock.Lock()
	defer M.lock.Unlock()

	temp := make(map[string]types.Address)
	for n, a := range M.recentSigners {
		temp[n.String()] = a
	}

	recentByes, err := json.Marshal(temp)
	if err != nil {
		return err
	}

	return rawdb.StoreRecentSigners(M.db, recentByes)
}
