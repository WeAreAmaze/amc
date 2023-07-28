// Copyright 2023 The AmazeChain Authors
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

package deposit

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

//go:embed abi.json
var abiJson embed.FS
var depositAbiCode []byte

var depositEventSignature = crypto.Keccak256Hash([]byte("DepositEvent(bytes,uint256,bytes)"))
var withdrawnSignature = crypto.Keccak256Hash([]byte("WithdrawnEvent(uint256)"))

const (
	//
	DayPerMonth = 30
	//
	fiftyDeposit       = 50
	OneHundredDeposit  = 100
	FiveHundredDeposit = 500
	//
	fiftyDepositMaxTaskPerEpoch       = 500
	OneHundredDepositMaxTaskPerEpoch  = 100
	FiveHundredDepositMaxTaskPerEpoch = 100
	//
	fiftyDepositRewardPerMonth       = 0.375 * params.AMT
	OneHundredDepositRewardPerMonth  = 1 * params.AMT
	FiveHundredDepositRewardPerMonth = 6.25 * params.AMT //max uint64 = ^uint64(0) ≈ 18.44 AMT so 15 AMT is ok
)

func init() {
	var err error
	depositAbiCode, err = abiJson.ReadFile("abi.json")
	if err != nil {
		panic("Could not open abi.json")
	}
}

func GetDepositInfo(tx kv.Tx, addr types.Address) *Info {

	pubkey, depositAmount, err := rawdb.GetDeposit(tx, addr)
	if err != nil {
		return nil
	}

	var (
		maxRewardPerEpoch *uint256.Int
		rewardPerBlock    *uint256.Int
	)
	depositEther := new(uint256.Int).Div(depositAmount, uint256.NewInt(params.AMT)).Uint64()
	switch depositEther {
	case fiftyDeposit:
		rewardPerBlock = new(uint256.Int).Div(uint256.NewInt(fiftyDepositRewardPerMonth), uint256.NewInt(DayPerMonth*fiftyDepositMaxTaskPerEpoch))
		maxRewardPerEpoch = new(uint256.Int).Mul(rewardPerBlock, uint256.NewInt(fiftyDepositMaxTaskPerEpoch))
	case OneHundredDeposit:
		rewardPerBlock = new(uint256.Int).Div(uint256.NewInt(OneHundredDepositRewardPerMonth), uint256.NewInt(DayPerMonth*OneHundredDepositMaxTaskPerEpoch))
		rewardPerBlock = new(uint256.Int).Add(rewardPerBlock, uint256.NewInt(params.Wei))
		maxRewardPerEpoch = new(uint256.Int).Mul(rewardPerBlock, uint256.NewInt(OneHundredDepositMaxTaskPerEpoch))
	case FiveHundredDeposit:
		rewardPerBlock = new(uint256.Int).Div(uint256.NewInt(FiveHundredDepositRewardPerMonth), uint256.NewInt(DayPerMonth*FiveHundredDepositMaxTaskPerEpoch))
		rewardPerBlock = new(uint256.Int).Add(rewardPerBlock, uint256.NewInt(params.Wei))
		//
		maxRewardPerEpoch = new(uint256.Int).Mul(rewardPerBlock, uint256.NewInt(FiveHundredDepositMaxTaskPerEpoch))
	case 10, 0: //todo
		return nil
	default:
		panic("wrong deposit amount")
	}

	return &Info{
		pubkey,
		depositAmount,
		rewardPerBlock,
		maxRewardPerEpoch,
	}
}

type Info struct {
	PublicKey         types.PublicKey `json:"PublicKey"`
	DepositAmount     *uint256.Int    `json:"DepositAmount"`
	RewardPerBlock    *uint256.Int    `json:"RewardPerBlock"`
	MaxRewardPerEpoch *uint256.Int    `json:"MaxRewardPerEpoch"`
}

//func NewInfo(depositAmount uint256.Int, publicKey bls.PublicKey) *Info {
//	return &Info{
//		PublicKey:     publicKey,
//		DepositAmount: depositAmount,
//	}
//}

type Deposit struct {
	ctx             context.Context
	cancel          context.CancelFunc
	consensusConfig *params.ConsensusConfig
	blockChain      common.IBlockChain
	db              kv.RwDB

	logsSub   event.Subscription // Subscription for new log event
	rmLogsSub event.Subscription // Subscription for removed log event

	logsCh   chan common.NewLogsEvent     // Channel to receive new log event
	rmLogsCh chan common.RemovedLogsEvent // Channel to receive removed log event
}

func NewDeposit(ctx context.Context, config *params.ConsensusConfig, bc common.IBlockChain, db kv.RwDB) *Deposit {
	c, cancel := context.WithCancel(ctx)
	d := &Deposit{
		ctx:             c,
		cancel:          cancel,
		consensusConfig: config,
		blockChain:      bc,
		db:              db,
		logsCh:          make(chan common.NewLogsEvent),
		rmLogsCh:        make(chan common.RemovedLogsEvent),
	}

	d.logsSub = event.GlobalEvent.Subscribe(d.logsCh)
	d.rmLogsSub = event.GlobalEvent.Subscribe(d.rmLogsCh)

	if d.logsSub == nil || d.rmLogsSub == nil {
		log.Error("Subscribe for event system failed")
	}
	return d
}

func (d Deposit) Start() {
	go d.eventLoop()
}

func (d Deposit) Stop() {
	d.cancel()
}

