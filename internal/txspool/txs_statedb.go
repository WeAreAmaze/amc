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

package txspool

import (
	"github.com/amazechain/amc/common/types"
)

type fakeStateDB struct{}

func (d *fakeStateDB) PrepareAccessList(sender types.Address, dest *types.Address, precompiles []types.Address) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) AddressInAccessList(addr types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SlotInAccessList(addr types.Address, slot types.Hash) (addressOk bool, slotOk bool) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) AddAddressToAccessList(addr types.Address) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) AddSlotToAccessList(addr types.Address, slot types.Hash) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SetNonce(addr types.Address, nonce uint64) error {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SetBalance(addr types.Address, amount types.Int256) error {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SubBalance(addr types.Address, amount types.Int256) error {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetCodeHash(addr types.Address) types.Hash {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetCode(addr types.Address) []byte {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SetCode(addr types.Address, code []byte) error {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetCodeSize(addr types.Address) int {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) AddRefund(u uint64) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SubRefund(u uint64) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetRefund() uint64 {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetCommittedState(address types.Address, hash types.Hash) types.Hash {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) GetState(address types.Address, hash types.Hash) types.Hash {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) SetState(address types.Address, hash types.Hash, hash2 types.Hash) {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) Suicide(address types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) HasSuicided(address types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) Exist(address types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) Empty(address types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (d *fakeStateDB) CreateAccount(address types.Address) {
}

func (*fakeStateDB) GetNonce(addr types.Address) uint64         { return 1337 }
func (*fakeStateDB) GetBalance(addr types.Address) types.Int256 { return types.NewInt64(100000) }
