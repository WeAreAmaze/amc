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

package rawdb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/bitmapdb"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
)

// GetAccount get account
func GetAccount(db db.IDatabase, changeDB kv.RoDB, blockNr types.Int256, addr types.Address) ([]byte, error) {
	r, err := db.OpenReader(accountsDB)
	if err != nil {
		return nil, err
	}
	data, err := findIndexAndChangeSet(changeDB, blockNr, addr)
	if err != nil && !errors.Is(err, kv.ErrKeyNotFound) {
		return nil, err
	}

	if len(data) > 0 {
		return data, nil
	}

	return r.Get(addr.Bytes())
}

// StoreAccount store account
func StoreAccount(db db.IDatabase, changeDB kv.RwDB, blockNr types.Int256, addr types.Address, data []byte) error {
	w, err := db.OpenWriter(accountsDB)
	if err != nil {
		return err
	}
	err = writeIndexAndChangeSet(changeDB, blockNr, addr, data)
	if err != nil {
		return err
	}

	return w.Put(addr.Bytes(), data)
}

// writeIndex
func writeIndexAndChangeSet(changeDB kv.RwDB, blockNr types.Int256, addr types.Address, data []byte) error {
	txn, err := changeDB.BeginRw(context.Background())
	if err != nil {
		return err
	}
	defer txn.Rollback()

	// 1. put change
	value := encodeAccounts(addr, data)
	key := encodeBlockNumber(blockNr.Uint64())
	err = txn.Put(kv.AccountChangeSet, key, utils.Copy(value))
	if err != nil {
		log.Debugf("appendDup key: %x, value: %x,  err: %w", key, value, err)
		return err
	}

	//2. get bitmap
	index, err := bitmapdb.Get64(txn, kv.AccountsHistory, addr.Bytes(), 0, bitmapdb.MaxUint32)
	if err != nil {
		return fmt.Errorf("find chunk failed: %w", err)
	}
	index.Add(blockNr.Uint64())
	//3. put bitmap
	if err = bitmapdb.WalkChunkWithKeys64(addr.Bytes(), index, bitmapdb.ChunkLimit, func(chunkKey []byte, chunk *roaring64.Bitmap) error {
		buf := bytes.NewBuffer(nil)
		if _, err = chunk.WriteTo(buf); err != nil {
			return err
		}
		//log.Debugf("put bitmap key %x, bitmap: %s", chunkKey, chunk.String())
		err = txn.Put(kv.AccountsHistory, chunkKey, utils.Copy(buf.Bytes()))
		if err != nil {
			return err
		}
		txn.Commit()
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// findIndexAndChangeSet
func findIndexAndChangeSet(changeDB kv.RoDB, blockNr types.Int256, addr types.Address) ([]byte, error) {

	var (
		searchKey        []byte
		findKey          []byte
		findValue        []byte
		seekErr          error
		bitmapIndex      *roaring64.Bitmap
		bitmapFound      uint64
		bitmapFoundOK    bool
		findChangeSetKey []byte
	)

	defer func() {
		//log.Tracef("search key: %x, find key: %x, find value %x", searchKey, findKey, findValue)
		if bitmapFoundOK {
			log.Tracef("find bitmap: %s, len: %d", bitmapIndex.String(), bitmapIndex.GetCardinality())
			log.Tracef("find changeSet value prefix:%x, for addr %x", findChangeSetKey, addr.Bytes())
		}
	}()

	txn, _ := changeDB.BeginRo(context.Background())
	defer txn.Rollback()

	changesC, _ := txn.CursorDupSort(kv.AccountChangeSet)
	historyC, _ := txn.Cursor(kv.AccountsHistory)

	defer changesC.Close()
	defer historyC.Close()

	searchKey = accountIndexChunkKey(addr.Bytes(), blockNr.Uint64())
	findKey, findValue, seekErr = historyC.Seek(searchKey)

	if seekErr != nil {
		return nil, seekErr
	}
	if findKey == nil {
		return nil, kv.ErrKeyNotFound
	}
	if !bytes.HasPrefix(findKey, addr.Bytes()) {
		return nil, kv.ErrKeyNotFound
	}

	bitmapIndex = roaring64.New()
	if _, err := bitmapIndex.ReadFrom(bytes.NewReader(findValue)); err != nil {
		return nil, err
	}

	bitmapFound, bitmapFoundOK = bitmapdb.SeekInBitmap64(bitmapIndex, blockNr.Uint64())
	changeSetBlock := bitmapFound
	if bitmapFoundOK {
		data, err := changesC.(kv.CursorDupSort).SeekBothRange(encodeBlockNumber(changeSetBlock), addr.Bytes())
		if err != nil {
			return nil, err
		}
		_, findChangeSetKey, data, err = decodeAccounts(findKey, data)
		if err != nil {
			return nil, err
		}

		if !bytes.HasPrefix(findChangeSetKey, addr.Bytes()) {
			return nil, nil
		}
		return data, nil
	} else {
		return nil, kv.ErrKeyNotFound
	}
}

func encodeAccounts(address types.Address, data []byte) []byte {
	addr := address.Bytes()
	newV := make([]byte, len(addr)+len(data))
	copy(newV, addr)
	copy(newV[len(addr):], data)
	return newV
}

func decodeAccounts(dbKey, dbValue []byte) (uint64, []byte, []byte, error) {
	blockN := binary.BigEndian.Uint64(dbKey)
	if len(dbValue) < types.AddressLength {
		return 0, nil, nil, fmt.Errorf("account changes purged for block %d", blockN)
	}
	k := dbValue[:types.AddressLength]
	v := dbValue[types.AddressLength:]
	return blockN, k, v, nil
}

func accountIndexChunkKey(key []byte, blockNumber uint64) []byte {
	blockNumBytes := make([]byte, types.AddressLength+8)
	copy(blockNumBytes, key)
	binary.BigEndian.PutUint64(blockNumBytes[types.AddressLength:], blockNumber)
	return blockNumBytes
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}
