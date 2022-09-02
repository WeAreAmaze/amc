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

package transaction

import (
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/types"
	log "github.com/amazechain/amc/log"
	"github.com/gogo/protobuf/proto"
	"sync/atomic"
	"time"
)

var (
	ErrGasFeeCapTooLow = fmt.Errorf("fee cap less than base fee")
)

// Transaction types.
const (
	LegacyTxType = iota
	AccessListTxType
	DynamicFeeTxType
)

type TxData interface {
	txType() byte // returns the type ID
	copy() TxData // creates a deep copy and initializes all fields

	chainID() types.Int256
	accessList() AccessList
	data() []byte
	gas() uint64
	gasPrice() types.Int256
	gasTipCap() types.Int256
	gasFeeCap() types.Int256
	value() types.Int256
	nonce() uint64
	to() *types.Address
	from() *types.Address
	sign() []byte

	rawSignatureValues() (v, r, s types.Int256)
	setSignatureValues(chainID, v, r, s types.Int256)

	//Marshal() ([]byte, error)
	//MarshalTo(data []byte) (n int, err error)
	//Unmarshal(data []byte) error
	//Size() int
}

type Transaction struct {
	inner TxData    // Consensus contents of a transaction
	time  time.Time // Time first seen locally (spam avoidance)

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// NewTx creates a new transaction.
func NewTx(inner TxData) *Transaction {
	tx := new(Transaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

func FromProtoMessage(message proto.Message) (*Transaction, error) {
	var (
		pbTx  *types_pb.Transaction
		ok    bool
		inner TxData
	)

	if pbTx, ok = message.(*types_pb.Transaction); !ok {
		return nil, fmt.Errorf("aa")
	}

	switch pbTx.Type {
	case LegacyTxType:
		var itx LegacyTx
		itx.To = &pbTx.To
		itx.From = &pbTx.From
		itx.Sign = pbTx.Sing
		itx.Nonce = pbTx.Nonce
		itx.Gas = pbTx.Gas
		itx.GasPrice = pbTx.GasPrice
		itx.Value = pbTx.Value
		itx.Data = pbTx.Data
		inner = &itx
	case AccessListTxType:
		var altt AccessListTx
		altt.ChainID = types.NewInt64(pbTx.ChainID)
		altt.Nonce = pbTx.Nonce
		altt.Gas = pbTx.Gas
		altt.GasPrice = pbTx.GasPrice
		altt.Value = pbTx.Value
		altt.Data = pbTx.Data
		altt.To = &pbTx.To
		altt.From = &pbTx.From
		altt.Sign = pbTx.Sing
		inner = &altt
	case DynamicFeeTxType:
		var dftt DynamicFeeTx
		dftt.ChainID = types.NewInt64(pbTx.ChainID)
		dftt.Nonce = pbTx.Nonce
		dftt.Gas = pbTx.Gas
		dftt.GasFeeCap = pbTx.FeePerGas
		dftt.GasTipCap = pbTx.PriorityFeePerGas
		dftt.Value = pbTx.Value
		dftt.Data = pbTx.Data
		dftt.To = &pbTx.To
		dftt.From = &pbTx.From
		dftt.Sign = pbTx.Sing
		inner = &dftt
	}

	return NewTx(inner), nil
}

func (tx *Transaction) ToProtoMessage() proto.Message {
	var pbTx types_pb.Transaction
	pbTx.Type = uint64(tx.inner.txType())

	switch t := tx.inner.(type) {
	case *AccessListTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
	case *LegacyTx:
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
	case *DynamicFeeTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
		pbTx.FeePerGas = t.GasFeeCap
		pbTx.PriorityFeePerGas = t.GasTipCap
	}

	return &pbTx
}

func (tx *Transaction) setDecoded(inner TxData, size int) {
	tx.inner = inner
	tx.time = time.Now()
	//if size > 0 {
	//	tx.size.Store(common.StorageSize(size))
	//}
}

func (tx *Transaction) RawSignatureValues() (v, r, s types.Int256) {
	return tx.inner.rawSignatureValues()
}

func (tx Transaction) Marshal() ([]byte, error) {
	var pbTx types_pb.Transaction
	pbTx.Type = uint64(tx.inner.txType())

	switch t := tx.inner.(type) {
	case *AccessListTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
	case *LegacyTx:
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
	case *DynamicFeeTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = tx.GasPrice()
		pbTx.Value = tx.Value()
		pbTx.Data = tx.Data()
		pbTx.To = *tx.To()
		pbTx.From, _ = tx.From()
		pbTx.Sing = t.Sign
		pbTx.FeePerGas = t.GasFeeCap
		pbTx.PriorityFeePerGas = t.GasTipCap
	}

	return proto.Marshal(&pbTx)
}

func (tx *Transaction) MarshalTo(data []byte) (n int, err error) {
	b, err := tx.Marshal()
	if err != nil {
		return 0, err
	}

	copy(data, b)
	return len(b), nil
}

func (tx *Transaction) Unmarshal(data []byte) error {
	var pbTx types_pb.Transaction
	err := proto.Unmarshal(data, &pbTx)
	if err != nil {
		return err
	}

	switch pbTx.Type {
	case LegacyTxType:
		//var itx LegacyTx
		//if pbTx.To
	case AccessListTxType:
	case DynamicFeeTxType:

	}

	return nil
}

func (tx *Transaction) Type() uint8 {
	return tx.inner.txType()
}

func (tx *Transaction) ChainId() types.Int256 {
	return tx.inner.chainID()
}

func (tx *Transaction) Data() []byte {
	return tx.inner.data()
}

func (tx *Transaction) AccessList() AccessList {
	return tx.inner.accessList()
}

func (tx *Transaction) Gas() uint64 {
	return tx.inner.gas()
}

func (tx *Transaction) GasPrice() types.Int256 {
	return tx.inner.gasPrice()
}

func (tx *Transaction) GasTipCap() types.Int256 {
	return tx.inner.gasTipCap()
}

func (tx *Transaction) GasFeeCap() types.Int256 {
	return tx.inner.gasFeeCap()
}

func (tx *Transaction) Value() types.Int256 {
	return tx.inner.value()
}

func (tx *Transaction) Nonce() uint64 {
	return tx.inner.nonce()
}

func (tx *Transaction) To() *types.Address {
	return tx.inner.to()
}

func (tx *Transaction) From() (types.Address, error) {
	return *tx.inner.from(), nil
}

func (tx *Transaction) SetFrom(addr types.Address) {
	switch t := tx.inner.(type) {
	case *AccessListTx:
		t.From = &addr
	case *LegacyTx:
		t.From = &addr
	case *DynamicFeeTx:
		t.From = &addr
	}
}

func (tx *Transaction) SetNonce(nonce uint64) {
	switch t := tx.inner.(type) {
	case *AccessListTx:
		t.Nonce = nonce
	case *LegacyTx:
		t.Nonce = nonce
	case *DynamicFeeTx:
		t.Nonce = nonce
	}
}

func (tx *Transaction) Sign() []byte {
	return tx.inner.sign()
}

func (tx *Transaction) Cost() types.Int256 {
	price := tx.inner.gasPrice()
	gas := types.NewInt64(tx.inner.gas())
	total := new(types.Int256).Mul(price, gas)
	total = total.Add(tx.Value())
	return total
}

func (tx *Transaction) Hash() (types.Hash, error) {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(types.Hash), nil
	}
	buf, err := tx.Marshal()
	if err != nil {
		log.Errorf("tx Marshal error: %v", err)
		return types.Hash{}, err
	}

	hash := types.BytesToHash(buf)
	tx.hash.Store(hash)
	return hash, nil
}

// EffectiveGasTipIntCmp compares the effective gasTipCap of a transaction to the given gasTipCap.
func (tx *Transaction) EffectiveGasTipIntCmp(other types.Int256, baseFee types.Int256) int {
	if baseFee.IsEmpty() {
		return tx.GasTipCapIntCmp(other)
	}
	return tx.EffectiveGasTipValue(baseFee).Compare(other)
}

// GasTipCapIntCmp compares the gasTipCap of the transaction against the given gasTipCap.
func (tx *Transaction) GasTipCapIntCmp(other types.Int256) int {
	return tx.inner.gasTipCap().Compare(other)
}

// EffectiveGasTipValue is identical to EffectiveGasTip, but does not return an
// error in case the effective gasTipCap is negative
func (tx *Transaction) EffectiveGasTipValue(baseFee types.Int256) types.Int256 {
	effectiveTip, _ := tx.EffectiveGasTip(baseFee)
	return effectiveTip
}

// EffectiveGasTip returns the effective miner gasTipCap for the given base fee.
// Note: if the effective gasTipCap is negative, this method returns both error
// the actual negative value, _and_ ErrGasFeeCapTooLow
func (tx *Transaction) EffectiveGasTip(baseFee types.Int256) (types.Int256, error) {
	if baseFee.IsEmpty() {
		return tx.GasTipCap(), nil
	}
	var err error
	gasFeeCap := tx.GasFeeCap()
	if gasFeeCap.Compare(baseFee) == -1 {
		err = ErrGasFeeCapTooLow
	}
	return types.Int256Min(tx.GasTipCap(), gasFeeCap.Sub(baseFee)), err
}

//type Transaction struct {
//	to        types.Address
//	from      types.Address
//	nonce     uint64
//	amount    types.Int256
//	gasLimit  uint64
//	gasPrice  types.Int256
//	gasFeeCap types.Int256
//	payload   []byte
//}

//func (t *Transaction) ToProtoMessage() proto.Message {
//	pbTx := types_pb.Transaction{
//		From:  types.Address(t.from),
//		To:    types.Address(t.to),
//		Value: t.amount,
//		Data:  t.payload,
//		Nonce: t.nonce,
//		S:     nil,
//		V:     nil,
//		R:     nil,
//	}
//
//	return &pbTx
//}

func copyAddressPtr(a *types.Address) *types.Address {
	if a == nil {
		return nil
	}
	cpy := *a
	return &cpy
}
