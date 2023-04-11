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

package db

type IDBReader interface {
	Get(key []byte) (value []byte, err error)
	Gets(key []byte, count uint) (keys [][]byte, value [][]byte, err error)
	GetIterator(key []byte) (iterator IIterator, err error)
}

type IDBWriter interface {
	Put(key []byte, value []byte) (err error)
	Puts(keys [][]byte, values [][]byte) (err error)
	Delete(key []byte) (err error)
	Drop() (err error)
}

/*
	IIterator mdbx must close
*/
type IIterator interface {
	Next() (err error)
	Prev() (err error)
	Key() (key []byte, err error)
	Value() (value []byte, err error)
	Close()
}

type IDBReaderWriter interface {
	IDBReader
	IDBWriter
}

type IDatabaseReader interface {
	IDBReader
}

type IDatabaseWriter interface {
	IDBWriter
}

type IDatabaseWriterReader interface {
	IDBReaderWriter
}

type IDatabase interface {
	OpenReader(dbName string) (reader IDatabaseReader, err error)
	OpenWriter(dbName string) (writer IDatabaseWriter, err error)
	Open(dbName string) (rw IDatabaseWriterReader, err error)
	Snapshot() (ISnapshot, error)
	Close() (err error)
}

type ISnapshot interface {
	Put(dbName string, key []byte, value []byte) (err error)
	Get(dbName string, key []byte) (value []byte, err error)
	Commit() error
	Rollback()
}
