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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"fmt"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/hash"

	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/amazechain/amc/log"
	"golang.org/x/crypto/sha3"
	"sort"
	"unsafe"

	"github.com/holiman/uint256"

	"github.com/amazechain/amc/params"
)

type revision struct {
	id           int
	journalIndex int
}

type StateTracer interface {
	CaptureAccountRead(account types.Address) error
	CaptureAccountWrite(account types.Address) error
}

// SystemAddress - sender address for internal state updates.
var SystemAddress = types.HexToAddress("0xfffffffffffffffffffffffffffffffffffffffe")

// BalanceIncrease represents the increase of balance of an account that did not require
// reading the account first
type BalanceIncrease struct {
	increase    uint256.Int
	transferred bool // Set to true when the corresponding stateObject is created and balance increase is transferred to the stateObject
	count       int  // Number of increases - this needs tracking for proper reversion
}

// IntraBlockState is responsible for caching and managing state changes
// that occur during block's execution.
// NOT THREAD SAFE!
type IntraBlockState struct {
	stateReader StateReader

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[types.Address]*stateObject
	stateObjectsDirty map[types.Address]struct{}

	nilAccounts map[types.Address]struct{} // Remember non-existent account to avoid reading them again

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by IntraBlockState.Commit.
	savedErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash types.Hash
	txIndex      int
	sender       types.Address
	logs         map[types.Hash][]*block.Log
	logSize      uint

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionID int
	tracer         StateTracer
	trace          bool
	accessList     *accessList
	balanceInc     map[types.Address]*BalanceIncrease // Map of balance increases (without first reading the account)

	snap    *Snapshot
	codeMap map[types.Hash][]byte
	height  uint64
}

// Create a new state from a given trie
func New(stateReader StateReader) *IntraBlockState {
	return &IntraBlockState{
		stateReader:       stateReader,
		stateObjects:      map[types.Address]*stateObject{},
		stateObjectsDirty: map[types.Address]struct{}{},
		nilAccounts:       map[types.Address]struct{}{},
		logs:              map[types.Hash][]*block.Log{},
		journal:           newJournal(),
		accessList:        newAccessList(),
		balanceInc:        map[types.Address]*BalanceIncrease{},
	}
}

func (sdb *IntraBlockState) Sender() types.Address {
	return sdb.sender
}

func (sdb *IntraBlockState) BeginWriteSnapshot() {
	sdb.snap = NewWritableSnapshot()
}

func (sdb *IntraBlockState) BeginWriteCodes() {
	sdb.codeMap = make(map[types.Hash][]byte, 0)
}

func (sdb *IntraBlockState) CodeHashes() map[types.Hash][]byte {
	// it should be called end of block execute
	for addr, stateObject := range sdb.stateObjects {
		_, isDirty := sdb.stateObjectsDirty[addr]
		if isDirty && (stateObject.created || !stateObject.selfdestructed) && stateObject.code != nil && stateObject.dirtyCode {
			h := types.Hash{}
			h.SetBytes(stateObject.CodeHash())
			sdb.codeMap[h] = stateObject.code
		}
	}
	return sdb.codeMap
}

func (sdb *IntraBlockState) SetHeight(height uint64) {
	sdb.height = height
}

func (sdb *IntraBlockState) Snap() *Snapshot {
	return sdb.snap
}

func (sdb *IntraBlockState) SetOutHash(hash types.Hash) {
	if sdb.snap == nil || !sdb.snap.CanWrite() {
		return
	}
	sdb.snap.OutHash = hash
}

func (sdb *IntraBlockState) WrittenSnapshot(hash types.Hash) []byte {
	if sdb.snap == nil || !sdb.snap.CanWrite() {
		return nil
	}
	if sdb.snap.Items != nil {
		sort.Sort(sdb.snap.Items)
	}
	sdb.snap.OutHash = hash
	b, err := rlp.EncodeToBytes(sdb.snap)
	if err != nil {
		return nil
	}
	sdb.snap = nil
	return b
}