func (d Deposit) eventLoop() {
	// Ensure all subscriptions get cleaned up
	defer func() {
		d.logsSub.Unsubscribe()
		d.rmLogsSub.Unsubscribe()
	}()

	var depositContractByes, _ = hexutil.Decode(d.consensusConfig.APos.DepositContract)
	for {
		select {
		case logEvent := <-d.logsCh:
			for _, l := range logEvent.Logs {
				if nil != d.consensusConfig.APos && bytes.Compare(l.Address[:], depositContractByes[:]) == 0 {
					if l.Topics[0] == depositEventSignature {
						d.handleDepositEvent(l.TxHash, l.Sender, l.Data)
					} else if l.Topics[0] == withdrawnSignature {
						d.handleWithdrawnEvent(l.TxHash, l.Sender, l.Data)
					}
				}
			}
		case logRemovedEvent := <-d.rmLogsCh:
			for _, l := range logRemovedEvent.Logs {
				if nil != d.consensusConfig.APos && bytes.Compare(l.Address[:], depositContractByes[:]) == 0 {
					log.Trace("log event topic[0]= ", "hash", l.Topics[0], "depositEventSignature", depositEventSignature, "withdrawnSignature", withdrawnSignature)
					if l.Topics[0] == depositEventSignature {
						d.handleUndoDepositEvent(l.TxHash, l.Sender, l.Data)
					} else if l.Topics[0] == withdrawnSignature {
						d.handleUndoWithdrawnEvent(l.TxHash, l.Sender, l.Data)
					}
				}
			}
		case <-d.logsSub.Err():
			return
		case <-d.rmLogsSub.Err():
			return
		case <-d.ctx.Done():
			return
		}
	}
}

func (d Deposit) verifySignature(sig []byte, pub []byte, depositAmount *uint256.Int) error {
	// 1
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		log.Warn("cannot unpack BLS signature", "signature", hexutil.Encode(sig), "err", err)
		return err
	}
	// 2
	publicKey, err := bls.PublicKeyFromBytes(pub)
	if err != nil {
		log.Warn("cannot unpack BLS publicKey", "publicKey", hexutil.Encode(pub), "err", err)
		return err
	}
	// 3
	log.Trace("DepositEvent verify:", "signature", hexutil.Encode(signature.Marshal()), "publicKey", hexutil.Encode(publicKey.Marshal()), "msg", hexutil.Encode(depositAmount.Bytes()))
	if !signature.Verify(publicKey, depositAmount.Bytes()) {
		return errors.New("cannot Verify signature")
	}

	return nil
}

func (d Deposit) handleDepositEvent(txHash types.Hash, txAddress types.Address, data []byte) {

	pb, amount, sig, err := UnpackDepositLogData(data)
	if err != nil {
		log.Warn("cannot unpack deposit log data")
		return
	}

	if err := d.verifySignature(sig, pb, amount); err != nil {
		log.Error("cannot Verify signature", "signature", hexutil.Encode(sig), "publicKey", hexutil.Encode(pb), "message", hexutil.Encode(amount.Bytes()), "err", err)
		return
	}

	rwTx, err := d.db.BeginRw(d.ctx)
	defer rwTx.Rollback()
	if err != nil {
		log.Error("cannot open db", "err", err)
		return
	}

	var pub types.PublicKey
	_ = pub.SetBytes(pb)
	//
	if err = rawdb.DoDeposit(rwTx, txAddress, pub, amount); err != nil {
		log.Error("cannot modify database", "err", err)
	}
	if err = rwTx.Commit(); err != nil {
		log.Error("cannot commit tx", "err", err)
	}
}

func (d Deposit) handleUndoDepositEvent(txHash types.Hash, txAddress types.Address, data []byte) {
	pb, amount, sig, err := UnpackDepositLogData(data)
	if err != nil {
		log.Warn("cannot unpack deposit log data")
		return
	}

	if err := d.verifySignature(sig, pb, amount); err != nil {
		log.Error("cannot Verify signature", "signature", hexutil.Encode(sig), "publicKey", hexutil.Encode(pb), "message", hexutil.Encode(amount.Bytes()), "err", err)
		return
	}

	rwTx, err := d.db.BeginRw(d.ctx)
	defer rwTx.Rollback()
	if err != nil {
		log.Error("cannot open db", "err", err)
		return
	}

	//
	if err = rawdb.UndoDeposit(rwTx, txAddress, amount); err != nil {
		log.Error("cannot modify database", "err", err)
	}
	if err = rwTx.Commit(); err != nil {
		log.Error("cannot commit tx", "err", err)
	}

}

func (d Deposit) handleWithdrawnEvent(txHash types.Hash, txAddress types.Address, data []byte) {
	// 1
	amount, err := UnpackWithdrawnLogData(data)
	if err != nil {
		log.Warn("cannot unpack deposit log data")
		return
	}

	rwTx, err := d.db.BeginRw(d.ctx)
	defer rwTx.Rollback()
	if err != nil {
		log.Error("cannot open db", "err", err)
		return
	}

	err = rawdb.DoWithdrawn(rwTx, txAddress, amount)
	if err != nil {
		log.Error("cannot Withdrawn deposit when handle Withdrawn Event", "err", err)
		return
	}
	if err = rwTx.Commit(); nil != err {
		log.Error("cannot commit when handle Withdrawn Event", "err", err)
	}
}

func (d Deposit) handleUndoWithdrawnEvent(txHash types.Hash, txAddress types.Address, data []byte) {
	amount, err := UnpackWithdrawnLogData(data)
	if err != nil {
		log.Warn("cannot unpack deposit log data")
		return
	}
	rwTx, err := d.db.BeginRw(d.ctx)
	defer rwTx.Rollback()

	if err = rawdb.UndoWithdrawn(rwTx, txAddress, amount); err != nil {
		log.Error("cannot Undo Withdrawn deposit when handle remove Withdrawn log Event", "err", err)
		return
	}

	if err = rwTx.Commit(); nil != err {
		log.Error("cannot commit when handle Withdrawn Event", "err", err)
	}
}
