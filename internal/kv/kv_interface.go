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

package kv

import (
	"context"
	"errors"
)

const ReadersLimit = 32000 // MDBX_READERS_LIMIT=32767

var (
	ErrAttemptToDeleteNonDeprecatedBucket = errors.New("only buckets from dbutils.ChaindataDeprecatedTables can be deleted")
	ErrUnknownBucket                      = errors.New("unknown bucket. add it to dbutils.ChaindataTables")
)

type DBVerbosityLvl int8
type Label uint8

const (
	ChainDB      Label = 0
	TxPoolDB     Label = 1
	SentryDB     Label = 2
	ConsensusDB  Label = 3
	DownloaderDB Label = 4
)

func (l Label) String() string {
	switch l {
	case ChainDB:
		return "chaindata"
	case TxPoolDB:
		return "txpool"
	case SentryDB:
		return "sentry"
	case ConsensusDB:
		return "consensus"
	case DownloaderDB:
		return "downloader"
	default:
		return "unknown"
	}
}

type Has interface {
	// Has indicates whether a key exists in the database.
	Has(bucket string, key []byte) (bool, error)
}
type GetPut interface {
	Getter
	Putter
}
type Getter interface {
	Has

	GetOne(bucket string, key []byte) (val []byte, err error)

	// ForEach iterates over entries with keys greater or equal to fromPrefix.
	// walker is called for each eligible entry.
	// If walker returns an error:
	//   - implementations of local db - stop
	//   - implementations of remote db - do not handle this error and may finish (send all entries to client) before error happen.
	ForEach(bucket string, fromPrefix []byte, walker func(k, v []byte) error) error
	ForPrefix(bucket string, prefix []byte, walker func(k, v []byte) error) error
	ForAmount(bucket string, prefix []byte, amount uint32, walker func(k, v []byte) error) error
}

// Putter wraps the database write operations.
type Putter interface {
	// Put inserts or updates a single entry.
	Put(table string, k, v []byte) error
}

// Deleter wraps the database delete operations.
type Deleter interface {
	// Delete removes a single entry.
	Delete(table string, k []byte) error
}

type Closer interface {
	Close()
}

// RoDB - Read-only version of KV.
type RoDB interface {
	Closer

	View(ctx context.Context, f func(tx Tx) error) error

	// BeginRo - creates transaction
	// 	tx may be discarded by .Rollback() method
	//
	// A transaction and its cursors must only be used by a single
	// 	thread (not goroutine), and a thread may only have a single transaction at a time.
	//  It happen automatically by - because this method calls runtime.LockOSThread() inside (Rollback/Commit releases it)
	//  By this reason application code can't call runtime.UnlockOSThread() - it leads to undefined behavior.
	//
	// If this `parent` is non-NULL, the new transaction
	//	will be a nested transaction, with the transaction indicated by parent
	//	as its parent. Transactions may be nested to any level. A parent
	//	transaction and its cursors may not issue any other operations than
	//	Commit and Rollback while it has active child transactions.
	BeginRo(ctx context.Context) (Tx, error)
	AllBuckets() TableCfg
	PageSize() uint64
}

// RwDB low-level database interface - main target is - to provide common abstraction over top of MDBX and RemoteKV.
//
// Common pattern for short-living transactions:
//
//	 if err := db.View(ctx, func(tx ethdb.Tx) error {
//	    ... code which uses database in transaction
//	 }); err != nil {
//			return err
//	}
//
// Common pattern for long-living transactions:
//
//	tx, err := db.Begin()
//	if err != nil {
//		return err
//	}
//	defer tx.Rollback()
//
//	... code which uses database in transaction
//
//	err := tx.Commit()
//	if err != nil {
//		return err
//	}
type RwDB interface {
	RoDB

	Update(ctx context.Context, f func(tx RwTx) error) error

	BeginRw(ctx context.Context) (RwTx, error)
}

type StatelessReadTx interface {
	Getter

	Commit() error // Commit all the operations of a transaction into the database.
	Rollback()     // Rollback - abandon all the operations of the transaction instead of saving them.

	// ReadSequence - allows to create a linear sequence of unique positive integers for each table.
	// Can be called for a read transaction to retrieve the current sequence value, and the increment must be zero.
	// Sequence changes become visible outside the current write transaction after it is committed, and discarded on abort.
	// Starts from 0.
	ReadSequence(bucket string) (uint64, error)

	BucketSize(bucket string) (uint64, error)
}

type StatelessWriteTx interface {
	Putter
	Deleter

	IncrementSequence(bucket string, amount uint64) (uint64, error)
	Append(bucket string, k, v []byte) error
	AppendDup(bucket string, k, v []byte) error
}

type StatelessRwTx interface {
	StatelessReadTx
	StatelessWriteTx
}

