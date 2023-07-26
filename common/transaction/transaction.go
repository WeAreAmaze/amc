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
	"bytes"
	"fmt"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/types"
)

var (
	ErrGasFeeCapTooLow = fmt.Errorf("fee cap less than base fee")
	ErrUnmarshalHash   = fmt.Errorf("hash verify falied")
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

	chainID() *uint256.Int
	accessList() AccessList
	data() []byte
	gas() uint64
	gasPrice() *uint256.Int
	gasTipCap() *uint256.Int
	gasFeeCap() *uint256.Int
	value() *uint256.Int
	nonce() uint64
	to() *types.Address
	from() *types.Address
	sign() []byte
	hash() types.Hash

	rawSignatureValues() (v, r, s *uint256.Int)
	setSignatureValues(chainID, v, r, s *uint256.Int)

	//Marshal() ([]byte, error)
	//MarshalTo(data []byte) (n int, err error)
	//Unmarshal(data []byte) error
	//Size() int
}

// Transactions implements DerivableList for transactions.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// EncodeIndex encodes the i'th transaction to w. Note that this does not check for errors
// because we assume that *Transaction will only ever contain valid txs that were either
// constructed by decoding or via public API in this package.
func (s Transactions) EncodeIndex(i int, w *bytes.Buffer) {
	tx := s[i]
	proto, _ := tx.Marshal()
	w.Write(proto)
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

func txDataFromProtoMessage(message proto.Message) (TxData, error) {
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
		if nil != pbTx.To {
			itx.To = utils.ConvertH160ToPAddress(pbTx.To)
			if *itx.To == (types.Address{}) {
				itx.To = nil
			}
		}
		itx.From = utils.ConvertH160ToPAddress(pbTx.From)
		itx.Sign = pbTx.Sign
		itx.Nonce = pbTx.Nonce
		itx.Gas = pbTx.Gas
		itx.GasPrice = utils.ConvertH256ToUint256Int(pbTx.GasPrice)
		itx.Value = utils.ConvertH256ToUint256Int(pbTx.Value)
		if nil != pbTx.V {
			itx.V = utils.ConvertH256ToUint256Int(pbTx.V)
		}
		if nil != pbTx.R {
			itx.R = utils.ConvertH256ToUint256Int(pbTx.R)
		}
		if nil != pbTx.S {
			itx.S = utils.ConvertH256ToUint256Int(pbTx.S)
		}
		itx.Data = pbTx.Data
		inner = &itx
	case AccessListTxType:
		var altt AccessListTx
		altt.ChainID = uint256.NewInt(pbTx.ChainID)
		altt.Nonce = pbTx.Nonce
		altt.Gas = pbTx.Gas
		altt.GasPrice = utils.ConvertH256ToUint256Int(pbTx.GasPrice)
		altt.Value = utils.ConvertH256ToUint256Int(pbTx.Value)
		if nil != pbTx.V {
			altt.V = utils.ConvertH256ToUint256Int(pbTx.V)
		}
		if nil != pbTx.R {
			altt.R = utils.ConvertH256ToUint256Int(pbTx.R)
		}
		if nil != pbTx.S {
			altt.S = utils.ConvertH256ToUint256Int(pbTx.S)
		}
		altt.Data = pbTx.Data
		if nil != pbTx.To {
			altt.To = utils.ConvertH160ToPAddress(pbTx.To)
			if *altt.To == (types.Address{}) {
				altt.To = nil
			}
		}
		//altt.To = utils.ConvertH160ToPAddress(pbTx.To)
		altt.From = utils.ConvertH160ToPAddress(pbTx.From)
		altt.Sign = pbTx.Sign
		inner = &altt
	case DynamicFeeTxType:
		var dftt DynamicFeeTx
		dftt.ChainID = uint256.NewInt(pbTx.ChainID)
		dftt.Nonce = pbTx.Nonce
		dftt.Gas = pbTx.Gas
		dftt.GasFeeCap = utils.ConvertH256ToUint256Int(pbTx.FeePerGas)
		dftt.GasTipCap = utils.ConvertH256ToUint256Int(pbTx.PriorityFeePerGas)
		dftt.Value = utils.ConvertH256ToUint256Int(pbTx.Value)
		if nil != pbTx.V {
			dftt.V = utils.ConvertH256ToUint256Int(pbTx.V)
		}
		if nil != pbTx.R {
			dftt.R = utils.ConvertH256ToUint256Int(pbTx.R)
		}
		if nil != pbTx.S {
			dftt.S = utils.ConvertH256ToUint256Int(pbTx.S)
		}
		dftt.Data = pbTx.Data
		if nil != pbTx.To {
			dftt.To = utils.ConvertH160ToPAddress(pbTx.To)
			if *dftt.To == (types.Address{}) {
				dftt.To = nil
			}
		}

		//dftt.To = utils.ConvertH160ToPAddress(pbTx.To)
		dftt.From = utils.ConvertH160ToPAddress(pbTx.From)
		dftt.Sign = pbTx.Sign
		inner = &dftt
	}

	// todo
	protoHash := utils.ConvertH256ToHash(pbTx.Hash)
	innerHash := inner.hash()
	if bytes.Compare(protoHash[:], innerHash[:]) != 0 {
		//return nil, ErrUnmarshalHash
	}

	return inner, nil
}

