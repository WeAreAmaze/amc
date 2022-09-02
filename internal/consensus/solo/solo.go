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

package solo

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/utils"
	"github.com/libp2p/go-libp2p-core/crypto"
	"go.uber.org/zap"
	"time"
)

const (
	name             = "SoloEngine"
	SoloMinerTimeout = time.Duration(3) * time.Second
)

type SoloMiner struct {
	ctx    context.Context
	cancel context.CancelFunc

	miner        crypto.PrivKey
	engineName   string
	chain        common.IBlockChain
	genesisBlock block.IBlock
	currentBlock block.IBlock
	txsPool      txs_pool.ITxsPool
	pubsub       common.IPubSub

	log *zap.SugaredLogger

	stopCh chan struct{} // todo need to close
}

func (s *SoloMiner) GetMiner(iBlock block.IBlock) types.Address {
	//TODO implement me
	panic("implement me")
}

func (s *SoloMiner) TxPool() txs_pool.ITxsPool {
	//TODO implement me
	panic("implement me")
}

func (s *SoloMiner) Author(header *block.Header) types.Address {
	//TODO implement me
	panic("implement me")
}

func (s *SoloMiner) Seal(block block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error {
	//TODO implement me
	panic("implement me")
}

func (s *SoloMiner) Prepare(header *block.Header) error {
	//TODO implement me
	panic("implement me")
}

func NewSoloEngine(ctx context.Context, miner string, start bool, genesisBlock block.IBlock, chain common.IBlockChain, pubsub common.IPubSub, txsPool txs_pool.ITxsPool) (consensus.IEngine, error) {
	var (
		minner_key crypto.PrivKey
		err        error
	)

	if len(miner) >= 64 {
		minner_key, err = utils.StringToPrivate(miner)
		if err != nil {
			return nil, err
		}
	} else {
		minner_key = nil
	}
	c, cancel := context.WithCancel(ctx)
	solo := &SoloMiner{
		ctx:          c,
		cancel:       cancel,
		miner:        minner_key,
		engineName:   name,
		chain:        chain,
		genesisBlock: genesisBlock,
		currentBlock: chain.CurrentBlock(),
		txsPool:      txsPool,
		stopCh:       make(chan struct{}),
		pubsub:       pubsub,
	}

	if start && minner_key != nil {
		if err := solo.Start(); err != nil {
			solo.log.Error("start solo miner failed", err)
			return nil, err
		}
	}

	return solo, nil
}

func (s *SoloMiner) VerifyHeader(header block.IHeader) error {
	return nil
}

func (s *SoloMiner) VerifyBlock(block block.IBlock) error {
	return nil
}
func (s *SoloMiner) GenerateBlock(parentBlock block.IBlock, txs []*transaction.Transaction) (block.IBlock, error) {
	if s.miner == nil {
		return nil, fmt.Errorf("No Miner permission")
	}
	//var txHash types.Hash
	//if txs != nil {
	//	//bTxs, err := proto.Marshal(txs)
	//	//if err != nil {
	//	//	s.log.Errorf("failed to marshal txs %v", err)
	//	//	txsHash = ""
	//	//} else {
	//	//	txsHash = utils.Hash256toS(bTxs)
	//	//}
	//} else {
	//	txsHash = utils.Hash256toS([]byte(""))
	//}

	miner := types.PublicToAddress(s.miner.GetPublic())

	header := &block.Header{
		ParentHash:  parentBlock.ParentHash(),
		Coinbase:    miner,
		Root:        types.Hash{},
		TxHash:      types.Hash{},
		ReceiptHash: types.Hash{},
		Difficulty:  types.Int256{},
		Number:      parentBlock.Number64().Add(types.NewInt64(1)),
		GasLimit:    0,
		GasUsed:     0,
		Time:        uint64(time.Now().Unix()),
		Extra:       nil,
		MixDigest:   types.Hash{},
		Nonce:       block.EncodeNonce(parentBlock.Nonce() + 1),
		BaseFee:     types.Int256{},
	}

	//s.log.Info("new block ", zap.String("number", header.Number.String()))

	return block.NewBlock(header, txs), nil

	//sign_str := fmt.Sprint(header.ParentHash, header.Coinbase, header.Number, header.TxHash, header.Time)
	//sign_b := bytes.NewBufferString(sign_str)
	//sign, err := s.miner.Sign(sign_b.Bytes())
	//if err != nil {
	//	return nil, err
	//}
	//
	//header.Sign = sign
	//
	//bHeader, err := proto.Marshal(&header)
	//if err != nil {
	//	s.log.Errorf("failed marshal header %v", header.String())
	//	return nil, err
	//}
	//
	//hash := utils.Hash256toS(bHeader)
	//return &block_proto.Block{
	//	Hash:   hash,
	//	Header: &header,
	//	Txs:    txs,
	//}, nil
}

func (s *SoloMiner) EngineName() string {
	return s.engineName
}
func (s *SoloMiner) Start() error {
	go s.workLoop()
	return nil
}

func (s *SoloMiner) workLoop() {
	ticker := time.NewTicker(SoloMinerTimeout)
	defer ticker.Stop()
	defer func() {
		s.log.Debug("solo miner loop quit..")
	}()

	//topic, err := s.pubsub.JoinTopic(fmt.Sprintf(message.GossipBlockMessage, 1))
	//if err != nil {
	//	s.log.Errorf("failed to ")
	//	return
	//}

	//topic := fmt.Sprintf(message.GossipBlockMessage, 1)

	for {
		select {
		case <-s.ctx.Done():
			s.log.Infof("solo miner engine quit...")
			return
		case <-s.stopCh:
			s.log.Infof("solo miner engine stop...")
			return
		case <-ticker.C:
			txs, err := s.txsPool.GetTransaction()
			if err != nil {
				s.log.Errorf("failed to get txs from txpool %v", err)
			}

			if block, err := s.GenerateBlock(s.currentBlock, txs); err == nil {
				s.currentBlock = block
				pbBlock := block.ToProtoMessage()
				//b, err := proto.Marshal(pbBlock)
				//if err != nil {
				//	s.log.Errorf("failed to marshal block")
				//	continue
				//}
				//msg := msg_proto.NewBlockMessageData{
				//	Hash:   block.Hash(),
				//	Number: block.Number64(),
				//	Block:  b,
				//}

				event.GlobalEvent.Send(pbBlock)
				s.log.Infof("miner new Block [%v]", block)
				if err := s.pubsub.Publish(message.GossipBlockMessage, pbBlock); err != nil {
					s.log.Errorf("failed to gossip block(number:%d), err:%v", block.Number64().Uint64(), err)
				}
			} else {
				s.log.Warnf("failed generate block, err: %v", err)
			}

			ticker.Reset(SoloMinerTimeout)
		}
	}
}

func (s *SoloMiner) Stop() error {
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("solo miner engine alredy close")
	default:
		s.stopCh <- struct{}{}
	}
	return nil
}
