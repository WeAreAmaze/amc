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
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
)

// LegacyTx is the transaction data of regular Ethereum transactions.
type LegacyTx struct {
	Nonce    uint64         // nonce of sender account
	GasPrice types.Int256   // wei per gas
	Gas      uint64         // gas limit
	To       *types.Address `rlp:"nil"` // nil means contract creation
	From     *types.Address `rlp:"nil"` // nil means contract creation
	Value    types.Int256   // wei amount
	Data     []byte         // contract invocation input data
	V, R, S  types.Int256   // signature values
	Sign     []byte
}

// NewTransaction creates an unsigned legacy transaction.
// Deprecated: use NewTx instead.
func NewTransaction(nonce uint64, from types.Address, to types.Address, amount types.Int256, gasLimit uint64, gasPrice types.Int256, data []byte) *Transaction {
	return NewTx(&LegacyTx{
		Nonce:    nonce,
		To:       &to,
		From:     &from,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})
}

// NewContractCreation creates an unsigned legacy transaction.
// Deprecated: use NewTx instead.
func NewContractCreation(nonce uint64, amount types.Int256, gasLimit uint64, gasPrice types.Int256, data []byte) *Transaction {
	return NewTx(&LegacyTx{
		Nonce:    nonce,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *LegacyTx) copy() TxData {
	cpy := &LegacyTx{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		From:  copyAddressPtr(tx.From),
		Data:  common.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are initialized below.
		Value:    types.NewInt64(0),
		GasPrice: types.NewInt64(0),
	}
	if !tx.Value.IsEmpty() {
		cpy.Value.Set(tx.Value)
	}
	if !tx.GasPrice.IsEmpty() {
		cpy.GasPrice.Set(tx.GasPrice)
	}
	if !tx.V.IsEmpty() {
		cpy.V.Set(tx.V)
	}
	if !tx.R.IsEmpty() {
		cpy.R.Set(tx.R)
	}
	if !tx.S.IsEmpty() {
		cpy.S.Set(tx.S)
	}
	if tx.sign() != nil {
		copy(cpy.Sign, tx.Sign)
	}

	return cpy
}

// accessors for innerTx.
func (tx *LegacyTx) txType() byte { return LegacyTxType }
func (tx *LegacyTx) chainID() types.Int256 {
	//bInt := deriveChainId(tx.V.ToBig())
	//id, _ := types.FromBig(bInt)
	//return id
	return types.NewInt64(0)
}
func (tx *LegacyTx) accessList() AccessList  { return nil }
func (tx *LegacyTx) data() []byte            { return tx.Data }
func (tx *LegacyTx) gas() uint64             { return tx.Gas }
func (tx *LegacyTx) gasPrice() types.Int256  { return tx.GasPrice }
func (tx *LegacyTx) gasTipCap() types.Int256 { return tx.GasPrice }
func (tx *LegacyTx) gasFeeCap() types.Int256 { return tx.GasPrice }
func (tx *LegacyTx) value() types.Int256     { return tx.Value }
func (tx *LegacyTx) nonce() uint64           { return tx.Nonce }
func (tx *LegacyTx) to() *types.Address      { return tx.To }
func (tx *LegacyTx) from() *types.Address    { return tx.From }
func (tx *LegacyTx) sign() []byte            { return tx.Sign }

func (tx *LegacyTx) rawSignatureValues() (v, r, s types.Int256) {
	return tx.V, tx.R, tx.S
}

func (tx *LegacyTx) setSignatureValues(chainID, v, r, s types.Int256) {
	tx.V, tx.R, tx.S = v, r, s
}

//func deriveChainId(v *big.Int) *big.Int {
//	if v.BitLen() <= 64 {
//		v := v.Uint64()
//		if v == 27 || v == 28 {
//			return new(big.Int)
//		}
//		return new(big.Int).SetUint64((v - 35) / 2)
//	}
//	v = new(big.Int).Sub(v, big.NewInt(35))
//	return v.Div(v, big.NewInt(2))
//}
