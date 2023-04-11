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
	"context"
	"github.com/torquem-ch/mdbx-go/mdbx"
	"sync"
)

type DBISnapshot struct {
	ctx context.Context
	txn *mdbx.Txn
	ok  bool

	lock sync.RWMutex
	once sync.Once

	dbi     map[string]mdbx.DBI
	dbiLock sync.RWMutex
}

func newSnapshot(ctx context.Context, parent *mdbx.Txn, env *mdbx.Env) (*DBISnapshot, error) {
	txn, err := env.BeginTxn(nil, 0)
	if err != nil {
		return nil, err
	}

	return &DBISnapshot{
		ctx: ctx,
		txn: txn,
		ok:  true,
		dbi: make(map[string]mdbx.DBI),
	}, nil
}

func (db *DBISnapshot) IsRunning() bool {
	return db.ok
}

func (db *DBISnapshot) Get(dbName string, key []byte) (value []byte, err error) {
	if !db.IsRunning() {
		return nil, errorSnapshotIsClose
	}
	db.lock.RLock()
	defer db.lock.RUnlock()

	dbi, err := db.openDBI(dbName)
	if err != nil {
		return nil, err
	}

	return db.txn.Get(*dbi, key)
}

//func (db *DBISnapshot) GetIterator(dbName string, key []byte) (iterator db.IIterator, err error) {
//	if !db.IsRunning() {
//		return nil, errorSnapshotIsClose
//	}
//
//	dbi, err := db.openDBI(dbName)
//	if err != nil {
//		return nil, err
//	}
//
//	return newIterator(dbi, key)
//}

func (db *DBISnapshot) Put(dbName string, key []byte, value []byte) (err error) {
	if !db.IsRunning() {
		return errorSnapshotIsClose
	}
	db.lock.RLock()
	defer db.lock.RUnlock()

	dbi, err := db.openDBI(dbName)
	if err != nil {
		return err
	}

	return db.txn.Put(*dbi, key, value, 0)
}

func (db *DBISnapshot) Commit() error {
	var err error
	db.once.Do(func() {
		if !db.IsRunning() {
			err = errorSnapshotIsClose
		}
		db.lock.Lock()
		defer db.lock.Unlock()
		db.ok = false
		_, err = db.txn.Commit()
	})

	return err
}

func (db *DBISnapshot) Rollback() {
	db.once.Do(func() {
		if db.IsRunning() {
			db.lock.Lock()
			defer db.lock.Unlock()
			db.ok = false
			db.txn.Abort()
		}
	})
}

func (db *DBISnapshot) openDBI(name string) (*mdbx.DBI, error) {
	if !db.IsRunning() {
		return nil, errorSnapshotIsClose
	}

	db.dbiLock.Lock()
	defer db.dbiLock.Unlock()

	if dbi, ok := db.dbi[name]; ok {
		return &dbi, nil
	}

	dbi, err := db.txn.OpenDBI(name, mdbx.Create, nil, nil)
	if err != nil {
		return nil, err
	}

	db.dbi[name] = dbi
	return &dbi, nil
}
