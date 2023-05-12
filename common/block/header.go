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
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/golang/protobuf/proto"
	"github.com/holiman/uint256"
	"sync/atomic"

	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
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

type Header struct {
	ParentHash  types.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase    types.Address `json:"miner"`
	Root        types.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      types.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash types.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       Bloom         `json:"logsBloom"        gencodec:"required"`
	Difficulty  *uint256.Int  `json:"difficulty"       gencodec:"required"`
	Number      *uint256.Int  `json:"number"           gencodec:"required"`
	GasLimit    uint64        `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64        `json:"gasUsed"          gencodec:"required"`
	Time        uint64        `json:"timestamp"        gencodec:"required"`
	MixDigest   types.Hash    `json:"mixHash"`
	Nonce       BlockNonce    `json:"nonce"`
	Extra       []byte        `json:"extraData"        gencodec:"required"`

	// BaseFee was added by EIP-1559 and is ignored in legacy headers.
	BaseFee *uint256.Int `json:"baseFeePerGas" rlp:"optional"`

	hash atomic.Value

	Signature types.Signature `json:"signature"`
}

func (h *Header) Number64() *uint256.Int {
	return h.Number
}

func (h *Header) StateRoot() types.Hash {
	return h.Root
}

func (h *Header) BaseFee64() *uint256.Int {
	return h.BaseFee
}

func (h Header) Hash() types.Hash {
	if hash := h.hash.Load(); hash != nil {
		return hash.(types.Hash)
	}

	if h.BaseFee == nil {
		h.BaseFee = uint256.NewInt(0)
	}

	if h.Difficulty == nil {
		h.Difficulty = uint256.NewInt(0)
	}

	buf, err := json.Marshal(h)
	if err != nil {
		return types.Hash{}
	}

	if h.Number.Uint64() == 0 {
		log.Tracef("genesis header json Marshal: %s", string(buf))
	}

	hash := types.BytesHash(buf)
	h.hash.Store(hash)
	return hash
}

func (h *Header) ToProtoMessage() proto.Message {
	return &types_pb.Header{
		ParentHash:  utils.ConvertHashToH256(h.ParentHash),
		Coinbase:    utils.ConvertAddressToH160(h.Coinbase),
		Root:        utils.ConvertHashToH256(h.Root),
		TxHash:      utils.ConvertHashToH256(h.TxHash),
		ReceiptHash: utils.ConvertHashToH256(h.ReceiptHash),
		Difficulty:  utils.ConvertUint256IntToH256(h.Difficulty),
		Number:      utils.ConvertUint256IntToH256(h.Number),
		GasLimit:    h.GasLimit,
		GasUsed:     h.GasUsed,
		Time:        h.Time,
		Nonce:       h.Nonce.Uint64(),
		BaseFee:     utils.ConvertUint256IntToH256(h.BaseFee),
		Extra:       h.Extra,
		Signature:   utils.ConvertSignatureToH768(h.Signature),
		Bloom:       utils.ConvertBytesToH2048(h.Bloom.Bytes()),
		MixDigest:   utils.ConvertHashToH256(h.MixDigest),
	}
}

func (h *Header) FromProtoMessage(message proto.Message) error {
	var (
		pbHeader *types_pb.Header
		ok       bool
	)

	if pbHeader, ok = message.(*types_pb.Header); !ok {
		return fmt.Errorf("type conversion failure")
	}

	h.ParentHash = utils.ConvertH256ToHash(pbHeader.ParentHash)
	h.Coinbase = utils.ConvertH160toAddress(pbHeader.Coinbase)
	h.Root = utils.ConvertH256ToHash(pbHeader.Root)
	h.TxHash = utils.ConvertH256ToHash(pbHeader.TxHash)
	h.ReceiptHash = utils.ConvertH256ToHash(pbHeader.ReceiptHash)
	h.Difficulty = utils.ConvertH256ToUint256Int(pbHeader.Difficulty)
	h.Number = utils.ConvertH256ToUint256Int(pbHeader.Number)
	h.GasLimit = pbHeader.GasLimit
	h.GasUsed = pbHeader.GasUsed
	h.Time = pbHeader.Time
	h.Nonce = EncodeNonce(pbHeader.Nonce)
	h.BaseFee = utils.ConvertH256ToUint256Int(pbHeader.BaseFee)
	h.Extra = pbHeader.Extra
	h.Signature = utils.ConvertH768ToSignature(pbHeader.Signature)
	h.Bloom = utils.ConvertH2048ToBloom(pbHeader.Bloom)
	h.MixDigest = utils.ConvertH256ToHash(pbHeader.MixDigest)
	return nil
}

func (h *Header) Marshal() ([]byte, error) {
	pbHeader := h.ToProtoMessage()
	return proto.Marshal(pbHeader)
}

func (h *Header) Unmarshal(data []byte) error {
	var pbHeader types_pb.Header
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

	if cpy.Difficulty = uint256.NewInt(0); h.Difficulty != nil {
		cpy.Difficulty.SetBytes(h.Difficulty.Bytes())
	}
	if cpy.Number = uint256.NewInt(0); h.Number != nil {
		cpy.Number.SetBytes(h.Number.Bytes())
	}

	if h.BaseFee != nil {
		cpy.BaseFee = uint256.NewInt(0).SetBytes(h.BaseFee.Bytes())
	}

	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	return &cpy
}

func CopyReward(rewards []*Reward) []*Reward {
	var cpyReward []*Reward

	for _, reward := range rewards {
		addr, amount := types.Address{}, uint256.Int{}

		addr.SetBytes(reward.Address[:])
		cpyReward = append(cpyReward, &Reward{
			Address: addr,
			Amount:  amount.SetBytes(reward.Amount.Bytes()),
		})
	}

	return cpyReward
}
