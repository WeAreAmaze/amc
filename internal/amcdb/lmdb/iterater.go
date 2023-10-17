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

package lmdb

import (
	"github.com/amazechain/amc/common/db"
	"github.com/erigontech/mdbx-go/mdbx"
	"runtime"
)

/*
 */
type Iterator struct {
	*mdbx.Cursor
	txn *mdbx.Txn

	key   []byte
	value []byte
	err   error
}

func newIterator(dbi *DBI, key []byte) (db.IIterator, error) {
	runtime.LockOSThread()
	txn, err := dbi.env.BeginTxn(nil, 0)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	cur, err := txn.OpenCursor(*dbi.DBI)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}

	var (
		k, v []byte
	)

	if key != nil {
		k, v, err = cur.Get(key, nil, mdbx.SetKey)
	} else {
		k, v, err = cur.Get(nil, nil, mdbx.First)
	}

	if err != nil {
		runtime.UnlockOSThread()
		cur.Close()
		txn.Abort()
		return nil, err
	}

	it := Iterator{
		Cursor: cur,
		txn:    txn,
		key:    k,
		value:  v,
		err:    nil,
	}
	return &it, nil
}

func (it *Iterator) Next() error {
	it.key, it.value, it.err = it.Cursor.Get(nil, nil, mdbx.Next)
	return it.err
}

func (it *Iterator) Key() ([]byte, error) {
	return it.key, it.err
}

func (it *Iterator) Prev() error {
	it.key, it.value, it.err = it.Cursor.Get(nil, nil, mdbx.Prev)
	return it.err
}

func (it *Iterator) Value() ([]byte, error) {
	return it.value, it.err
}
func (it *Iterator) Close() {
	it.Cursor.Close()
	it.txn.Abort()
	runtime.UnlockOSThread()
}
