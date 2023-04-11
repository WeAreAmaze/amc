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

package statedb

import (
	"bytes"
	"fmt"
	"github.com/amazechain/amc/api/protocol/state"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/utils"
	"github.com/gogo/protobuf/proto"
)

var (
	// emptyRoot todo remove
	emptyRoot     = types.BytesToHash(utils.Keccak256(nil))
	emptyCodeHash = utils.Keccak256(nil)
)

type StateAccount struct {
	Nonce    uint64
	Balance  types.Int256
	Root     types.Hash // todo remove
	CodeHash []byte
}

type Code []byte

func (c Code) String() string {
	return string(c) //strings.Join(Disassemble(c), " ")
}

type Storage map[types.Hash]types.Hash

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// stateObject
type stateObject struct {
	address  types.Address
	addrHash types.Hash
	data     StateAccount
	db       *StateDB

	// DB error.
	dbErr error

	code Code //

	dirtyStorage Storage // Storage
	fakeStorage  Storage // Fake storage which constructed by caller for debugging purpose.

	dirtyCode bool
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// newObject creates a state object.
func newObject(db *StateDB, address types.Address, data StateAccount) *stateObject {

	//if data.Balance.IsEmpty() {
	//	data.Balance = types.NewInt64(0)
	//}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	if data.Root == (types.Hash{}) {
		data.Root = emptyRoot
	}

	return &stateObject{
		address:      address,
		addrHash:     types.BytesToHash(address[:]),
		db:           db,
		data:         data,
		dirtyStorage: make(Storage),
		//fakeStorage:  make(Storage), when use SetStorage to init fakeStorage
	}
}

//// Encode protobuf.
//func (s *stateObject) Encode() ([]byte, error) {
//	msg := s.ToProtoMessage()
//	return proto.Marshal(msg)
//}

// ToProtoMessage to protobuf message
func (s *stateObject) ToProtoMessage() proto.Message {

	var bpAccount state.Account
	bpAccount.Nonce = s.data.Nonce
	bpAccount.Balance = s.data.Balance
	bpAccount.Root = s.data.Root
	bpAccount.CodeHash = s.data.CodeHash
	bpAccount.Suicided = s.suicided
	bpAccount.Code = s.code
	bpAccount.State = make([]*state.HashMap, 0, len(s.dirtyStorage))
	for k, v := range s.dirtyStorage {
		bpAccount.State = append(bpAccount.State, &state.HashMap{
			Key:   k,
			Value: v,
		})
		//log.Infof("save addr %s, state key: %s, value %s", s.address.String(), k.String(), v.String())
	}

	return &bpAccount
}

func (s *stateObject) FromProtoMessage(db *StateDB, addr types.Address, message proto.Message) error {
	var (
		account *state.Account
		ok      bool
	)

	if account, ok = message.(*state.Account); !ok {
		return fmt.Errorf("type conversion failure")
	}

	s.address = addr
	s.addrHash = types.BytesToHash(addr[:])

	s.data = StateAccount{
		Nonce:    account.Nonce,
		Balance:  account.Balance,
		Root:     account.Root,
		CodeHash: account.CodeHash,
	}
	s.db = db
	s.suicided = account.Suicided
	s.code = account.Code
	s.dirtyStorage = make(Storage)
	for i, _ := range account.State {
		s.dirtyStorage[account.State[i].Key] = account.State[i].Value
		//log.Infof("load addr %s, state key: %s, value %s", addr.String(), account.State[i].Key.String(), account.State[i].Value.String())
	}
	return nil
}

// setError remembers the first non-nil error it is called with.
func (s *stateObject) setError(err error) {
	if s.dbErr == nil {
		s.dbErr = err
	}
}

func (s *stateObject) markSuicided() {
	s.suicided = true
}

func (s *stateObject) touch() {
	s.db.journal.append(touchChange{
		account: &s.address,
	})
	if s.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		s.db.journal.dirty(s.address)
	}
}

