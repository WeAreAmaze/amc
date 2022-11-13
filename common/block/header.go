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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common/hexutil"
	"github.com/gogo/protobuf/proto"
	"reflect"
	"sync/atomic"
)

type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText todo copy eth
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

var headerSize = types.StorageSize(reflect.TypeOf(Header{}).Size())

type Header struct {
	ParentHash  types.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase    types.Address `json:"miner"`
	Root        types.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      types.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash types.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Difficulty  types.Int256  `json:"difficulty"       gencodec:"required"`
	Number      types.Int256  `json:"number"           gencodec:"required"`
	GasLimit    uint64        `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64        `json:"gasUsed"          gencodec:"required"`
	Time        uint64        `json:"timestamp"        gencodec:"required"`
	MixDigest   types.Hash    `json:"mixHash"`
	Nonce       BlockNonce    `json:"nonce"`
	Extra       []byte        `json:"-"        gencodec:"required"`

	// BaseFee was added by EIP-1559 and is ignored in legacy headers.
	BaseFee types.Int256 `json:"baseFeePerGas" rlp:"optional"`

	hash atomic.Value
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() types.StorageSize {
	return headerSize + types.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen())/8)
}

func (h *Header) Number64() types.Int256 {
	return h.Number
}

func (h *Header) StateRoot() types.Hash {
	return h.Root
}

func (h *Header) BaseFee64() types.Int256 {
	return h.BaseFee
}

func (h *Header) Hash() types.Hash {
	//todo
	//if hash := h.hash.Load(); hash != nil {
	//	return hash.(types.Hash)
	//}

	buf, err := json.Marshal(h)
	if err != nil {
		return types.Hash{}
	}

	hash := types.BytesToHash(buf)
	h.hash.Store(hash)
	return hash
}

func (h *Header) ToProtoMessage() proto.Message {
	return &types_pb.PBHeader{
		ParentHash:  h.ParentHash,
		Coinbase:    h.Coinbase,
		Root:        h.Root,
		TxHash:      h.TxHash,
		ReceiptHash: h.ReceiptHash,
		Difficulty:  h.Difficulty,
		Number:      h.Number,
		GasLimit:    h.GasLimit,
		GasUsed:     h.GasUsed,
		Time:        h.Time,
		Nonce:       h.Nonce.Uint64(),
		BaseFee:     h.BaseFee,
		Extra:       h.Extra,
	}
}

func (h *Header) FromProtoMessage(message proto.Message) error {
	var (
		pbHeader *types_pb.PBHeader
		ok       bool
	)

	if pbHeader, ok = message.(*types_pb.PBHeader); !ok {
		return fmt.Errorf("type conversion failure")
	}

	h.ParentHash = pbHeader.ParentHash
	h.Coinbase = pbHeader.Coinbase
	h.Root = pbHeader.Root
	h.TxHash = pbHeader.TxHash
	h.ReceiptHash = pbHeader.ReceiptHash
	h.Difficulty = pbHeader.Difficulty
	h.Number = pbHeader.Number
	h.GasLimit = pbHeader.GasLimit
	h.GasUsed = pbHeader.GasUsed
	h.Time = pbHeader.Time
	h.Nonce = EncodeNonce(pbHeader.Nonce)
	h.BaseFee = pbHeader.BaseFee
	h.Extra = pbHeader.Extra

	return nil
}

func (h *Header) Marshal() ([]byte, error) {
	pbHeader := h.ToProtoMessage()
	return proto.Marshal(pbHeader)
}

func (h *Header) Unmarshal(data []byte) error {
	var pbHeader types_pb.PBHeader
	if err := proto.Unmarshal(data, &pbHeader); err != nil {
		return err
	}
	if err := h.FromProtoMessage(&pbHeader); err != nil {
		return err
	}
	return nil
}

func CopyHeader(h *Header) *Header {
	cpy := *h
	return &cpy
}
