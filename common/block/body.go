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

package block

import (
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/utils"
	"google.golang.org/protobuf/proto"
)

type Body struct {
	Txs       []*transaction.Transaction
	Verifiers []*Verify
	Rewards   []*Reward
}

func (b *Body) ToProtoMessage() proto.Message {
	var pbTxs []*types_pb.Transaction
	var pbVerifiers []*types_pb.Verifier
	var pbRewards []*types_pb.Reward

	for _, v := range b.Txs {
		pbTx := v.ToProtoMessage()
		if pbTx != nil {
			pbTxs = append(pbTxs, pbTx.(*types_pb.Transaction))
		}
	}

	for _, reward := range b.Rewards {
		pbReward := reward.ToProtoMessage()
		if pbReward != nil {
			pbRewards = append(pbRewards, pbReward.(*types_pb.Reward))
		}
	}

	for _, verifier := range b.Verifiers {
		pbVerifier := verifier.ToProtoMessage()
		if pbVerifier != nil {
			pbVerifiers = append(pbVerifiers, pbVerifier.(*types_pb.Verifier))
		}
	}

	pBody := types_pb.Body{
		Txs:       pbTxs,
		Verifiers: pbVerifiers,
		Rewards:   pbRewards,
	}

	return &pBody
}

func (b *Body) FromProtoMessage(message proto.Message) error {
	var (
		pBody *types_pb.Body
		ok    bool
	)

	if pBody, ok = message.(*types_pb.Body); !ok {
		return fmt.Errorf("type conversion failure")
	}

	var txs []*transaction.Transaction
	//
	for _, v := range pBody.Txs {
		tx, err := transaction.FromProtoMessage(v)
		if err != nil {
			return err
		}
		txs = append(txs, tx)
	}
	//
	b.Txs = txs

	//verifiers
	var verifiers []*Verify
	for _, v := range pBody.Verifiers {
		verify := new(Verify).FromProtoMessage(v)
		verifiers = append(verifiers, verify)
	}
	b.Verifiers = verifiers

	//Reward
	var rewards []*Reward
	for _, v := range pBody.Rewards {
		reward := new(Reward).FromProtoMessage(v)
		rewards = append(rewards, reward)
	}
	b.Rewards = rewards

	return nil
}

func (b *Body) Transactions() []*transaction.Transaction {
	return b.Txs
}
func (b *Body) Verifier() []*Verify {
	return b.Verifiers
}

func (b *Body) Reward() []*Reward {
	return b.Rewards
}

func (b *Body) reward() []*types_pb.H256 {
	var rewardAmount []*types_pb.H256
	if len(b.Rewards) > 0 {
		for _, reward := range b.Rewards {
			rewardAmount = append(rewardAmount, utils.ConvertUint256IntToH256(reward.Amount))
		}
	}
	return rewardAmount
}

func (b *Body) rewardAddress() []types.Address {
	var rewardAddress []types.Address
	for _, reward := range b.Rewards {
		rewardAddress = append(rewardAddress, reward.Address)
	}
	return rewardAddress
}

func (b *Body) SendersFromTxs() []types.Address {
	senders := make([]types.Address, len(b.Transactions()))
	for i, tx := range b.Transactions() {
		senders[i] = *tx.From()
	}
	return senders
}

func (b *Body) SendersToTxs(senders []types.Address) {
	if senders == nil {
		return
	}

	//todo
	//for i, tx := range b.Txs {
	//	//tx.SetFrom(senders[i])
	//}
}

type BodyForStorage struct {
	BaseTxId uint64
	TxAmount uint32
}

func NewBlockFromStorage(hash types.Hash, header *Header, body *Body) *Block {
	b := &Block{header: header, body: body}
	b.hash.Store(hash)
	return b
}

type RawBody struct {
	Transactions [][]byte
}
