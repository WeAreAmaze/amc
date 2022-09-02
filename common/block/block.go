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
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/gogo/protobuf/proto"
	"sync/atomic"
	"time"
)

type writeCounter types.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

type Block struct {
	header *Header
	body   *Body

	hash atomic.Value
	size atomic.Value

	ReceiveAt    time.Time
	ReceivedFrom interface{}
}

func (b *Block) Transactions() []*transaction.Transaction {
	if b.body != nil {
		return b.body.Transactions()
	}

	return nil
}

func (b *Block) Hash() types.Hash {
	return b.Header().Hash()
}

func (b *Block) Marshal() ([]byte, error) {
	bpBlock := b.ToProtoMessage()
	return proto.Marshal(bpBlock)
}

func (b *Block) Unmarshal(data []byte) error {
	var pBlock types_pb.PBlock
	if err := proto.Unmarshal(data, &pBlock); err != nil {
		return err
	}
	if err := b.FromProtoMessage(&pBlock); err != nil {
		return err
	}
	return nil
}

func (b *Block) Size() types.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(types.StorageSize)
	}
	c := writeCounter(0)
	//todo use protobuf instead
	rlp.Encode(&c, b)
	b.size.Store(types.StorageSize(c))
	return types.StorageSize(c)
}

func NewBlock(h IHeader, txs []*transaction.Transaction) IBlock {
	block := &Block{
		header:       h.(*Header),
		body:         &Body{Txs: txs},
		ReceiveAt:    time.Now(),
		ReceivedFrom: nil,
	}
	return block
}

func NewBlockFromReceipt(h IHeader, txs []*transaction.Transaction, receipts []*Receipt) IBlock {
	b := &Block{header: CopyHeader(h.(*Header))}
	if len(txs) == 0 {
		b.header.TxHash = types.Hash{0}
	} else {
		// todo calculate txs hash
	}
	b.body = &Body{Txs: txs}

	if len(receipts) == 0 {
		b.header.ReceiptHash = types.Hash{0}
	} else {
		// todo calculate receipts Hash
	}

	return b
}

func (b *Block) Header() IHeader {
	return b.header
}

func (b *Block) Body() IBody {
	return b.body
}

func (b *Block) Number64() types.Int256 {
	return b.header.Number
}

func (b *Block) BaseFee64() types.Int256 {
	return b.header.BaseFee
}

func (b *Block) Difficulty() types.Int256 {
	return b.header.Difficulty
}

func (b *Block) Time() uint64 {
	return b.header.Time
}

func (b *Block) GasLimit() uint64 {
	return b.header.GasLimit
}

func (b *Block) GasUsed() uint64 {
	return b.header.GasUsed
}

func (b *Block) Nonce() uint64 {
	return b.header.Nonce.Uint64()
}

func (b *Block) Coinbase() types.Address {
	return b.header.Coinbase
}

func (b *Block) ParentHash() types.Hash {
	return b.header.ParentHash
}

func (b *Block) TxHash() types.Hash {
	return b.header.TxHash
}

func (b *Block) Transaction(hash types.Hash) *transaction.Transaction {
	return nil
}

func (b *Block) ToProtoMessage() proto.Message {
	pbHeader := b.header.ToProtoMessage()
	pbBody := b.body.ToProtoMessage()
	pBlock := types_pb.PBlock{
		Header: pbHeader.(*types_pb.PBHeader),
		Txs:    pbBody.(*types_pb.PBody),
	}

	return &pBlock
}

func (b *Block) FromProtoMessage(message proto.Message) error {
	var (
		pBlock *types_pb.PBlock
		header Header
		body   Body
		ok     bool
	)

	if pBlock, ok = message.(*types_pb.PBlock); !ok {
		return fmt.Errorf("type conversion failure")
	}

	if err := header.FromProtoMessage(pBlock.Header); err != nil {
		return err
	}

	if err := body.FromProtoMessage(pBlock.Txs); err != nil {
		return err
	}

	b.header = &header
	b.body = &body
	b.ReceiveAt = time.Now()
	return nil
}
