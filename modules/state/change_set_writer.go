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
	"fmt"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/modules/changeset"

	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/modules"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// ChangeSetWriter is a mock StateWriter that accumulates changes in-memory into ChangeSets.
type ChangeSetWriter struct {
	db             kv.RwTx
	accountChanges map[types.Address][]byte
	storageChanged map[types.Address]bool
	storageChanges map[string][]byte
	blockNumber    uint64
}

func NewChangeSetWriter() *ChangeSetWriter {
	return &ChangeSetWriter{
		accountChanges: make(map[types.Address][]byte),
		storageChanged: make(map[types.Address]bool),
		storageChanges: make(map[string][]byte),
	}
}
func NewChangeSetWriterPlain(db kv.RwTx, blockNumber uint64) *ChangeSetWriter {
	return &ChangeSetWriter{
		db:             db,
		accountChanges: make(map[types.Address][]byte),
		storageChanged: make(map[types.Address]bool),
		storageChanges: make(map[string][]byte),
		blockNumber:    blockNumber,
	}
}

func (w *ChangeSetWriter) GetAccountChanges() (*changeset.ChangeSet, error) {
	cs := changeset.NewAccountChangeSet()
	for address, val := range w.accountChanges {
		if err := cs.Add(common.CopyBytes(address[:]), val); err != nil {
			return nil, err
		}
	}
	return cs, nil
}
func (w *ChangeSetWriter) GetStorageChanges() (*changeset.ChangeSet, error) {
	cs := changeset.NewStorageChangeSet()
	for key, val := range w.storageChanges {
		if err := cs.Add([]byte(key), val); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

func accountsEqual(a1, a2 *account.StateAccount) bool {
	if a1.Nonce != a2.Nonce {
		return false
	}
	if !a1.Initialised {
		if a2.Initialised {
			return false
		}
	} else if !a2.Initialised {
		return false
	} else if a1.Balance.Cmp(&a2.Balance) != 0 {
		return false
	}
	if a1.IsEmptyCodeHash() {
		if !a2.IsEmptyCodeHash() {
			return false
		}
	} else if a2.IsEmptyCodeHash() {
		return false
	} else if a1.CodeHash != a2.CodeHash {
		return false
	}
	return true
}

func (w *ChangeSetWriter) UpdateAccountData(address types.Address, original, account *account.StateAccount) error {
	//fmt.Printf("balance,%x,%d\n", address, &account.Balance)
	if !accountsEqual(original, account) || w.storageChanged[address] {
		w.accountChanges[address] = originalAccountData(original, true /*omitHashes*/)
	}
	return nil
}

func (w *ChangeSetWriter) UpdateAccountCode(address types.Address, incarnation uint16, codeHash types.Hash, code []byte) error {
	//fmt.Printf("code,%x,%x\n", address, code)
	return nil
}

func (w *ChangeSetWriter) DeleteAccount(address types.Address, original *account.StateAccount) error {
	//fmt.Printf("delete,%x\n", address)
	if original == nil || !original.Initialised {
		return nil
	}
	w.accountChanges[address] = originalAccountData(original, false)
	return nil
}

func (w *ChangeSetWriter) WriteAccountStorage(address types.Address, incarnation uint16, key *types.Hash, original, value *uint256.Int) error {
	//fmt.Printf("storage,%x,%x,%x\n", address, *key, value.Bytes())
	if *original == *value {
		return nil
	}

	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	w.storageChanges[string(compositeKey)] = original.Bytes()
	w.storageChanged[address] = true

	return nil
}

func (w *ChangeSetWriter) CreateContract(address types.Address) error {
	return nil
}

func (w *ChangeSetWriter) WriteChangeSets() error {
	if w.blockNumber == 547600 {
		changeset.Truncate(w.db, 547600)
	}
	accountChanges, err := w.GetAccountChanges()
	if err != nil {
		return err
	}
	if err = changeset.Mapper[modules.AccountChangeSet].Encode(w.blockNumber, accountChanges, func(k, v []byte) error {
		if err = w.db.AppendDup(modules.AccountChangeSet, k, v); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	storageChanges, err := w.GetStorageChanges()
	if err != nil {
		return err
	}
	if storageChanges.Len() == 0 {
		return nil
	}
	if err = changeset.Mapper[modules.StorageChangeSet].Encode(w.blockNumber, storageChanges, func(k, v []byte) error {
		if err = w.db.AppendDup(modules.StorageChangeSet, k, v); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (w *ChangeSetWriter) WriteHistory() error {
	accountChanges, err := w.GetAccountChanges()
	if err != nil {
		return err
	}
	err = writeIndex(w.blockNumber, accountChanges, modules.AccountsHistory, w.db)
	if err != nil {
		return err
	}

	storageChanges, err := w.GetStorageChanges()
	if err != nil {
		return err
	}
	err = writeIndex(w.blockNumber, storageChanges, modules.StorageHistory, w.db)
	if err != nil {
		return err
	}

	return nil
}

func (w *ChangeSetWriter) PrintChangedAccounts() {
	fmt.Println("Account Changes")
	for k := range w.accountChanges {
		fmt.Println(hexutil.Encode(k.Bytes()))
	}
	fmt.Println("------------------------------------------")
}
