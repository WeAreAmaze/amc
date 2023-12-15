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
	"github.com/amazechain/amc/common/types"

	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
)

type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address types.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []types.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber *uint256.Int `json:"blockNumber"`
	// hash of the transaction
	TxHash types.Hash `json:"transactionHash" gencodec:"required"`

	// Address of the transaction
	Sender types.Address `json:"-"`

	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex" gencodec:"required"`
	// hash of the block in which the transaction was included
	BlockHash types.Hash `json:"blockHash"`
	// index of the log in the receipt
	Index uint `json:"logIndex" gencodec:"required"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed"`
}

func (l *Log) ToProtoMessage() proto.Message {
	return &types_pb.Log{
		Address:     utils.ConvertAddressToH160(l.Address),
		Topics:      utils.ConvertHashesToH256(l.Topics),
		Data:        l.Data,
		BlockNumber: utils.ConvertUint256IntToH256(l.BlockNumber),
		TxHash:      utils.ConvertHashToH256(l.TxHash),
		TxIndex:     uint64(l.TxIndex),
		BlockHash:   utils.ConvertHashToH256(l.BlockHash),
		Index:       uint64(l.Index),
		Removed:     l.Removed,
		Sender:      utils.ConvertAddressToH160(l.Sender),
	}
}

func (l *Log) FromProtoMessage(message proto.Message) error {
	var (
		pLog *types_pb.Log
		ok   bool
	)

	if pLog, ok = message.(*types_pb.Log); !ok {
		return fmt.Errorf("type conversion failure")
	}

	l.Address = utils.ConvertH160toAddress(pLog.Address)
	l.Topics = utils.H256sToHashes(pLog.Topics)
	l.Data = pLog.Data
	l.BlockNumber = utils.ConvertH256ToUint256Int(pLog.BlockNumber)
	l.TxHash = utils.ConvertH256ToHash(pLog.TxHash)
	l.TxIndex = uint(pLog.TxIndex)
	l.BlockHash = utils.ConvertH256ToHash(pLog.BlockHash)
	l.Index = uint(pLog.Index)
	l.Removed = pLog.Removed
	l.Sender = utils.ConvertH160toAddress(pLog.Sender)

	return nil
}

type Logs []*Log

func (l *Logs) Marshal() ([]byte, error) {
	pb := new(types_pb.Logs)
	for _, log := range *l {
		pb.Logs = append(pb.Logs, log.ToProtoMessage().(*types_pb.Log))
	}

	return proto.Marshal(pb)
}

func (l *Logs) Unmarshal(data []byte) error {
	pb := new(types_pb.Logs)
	if err := proto.Unmarshal(data, pb); nil != err {
		return err
	}

	body := make([]*Log, len(pb.Logs))
	for i, p := range pb.Logs {
		if err := body[i].FromProtoMessage(p); nil != err {
			return err
		}
	}
	*l = body
	return nil
}