func (s *IntraBlockState) SetGetOneFun(f1 GetOneFun) {
	s.snap.SetGetFun(f1)
}

func (sdb *IntraBlockState) PrepareReadableSnapshot(data []byte) error {
	snap, err := ReadSnapshotData(data)
	if err != nil {
		return err
	}
	sdb.snap = snap
	return nil
}

func (sdb *IntraBlockState) SetSnapshot(snap *Snapshot) {
	snap.accounts = make(map[string]int, len(snap.Items))
	snap.storage = make(map[string]int, len(snap.Items))
	for k, v := range snap.Items {
		if len(v.Key) == types.AddressLength {
			snap.accounts[*(*string)(unsafe.Pointer(&v.Key))] = k
		} else {
			snap.storage[*(*string)(unsafe.Pointer(&v.Key))] = k
		}
	}
	sdb.snap = snap
}

func (sdb *IntraBlockState) SetTracer(tracer StateTracer) {
	sdb.tracer = tracer
}

func (sdb *IntraBlockState) SetTrace(trace bool) {
	sdb.trace = trace
}

// setErrorUnsafe sets error but should be called in medhods that already have locks
func (sdb *IntraBlockState) setErrorUnsafe(err error) {
	if sdb.savedErr == nil {
		sdb.savedErr = err
	}
}

func (sdb *IntraBlockState) Error() error {
	return sdb.savedErr
}

func (sdb *IntraBlockState) SetStateReader(reader StateReader) {
	sdb.stateReader = reader
}

