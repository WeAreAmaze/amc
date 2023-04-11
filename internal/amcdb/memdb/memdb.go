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

package memdb

import (
	"fmt"
	"github.com/amazechain/amc/common/db"
	"sync"
)

type MemoryDB struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func (m *MemoryDB) Snapshot() (db.ISnapshot, error) {
	//TODO implement me
	panic("implement me")
}

func NewMemDB() db.IDatabase {
	return &MemoryDB{
		db: make(map[string][]byte),
	}
}

func (m *MemoryDB) OpenReader(dbName string) (db.IDatabaseReader, error) {
	return m, nil
}

func (m *MemoryDB) OpenWriter(dbName string) (db.IDatabaseWriter, error) {
	return nil, nil
}

func (m *MemoryDB) Open(dbName string) (db.IDatabaseWriterReader, error) {
	return nil, nil
}

func (m *MemoryDB) Close() error {
	return nil
}

func (m *MemoryDB) Get(key []byte) ([]byte, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if _, ok := m.db[string(key)]; ok {
		v := m.db[string(key)]
		return v, nil
	}

	return nil, fmt.Errorf("invalid key")
}

func (m *MemoryDB) Gets(key []byte, count uint) ([][]byte, [][]byte, error) {
	return nil, nil, nil
}

func (m *MemoryDB) GetIterator(key []byte) (db.IIterator, error) {
	return nil, nil
}

/*
type IDBWriter interface {
	Put(key []byte, value []byte) (err error)
	Puts(keys [][]byte, values [][]byte) (err error)
	Delete(key []byte) (err error)
	Drop() (err error)
}
*/

func (m *MemoryDB) Put(key []byte, value []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.db[string(key)]; ok {
		return fmt.Errorf("already add")
	}

	m.db[string(key)] = value

	return nil
}

func (m *MemoryDB) Puts(keys [][]byte, values [][]byte) error {
	for i := 0; i < len(keys); i++ {
		if err := m.Put(keys[i], values[i]); err != nil {
			return err
		}
	}

	return nil
}

func (m *MemoryDB) Delete(key []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.db[string(key)]; ok {
		delete(m.db, string(key))
		return nil
	}

	return fmt.Errorf("invalid key")
}

func (m *MemoryDB) Drop() error {
	return nil
}