type Tx interface {
	StatelessReadTx

	// ID returns the identifier associated with this transaction. For a
	// read-only transaction, this corresponds to the snapshot being read;
	// concurrent readers will frequently have the same transaction ID.
	ViewID() uint64

	// Cursor - creates cursor object on top of given bucket. Type of cursor - depends on bucket configuration.
	// If bucket was created with mdbx.DupSort flag, then cursor with interface CursorDupSort created
	// Otherwise - object of interface Cursor created
	//
	// Cursor, also provides a grain of magic - it can use a declarative configuration - and automatically break
	// long keys into DupSort key/values. See docs for `bucket.go:TableCfgItem`
	Cursor(bucket string) (Cursor, error)
	CursorDupSort(bucket string) (CursorDupSort, error) // CursorDupSort - can be used if bucket has mdbx.DupSort flag

	ForEach(bucket string, fromPrefix []byte, walker func(k, v []byte) error) error
	ForPrefix(bucket string, prefix []byte, walker func(k, v []byte) error) error
	ForAmount(bucket string, prefix []byte, amount uint32, walker func(k, v []byte) error) error

	DBSize() (uint64, error)
}

type RwTx interface {
	Tx
	StatelessWriteTx
	BucketMigrator

	RwCursor(bucket string) (RwCursor, error)
	RwCursorDupSort(bucket string) (RwCursorDupSort, error)

	// CollectMetrics - does collect all DB-related and Tx-related metrics
	// this method exists only in RwTx to avoid concurrency
	CollectMetrics()
	Reset() error
}

// BucketMigrator used for buckets migration, don't use it in usual app code
type BucketMigrator interface {
	DropBucket(string) error
	CreateBucket(string) error
	ExistsBucket(string) (bool, error)
	ClearBucket(string) error
	ListBuckets() ([]string, error)
}

// Cursor - class for navigating through a database
// CursorDupSort are inherit this class
//
// If methods (like First/Next/Seek) return error, then returned key SHOULD not be nil (can be []byte{} for example).
// Then looping code will look as:
// c := kv.Cursor(bucketName)
//
//	for k, v, err := c.First(); k != nil; k, v, err = c.Next() {
//	   if err != nil {
//	       return err
//	   }
//	   ... logic
//	}
type Cursor interface {
	First() ([]byte, []byte, error)               // First - position at first key/data item
	Seek(seek []byte) ([]byte, []byte, error)     // Seek - position at first key greater than or equal to specified key
	SeekExact(key []byte) ([]byte, []byte, error) // SeekExact - position at exact matching key if exists
	Next() ([]byte, []byte, error)                // Next - position at next key/value (can iterate over DupSort key/values automatically)
	Prev() ([]byte, []byte, error)                // Prev - position at previous key
	Last() ([]byte, []byte, error)                // Last - position at last key and last possible value
	Current() ([]byte, []byte, error)             // Current - return key/data at current cursor position

	Count() (uint64, error) // Count - fast way to calculate amount of keys in bucket. It counts all keys even if Prefix was set.

	Close()
}

type RwCursor interface {
	Cursor

	Put(k, v []byte) error           // Put - based on order
	Append(k []byte, v []byte) error // Append - append the given key/data pair to the end of the database. This option allows fast bulk loading when keys are already known to be in the correct order.
	Delete(k []byte) error           // Delete - short version of SeekExact+DeleteCurrent or SeekBothExact+DeleteCurrent

	// DeleteCurrent This function deletes the key/data pair to which the cursor refers.
	// This does not invalidate the cursor, so operations such as MDB_NEXT
	// can still be used on it.
	// Both MDB_NEXT and MDB_GET_CURRENT will return the same record after
	// this operation.
	DeleteCurrent() error
}

type CursorDupSort interface {
	Cursor

	// SeekBothExact -
	// second parameter can be nil only if searched key has no duplicates, or return error
	SeekBothExact(key, value []byte) ([]byte, []byte, error)
	SeekBothRange(key, value []byte) ([]byte, error) // SeekBothRange - exact match of the key, but range match of the value
	FirstDup() ([]byte, error)                       // FirstDup - position at first data item of current key
	NextDup() ([]byte, []byte, error)                // NextDup - position at next data item of current key
	NextNoDup() ([]byte, []byte, error)              // NextNoDup - position at first data item of next key
	LastDup() ([]byte, error)                        // LastDup - position at last data item of current key

	CountDuplicates() (uint64, error) // CountDuplicates - number of duplicates for the current key
}

type RwCursorDupSort interface {
	CursorDupSort
	RwCursor

	PutNoDupData(key, value []byte) error // PutNoDupData - inserts key without dupsort
	DeleteCurrentDuplicates() error       // DeleteCurrentDuplicates - deletes all of the data items for the current key
	DeleteExact(k1, k2 []byte) error      // DeleteExact - delete 1 value from given key
	AppendDup(key, value []byte) error    // AppendDup - same as Append, but for sorted dup data
}

var ErrNotSupported = errors.New("not supported")
