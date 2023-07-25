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
	"github.com/amazechain/amc/common/hash"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"strings"
	"sync/atomic"
	"time"
)

//type writeCounter types.StorageSize
//
//func (c *writeCounter) Write(b []byte) (int, error) {
//	*c += writeCounter(len(b))
//	return len(b), nil
//}

type Block struct {
	header *Header
	body   *Body

	hash atomic.Value
	size atomic.Value

	ReceiveAt    time.Time
	ReceivedFrom interface{}
}

func (b *Block) Copy() *Block {
	var hashValue atomic.Value
	if value := b.hash.Load(); value != nil {
		hash := value.(types.Hash)
		hashCopy := types.BytesToHash(hash.Bytes())
		hashValue.Store(hashCopy)
	}

	var sizeValue atomic.Value
	if size := b.size.Load(); size != nil {
		sizeValue.Store(size)
	}
	pb := b.body.ToProtoMessage()
	cbody := new(Body)
	if err := cbody.FromProtoMessage(pb); nil != err {
		log.Warn("body copy failed!", err)
	}
	return &Block{
		header: CopyHeader(b.header),
		body:   cbody,
		hash:   hashValue,
		size:   sizeValue,
	}
}

type Verify struct {
	Address   types.Address
	PublicKey types.PublicKey
}

func (v *Verify) ToProtoMessage() proto.Message {
	var pbVerifier types_pb.Verifier
	pbVerifier.Address = utils.ConvertAddressToH160(v.Address)
	pbVerifier.PublicKey = utils.ConvertPublicKeyToH384(v.PublicKey)
	return &pbVerifier
}

func (v *Verify) FromProtoMessage(pbVerifier *types_pb.Verifier) *Verify {
	v.Address = utils.ConvertH160toAddress(pbVerifier.Address)
	v.PublicKey = utils.ConvertH384ToPublicKey(pbVerifier.PublicKey)
	return v
}

type Reward struct {
	Address types.Address
	Amount  *uint256.Int
}

type Rewards []*Reward

func (r Rewards) Len() int {
	return len(r)
}

func (r Rewards) Less(i, j int) bool {
	return strings.Compare(r[i].Address.String(), r[j].Address.String()) > 0
}

func (r Rewards) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r *Reward) ToProtoMessage() proto.Message {
	var pbReward types_pb.Reward
	pbReward.Address = utils.ConvertAddressToH160(r.Address)
	pbReward.Amount = utils.ConvertUint256IntToH256(r.Amount)
	return &pbReward
}

func (r *Reward) FromProtoMessage(pbReward *types_pb.Reward) *Reward {
	r.Address = utils.ConvertH160toAddress(pbReward.Address)
	r.Amount = utils.ConvertH256ToUint256Int(pbReward.Amount)
	return r
}

func (b *Block) Transactions() []*transaction.Transaction {
	if b.body != nil {
		return b.body.Transactions()
	}

	return nil
}

func (b *Block) StateRoot() types.Hash {
	return b.header.Root
}

func (b *Block) Hash() types.Hash {
	return b.Header().Hash()
}

func (b *Block) Marshal() ([]byte, error) {
	bpBlock := b.ToProtoMessage()
	return proto.Marshal(bpBlock)
}

func (b *Block) Unmarshal(data []byte) error {
	var pBlock types_pb.Block
	if err := proto.Unmarshal(data, &pBlock); err != nil {
		return err
	}
	if err := b.FromProtoMessage(&pBlock); err != nil {
		return err
	}
	return nil
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewBlock(h IHeader, txs []*transaction.Transaction) IBlock {

	block := &Block{
		header:       CopyHeader(h.(*Header)),
		body:         &Body{Txs: txs},
		ReceiveAt:    time.Now(),
		ReceivedFrom: nil,
	}
	return block
}

func NewBlockFromReceipt(h IHeader, txs []*transaction.Transaction, uncles []IHeader, receipts []*Receipt, reward []*Reward) IBlock {

	block := &Block{
		header:       CopyHeader(h.(*Header)),
		body:         &Body{Txs: txs, Rewards: CopyReward(reward)},
		ReceiveAt:    time.Now(),
		ReceivedFrom: nil,
	}
	//if len(receipts) > 0 {
	//	print(receipts)
	//}

	block.header.Bloom = CreateBloom(receipts)
	block.header.TxHash = hash.DeriveSha(transaction.Transactions(txs))
	block.header.ReceiptHash = hash.DeriveSha(Receipts(receipts))

	return block
}

func (b *Block) Header() IHeader {
	return CopyHeader(b.header)
}

func (b *Block) Body() IBody {
	return b.body
}

func (b *Block) Number64() *uint256.Int {
	return b.header.Number
}

func (b *Block) BaseFee64() *uint256.Int {
	return b.header.BaseFee
}

func (b *Block) Difficulty() *uint256.Int {
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

func (b *Block) WithSeal(header IHeader) *Block {
	b.header = CopyHeader(header.(*Header))
	return b
}

func (b *Block) Transaction(hash types.Hash) *transaction.Transaction {
	return nil
}

func (b *Block) ToProtoMessage() proto.Message {
	pbHeader := b.header.ToProtoMessage()
	pbBody := b.body.ToProtoMessage()
	pBlock := types_pb.Block{
		Header: pbHeader.(*types_pb.Header),
		Body:   pbBody.(*types_pb.Body),
	}

	return &pBlock
}

func (b *Block) FromProtoMessage(message proto.Message) error {
	var (
		pBlock *types_pb.Block
		header Header
		body   Body
		ok     bool
	)

	if pBlock, ok = message.(*types_pb.Block); !ok {
		return fmt.Errorf("type conversion failure")
	}

	if err := header.FromProtoMessage(pBlock.Header); err != nil {
		return err
	}

	if err := body.FromProtoMessage(pBlock.Body); err != nil {
		return err
	}

	b.header = &header
	b.body = &body
	b.ReceiveAt = time.Now()
	return nil
}

func (b *Block) SendersToTxs(senders []types.Address) {
	//todo
}

func (b *Block) Uncles() []*Header {
	return nil
}
