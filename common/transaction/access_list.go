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
	"github.com/amazechain/amc/common/hash"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
)

type AccessList []AccessTuple

// AccessTuple is the element type of an access list.
type AccessTuple struct {
	Address     types.Address `json:"address"        gencodec:"required"`
	StorageKeys []types.Hash  `json:"storageKeys"    gencodec:"required"`
}

// StorageKeys returns the total number of storage keys in the access list.
func (al AccessList) StorageKeys() int {
	sum := 0
	for _, tuple := range al {
		sum += len(tuple.StorageKeys)
	}
	return sum
}

type AccessListTx struct {
	ChainID    *uint256.Int   // destination chain ID
	Nonce      uint64         // nonce of sender account
	GasPrice   *uint256.Int   // wei per gas
	Gas        uint64         // gas limit
	To         *types.Address `rlp:"nil"` // nil means contract creation
	From       *types.Address `rlp:"nil"` // nil means contract creation
	Value      *uint256.Int   // wei amount
	Data       []byte         // contract invocation input data
	AccessList AccessList     // EIP-2930 access list
	Sign       []byte         // signature values
	V, R, S    *uint256.Int   // signature values
}

func (tx *AccessListTx) copy() TxData {
	cpy := &AccessListTx{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		From:  copyAddressPtr(tx.From),
		Data:  types.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are copied below.
		AccessList: make(AccessList, len(tx.AccessList)),
		Value:      new(uint256.Int),
		ChainID:    new(uint256.Int),
		GasPrice:   new(uint256.Int),
	}
	copy(cpy.AccessList, tx.AccessList)
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.GasPrice != nil {
		cpy.GasPrice.Set(tx.GasPrice)
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
func (tx *AccessListTx) txType() byte            { return AccessListTxType }
func (tx *AccessListTx) chainID() *uint256.Int   { return tx.ChainID }
func (tx *AccessListTx) accessList() AccessList  { return tx.AccessList }
func (tx *AccessListTx) data() []byte            { return tx.Data }
func (tx *AccessListTx) gas() uint64             { return tx.Gas }
func (tx *AccessListTx) gasPrice() *uint256.Int  { return tx.GasPrice }
func (tx *AccessListTx) gasTipCap() *uint256.Int { return tx.GasPrice }
func (tx *AccessListTx) gasFeeCap() *uint256.Int { return tx.GasPrice }
func (tx *AccessListTx) value() *uint256.Int     { return tx.Value }
func (tx *AccessListTx) nonce() uint64           { return tx.Nonce }
func (tx *AccessListTx) to() *types.Address      { return tx.To }
func (tx *AccessListTx) from() *types.Address    { return tx.From }
func (tx *AccessListTx) sign() []byte            { return tx.Sign }

func (tx *AccessListTx) hash() types.Hash {
	hash := hash.PrefixedRlpHash(AccessListTxType, []interface{}{
		tx.ChainID,
		tx.Nonce,
		tx.GasPrice,
		tx.Gas,
		tx.To,
		tx.Value,
		tx.Data,
		tx.AccessList,
		tx.V, tx.R, tx.S,
	})
	return hash
}

func (tx *AccessListTx) rawSignatureValues() (v, r, s *uint256.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *AccessListTx) setSignatureValues(chainID, v, r, s *uint256.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}
