// Copyright 2023 The AmazeChain Authors
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

package state

import (
	"encoding/binary"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

var _ WriterWithChangeSets = (*PlainStateWriter)(nil)

type putDel interface {
	kv.Putter
	kv.Deleter
}
type PlainStateWriter struct {
	db  putDel
	csw *ChangeSetWriter
	//accumulator *shards.Accumulator
}

func NewPlainStateWriter(db putDel, changeSetsDB kv.RwTx, blockNumber uint64) *PlainStateWriter {
	return &PlainStateWriter{
		db:  db,
		csw: NewChangeSetWriterPlain(changeSetsDB, blockNumber),
	}
}

func NewPlainStateWriterNoHistory(db putDel) *PlainStateWriter {
	return &PlainStateWriter{
		db: db,
	}
}

//func (w *PlainStateWriter) SetAccumulator(accumulator *shards.Accumulator) *PlainStateWriter {
//	w.accumulator = accumulator
//	return w
//}

func (w *PlainStateWriter) UpdateAccountData(address types.Address, original, account *account.StateAccount) error {
	//fmt.Printf("balance,%x,%d\n", address, &account.Balance)
	if w.csw != nil {
		if err := w.csw.UpdateAccountData(address, original, account); err != nil {
			return err
		}
	}
	value := make([]byte, account.EncodingLengthForStorage())
	account.EncodeForStorage(value)
	return w.db.Put(modules.Account, address[:], value)
}

func (w *PlainStateWriter) UpdateAccountCode(address types.Address, incarnation uint16, codeHash types.Hash, code []byte) error {
	//fmt.Printf("code,%x,%x\n", address, code)
	if w.csw != nil {
		if err := w.csw.UpdateAccountCode(address, incarnation, codeHash, code); err != nil {
			return err
		}
	}
	//if w.accumulator != nil {
	//	w.accumulator.ChangeCode(address, incarnation, code)
	//}
	if err := w.db.Put(modules.Code, codeHash[:], code); err != nil {
		return err
	}
	return w.db.Put(modules.PlainContractCode, modules.PlainGenerateStoragePrefix(address[:], incarnation), codeHash[:])
}

func (w *PlainStateWriter) DeleteAccount(address types.Address, original *account.StateAccount) error {
	//fmt.Printf("delete,%x\n", address)
	if w.csw != nil {
		if err := w.csw.DeleteAccount(address, original); err != nil {
			return err
		}
	}
	//if w.accumulator != nil {
	//	w.accumulator.DeleteAccount(address)
	//}
	if err := w.db.Delete(modules.Account, address[:]); err != nil {
		return err
	}
	if original.Incarnation > 0 {
		var b [8]byte
		binary.BigEndian.PutUint16(b[:], original.Incarnation)
		if err := w.db.Put(modules.IncarnationMap, address[:], b[:]); err != nil {
			return err
		}
	}
	return nil
}

func (w *PlainStateWriter) WriteAccountStorage(address types.Address, incarnation uint16, key *types.Hash, original, value *uint256.Int) error {
	//fmt.Printf("storage,%x,%x,%x\n", address, *key, value.Bytes())
	if w.csw != nil {
		if err := w.csw.WriteAccountStorage(address, incarnation, key, original, value); err != nil {
			return err
		}
	}
	if *original == *value {
		return nil
	}
	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	v := value.Bytes()
	//if w.accumulator != nil {
	//	w.accumulator.ChangeStorage(address, incarnation, *key, v)
	//}
	if len(v) == 0 {
		return w.db.Delete(modules.Storage, compositeKey)
	}
	return w.db.Put(modules.Storage, compositeKey, v)
}

func (w *PlainStateWriter) CreateContract(address types.Address) error {
	if w.csw != nil {
		if err := w.csw.CreateContract(address); err != nil {
			return err
		}
	}
	return nil
}

func (w *PlainStateWriter) WriteChangeSets() error {
	if w.csw != nil {
		return w.csw.WriteChangeSets()
	}

	return nil
}

func (w *PlainStateWriter) WriteHistory() error {
	if w.csw != nil {
		return w.csw.WriteHistory()
	}

	return nil
}

func (w *PlainStateWriter) ChangeSetWriter() *ChangeSetWriter {
	return w.csw
}
