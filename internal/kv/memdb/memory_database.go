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
	"context"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/internal/kv/mdbx"
	"testing"
)

func New() kv.RwDB {
	return mdbx.NewMDBX().InMem().MustOpen()
}

func NewPoolDB() kv.RwDB {
	return mdbx.NewMDBX().InMem().Label(kv.TxPoolDB).WithTablessCfg(func(_ kv.TableCfg) kv.TableCfg { return kv.TxpoolTablesCfg }).MustOpen()
}
func NewDownloaderDB() kv.RwDB {
	return mdbx.NewMDBX().InMem().Label(kv.DownloaderDB).WithTablessCfg(func(_ kv.TableCfg) kv.TableCfg { return kv.DownloaderTablesCfg }).MustOpen()
}
func NewSentryDB() kv.RwDB {
	return mdbx.NewMDBX().InMem().Label(kv.SentryDB).WithTablessCfg(func(_ kv.TableCfg) kv.TableCfg { return kv.SentryTablesCfg }).MustOpen()
}

func NewTestDB(tb testing.TB) kv.RwDB {
	tb.Helper()
	db := New()
	tb.Cleanup(db.Close)
	return db
}

func NewTestPoolDB(tb testing.TB) kv.RwDB {
	tb.Helper()
	db := NewPoolDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestDownloaderDB(tb testing.TB) kv.RwDB {
	tb.Helper()
	db := NewDownloaderDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestSentrylDB(tb testing.TB) kv.RwDB {
	tb.Helper()
	db := NewPoolDB()
	tb.Cleanup(db.Close)
	return db
}

func NewTestTx(tb testing.TB) (kv.RwDB, kv.RwTx) {
	tb.Helper()
	db := New()
	tb.Cleanup(db.Close)
	tx, err := db.BeginRw(context.Background())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(tx.Rollback)
	return db, tx
}

func NewTestPoolTx(tb testing.TB) (kv.RwDB, kv.RwTx) {
	tb.Helper()
	db := NewTestPoolDB(tb)
	tx, err := db.BeginRw(context.Background()) //nolint
	if err != nil {
		tb.Fatal(err)
	}
	if tb != nil {
		tb.Cleanup(tx.Rollback)
	}
	return db, tx
}

func NewTestSentryTx(tb testing.TB) (kv.RwDB, kv.RwTx) {
	tb.Helper()
	db := NewTestSentrylDB(tb)
	tx, err := db.BeginRw(context.Background()) //nolint
	if err != nil {
		tb.Fatal(err)
	}
	if tb != nil {
		tb.Cleanup(tx.Rollback)
	}
	return db, tx
}