// GetState retrieves a value from the account storage tree.
func (s *stateObject) GetState(db db.IDatabase, key types.Hash) types.Hash {
	// for debug
	if s.fakeStorage != nil {
		return s.fakeStorage[key]
	}
	// If we have a dirty value for this state entry, return it
	value, dirty := s.dirtyStorage[key]
	if dirty {
		return value
	}
	return s.GetCommittedState(db, key)
}

// GetCommittedState retrieves a value from the committed account storage tree.
func (s *stateObject) GetCommittedState(db db.IDatabase, key types.Hash) types.Hash {
	// todo
	return types.Hash{}
}

// SetState updates a value in account storage.
func (s *stateObject) SetState(db db.IDatabase, key, value types.Hash) {
	if s.fakeStorage != nil {
		s.fakeStorage[key] = value
		return
	}

	prev := s.GetState(db, key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:  &s.address,
		key:      key,
		prevalue: prev,
	})

	s.setState(key, value)
}

// SetStorage for debugging.
func (s *stateObject) SetStorage(storage map[types.Hash]types.Hash) {
	// Allocate fake storage if it's nil.
	if s.fakeStorage == nil {
		s.fakeStorage = make(Storage)
	}
	for key, value := range storage {
		s.fakeStorage[key] = value
	}
}

func (s *stateObject) setState(key, value types.Hash) {
	s.dirtyStorage[key] = value
}

// UpdateRoot sets the tree root to the current root hash of
func (s *stateObject) updateRoot(db db.IDatabase) {

}

// AddBalance add balance
func (s *stateObject) AddBalance(amount types.Int256) {
	if amount.Sign() == 0 {
		if s.empty() {
			s.touch()
		}
		return
	}
	s.SetBalance(s.Balance().Add(amount))
}

// SubBalance sub Balance
func (s *stateObject) SubBalance(amount types.Int256) {
	if amount.Sign() == 0 {
		return
	}
	s.SetBalance(s.Balance().Sub(amount))
}

func (s *stateObject) SetBalance(amount types.Int256) {
	s.db.journal.append(balanceChange{
		account: &s.address,
		prev:    s.data.Balance,
	})
	s.setBalance(amount)
}

func (s *stateObject) setBalance(amount types.Int256) {
	s.data.Balance = amount
}

func (s *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, s.address, s.data)

	stateObject.code = s.code
	stateObject.dirtyStorage = s.dirtyStorage.Copy()
	stateObject.suicided = s.suicided
	stateObject.dirtyCode = s.dirtyCode
	stateObject.deleted = s.deleted
	return stateObject
}

//
// Attribute accessors
//

// Address Returns the address of the contract/account
func (s *stateObject) Address() types.Address {
	return s.address
}

// Code
func (s *stateObject) Code(db db.IDatabase) []byte {
	if s.code != nil {
		return s.code
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return nil
	}
	return nil
}

// CodeSize
func (s *stateObject) CodeSize(db db.IDatabase) int {
	if s.code != nil {
		return len(s.code)
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return 0
	}
	return 0
}

func (s *stateObject) SetCode(codeHash types.Hash, code []byte) {
	prevcode := s.Code(s.db.db)
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash(),
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *stateObject) setCode(codeHash types.Hash, code []byte) {
	s.code = code
	s.data.CodeHash = codeHash[:]
	s.dirtyCode = true
}

func (s *stateObject) SetNonce(nonce uint64) {
	s.db.journal.append(nonceChange{
		account: &s.address,
		prev:    s.data.Nonce,
	})
	s.setNonce(nonce)
}

func (s *stateObject) setNonce(nonce uint64) {
	s.data.Nonce = nonce
}

func (s *stateObject) CodeHash() []byte {
	return s.data.CodeHash
}

func (s *stateObject) Balance() types.Int256 {
	return s.data.Balance
}

func (s *stateObject) Nonce() uint64 {
	return s.data.Nonce
}
