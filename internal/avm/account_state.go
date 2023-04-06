package avm

//import (
//	"bytes"
//	"encoding/json"
//	"fmt"
//	"github.com/amazechain/amc/internal/avm/common"
//	"github.com/amazechain/amc/common/crypto"
//	"github.com/amazechain/amc/internal/avm/types"
//	"math/big"
//	"os"
//)
//
////var emptyCodeHash = crypto.Keccak256(nil)
//
//type accountObject struct {
//	Address      common.Address              `json:"address,omitempty"`
//	AddrHash     types.Hash                 `json:"addr_hash,omitempty"` // hash of ethereum address of the account
//	ByteCode     []byte                      `json:"byte_code,omitempty"`
//	Data         accountData                 `json:"data,omitempty"`
//	CacheStorage map[types.Hash]types.Hash `json:"cache_storage,omitempty"` //
//}
//
//type accountData struct {
//	Nonce    uint64      `json:"nonce,omitempty"`
//	Balance  *big.Int    `json:"balance,omitempty"`
//	Root     types.Hash `json:"root,omitempty"` // merkle root of the storage trie
//	CodeHash []byte      `json:"code_hash,omitempty"`
//}
//
//// newObject creates a state object.
//func newAccountObject(address common.Address, data accountData) *accountObject {
//	if data.Balance == nil {
//		data.Balance = new(big.Int)
//	}
//	if data.CodeHash == nil {
//		data.CodeHash = emptyCodeHash
//	}
//	return &accountObject{
//		Address:      address,
//		AddrHash:     common.BytesToHash(crypto.Keccak256(address[:])),
//		Data:         data,
//		CacheStorage: make(map[types.Hash]types.Hash),
//	}
//}
//
////balance--
//func (object *accountObject) Balance() *big.Int {
//	return object.Data.Balance
//}
//
//func (object *accountObject) SubBalance(amount *big.Int) {
//	if amount.Sign() == 0 {
//		return
//	}
//	object.Data.Balance = new(big.Int).Sub(object.Balance(), amount)
//}
//
//func (object *accountObject) AddBalance(amount *big.Int) {
//	if amount.Sign() == 0 {
//		return
//	}
//	object.Data.Balance = new(big.Int).Add(object.Balance(), amount)
//}
//
//// nonce--
//func (object *accountObject) Nonce() uint64 {
//	return object.Data.Nonce
//}
//
//func (object *accountObject) SetNonce(nonce uint64) {
//	object.Data.Nonce = nonce
//}
//
//// code-----
//
//func (object *accountObject) CodeHash() []byte {
//	return object.Data.CodeHash
//}
//
//func (object *accountObject) Code() []byte {
//	return object.ByteCode
//}
//
//func (object *accountObject) SetCode(codeHash []byte, code []byte) {
//	object.Data.CodeHash = codeHash
//	object.ByteCode = code
//}
//
//// storage sate-------
//func (object *accountObject) GetStorageState(key types.Hash) types.Hash {
//	value, exist := object.CacheStorage[key]
//	if exist {
//		// fmt.Println("exist cache ", " key: ", key, " value: ", value)
//		return value
//	}
//	return types.Hash{}
//}
//
//func (object *accountObject) SetStorageState(key, value types.Hash) {
//	object.CacheStorage[key] = value
//}
//
//func (object *accountObject) Empty() bool {
//	return object.Data.Nonce == 0 && object.Data.Balance.Sign() == 0 && bytes.Equal(object.Data.CodeHash, emptyCodeHash)
//}
//
////AccountState
//type AccountState struct {
//	Accounts map[common.Address]*accountObject `json:"accounts,omitempty"`
//}
//
//// NewAccountStateDb new instance
//func NewAccountStateDb() *AccountState {
//	return &AccountState{
//		Accounts: make(map[common.Address]*accountObject),
//	}
//}
//
//func (accSt *AccountState) getAccountObject(addr common.Address) *accountObject {
//	if value, exist := accSt.Accounts[addr]; exist {
//		return value
//	}
//	return nil
//}
//
//func (accSt *AccountState) setAccountObject(obj *accountObject) {
//	accSt.Accounts[obj.Address] = obj
//}
//
////
//func (accSt *AccountState) getOrsetAccountObject(addr common.Address) *accountObject {
//	get := accSt.getAccountObject(addr)
//	if get != nil {
//		return get
//	}
//	set := newAccountObject(addr, accountData{})
//	accSt.setAccountObject(set)
//	return set
//}
//
//// -------
//
////CreateAccount
//func (accSt *AccountState) CreateAccount(addr common.Address) {
//	if accSt.getAccountObject(addr) != nil {
//		return
//	}
//	obj := newAccountObject(addr, accountData{})
//	accSt.setAccountObject(obj)
//}
//
//// SubBalance
//func (accSt *AccountState) SubBalance(addr common.Address, amount *big.Int) {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		stateObject.SubBalance(amount)
//	}
//}
//
//// AddBalance
//func (accSt *AccountState) AddBalance(addr common.Address, amount *big.Int) {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		stateObject.AddBalance(amount)
//	}
//}
//
//// GetBalance
//func (accSt *AccountState) GetBalance(addr common.Address) *big.Int {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		return stateObject.Balance()
//	}
//	return new(big.Int).SetInt64(0)
//}
//
////GetNonce
//func (accSt *AccountState) GetNonce(addr common.Address) uint64 {
//	stateObject := accSt.getAccountObject(addr)
//	if stateObject != nil {
//		return stateObject.Nonce()
//	}
//	return 0
//}
//
//// SetNonce
//func (accSt *AccountState) SetNonce(addr common.Address, nonce uint64) {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		stateObject.SetNonce(nonce)
//	}
//}
//
//// GetCodeHash
//func (accSt *AccountState) GetCodeHash(addr common.Address) types.Hash {
//	stateObject := accSt.getAccountObject(addr)
//	if stateObject == nil {
//		return types.Hash{}
//	}
//	return common.BytesToHash(stateObject.CodeHash())
//}
//
////GetCode
//func (accSt *AccountState) GetCode(addr common.Address) []byte {
//	stateObject := accSt.getAccountObject(addr)
//	if stateObject != nil {
//		return stateObject.Code()
//	}
//	return nil
//}
//
////SetCode
//func (accSt *AccountState) SetCode(addr common.Address, code []byte) {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		stateObject.SetCode(crypto.Keccak256(code), code)
//	}
//}
//
//// GetCodeSize
//func (accSt *AccountState) GetCodeSize(addr common.Address) int {
//	stateObject := accSt.getAccountObject(addr)
//	if stateObject == nil {
//		return 0
//	}
//	if stateObject.ByteCode != nil {
//		return len(stateObject.ByteCode)
//	}
//	return 0
//}
//
//// AddRefund
//func (accSt *AccountState) AddRefund(uint64) {
//	return
//}
//
////GetRefund ...
//func (accSt *AccountState) GetRefund() uint64 {
//	return 0
//}
//
//// GetState SetState
//func (accSt *AccountState) GetState(addr common.Address, key types.Hash) types.Hash {
//	stateObject := accSt.getAccountObject(addr)
//	if stateObject != nil {
//		return stateObject.GetStorageState(key)
//	}
//	return types.Hash{}
//}
//
//// SetState
//func (accSt *AccountState) SetState(addr common.Address, key types.Hash, value types.Hash) {
//	stateObject := accSt.getOrsetAccountObject(addr)
//	if stateObject != nil {
//		fmt.Printf("SetState key: %x value: %s", key, new(big.Int).SetBytes(value[:]).String())
//		stateObject.SetStorageState(key, value)
//	}
//}
//
//// Suicide
//func (accSt *AccountState) Suicide(common.Address) bool {
//	return false
//}
//
//// HasSuicided ...
//func (accSt *AccountState) HasSuicided(common.Address) bool {
//	return false
//}
//
//// Exist
//func (accSt *AccountState) Exist(addr common.Address) bool {
//	return accSt.getAccountObject(addr) != nil
//}
//
////Empty
//func (accSt *AccountState) Empty(addr common.Address) bool {
//	so := accSt.getAccountObject(addr)
//	return so == nil || so.Empty()
//}
//
//// RevertToSnapshot ...
//func (accSt *AccountState) RevertToSnapshot(int) {
//
//}
//
//// Snapshot ...
//func (accSt *AccountState) Snapshot() int {
//	return 0
//}
//
//// AddLog
//func (accSt *AccountState) AddLog(log *types.Log) {
//	//fmt.Printf("log: %v", log)
//}
//
//// AddPreimage
//func (accSt *AccountState) AddPreimage(types.Hash, []byte) {
//
//}
//
//// ForEachStorage
//func (accSt *AccountState) ForEachStorage(common.Address, func(types.Hash, types.Hash) bool) {
//
//}
//
//// Commit
//func (accSt *AccountState) Commit() error {
//	//
//	file, err := os.Create("./account_sate.db")
//	if err != nil {
//		return err
//	}
//	err = json.NewEncoder(file).Encode(accSt)
//	//fmt.Println("len(binCode): ", len(binCode), " code: ", binCode)
//	// bufW := bufio.NewWriter(file)
//	// bufW.Write(binCode)
//	// // bufW.WriteByte('\n')
//	// bufW.Flush()
//	file.Close()
//	return err
//}
//
////TryLoadFromDisk
//func TryLoadFromDisk() (*AccountState, error) {
//	file, err := os.Open("./account_sate.db")
//	if err != nil && os.IsNotExist(err) {
//		return NewAccountStateDb(), nil
//	}
//	if err != nil {
//		return nil, err
//	}
//
//	// stat, _ := file.Stat()
//	// // buf := stat.Size()
//	var accStat AccountState
//
//	err = json.NewDecoder(file).Decode(&accStat)
//	return &accStat, err
//}
