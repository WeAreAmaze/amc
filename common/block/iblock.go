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
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/gogo/protobuf/proto"
)

type IHeader interface {
	Number64() types.Int256
	BaseFee64() types.Int256
	Hash() types.Hash
	ToProtoMessage() proto.Message
	FromProtoMessage(message proto.Message) error
	Marshal() ([]byte, error)
	StateRoot() types.Hash
}

type IBody interface {
	Transactions() []*transaction.Transaction
	ToProtoMessage() proto.Message
	FromProtoMessage(message proto.Message) error
}

type IBlock interface {
	IHeader
	Header() IHeader
	Body() IBody
	Transaction(hash types.Hash) *transaction.Transaction
	Transactions() []*transaction.Transaction
	Number64() types.Int256
	Difficulty() types.Int256
	Time() uint64
	GasLimit() uint64
	GasUsed() uint64
	Nonce() uint64
	Coinbase() types.Address
	ParentHash() types.Hash
	TxHash() types.Hash
	Size() types.StorageSize
	//ToProtoMessage() proto.Message
	//FromProtoMessage(message proto.Message) error
}

type Blocks []IBlock
