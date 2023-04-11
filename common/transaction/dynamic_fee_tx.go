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
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
)

type DynamicFeeTx struct {
	ChainID    *uint256.Int
	Nonce      uint64
	GasTipCap  *uint256.Int // a.k.a. maxPriorityFeePerGas
	GasFeeCap  *uint256.Int // a.k.a. maxFeePerGas
	Gas        uint64
	To         *types.Address `rlp:"nil"` // nil means contract creation
	From       *types.Address `rlp:"nil"` // nil means contract creation
	Value      *uint256.Int
	Data       []byte
	AccessList AccessList
	Sign       []byte       // Signature values
	V, R, S    *uint256.Int // signature values
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
		Value:      new(uint256.Int),
		ChainID:    new(uint256.Int),
		GasTipCap:  new(uint256.Int),
		GasFeeCap:  new(uint256.Int),
		V:          new(uint256.Int),
		R:          new(uint256.Int),
		S:          new(uint256.Int),
	}
	copy(cpy.AccessList, tx.AccessList)
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.GasTipCap != nil {
		cpy.GasTipCap.Set(tx.GasTipCap)
	}
	if tx.GasFeeCap != nil {
		cpy.GasFeeCap.Set(tx.GasFeeCap)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	if tx.Sign != nil {
		copy(cpy.Sign, tx.Sign)
	}
	return cpy
}

// accessors for innerTx.
func (tx *DynamicFeeTx) txType() byte            { return DynamicFeeTxType }
func (tx *DynamicFeeTx) chainID() *uint256.Int   { return tx.ChainID }
func (tx *DynamicFeeTx) accessList() AccessList  { return tx.AccessList }
func (tx *DynamicFeeTx) data() []byte            { return tx.Data }
func (tx *DynamicFeeTx) gas() uint64             { return tx.Gas }
func (tx *DynamicFeeTx) gasFeeCap() *uint256.Int { return tx.GasFeeCap }
func (tx *DynamicFeeTx) gasTipCap() *uint256.Int { return tx.GasTipCap }
func (tx *DynamicFeeTx) gasPrice() *uint256.Int  { return tx.GasFeeCap }
func (tx *DynamicFeeTx) value() *uint256.Int     { return tx.Value }
func (tx *DynamicFeeTx) nonce() uint64           { return tx.Nonce }
func (tx *DynamicFeeTx) to() *types.Address      { return tx.To }
func (tx *DynamicFeeTx) from() *types.Address    { return tx.From }
func (tx *DynamicFeeTx) sign() []byte            { return tx.Sign }

// Hash computes the hash (but not for signatures!)
func (tx *DynamicFeeTx) hash() types.Hash {
	hash := utils.PrefixedRlpHash(DynamicFeeTxType, []interface{}{
		tx.ChainID,
		tx.Nonce,
		tx.GasTipCap,
		tx.GasFeeCap,
		tx.Gas,
		tx.To,
		tx.Value,
		tx.Data,
		tx.AccessList,
		tx.V, tx.R, tx.S,
	})
	return hash
}

func (tx *DynamicFeeTx) rawSignatureValues() (v, r, s *uint256.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *DynamicFeeTx) setSignatureValues(chainID, v, r, s *uint256.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}
