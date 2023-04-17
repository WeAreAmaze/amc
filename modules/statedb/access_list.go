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

// It allows transaction senders to provide additional storage slots to transactions executed on Ethereum,
// which will be allocated to specified addresses.
type accessList struct {
	// store the mapping relationship between addresses and integers.
	addresses map[types.Address]int
	// store slices of hash values, each hash value corresponds to a storage slot containing the corresponding address.
	slots []map[types.Hash]struct{}
}

// ContainsAddress check if a given address is present in the accessList struct.
func (al *accessList) ContainsAddress(address types.Address) bool {
	_, ok := al.addresses[address]
	return ok
}

// Contains check if a given address and slot are present in the accessList struct.
func (al *accessList) Contains(address types.Address, slot types.Hash) (addressPresent bool, slotPresent bool) {
	// first checks if the addresses map in the accessList struct contains the given address.
	idx, ok := al.addresses[address]
	if !ok {
		// no such address (and hence zero slots)
		return false, false
	}
	if idx == -1 {
		// address yes, but no slots
		return true, false
	}
	// If the address exists and has an associated slot, the method retrieves the slot map corresponding to the address
	// from the slots slice. It then checks whether the slot is present in the map and stores the result in slotPresent.
	// Finally, it returns one true and slotPresent to indicate that the address and slot both exist.
	_, slotPresent = al.slots[idx][slot]
	return true, slotPresent
}

// newAccessList create a new instance of the accessList struct, with an empty addresses map.
func newAccessList() *accessList {
	return &accessList{
		addresses: make(map[types.Address]int),
	}
}

// Copy create a deep copy of the accessList structure, so that the modification of the original accessList
// structure will not affect the copy accessList.
func (a *accessList) Copy() *accessList {
	// first creates a new pointer cp of accessList type
	cp := newAccessList()
	// copies the addresses in the original accessList to the new accessList pointed to by cp
	for k, v := range a.addresses {
		cp.addresses[k] = v
	}
	// creates a new slots slice and sets its length to be the same as the slots slice in the original accessList.
	cp.slots = make([]map[types.Hash]struct{}, len(a.slots))
	// loops through each slotMap in the original accessList, and for each slotMap creates a new empty map,
	// which is then copied into the corresponding location of the accessList pointed to by the new cp.
	for i, slotMap := range a.slots {
		newSlotmap := make(map[types.Hash]struct{}, len(slotMap))
		for k := range slotMap {
			newSlotmap[k] = struct{}{}
		}
		cp.slots[i] = newSlotmap
	}
	// returns a pointer cp of the new accessList type.
	return cp
}

// AddAddress Add addresses to the addresses map
func (al *accessList) AddAddress(address types.Address) bool {
	// If the address is already present in the map, the function returns false and does not add the address again.
	if _, present := al.addresses[address]; present {
		return false
	}
	// If the address is not already present in the map, the function adds it to the addresses map with a value of -1.
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
	// checks to see if the address parameter exists in a map called addresses of type accessList,
	// and if not, a panic exception is thrown.
	idx, addrOk := al.addresses[address]
	if !addrOk {
		panic("reverting slot change, address not present in list")
	}
	// retrieves the index idx of address in a slice called slots of type accessList and stores it in a variable
	slotmap := al.slots[idx]
	// delete the specified slot
	delete(slotmap, slot)
	// If the slotmap mapping is empty, delete the slotmap mapping from the slots slice, and map address to -1
	// to indicate that the address is no longer associated with any slot.
	if len(slotmap) == 0 {
		al.slots = al.slots[:idx]
		al.addresses[address] = -1
	}
}

// DeleteAddress delete the specified address parameter from a map named addresses of the accessList type.
func (al *accessList) DeleteAddress(address types.Address) {
	delete(al.addresses, address)
}