func (sdb *IntraBlockState) GetStateReader() StateReader {
	return sdb.stateReader
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (sdb *IntraBlockState) Reset() {
	sdb.stateObjects = make(map[types.Address]*stateObject)
	sdb.stateObjectsDirty = make(map[types.Address]struct{})
	sdb.thash = types.Hash{}
	sdb.bhash = types.Hash{}
	sdb.txIndex = 0
	sdb.logs = make(map[types.Hash][]*block.Log)
	sdb.logSize = 0
	sdb.clearJournalAndRefund()
	sdb.accessList = newAccessList()
	sdb.balanceInc = make(map[types.Address]*BalanceIncrease)
}

func (sdb *IntraBlockState) AddLog(log2 *block.Log) {
	sdb.journal.append(addLogChange{txhash: sdb.thash})
	log2.TxHash = sdb.thash
	log2.BlockHash = sdb.bhash
	log2.TxIndex = uint(sdb.txIndex)
	log2.Index = sdb.logSize
	sdb.logs[sdb.thash] = append(sdb.logs[sdb.thash], log2)
	sdb.logSize++
}

func (sdb *IntraBlockState) GetLogs(hash types.Hash) []*block.Log {
	return sdb.logs[hash]
}

func (sdb *IntraBlockState) Logs() []*block.Log {
	var logs []*block.Log
	for _, lgs := range sdb.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddRefund adds gas to the refund counter
func (sdb *IntraBlockState) AddRefund(gas uint64) {
	sdb.journal.append(refundChange{prev: sdb.refund})
	sdb.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (sdb *IntraBlockState) SubRefund(gas uint64) {
	sdb.journal.append(refundChange{prev: sdb.refund})
	if gas > sdb.refund {
		sdb.setErrorUnsafe(fmt.Errorf("refund counter below zero"))
	}
	sdb.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (sdb *IntraBlockState) Exist(addr types.Address) bool {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	s := sdb.getStateObject(addr)
	return s != nil && !s.deleted
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (sdb *IntraBlockState) Empty(addr types.Address) bool {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	so := sdb.getStateObject(addr)
	return so == nil || so.deleted || so.empty()
}

// GetBalance retrieves the balance from the given address or 0 if object not found
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetBalance(addr types.Address) *uint256.Int {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil && !stateObject.deleted {
		return stateObject.Balance()
	}
	return uint256.NewInt(0)
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetNonce(addr types.Address) uint64 {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil && !stateObject.deleted {
		return stateObject.Nonce()
	}

	return 0
}

// TxIndex returns the current transaction index set by Prepare.
func (sdb *IntraBlockState) TxIndex() int {
	return sdb.txIndex
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetCode(addr types.Address) []byte {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil && !stateObject.deleted {
		if sdb.trace {
			fmt.Printf("GetCode %x, returned %d\n", addr, len(stateObject.Code()))
		}
		code := stateObject.Code()
		if len(code) > 0 && sdb.codeMap != nil {
			sdb.codeMap[types.BytesToHash(stateObject.CodeHash())] = code
		}
		return code
	}
	if sdb.trace {
		fmt.Printf("GetCode %x, returned nil\n", addr)
	}
	return nil
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetCodeSize(addr types.Address) int {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	len, err := sdb.stateReader.ReadAccountCodeSize(addr, stateObject.data.Incarnation, stateObject.data.CodeHash)
	if err != nil {
		sdb.setErrorUnsafe(err)
	}
	if len > 0 && sdb.codeMap != nil {
		code, _ := sdb.stateReader.ReadAccountCode(addr, stateObject.data.Incarnation, types.BytesToHash(stateObject.CodeHash()))
		sdb.codeMap[types.BytesToHash(stateObject.CodeHash())] = code
	}
	return len
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetCodeHash(addr types.Address) types.Hash {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		return types.Hash{}
	}
	return types.BytesToHash(stateObject.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetState(addr types.Address, key *types.Hash, value *uint256.Int) {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil && !stateObject.deleted {
		stateObject.GetState(key, value)
	} else {
		value.Clear()
	}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) GetCommittedState(addr types.Address, key *types.Hash, value *uint256.Int) {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil && !stateObject.deleted {
		stateObject.GetCommittedState(key, value)
	} else {
		value.Clear()
	}
}

func (sdb *IntraBlockState) HasSuicided(addr types.Address) bool {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	if stateObject.deleted {
		return false
	}
	if stateObject.created {
		return false
	}
	return stateObject.selfdestructed
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) AddBalance(addr types.Address, amount *uint256.Int) {
	if sdb.trace {
		fmt.Printf("AddBalance %x, %d\n", addr, amount)
	}
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}
	}
	// If this account has not been read, add to the balance increment map
	_, needAccount := sdb.stateObjects[addr]
	if !needAccount && addr == ripemd && amount.IsZero() {
		needAccount = true
	}
	if !needAccount {
		sdb.journal.append(balanceIncrease{
			account:  &addr,
			increase: *amount,
		})
		bi, ok := sdb.balanceInc[addr]
		if !ok {
			bi = &BalanceIncrease{}
			sdb.balanceInc[addr] = bi
		}
		bi.increase.Add(&bi.increase, amount)
		bi.count++
		return
	}

	stateObject := sdb.GetOrNewStateObject(addr)
	stateObject.AddBalance(amount)
}

// SubBalance subtracts amount from the account associated with addr.
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) SubBalance(addr types.Address, amount *uint256.Int) {
	if sdb.trace {
		fmt.Printf("SubBalance %x, %d\n", addr, amount)
	}
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}

	}

	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) SetBalance(addr types.Address, amount *uint256.Int) {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}
	}

	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) SetNonce(addr types.Address, nonce uint64) {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}
	}

	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

// DESCRIBED: docs/programmers_guide/guide.md#code-hash
// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) SetCode(addr types.Address, code []byte) {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}
	}

	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// DESCRIBED: docs/programmers_guide/guide.md#address---identifier-of-an-account
func (sdb *IntraBlockState) SetState(addr types.Address, key *types.Hash, value uint256.Int) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(key, value)
	}
}

// SetStorage replaces the entire storage for the specified account with given
// storage. This function should only be used for debugging.
func (sdb *IntraBlockState) SetStorage(addr types.Address, storage Storage) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetStorage(storage)
	}
}

