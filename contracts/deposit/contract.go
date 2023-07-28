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
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
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
	case 10: //todo
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
	consensusConfig *conf.ConsensusConfig
	blockChain      common.IBlockChain
	db              kv.RwDB

	logsSub   event.Subscription // Subscription for new log event
	rmLogsSub event.Subscription // Subscription for removed log event

	logsCh   chan common.NewLogsEvent     // Channel to receive new log event
	rmLogsCh chan common.RemovedLogsEvent // Channel to receive removed log event
}

func NewDeposit(ctx context.Context, config *conf.ConsensusConfig, bc common.IBlockChain, db kv.RwDB) *Deposit {
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
						d.handleDepositEvent(l.TxHash, l.Data)
					} else if l.Topics[0] == withdrawnSignature {
						d.handleWithdrawnEvent(l.TxHash, l.Data)
					}
				}
			}
		case logRemovedEvent := <-d.rmLogsCh:
			for _, l := range logRemovedEvent.Logs {
				log.Info("logEvent", "address", l.Address, "data", l.Data, "")
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

func (d Deposit) handleDepositEvent(txHash types.Hash, data []byte) {
	// 1
	pb, amount, sig, err := UnpackDepositLogData(data)
	if err != nil {
		log.Warn("cannot unpack deposit log data")
		return
	}
	// 2
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		log.Warn("cannot unpack BLS signature", "signature", hexutil.Encode(sig), "err", err)
		return
	}
	// 3
	publicKey, err := bls.PublicKeyFromBytes(pb)
	if err != nil {
		log.Warn("cannot unpack BLS publicKey", "publicKey", hexutil.Encode(pb), "err", err)
		return
	}
	// 4
	log.Trace("DepositEvent verify:", "signature", hexutil.Encode(signature.Marshal()), "publicKey", hexutil.Encode(publicKey.Marshal()), "msg", hexutil.Encode(amount.Bytes()))
	if signature.Verify(publicKey, amount.Bytes()) {
		var tx *transaction.Transaction
		rwTx, err := d.db.BeginRw(d.ctx)
		defer rwTx.Rollback()
		if err != nil {
			log.Error("cannot open db", "err", err)
			return
		}

		tx, _, _, _, err = rawdb.ReadTransactionByHash(rwTx, txHash)
		if err != nil {
			log.Error("rawdb.ReadTransactionByHash", "err", err, "hash", txHash)
		}

		if tx != nil {
			log.Info("add Deposit info", "address", tx.From(), "amount", amount.String())

			var pub types.PublicKey
			pub.SetBytes(publicKey.Marshal())
			//
			rawdb.PutDeposit(rwTx, *tx.From(), pub, *amount)
			rwTx.Commit()
		}
	} else {
		log.Error("DepositEvent cannot Verify signature", "signature", hexutil.Encode(sig), "publicKey", hexutil.Encode(pb), "message", hexutil.Encode(amount.Bytes()), "err", err)
	}
}

func (d Deposit) handleWithdrawnEvent(txHash types.Hash, data []byte) {
	var tx *transaction.Transaction

	rwTx, err := d.db.BeginRw(d.ctx)
	defer rwTx.Rollback()
	if err != nil {
		log.Error("cannot open db", "err", err)
		return
	}
	tx, _, _, _, err = rawdb.ReadTransactionByHash(rwTx, txHash)
	if err != nil {
		log.Error("rawdb.ReadTransactionByHash", "err", err, "hash", txHash)
		return
	}
	if tx == nil {
		log.Error("cannot find Transaction", "err", err, "hash", txHash)
		return
	}

	err = rawdb.DeleteDeposit(rwTx, *tx.From())
	if err != nil {
		log.Error("cannot delete deposit", "err", err)
		return
	}
	rwTx.Commit()
}
