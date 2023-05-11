// Copyright 2023 The AmazeChain Authors
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

package modules

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/types"
)

const NumberLength = 8
const Incarnation = 2

var configPrefix = []byte("chainConfig-")

// EncodeBlockNumber encodes a block number as big endian uint64
func EncodeBlockNumber(number uint64) []byte {
	enc := make([]byte, NumberLength)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

var ErrInvalidSize = errors.New("bit endian number has an invalid size")

func DecodeBlockNumber(number []byte) (uint64, error) {
	if len(number) != NumberLength {
		return 0, fmt.Errorf("%w: %d", ErrInvalidSize, len(number))
	}
	return binary.BigEndian.Uint64(number), nil
}

// HeaderKey = num (uint64 big endian) + hash
func HeaderKey(number uint64, hash types.Hash) []byte {
	k := make([]byte, NumberLength+types.HashLength)
	binary.BigEndian.PutUint64(k, number)
	copy(k[NumberLength:], hash[:])
	return k
}

// BlockBodyKey = num (uint64 big endian) + hash
func BlockBodyKey(number uint64, hash types.Hash) []byte {
	k := make([]byte, NumberLength+types.HashLength)
	binary.BigEndian.PutUint64(k, number)
	copy(k[NumberLength:], hash[:])
	return k
}

func BodyStorageValue(baseTx uint64, txAmount uint32) []byte {
	v := make([]byte, 8+4)
	binary.BigEndian.PutUint64(v, baseTx)
	binary.BigEndian.PutUint32(v[8:], txAmount)
	return v
}

// LogKey = blockN (uint64 big endian) + txId (uint32 big endian)
func LogKey(blockNumber uint64, txId uint32) []byte {
	newK := make([]byte, 8+4)
	binary.BigEndian.PutUint64(newK, blockNumber)
	binary.BigEndian.PutUint32(newK[8:], txId)
	return newK
}

func PlainParseCompositeStorageKey(compositeKey []byte) (types.Address, uint16, types.Hash) {
	prefixLen := types.AddressLength + types.IncarnationLength
	addr, inc := PlainParseStoragePrefix(compositeKey[:prefixLen])
	var key types.Hash
	copy(key[:], compositeKey[prefixLen:prefixLen+types.HashLength])
	return addr, inc, key
}

// AddrHash + incarnation + StorageHashPrefix
func GenerateCompositeStoragePrefix(addressHash []byte, incarnation uint16, storageHashPrefix []byte) []byte {
	key := make([]byte, types.HashLength+types.IncarnationLength+len(storageHashPrefix))
	copy(key, addressHash)
	binary.BigEndian.PutUint16(key[types.HashLength:], incarnation)
	copy(key[types.HashLength+types.IncarnationLength:], storageHashPrefix)
	return key
}

// address hash + incarnation prefix
func GenerateStoragePrefix(addressHash []byte, incarnation uint16) []byte {
	prefix := make([]byte, types.HashLength+NumberLength)
	copy(prefix, addressHash)
	binary.BigEndian.PutUint16(prefix[types.HashLength:], incarnation)
	return prefix
}

// address hash + incarnation prefix (for plain state)
func PlainGenerateStoragePrefix(address []byte, incarnation uint16) []byte {
	prefix := make([]byte, types.AddressLength+NumberLength)
	copy(prefix, address)
	binary.BigEndian.PutUint16(prefix[types.AddressLength:], incarnation)
	return prefix
}

// AddrHash + incarnation + KeyHash
// For contract storage (for plain state)
func PlainGenerateCompositeStorageKey(address []byte, incarnation uint16, key []byte) []byte {
	compositeKey := make([]byte, types.AddressLength+types.IncarnationLength+types.HashLength)
	copy(compositeKey, address)
	binary.BigEndian.PutUint16(compositeKey[types.AddressLength:], incarnation)
	copy(compositeKey[types.AddressLength+types.IncarnationLength:], key)
	return compositeKey
}

func StorageIndexChunkKey(key []byte, blockNumber uint64) []byte {
	//remove incarnation and add block number
	blockNumBytes := make([]byte, types.AddressLength+types.HashLength+8)
	copy(blockNumBytes, key[:types.AddressLength])
	copy(blockNumBytes[types.AddressLength:], key[types.AddressLength+types.IncarnationLength:])
	binary.BigEndian.PutUint64(blockNumBytes[types.AddressLength+types.HashLength:], blockNumber)

	return blockNumBytes
}

func AccountIndexChunkKey(key []byte, blockNumber uint64) []byte {
	blockNumBytes := make([]byte, types.AddressLength+8)
	copy(blockNumBytes, key)
	binary.BigEndian.PutUint64(blockNumBytes[types.AddressLength:], blockNumber)

	return blockNumBytes
}

func PlainParseStoragePrefix(prefix []byte) (types.Address, uint16) {
	var addr types.Address
	copy(addr[:], prefix[:types.AddressLength])
	inc := binary.BigEndian.Uint16(prefix[types.AddressLength : types.AddressLength+types.IncarnationLength])
	return addr, inc
}

func CompositeKeyWithoutIncarnation(key []byte) []byte {
	if len(key) == types.HashLength*2+types.IncarnationLength {
		kk := make([]byte, types.HashLength*2)
		copy(kk, key[:types.HashLength])
		copy(kk[types.HashLength:], key[types.HashLength+types.IncarnationLength:])
		return kk
	}
	if len(key) == types.AddressLength+types.HashLength+types.IncarnationLength {
		kk := make([]byte, types.AddressLength+types.HashLength)
		copy(kk, key[:types.AddressLength])
		copy(kk[types.AddressLength:], key[types.AddressLength+types.IncarnationLength:])
		return kk
	}
	return key
}

// ConfigKey = configPrefix + hash
func ConfigKey(hash types.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}
