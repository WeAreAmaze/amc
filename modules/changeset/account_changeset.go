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

package changeset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"sort"

	"github.com/ledgerwatch/erigon-lib/common/length"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type Encoder func(blockN uint64, s *ChangeSet, f func(k, v []byte) error) error
type Decoder func(dbKey, dbValue []byte) (blockN uint64, k, v []byte, err error)

func NewAccountChangeSet() *ChangeSet {
	return &ChangeSet{
		Changes: make([]Change, 0),
		keyLen:  length.Addr,
	}
}

func EncodeAccounts(blockN uint64, s *ChangeSet, f func(k, v []byte) error) error {
	sort.Sort(s)
	newK := modules.EncodeBlockNumber(blockN)
	for _, cs := range s.Changes {
		newV := make([]byte, len(cs.Key)+len(cs.Value))
		copy(newV, cs.Key)
		copy(newV[len(cs.Key):], cs.Value)
		if err := f(newK, newV); err != nil {
			return err
		}
	}
	return nil
}

func DecodeAccounts(dbKey, dbValue []byte) (uint64, []byte, []byte, error) {
	blockN := binary.BigEndian.Uint64(dbKey)
	if len(dbValue) < length.Addr {
		return 0, nil, nil, fmt.Errorf("account changes purged for block %d", blockN)
	}
	k := dbValue[:length.Addr]
	v := dbValue[length.Addr:]
	return blockN, k, v, nil
}

func FindAccount(c kv.CursorDupSort, blockNumber uint64, key []byte) ([]byte, error) {
	k := modules.EncodeBlockNumber(blockNumber)
	v, err := c.SeekBothRange(k, key)
	if err != nil {
		return nil, err
	}
	_, k, v, err = DecodeAccounts(k, v)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(k, key) {
		return nil, nil
	}
	return v, nil
}

// GetModifiedAccounts returns a list of addresses that were modified in the block range
// [startNum:endNum)
func GetModifiedAccounts(db kv.Tx, startNum, endNum uint64) ([]types.Address, error) {
	changedAddrs := make(map[types.Address]struct{})
	if err := ForRange(db, modules.AccountChangeSet, startNum, endNum, func(blockN uint64, k, v []byte) error {
		changedAddrs[types.BytesToAddress(k)] = struct{}{}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(changedAddrs) == 0 {
		return nil, nil
	}

	idx := 0
	result := make([]types.Address, len(changedAddrs))
	for addr := range changedAddrs {
		copy(result[idx][:], addr[:])
		idx++
	}

	return result, nil
}
