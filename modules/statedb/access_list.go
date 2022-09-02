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
	"github.com/amazechain/amc/common/types"
)

type accessList struct {
	addresses map[types.Address]int
	slots     []map[types.Hash]struct{}
}

func (al *accessList) ContainsAddress(address types.Address) bool {
	_, ok := al.addresses[address]
	return ok
}

func (al *accessList) Contains(address types.Address, slot types.Hash) (addressPresent bool, slotPresent bool) {
	idx, ok := al.addresses[address]
	if !ok {
		// no such address (and hence zero slots)
		return false, false
	}
	if idx == -1 {
		// address yes, but no slots
		return true, false
	}
	_, slotPresent = al.slots[idx][slot]
	return true, slotPresent
}

func newAccessList() *accessList {
	return &accessList{
		addresses: make(map[types.Address]int),
	}
}

func (a *accessList) Copy() *accessList {
	cp := newAccessList()
	for k, v := range a.addresses {
		cp.addresses[k] = v
	}
	cp.slots = make([]map[types.Hash]struct{}, len(a.slots))
	for i, slotMap := range a.slots {
		newSlotmap := make(map[types.Hash]struct{}, len(slotMap))
		for k := range slotMap {
			newSlotmap[k] = struct{}{}
		}
		cp.slots[i] = newSlotmap
	}
	return cp
}

func (al *accessList) AddAddress(address types.Address) bool {
	if _, present := al.addresses[address]; present {
		return false
	}
	al.addresses[address] = -1
	return true
}

// AddSlot adds the specified (addr, slot) combo to the access list.
// Return values are:
// - address added
// - slot added
// For any 'true' value returned, a corresponding journal entry must be made.
func (al *accessList) AddSlot(address types.Address, slot types.Hash) (addrChange bool, slotChange bool) {
	idx, addrPresent := al.addresses[address]
	if !addrPresent || idx == -1 {
		// Address not present, or addr present but no slots there
		al.addresses[address] = len(al.slots)
		slotmap := map[types.Hash]struct{}{slot: {}}
		al.slots = append(al.slots, slotmap)
		return !addrPresent, true
	}
	slotmap := al.slots[idx]
	if _, ok := slotmap[slot]; !ok {
		slotmap[slot] = struct{}{}
		// Journal add slot change
		return false, true
	}
	// No changes required
	return false, false
}

// DeleteSlot removes an (address, slot)-tuple from the access list.
func (al *accessList) DeleteSlot(address types.Address, slot types.Hash) {
	idx, addrOk := al.addresses[address]
	if !addrOk {
		panic("reverting slot change, address not present in list")
	}
	slotmap := al.slots[idx]
	delete(slotmap, slot)
	if len(slotmap) == 0 {
		al.slots = al.slots[:idx]
		al.addresses[address] = -1
	}
}

func (al *accessList) DeleteAddress(address types.Address) {
	delete(al.addresses, address)
}