func FromProtoMessage(message proto.Message) (*Transaction, error) {
	inner, err := txDataFromProtoMessage(message)
	if err != nil {
		return nil, err
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
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
	case *LegacyTx:
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		//pbTx.To = utils.ConvertAddressToH160(*tx.To())
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
	case *DynamicFeeTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		//pbTx.To = utils.ConvertAddressToH160(*tx.To())
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
		pbTx.FeePerGas = utils.ConvertUint256IntToH256(t.GasFeeCap)
		pbTx.PriorityFeePerGas = utils.ConvertUint256IntToH256(t.GasTipCap)
	}
	if tx.To() != nil {
		pbTx.To = utils.ConvertAddressToH160(*tx.To())
	}

	pbTx.Hash = utils.ConvertHashToH256(tx.Hash())

	v, r, s := tx.RawSignatureValues()
	if nil != v {
		pbTx.V = utils.ConvertUint256IntToH256(v)
	}
	if nil != r {
		pbTx.R = utils.ConvertUint256IntToH256(r)
	}
	if nil != s {
		pbTx.S = utils.ConvertUint256IntToH256(s)
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

func (tx *Transaction) RawSignatureValues() (v, r, s *uint256.Int) {
	return tx.inner.rawSignatureValues()
}

// WithSignature todo
func (tx *Transaction) WithSignatureValues(v, r, s *uint256.Int) (*Transaction, error) {
	//todo
	tx.inner.setSignatureValues(uint256.NewInt(100100100), v, r, s)
	return tx, nil
}

func (tx Transaction) Marshal() ([]byte, error) {
	var pbTx types_pb.Transaction
	pbTx.Type = uint64(tx.inner.txType())

	switch t := tx.inner.(type) {
	case *AccessListTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
	case *LegacyTx:
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		//pbTx.To = utils.ConvertAddressToH160(*tx.To())
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
	case *DynamicFeeTx:
		pbTx.ChainID = t.ChainID.Uint64()
		pbTx.Nonce = tx.Nonce()
		pbTx.Gas = tx.Gas()
		pbTx.GasPrice = utils.ConvertUint256IntToH256(tx.GasPrice())
		pbTx.Value = utils.ConvertUint256IntToH256(tx.Value())
		pbTx.Data = tx.Data()
		//pbTx.To = utils.ConvertAddressToH160(*tx.To())
		pbTx.From = utils.ConvertAddressToH160(*tx.From())
		pbTx.Sign = t.Sign
		pbTx.FeePerGas = utils.ConvertUint256IntToH256(t.GasFeeCap)
		pbTx.PriorityFeePerGas = utils.ConvertUint256IntToH256(t.GasTipCap)
	}
	if tx.To() != nil {
		pbTx.To = utils.ConvertAddressToH160(*tx.To())
	}
	v, r, s := tx.RawSignatureValues()
	if nil != v {
		pbTx.V = utils.ConvertUint256IntToH256(v)
	}
	if nil != r {
		pbTx.R = utils.ConvertUint256IntToH256(r)
	}
	if nil != s {
		pbTx.S = utils.ConvertUint256IntToH256(s)
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
	var err error
	var inner TxData
	err = proto.Unmarshal(data, &pbTx)
	if err != nil {
		return err
	}

	inner, err = txDataFromProtoMessage(&pbTx)
	if err != nil {
		return err
	}

	tx.setDecoded(inner, 0)
	return err
}

func (tx *Transaction) Type() uint8 {
	return tx.inner.txType()
}

func (tx *Transaction) ChainId() *uint256.Int {
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

func (tx *Transaction) GasPrice() *uint256.Int {
	return tx.inner.gasPrice()
}

func (tx *Transaction) GasTipCap() *uint256.Int {
	return tx.inner.gasTipCap()
}

func (tx *Transaction) GasFeeCap() *uint256.Int {
	return tx.inner.gasFeeCap()
}

func (tx *Transaction) Value() *uint256.Int {
	return tx.inner.value()
}

func (tx *Transaction) Nonce() uint64 {
	return tx.inner.nonce()
}

func (tx *Transaction) To() *types.Address {
	return copyAddressPtr(tx.inner.to())
}

func (tx *Transaction) From() *types.Address {
	return tx.inner.from()
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

func (tx *Transaction) Cost() *uint256.Int {
	price := tx.inner.gasPrice()
	gas := uint256.NewInt(tx.inner.gas())
	total := new(uint256.Int).Mul(price, gas)
	total = total.Add(total, tx.Value())
	return total
}

func (tx *Transaction) Hash() types.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(types.Hash)
	}
	h := tx.inner.hash()
	tx.hash.Store(h)
	return h
}

// GasFeeCapCmp compares the fee cap of two transactions.
func (tx *Transaction) GasFeeCapCmp(other *Transaction) int {
	return tx.inner.gasFeeCap().Cmp(other.inner.gasFeeCap())
}

// EffectiveGasTipIntCmp compares the effective gasTipCap of a transaction to the given gasTipCap.
func (tx *Transaction) EffectiveGasTipIntCmp(other *uint256.Int, baseFee *uint256.Int) int {
	if baseFee == nil {
		return tx.GasTipCapIntCmp(other)
	}
	return tx.EffectiveGasTipValue(baseFee).Cmp(other)
}

// GasTipCapCmp compares the gasTipCap of two transactions.
func (tx *Transaction) GasTipCapCmp(other *Transaction) int {
	return tx.inner.gasTipCap().Cmp(other.inner.gasTipCap())
}

// GasFeeCapIntCmp compares the fee cap of the transaction against the given fee cap.
func (tx *Transaction) GasFeeCapIntCmp(other *uint256.Int) int {
	return tx.inner.gasFeeCap().Cmp(other)
}

// GasTipCapIntCmp compares the gasTipCap of the transaction against the given gasTipCap.
func (tx *Transaction) GasTipCapIntCmp(other *uint256.Int) int {
	return tx.inner.gasTipCap().Cmp(other)
}

// EffectiveGasTipValue is identical to EffectiveGasTip, but does not return an
// error in case the effective gasTipCap is negative
func (tx *Transaction) EffectiveGasTipValue(baseFee *uint256.Int) *uint256.Int {
	effectiveTip, _ := tx.EffectiveGasTip(baseFee)
	return effectiveTip
}

// EffectiveGasTipCmp compares the effective gasTipCap of two transactions assuming the given base fee.
func (tx *Transaction) EffectiveGasTipCmp(other *Transaction, baseFee *uint256.Int) int {
	if baseFee == nil {
		return tx.GasTipCapCmp(other)
	}
	return tx.EffectiveGasTipValue(baseFee).Cmp(other.EffectiveGasTipValue(baseFee))
}

// EffectiveGasTip returns the effective miner gasTipCap for the given base fee.
// Note: if the effective gasTipCap is negative, this method returns both error
// the actual negative value, _and_ ErrGasFeeCapTooLow
func (tx *Transaction) EffectiveGasTip(baseFee *uint256.Int) (*uint256.Int, error) {
	if baseFee == nil {
		return tx.GasTipCap(), nil
	}
	var err error
	gasFeeCap := tx.GasFeeCap()
	if gasFeeCap.Cmp(baseFee) == -1 {
		err = ErrGasFeeCapTooLow
	}
	return uint256Min(tx.GasTipCap(), new(uint256.Int).Sub(gasFeeCap, baseFee)), err
}

func uint256Min(x, y *uint256.Int) *uint256.Int {
	if x.Cmp(y) == 1 {
		return x
	}
	return y
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28 && v != 1 && v != 0
	}
	// anything not 27 or 28 is considered protected
	return true
}

// Protected says whether the transaction is replay-protected.
func (tx *Transaction) Protected() bool {
	switch tx := tx.inner.(type) {
	case *LegacyTx:
		return tx.V.ToBig() != nil && isProtectedV(tx.V.ToBig())
	default:
		return true
	}
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := tx.inner.copy()
	var chainID *uint256.Int
	if signer.ChainID() == nil {
		chainID = nil
	} else {
		chainID, _ = uint256.FromBig(signer.ChainID())
	}

	v1, _ := uint256.FromBig(v)
	r1, _ := uint256.FromBig(r)
	s1, _ := uint256.FromBig(s)
	cpy.setSignatureValues(chainID, v1, r1, s1)
	return &Transaction{inner: cpy, time: tx.time}, nil
}

//type Transaction struct {
//	to        types.Address
//	from      types.Address
//	nonce     uint64
//	amount    uint256.Int
//	gasLimit  uint64
//	gasPrice  uint256.Int
//	gasFeeCap uint256.Int
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

// Message is a fully derived transaction and implements core.Message
type Message struct {
	to         *types.Address
	from       types.Address
	nonce      uint64
	amount     uint256.Int
	gasLimit   uint64
	gasPrice   uint256.Int
	feeCap     uint256.Int
	tip        uint256.Int
	data       []byte
	accessList AccessList
	checkNonce bool
	isFree     bool
}

func NewMessage(from types.Address, to *types.Address, nonce uint64, amount *uint256.Int, gasLimit uint64, gasPrice *uint256.Int, feeCap, tip *uint256.Int, data []byte, accessList AccessList, checkNonce bool, isFree bool) Message {
	m := Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     *amount,
		gasLimit:   gasLimit,
		data:       data,
		accessList: accessList,
		checkNonce: checkNonce,
		isFree:     isFree,
	}
	if gasPrice != nil {
		m.gasPrice.Set(gasPrice)
	}
	if tip != nil {
		m.tip.Set(tip)
	}
	if feeCap != nil {
		m.feeCap.Set(feeCap)
	}
	return m
}

// AsMessage returns the transaction as a core.Message.
func (tx *Transaction) AsMessage(s Signer, baseFee *uint256.Int) (Message, error) {
	msg := Message{
		nonce:    tx.Nonce(),
		gasLimit: tx.Gas(),
		gasPrice: *new(uint256.Int).Set(tx.GasPrice()),
		feeCap:   *new(uint256.Int).Set(tx.GasFeeCap()),
		tip:      *new(uint256.Int).Set(tx.GasTipCap()),
		//gasFeeCap:  new(big.Int).Set(tx.GasFeeCap()),
		//gasTipCap:  new(big.Int).Set(tx.GasTipCap()),
		to:         tx.To(),
		amount:     *tx.Value(),
		data:       tx.Data(),
		accessList: tx.AccessList(),
		checkNonce: false,
		//isFake:     false,
	}

	// If baseFee provided, set gasPrice to effectiveGasPrice.
	if baseFee != nil {
		msg.gasPrice.Add(&msg.tip, baseFee)

		if msg.gasPrice.Gt(&msg.feeCap) {
			msg.gasPrice = msg.feeCap
		}
	}
	//var err error
	//msg.from, err = Sender(s, tx)
	//if nil != err {
	//	return msg, err
	//}
	msg.from = *tx.From()

	return msg, nil
}
func (m Message) From() types.Address    { return m.from }
func (m Message) To() *types.Address     { return m.to }
func (m Message) GasPrice() *uint256.Int { return &m.gasPrice }
func (m Message) FeeCap() *uint256.Int   { return &m.feeCap }
func (m Message) Tip() *uint256.Int      { return &m.tip }
func (m Message) Value() *uint256.Int    { return &m.amount }
func (m Message) Gas() uint64            { return m.gasLimit }
func (m Message) Nonce() uint64          { return m.nonce }
func (m Message) Data() []byte           { return m.data }
func (m Message) AccessList() AccessList { return m.accessList }
func (m Message) CheckNonce() bool       { return m.checkNonce }
func (m *Message) SetCheckNonce(checkNonce bool) {
	m.checkNonce = checkNonce
}
func (m Message) IsFree() bool { return m.isFree }
func (m *Message) SetIsFree(isFree bool) {
	m.isFree = isFree
}