// SetIncarnation sets incarnation for account if account exists
func (sdb *IntraBlockState) SetIncarnation(addr types.Address, incarnation uint16) {
	stateObject := sdb.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.setIncarnation(incarnation)
	}
}

func (sdb *IntraBlockState) GetIncarnation(addr types.Address) uint16 {
	stateObject := sdb.getStateObject(addr)
	if stateObject != nil {
		return stateObject.data.Incarnation
	}
	return 0
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (sdb *IntraBlockState) Suicide(addr types.Address) bool {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountRead err", err)
		}
		err = sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace {
			fmt.Println("CaptureAccountWrite err", err)
		}
	}
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		return false
	}
	sdb.journal.append(selfdestructChange{
		account:     &addr,
		prev:        stateObject.selfdestructed,
		prevbalance: *stateObject.Balance(),
	})
	stateObject.markSelfdestructed()
	stateObject.created = false
	stateObject.data.Balance.Clear()

	return true
}

func (sdb *IntraBlockState) getStateObject(addr types.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := sdb.stateObjects[addr]; obj != nil {
		return obj
	}

	// Load the object from the database.
	if _, ok := sdb.nilAccounts[addr]; ok {
		if bi, ok := sdb.balanceInc[addr]; ok && !bi.transferred {
			return sdb.createObject(addr, nil)
		}
		return nil
	}
	account, err := sdb.stateReader.ReadAccountData(addr)
	if err != nil {
		sdb.setErrorUnsafe(err)
		return nil
	}
	if account == nil {
		sdb.nilAccounts[addr] = struct{}{}
		if bi, ok := sdb.balanceInc[addr]; ok && !bi.transferred {
			return sdb.createObject(addr, nil)
		}
		return nil
	}

	// snap write
	if sdb.snap != nil && sdb.snap.CanWrite() {
		sdb.snap.AddAccount(addr, account)
	}

	// Insert into the live set.
	obj := newObject(sdb, addr, account, account)
	sdb.setStateObject(addr, obj)
	return obj
}

func (sdb *IntraBlockState) setStateObject(addr types.Address, object *stateObject) {
	if bi, ok := sdb.balanceInc[addr]; ok && !bi.transferred {
		object.data.Balance.Add(&object.data.Balance, &bi.increase)
		bi.transferred = true
		sdb.journal.append(balanceIncreaseTransfer{bi: bi})
	}
	sdb.stateObjects[addr] = object
}

