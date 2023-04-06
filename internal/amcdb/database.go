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

package amcdb

type DBType uint8

const (
	DBMem = iota
	DBLmdb
	DBUnknown
)

func (dt DBType) String() string {
	switch dt {
	case DBMem:
		return "memory"
	case DBLmdb:
		return "lmdb"
	default:
		return "unknown db"
	}
}

func ToDBType(name string) DBType {
	switch name {
	case "memory":
		return DBMem
	case "lmdb":
		return DBLmdb
	default:
		return DBUnknown
	}
}

//func OpenDB(ctx context.Context, nodeConfig *conf.NodeConfig, config *conf.DatabaseConfig) (ethdb.Database, error) {
//	dType := ToDBType(config.DBType)
//	switch dType {
//	case DBLmdb:
//		return lmdb.NewLMDB(ctx, nodeConfig, config)
//	case DBMem:
//		return memdb.NewMemDB(), nil
//	default:
//		return nil, fmt.Errorf("failed open db, err: invalid db type")
//	}
//}
