// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package hashing

import (
	"bytes"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"sync"

	"golang.org/x/crypto/sha3"
)

// hasherPool holds LegacyKeccak256 hashers for rlpHash.
var hasherPool = sync.Pool{
	New: func() interface{} { return sha3.NewLegacyKeccak256() },
}

// encodeBufferPool holds temporary encoder buffers for DeriveSha and TX encoding.
var encodeBufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// DerivableList is the input to DeriveSha.
// It is implemented by the 'Transactions' and 'Receipts' types.
// This is internal, do not use these methods.
type DerivableList interface {
	Len() int
	EncodeIndex(int, *bytes.Buffer)
}

func encodeForDerive(list DerivableList, i int, buf *bytes.Buffer) []byte {
	buf.Reset()
	list.EncodeIndex(i, buf)
	// It's really unfortunate that we need to do perform this copy.
	// StackTrie holds onto the values until Hash is called, so the values
	// written to it must not alias.
	return types.CopyBytes(buf.Bytes())
}

// DeriveSha creates the tree hashes of transactions and receipts in a block header.
func DeriveSha(list DerivableList) (h types.Hash) {

	sha := hasherPool.Get().(crypto.KeccakState)
	defer hasherPool.Put(sha)
	sha.Reset()

	valueBuf := encodeBufferPool.Get().(*bytes.Buffer)
	defer encodeBufferPool.Put(valueBuf)

	for i := 0; i < list.Len(); i++ {
		value := encodeForDerive(list, i, valueBuf)
		sha.Write(value)
	}
	sha.Read(h[:])
	return h
}
