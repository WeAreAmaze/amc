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
	"fmt"
	"github.com/amazechain/amc/common/db"
	"github.com/erigontech/mdbx-go/mdbx"
	"sync"
)

type DBI struct {
	*mdbx.DBI
	env    *mdbx.Env
	ctx    context.Context
	cancel context.CancelFunc

	lock sync.RWMutex
	name string

	//log *zap.SugaredLogger
}

func newDBI(ctx context.Context, env *mdbx.Env, name string) (*DBI, error) {
	var (
		mdb mdbx.DBI
		err error
	)
	err = env.Update(func(txn *mdbx.Txn) error {
		mdb, err = txn.OpenDBI(name, mdbx.Create, nil, nil)
		return err
	})

	if err != nil {
		return nil, err
	}
	c, cancel := context.WithCancel(ctx)
	dbi := DBI{
		DBI:    &mdb,
		env:    env,
		ctx:    c,
		cancel: cancel,
		name:   name,
	}

	return &dbi, nil
}

func (db *DBI) Get(key []byte) (value []byte, err error) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	err = db.env.View(func(txn *mdbx.Txn) error {
		value, err = txn.Get(*db.DBI, key)
		return err
	})

	return value, err
}

func (db *DBI) Gets(key []byte, count uint) (keys [][]byte, values [][]byte, err error) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	err = db.env.View(func(txn *mdbx.Txn) error {
		cur, err := txn.OpenCursor(*db.DBI)
		if err != nil {
			return err
		}
		defer cur.Close()

		k, v, err := cur.Get(key, nil, mdbx.Set) //SetRange
		if err != nil {
			return err
		}

		keys = append(keys, k)
		values = append(values, v)
		for {
			k, v, err = cur.Get(nil, nil, mdbx.Next)
			if mdbx.IsNotFound(err) {
				break
			}
			if err != nil {
				return err
			}

			keys = append(keys, k)
			values = append(values, v)
			if len(keys) >= int(count) {
				break
			}
		}

		return nil
	})

	return keys, values, err
}

func (db *DBI) GetIterator(key []byte) (iterator db.IIterator, err error) {
	//panic(fmt.Errorf("don`t support Iterator current"))
	return newIterator(db, key)
}

func (db *DBI) Put(key []byte, value []byte) (err error) {
	err = db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Put(*db.DBI, key, value, 0) //mdbx.NoOverwrite
	})

	return err
}

func (db *DBI) Puts(keys [][]byte, values [][]byte) (err error) {
	db.lock.Lock()
	defer db.lock.Unlock()
	if len(keys) != len(values) {
		return fmt.Errorf("failed the number of keys and values is inconsistent")
	}
	err = db.env.Update(func(txn *mdbx.Txn) error {
		for i := 0; i < len(keys); i++ {
			err = txn.Put(*db.DBI, keys[i], values[i], 0)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (db *DBI) Drop() (err error) {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Drop(*db.DBI, true)
	})
}

func (db *DBI) Delete(key []byte) (err error) {
	db.lock.Lock()
	defer db.lock.Unlock()
	return db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Del(*db.DBI, key, nil)
	})
}
