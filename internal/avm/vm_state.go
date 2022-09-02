package avm

import (
	amc_common "github.com/amazechain/amc/common"
	amc_types "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/types"
	"math/big"
)

type DBStates struct {
	db amc_common.IStateDB
}

func (D *DBStates) Prepare(thash common.Hash, ti int) {
	D.db.Prepare(types.ToAmcHash(thash), ti)
}

func (D *DBStates) TxIndex() int {
	return D.db.TxIndex()
}

func (D *DBStates) CreateAccount(address common.Address) {
	addr := types.ToAmcAddress(address)
	D.db.CreateAccount(addr)
}

func (D *DBStates) SubBalance(address common.Address, b *big.Int) {
	addr := types.ToAmcAddress(address)
	balance, _ := amc_types.FromBig(b)
	D.db.SubBalance(addr, balance)
}

func (D *DBStates) AddBalance(address common.Address, b *big.Int) {
	addr := types.ToAmcAddress(address)
	balance, _ := amc_types.FromBig(b)
	D.db.AddBalance(addr, balance)
}

func (D *DBStates) GetBalance(address common.Address) *big.Int {
	addr := types.ToAmcAddress(address)
	b := D.db.GetBalance(addr)
	return b.ToBig()
}

func (D *DBStates) GetNonce(address common.Address) uint64 {
	addr := types.ToAmcAddress(address)
	return D.db.GetNonce(addr)
}

func (D *DBStates) SetNonce(address common.Address, u uint64) {
	addr := types.ToAmcAddress(address)
	D.db.SetNonce(addr, u)
}

func (D *DBStates) GetCodeHash(address common.Address) common.Hash {
	addr := types.ToAmcAddress(address)
	hash := D.db.GetCodeHash(addr)
	return types.FromAmcHash(hash)
}

func (D *DBStates) GetCode(address common.Address) []byte {
	addr := types.ToAmcAddress(address)
	return D.db.GetCode(addr)
}

func (D *DBStates) SetCode(address common.Address, bytes []byte) {
	addr := types.ToAmcAddress(address)
	D.db.SetCode(addr, bytes)
}

func (D *DBStates) GetCodeSize(address common.Address) int {
	addr := types.ToAmcAddress(address)
	return D.db.GetCodeSize(addr)
}

func (D *DBStates) AddRefund(u uint64) {
	D.db.AddRefund(u)
}

func (D *DBStates) SubRefund(u uint64) {
	D.db.SubRefund(u)
}

func (D *DBStates) GetRefund() uint64 {
	return D.db.GetRefund()
}

func (D *DBStates) GetCommittedState(address common.Address, hash common.Hash) common.Hash {
	addr := types.ToAmcAddress(address)
	h := types.ToAmcHash(hash)
	newHash := D.db.GetCommittedState(addr, h)
	return types.FromAmcHash(newHash)
}

func (D *DBStates) GetState(address common.Address, hash common.Hash) common.Hash {
	addr := types.ToAmcAddress(address)
	h := types.ToAmcHash(hash)
	newHash := D.db.GetState(addr, h)
	return types.FromAmcHash(newHash)
}

func (D *DBStates) SetState(address common.Address, hash common.Hash, hash2 common.Hash) {
	addr := types.ToAmcAddress(address)
	h1 := types.ToAmcHash(hash)
	h2 := types.ToAmcHash(hash2)
	D.db.SetState(addr, h1, h2)

}

func (D *DBStates) Suicide(address common.Address) bool {
	addr := types.ToAmcAddress(address)
	return D.db.Suicide(addr)
}

func (D *DBStates) HasSuicided(address common.Address) bool {
	addr := types.ToAmcAddress(address)
	return D.db.HasSuicided(addr)
}

func (D *DBStates) Exist(address common.Address) bool {
	addr := types.ToAmcAddress(address)
	return D.db.Exist(addr)
}

func (D *DBStates) Empty(address common.Address) bool {
	addr := types.ToAmcAddress(address)
	return D.db.Empty(addr)
}

func (D *DBStates) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {

	destAddress := amc_types.Address{0}
	if dest != nil {
		destAddress = types.ToAmcAddress(*dest)
	}

	var amcPrecompile []amc_types.Address
	for _, precompile := range precompiles {
		amcPrecompile = append(amcPrecompile, types.ToAmcAddress(precompile))
	}
	D.db.PrepareAccessList(types.ToAmcAddress(sender), &destAddress, amcPrecompile, types.ToAmcAccessList(txAccesses))
}

func (D *DBStates) AddressInAccessList(addr common.Address) bool {
	return D.db.AddressInAccessList(types.ToAmcAddress(addr))
}

func (D *DBStates) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return D.db.SlotInAccessList(types.ToAmcAddress(addr), types.ToAmcHash(slot))
}

func (D *DBStates) AddAddressToAccessList(addr common.Address) {
	D.db.AddAddressToAccessList(types.ToAmcAddress(addr))
}

func (D *DBStates) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	D.db.AddSlotToAccessList(types.ToAmcAddress(addr), types.ToAmcHash(slot))
}

func (D *DBStates) RevertToSnapshot(i int) {
	D.db.RevertToSnapshot(i)
}

func (D *DBStates) Snapshot() int {
	return D.db.Snapshot()
}

func (D *DBStates) AddLog(log *types.Log) {
	D.db.AddLog(types.ToAmcLog(log))
}

func (D *DBStates) GetLogs(hash common.Hash, blockHash common.Hash) []*types.Log {
	return types.FromAmcLogs(D.db.GetLogs(types.ToAmcHash(hash), types.ToAmcHash(blockHash)))
}

func (D *DBStates) AddPreimage(hash common.Hash, bytes []byte) {
	//panic("implement me")
}

func (D *DBStates) ForEachStorage(address common.Address, f func(common.Hash, common.Hash) bool) error {
	//panic("implement me")
	return nil
}

func NewDBStates(db amc_common.IStateDB) *DBStates {
	return &DBStates{db: db}
}
