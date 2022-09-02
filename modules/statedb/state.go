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
	"crypto/sha256"
	"fmt"
	"github.com/amazechain/amc/api/protocol/state"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/utils"
	"github.com/gogo/protobuf/proto"
	"sort"
)

type StateDB struct {
	db       db.IDatabase
	changeDB kv.RwDB
	root     types.Hash
	blockNr  types.Int256
	//cache
	stateObjects      map[types.Address]*stateObject
	stateObjectsDirty map[types.Address]struct{} // State objects modified in the current execution

	// Per-transaction access list
	accessList *accessList

	refund  uint64
	txHash  types.Hash
	txIndex int
	logs    map[types.Hash][]*block.Log
	logSize uint

	journal        *journal
	validRevisions []revision
	nextRevisionId int

	preimages map[types.Hash][]byte
}

func NewStateDB(root types.Hash, db db.IDatabase, changeDB kv.RwDB) *StateDB {

	blockNr, _ := rawdb.GetHashNumber(db, root)
	sdb := &StateDB{
		db:                db,
		changeDB:          changeDB,
		blockNr:           blockNr,
		root:              root,
		stateObjects:      make(map[types.Address]*stateObject),
		logs:              make(map[types.Hash][]*block.Log),
		stateObjectsDirty: make(map[types.Address]struct{}),
		preimages:         make(map[types.Hash][]byte),
		journal:           newJournal(),
		accessList:        newAccessList(),
	}

	return sdb
}

func (s *StateDB) setStateObject(object *stateObject) {
	s.stateObjects[object.Address()] = object
}

func (s *StateDB) getStateObject(addr types.Address) *stateObject {
	if obj := s.getDeletedStateObject(addr); obj != nil && !obj.deleted {
		return obj
	}
	return nil
}

func (s *StateDB) getDeletedStateObject(addr types.Address) *stateObject {
	// Prefer live objects if any is available
	if obj := s.stateObjects[addr]; obj != nil {
		return obj
	}
	obj, err := s.getAccount(addr)
	if err != nil {
		return nil
	}
	s.setStateObject(obj)
	return obj
}

func (s *StateDB) GetOrNewStateObject(addr types.Address) *stateObject {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		stateObject, _ = s.createObject(addr)
	}
	return stateObject
}

func (s *StateDB) createObject(addr types.Address) (newobj, prev *stateObject) {
	prev = s.getDeletedStateObject(addr)

	var prevdestruct bool

	newobj = newObject(s, addr, StateAccount{})
	if prev == nil {
		s.journal.append(createObjectChange{account: &addr})
	} else {
		s.journal.append(resetObjectChange{prev: prev, prevdestruct: prevdestruct})
	}
	s.setStateObject(newobj)
	if prev != nil && !prev.deleted {
		return newobj, prev
	}
	return newobj, nil
}

//IntermediateRoot root
func (s *StateDB) IntermediateRoot() types.Hash {
	return s.GenerateRootHash()
}

//Commit commit all data
func (s *StateDB) Commit(blockNr types.Int256) (root types.Hash, err error) {
	root = s.GenerateRootHash()

	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}

	for address, _ := range s.stateObjectsDirty {
		obj := s.getDeletedStateObject(address)
		//todo setAccount  batch?
		//
		err := s.setAccount(address, s.changeDB, blockNr, obj)
		if err != nil {
			return types.Hash{}, err
		}
	}
	s.clearJournalAndRefund()
	return root, nil
}

func (s *StateDB) RevertToSnapshot(revid int) {

	idx := sort.Search(len(s.validRevisions), func(i int) bool {
		return s.validRevisions[i].id >= revid
	})
	if idx == len(s.validRevisions) || s.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateDB) Snapshot() int {
	id := s.nextRevisionId
	s.nextRevisionId++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	return id
}

func (s *StateDB) clearJournalAndRefund() {
	if len(s.journal.entries) > 0 {
		s.journal = newJournal()
		s.refund = 0
	}
	s.validRevisions = s.validRevisions[:0] // Snapshots can be created without journal entires
}

func (s *StateDB) getAccount(addr types.Address) (*stateObject, error) {

	v, err := rawdb.GetAccount(s.db, s.changeDB, s.blockNr, addr)
	if err != nil {
		return nil, err
	}

	var (
		pbAccount state.Account
		obj       stateObject
	)
	if err := proto.Unmarshal(v, &pbAccount); err != nil {
		return nil, err
	}
	if err := obj.FromProtoMessage(s, addr, &pbAccount); err != nil {
		return nil, err
	}

	s.setStateObject(&obj)

	return &obj, nil
}

func (s *StateDB) setAccount(addr types.Address, changeDB kv.RwDB, blockNr types.Int256, obj *stateObject) error {
	message := obj.ToProtoMessage()
	v, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	err = rawdb.StoreAccount(s.db, changeDB, blockNr, addr, v)
	if err != nil {
		return err
	}
	return nil
}

// CreateAccount create account
func (s *StateDB) CreateAccount(addr types.Address) {
	newObj, prev := s.createObject(addr)
	if prev != nil {
		newObj.setBalance(prev.data.Balance)
	}
}