// Retrieve a state object or create a new state object if nil.
func (sdb *IntraBlockState) GetOrNewStateObject(addr types.Address) *stateObject {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject = sdb.createObject(addr, stateObject /* previous */)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten.
func (sdb *IntraBlockState) createObject(addr types.Address, previous *stateObject) (newobj *stateObject) {
	ac := new(account.StateAccount)
	var original *account.StateAccount
	if previous == nil {
		original = &account.StateAccount{}
	} else {
		original = &previous.original
	}
	newobj = newObject(sdb, addr, ac, original)
	newobj.setNonce(0) // sets the object to dirty
	if previous == nil {
		sdb.journal.append(createObjectChange{account: &addr})
	} else {
		sdb.journal.append(resetObjectChange{account: &addr, prev: previous})
	}
	sdb.setStateObject(addr, newobj)
	return newobj
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//  1. sends funds to sha(account ++ (nonce + 1))
//  2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that AMT doesn't disappear.
func (sdb *IntraBlockState) CreateAccount(addr types.Address, contractCreation bool) {
	if sdb.tracer != nil {
		err := sdb.tracer.CaptureAccountRead(addr)
		if sdb.trace && err != nil {
			log.Error("error while CaptureAccountRead", "err", err)
		}

		err = sdb.tracer.CaptureAccountWrite(addr)
		if sdb.trace && err != nil {
			log.Error("error while CaptureAccountWrite", "err", err)
		}
	}

	var prevInc uint16
	previous := sdb.getStateObject(addr)
	if contractCreation {
		if previous != nil && previous.selfdestructed {
			prevInc = previous.data.Incarnation
		} else {
			inc, err := sdb.stateReader.ReadAccountIncarnation(addr)
			if sdb.trace && err != nil {
				log.Error("error while ReadAccountIncarnation", "err", err)
			}
			if err == nil {
				prevInc = inc
			}
		}
	}

	newObj := sdb.createObject(addr, previous)
	if previous != nil {
		newObj.data.Balance.Set(&previous.data.Balance)
		newObj.data.Initialised = true
	}

	if contractCreation {
		newObj.created = true
		newObj.data.Incarnation = prevInc + 1
	} else {
		newObj.selfdestructed = false
	}
}

// Snapshot returns an identifier for the current revision of the state.
func (sdb *IntraBlockState) Snapshot() int {
	id := sdb.nextRevisionID
	sdb.nextRevisionID++
	sdb.validRevisions = append(sdb.validRevisions, revision{id, sdb.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (sdb *IntraBlockState) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(sdb.validRevisions), func(i int) bool {
		return sdb.validRevisions[i].id >= revid
	})
	if idx == len(sdb.validRevisions) || sdb.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := sdb.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	sdb.journal.revert(sdb, snapshot)
	sdb.validRevisions = sdb.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (sdb *IntraBlockState) GetRefund() uint64 {
	return sdb.refund
}

func updateAccount(EIP161Enabled bool, isAura bool, stateWriter StateWriter, addr types.Address, stateObject *stateObject, isDirty bool) error {
	emptyRemoval := EIP161Enabled && stateObject.empty() && (!isAura || addr != SystemAddress)
	if stateObject.selfdestructed || (isDirty && emptyRemoval) {
		if err := stateWriter.DeleteAccount(addr, &stateObject.original); err != nil {
			return err
		}
		stateObject.deleted = true
	}
	if isDirty && (stateObject.created || !stateObject.selfdestructed) && !emptyRemoval {
		stateObject.deleted = false
		// Write any contract code associated with the state object
		if stateObject.code != nil && stateObject.dirtyCode {
			if err := stateWriter.UpdateAccountCode(addr, stateObject.data.Incarnation, stateObject.data.CodeHash, stateObject.code); err != nil {
				return err
			}
		}
		if stateObject.created {
			if err := stateWriter.CreateContract(addr); err != nil {
				return err
			}
		}
		if err := stateObject.updateTrie(stateWriter); err != nil {
			return err
		}
		if err := stateWriter.UpdateAccountData(addr, &stateObject.original, &stateObject.data); err != nil {
			return err
		}
	}
	return nil
}

func printAccount(addr types.Address, stateObject *stateObject, isDirty bool) {
	emptyRemoval := stateObject.empty()
	if stateObject.selfdestructed || (isDirty && emptyRemoval) {
		fmt.Printf("delete: %x\n", addr)
	}
	if isDirty && (stateObject.created || !stateObject.selfdestructed) && !emptyRemoval {
		// Write any contract code associated with the state object
		if stateObject.code != nil && stateObject.dirtyCode {
			fmt.Printf("UpdateCode: %x,%x\n", addr, stateObject.CodeHash())
		}
		if stateObject.created {
			fmt.Printf("CreateContract: %x\n", addr)
		}
		stateObject.printTrie()
		if stateObject.data.Balance.IsUint64() {
			fmt.Printf("UpdateAccountData: %x, balance=%d, nonce=%d\n", addr, stateObject.data.Balance.Uint64(), stateObject.data.Nonce)
		} else {
			div := uint256.NewInt(1_000_000_000)
			fmt.Printf("UpdateAccountData: %x, balance=%d*%d, nonce=%d\n", addr, uint256.NewInt(0).Div(&stateObject.data.Balance, div).Uint64(), div.Uint64(), stateObject.data.Nonce)
		}
	}
}

// FinalizeTx should be called after every transaction.
func (sdb *IntraBlockState) FinalizeTx(chainRules *params.Rules, stateWriter StateWriter) error {
	for addr, bi := range sdb.balanceInc {
		if !bi.transferred {
			sdb.getStateObject(addr)
		}
	}
	for addr := range sdb.journal.dirties {
		so, exist := sdb.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `sdb.journal.dirties` but not in `sdb.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if err := updateAccount(chainRules.IsSpuriousDragon, chainRules.IsAura, stateWriter, addr, so, true); err != nil {
			return err
		}

		sdb.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	sdb.clearJournalAndRefund()
	return nil
}

func (sdb *IntraBlockState) SoftFinalise() {
	for addr := range sdb.journal.dirties {
		_, exist := sdb.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `sdb.journal.dirties` but not in `sdb.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}
		sdb.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	sdb.clearJournalAndRefund()
}

// CommitBlock finalizes the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (sdb *IntraBlockState) CommitBlock(chainRules *params.Rules, stateWriter StateWriter) error {
	for addr, bi := range sdb.balanceInc {
		if !bi.transferred {
			sdb.getStateObject(addr)
		}
	}
	return sdb.MakeWriteSet(chainRules, stateWriter)
}

func (sdb *IntraBlockState) BalanceIncreaseSet() map[types.Address]uint256.Int {
	s := map[types.Address]uint256.Int{}
	for addr, bi := range sdb.balanceInc {
		if !bi.transferred {
			s[addr] = bi.increase
		}
	}
	return s
}

func (sdb *IntraBlockState) MakeWriteSet(chainRules *params.Rules, stateWriter StateWriter) error {
	for addr := range sdb.journal.dirties {
		sdb.stateObjectsDirty[addr] = struct{}{}
	}
	for addr, stateObject := range sdb.stateObjects {
		_, isDirty := sdb.stateObjectsDirty[addr]
		if err := updateAccount(chainRules.IsSpuriousDragon, chainRules.IsAura, stateWriter, addr, stateObject, isDirty); err != nil {
			return err
		}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	sdb.clearJournalAndRefund()
	return nil
}

func (sdb *IntraBlockState) Print() {
	for addr, stateObject := range sdb.stateObjects {
		_, isDirty := sdb.stateObjectsDirty[addr]
		_, isDirty2 := sdb.journal.dirties[addr]

		printAccount(addr, stateObject, isDirty || isDirty2)
	}
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (sdb *IntraBlockState) Prepare(thash, bhash types.Hash, ti int, sender types.Address) {
	sdb.thash = thash
	sdb.bhash = bhash
	sdb.txIndex = ti
	sdb.accessList = newAccessList()
	sdb.sender = sender
}

// no not lock
func (sdb *IntraBlockState) clearJournalAndRefund() {
	sdb.journal = newJournal()
	sdb.validRevisions = sdb.validRevisions[:0]
	sdb.refund = 0
}

// PrepareAccessList handles the preparatory steps for executing a state transition with
// regards to both EIP-2929 and EIP-2930:
//
// - Add sender to access list (2929)
// - Add destination to access list (2929)
// - Add precompiles to access list (2929)
// - Add the contents of the optional tx access list (2930)
//
// This method should only be called if Yolov3/Berlin/2929+2930 is applicable at the current number.
func (sdb *IntraBlockState) PrepareAccessList(sender types.Address, dst *types.Address, precompiles []types.Address, list transaction.AccessList) {
	sdb.AddAddressToAccessList(sender)
	if dst != nil {
		sdb.AddAddressToAccessList(*dst)
		// If it's a create-tx, the destination will be added inside evm.create
	}
	for _, addr := range precompiles {
		sdb.AddAddressToAccessList(addr)
	}
	for _, el := range list {
		sdb.AddAddressToAccessList(el.Address)
		for _, key := range el.StorageKeys {
			sdb.AddSlotToAccessList(el.Address, key)
		}
	}
}

// AddAddressToAccessList adds the given address to the access list
func (sdb *IntraBlockState) AddAddressToAccessList(addr types.Address) {
	if sdb.accessList.AddAddress(addr) {
		sdb.journal.append(accessListAddAccountChange{&addr})
	}
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (sdb *IntraBlockState) AddSlotToAccessList(addr types.Address, slot types.Hash) {
	addrMod, slotMod := sdb.accessList.AddSlot(addr, slot)
	if addrMod {
		// In practice, this should not happen, since there is no way to enter the
		// scope of 'address' without having the 'address' become already added
		// to the access list (via call-variant, create, etc).
		// Better safe than sorry, though
		sdb.journal.append(accessListAddAccountChange{&addr})
	}
	if slotMod {
		sdb.journal.append(accessListAddSlotChange{
			address: &addr,
			slot:    &slot,
		})
	}
}

// AddressInAccessList returns true if the given address is in the access list.
func (sdb *IntraBlockState) AddressInAccessList(addr types.Address) bool {
	return sdb.accessList.ContainsAddress(addr)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (sdb *IntraBlockState) SlotInAccessList(addr types.Address, slot types.Hash) (addressPresent bool, slotPresent bool) {
	return sdb.accessList.Contains(addr, slot)
}

// GenerateRootHash calculate root hash
func (s *IntraBlockState) GenerateRootHash() types.Hash {
	if len(s.stateObjectsDirty) == 0 {
		return hash.NilHash
	}

	sortAds := make(types.Addresses, 0, len(s.stateObjectsDirty))
	for a := range s.stateObjectsDirty {
		sortAds = append(sortAds, a)
	}
	sort.Sort(sortAds)
	//
	sha := hash.HasherPool.Get().(crypto.KeccakState)
	defer hash.HasherPool.Put(sha)
	sha.Reset()

	for _, address := range sortAds {
		stateObject := s.getStateObject(address).data
		err := rlp.Encode(sha, []interface{}{
			stateObject.Incarnation,
			stateObject.Balance,
			stateObject.Nonce,
			stateObject.Initialised,
			stateObject.CodeHash,
			stateObject.Root,
		})
		if err != nil {
			panic("can't encode: " + err.Error())
		}
		//log.Info("GenerateRootHash", "address", address)
	}

	var root types.Hash
	sha.Read(root[:])

	//log.Info("GenerateRootHash ", "root", root.Hex())

	return root
}

// IntermediateRoot root
func (s *IntraBlockState) IntermediateRoot() types.Hash {
	return s.GenerateRootHash()
}

func (sdb *IntraBlockState) HasSelfdestructed(addr types.Address) bool {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	if stateObject.deleted {
		return false
	}
	if stateObject.created {
		return false
	}
	return stateObject.selfdestructed
}

// Selfdestruct marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (sdb *IntraBlockState) Selfdestruct(addr types.Address) bool {
	stateObject := sdb.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		return false
	}
	sdb.journal.append(selfdestructChange{
		account:     &addr,
		prev:        stateObject.selfdestructed,
		prevbalance: *stateObject.Balance(),
	})
	stateObject.markSelfdestructed()
	stateObject.created = false
	stateObject.data.Balance.Clear()

	return true
}

// BeforeStateRoot calculate used state hash
//
// it should be invoked after all txs exec
func (sdb *IntraBlockState) BeforeStateRoot() (hash types.Hash) {
	if sdb.snap == nil {
		return types.Hash{}
	}
	sort.Sort(sdb.snap.Items)
	ch := sdb.CodeHashes()
	hs := make(HashCodes, 0, len(ch))
	for k, v := range ch {
		hs = append(hs, &HashCode{Hash: k, Code: v})
	}
	sort.Sort(hs)
	hasher := sha3.NewLegacyKeccak256()
	EncodeBeforeState(hasher, sdb.snap.Items, hs)
	hasher.(crypto.KeccakState).Read(hash[:])
	return
}
