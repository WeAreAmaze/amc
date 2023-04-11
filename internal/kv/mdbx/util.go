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

package mdbx

import (
	"github.com/amazechain/amc/internal/kv"
	mdbxbind "github.com/torquem-ch/mdbx-go/mdbx"
)

func MustOpen(path string) kv.RwDB {
	db, err := Open(path, false)
	if err != nil {
		panic(err)
	}
	return db
}

func MustOpenRo(path string) kv.RoDB {
	db, err := Open(path, true)
	if err != nil {
		panic(err)
	}
	return db
}

// Open - main method to open database.
func Open(path string, readOnly bool) (kv.RwDB, error) {
	var db kv.RwDB
	var err error
	opts := NewMDBX().Path(path)
	if readOnly {
		opts = opts.Flags(func(flags uint) uint { return flags | mdbxbind.Readonly })
	}
	db, err = opts.Open()

	if err != nil {
		return nil, err
	}
	return db, nil
}