func (s *StateDB) GetNonce(addr types.Address) uint64 {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}
	return 0
}
func (s *StateDB) GetBalance(addr types.Address) types.Int256 {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return types.NewInt64(0)
}

func (s *StateDB) SetBalance(addr types.Address, amount types.Int256) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.setBalance(amount)
	}
}

func (s *StateDB) SubBalance(addr types.Address, amount types.Int256) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (s *StateDB) AddBalance(addr types.Address, amount types.Int256) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

func (s *StateDB) SetNonce(addr types.Address, nonce uint64) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.setNonce(nonce)
	}
}

func (s *StateDB) GetCodeHash(addr types.Address) types.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return types.Hash{}
	}

	var h types.Hash
	h.SetBytes(stateObject.CodeHash())
	return h
}

func (s *StateDB) GetCode(addr types.Address) []byte {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(s.db)
	}
	return nil
}

func (s *StateDB) SetCode(addr types.Address, code []byte) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(types.BytesToHash(utils.Keccak256(code)), code)
	}
}

func (s *StateDB) GetCodeSize(addr types.Address) int {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.CodeSize(s.db)
	}
	return 0
}

func (s *StateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

func (s *StateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, s.refund))
	}
	s.refund -= gas
}

func (s *StateDB) GetRefund() uint64 {
	return s.refund
}

func (s *StateDB) GetCommittedState(addr types.Address, hash types.Hash) types.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(s.db, hash)
	}
	return types.Hash{}
}

func (s *StateDB) GetState(addr types.Address, hash types.Hash) types.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(s.db, hash)
	}
	return types.Hash{}
}

func (s *StateDB) SetState(addr types.Address, key types.Hash, value types.Hash) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(s.db, key, value)
	}
}

// SetStorage replaces the entire storage for the specified account with given
// storage. This function should only be used for debugging.
func (s *StateDB) SetStorage(addr types.Address, storage map[types.Hash]types.Hash) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetStorage(storage)
	}
}

func (s *StateDB) Suicide(addr types.Address) bool {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	s.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: stateObject.Balance(),
	})

	stateObject.markSuicided()
	stateObject.data.Balance = types.NewInt64(0)
	return true
}

func (s *StateDB) HasSuicided(addr types.Address) bool {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

func (s *StateDB) Exist(addr types.Address) bool {
	return s.getStateObject(addr) != nil
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (s *StateDB) Empty(addr types.Address) bool {
	obj := s.getStateObject(addr)
	return obj == nil || obj.empty()
}

// GenerateRootHash calculate root hash
func (s *StateDB) GenerateRootHash() types.Hash {
	h := sha256.New()
	for address, _ := range s.stateObjectsDirty {
		obj, _ := s.getAccount(address)
		h.Write([]byte(obj.ToProtoMessage().String()))
	}
	return types.BytesToHash(h.Sum(nil))
}

func (s *StateDB) Prepare(thash types.Hash, ti int) {
	s.txHash = thash
	s.txIndex = ti
}

func (s *StateDB) TxIndex() int {
	return s.txIndex
}

func (s *StateDB) PrepareAccessList(sender types.Address, dst *types.Address, precompiles []types.Address, list transaction.AccessList) {
	// Clear out any leftover from previous executions
	s.accessList = newAccessList()

	s.AddAddressToAccessList(sender)
	if dst != nil {
		s.AddAddressToAccessList(*dst)
		// If it's a create-tx, the destination will be added inside evm.create
	}
	for _, addr := range precompiles {
		s.AddAddressToAccessList(addr)
	}
	for _, el := range list {
		s.AddAddressToAccessList(el.Address)
		for _, key := range el.StorageKeys {
			s.AddSlotToAccessList(el.Address, key)
		}
	}
}

func (s *StateDB) AddressInAccessList(addr types.Address) bool {
	return s.accessList.ContainsAddress(addr)
}

func (s *StateDB) SlotInAccessList(addr types.Address, slot types.Hash) (addressOk bool, slotOk bool) {
	return s.accessList.Contains(addr, slot)
}

func (s *StateDB) AddAddressToAccessList(addr types.Address) {
	if s.accessList.AddAddress(addr) {
		s.journal.append(accessListAddAccountChange{&addr})
	}
}

func (s *StateDB) AddSlotToAccessList(addr types.Address, slot types.Hash) {
	addrMod, slotMod := s.accessList.AddSlot(addr, slot)
	if addrMod {
		s.journal.append(accessListAddAccountChange{&addr})
	}
	if slotMod {
		s.journal.append(accessListAddSlotChange{
			address: &addr,
			slot:    &slot,
		})
	}
}

func (s *StateDB) AddLog(log *block.Log) {
	s.journal.append(addLogChange{txhash: s.txHash})

	log.TxHash = s.txHash
	log.TxIndex = uint(s.txIndex)
	log.Index = s.logSize
	s.logs[s.txHash] = append(s.logs[s.txHash], log)
	s.logSize++
}

func (s *StateDB) GetLogs(hash types.Hash, blockHash types.Hash) []*block.Log {
	logs := s.logs[hash]
	for _, l := range logs {
		l.BlockHash = blockHash
	}
	return logs
}
