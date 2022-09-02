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

type DynamicFeeTx struct {
	ChainID    types.Int256
	Nonce      uint64
	GasTipCap  types.Int256 // a.k.a. maxPriorityFeePerGas
	GasFeeCap  types.Int256 // a.k.a. maxFeePerGas
	Gas        uint64
	To         *types.Address `rlp:"nil"` // nil means contract creation
	From       *types.Address `rlp:"nil"` // nil means contract creation
	Value      types.Int256
	Data       []byte
	AccessList AccessList
	Sign       []byte       // Signature values
	V, R, S    types.Int256 // signature values
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *DynamicFeeTx) copy() TxData {
	cpy := &DynamicFeeTx{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		From:  copyAddressPtr(tx.From),
		Data:  common.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are copied below.
		AccessList: make(AccessList, len(tx.AccessList)),
		Value:      types.NewInt64(0),
		ChainID:    types.NewInt64(0),
		GasTipCap:  types.NewInt64(0),
		GasFeeCap:  types.NewInt64(0),
	}
	copy(cpy.AccessList, tx.AccessList)
	if !tx.Value.IsEmpty() {
		cpy.Value.Set(tx.Value)
	}
	if !tx.ChainID.IsEmpty() {
		cpy.ChainID.Set(tx.ChainID)
	}
	if !tx.GasTipCap.IsEmpty() {
		cpy.GasTipCap.Set(tx.GasTipCap)
	}
	if !tx.GasFeeCap.IsEmpty() {
		cpy.GasFeeCap.Set(tx.GasFeeCap)
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
	if tx.Sign != nil {
		copy(cpy.Sign, tx.Sign)
	}
	return cpy
}

// accessors for innerTx.
func (tx *DynamicFeeTx) txType() byte            { return DynamicFeeTxType }
func (tx *DynamicFeeTx) chainID() types.Int256   { return tx.ChainID }
func (tx *DynamicFeeTx) accessList() AccessList  { return tx.AccessList }
func (tx *DynamicFeeTx) data() []byte            { return tx.Data }
func (tx *DynamicFeeTx) gas() uint64             { return tx.Gas }
func (tx *DynamicFeeTx) gasFeeCap() types.Int256 { return tx.GasFeeCap }
func (tx *DynamicFeeTx) gasTipCap() types.Int256 { return tx.GasTipCap }
func (tx *DynamicFeeTx) gasPrice() types.Int256  { return tx.GasFeeCap }
func (tx *DynamicFeeTx) value() types.Int256     { return tx.Value }
func (tx *DynamicFeeTx) nonce() uint64           { return tx.Nonce }
func (tx *DynamicFeeTx) to() *types.Address      { return tx.To }
func (tx *DynamicFeeTx) from() *types.Address    { return tx.From }
func (tx *DynamicFeeTx) sign() []byte            { return tx.Sign }

func (tx *DynamicFeeTx) rawSignatureValues() (v, r, s types.Int256) {
	return tx.V, tx.R, tx.S
}

func (tx *DynamicFeeTx) setSignatureValues(chainID, v, r, s types.Int256) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}
