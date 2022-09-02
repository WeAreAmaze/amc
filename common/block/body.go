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
	"github.com/gogo/protobuf/proto"
)

type Body struct {
	Txs []*transaction.Transaction
}

func (b *Body) ToProtoMessage() proto.Message {
	var pbTxs []*types_pb.Transaction
	for _, v := range b.Txs {
		pbTx := v.ToProtoMessage()
		if pbTx != nil {
			pbTxs = append(pbTxs, pbTx.(*types_pb.Transaction))
		}
	}

	pBody := types_pb.PBody{
		Txs: pbTxs,
	}

	return &pBody
}

func (b *Body) FromProtoMessage(message proto.Message) error {
	var (
		pBody *types_pb.PBody
		ok    bool
	)

	if pBody, ok = message.(*types_pb.PBody); !ok {
		return fmt.Errorf("type conversion failure")
	}

	var txs []*transaction.Transaction

	for _, v := range pBody.Txs {
		tx, err := transaction.FromProtoMessage(v)
		if err == nil {
			txs = append(txs, tx)
		}
	}

	b.Txs = txs

	return nil
}

func (b *Body) Transactions() []*transaction.Transaction {
	return b.Txs
}
