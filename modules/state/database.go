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

//nolint:scopelint
package state

import (
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
)

const (
	//FirstContractIncarnation - first incarnation for contract accounts. After 1 it increases by 1.
	FirstContractIncarnation = 1
	//NonContractIncarnation incarnation for non contracts
	NonContractIncarnation = 0
)

type StateReader interface {
	ReadAccountData(address types.Address) (*account.StateAccount, error)
	ReadAccountStorage(address types.Address, incarnation uint16, key *types.Hash) ([]byte, error)
	ReadAccountCode(address types.Address, incarnation uint16, codeHash types.Hash) ([]byte, error)
	ReadAccountCodeSize(address types.Address, incarnation uint16, codeHash types.Hash) (int, error)
	ReadAccountIncarnation(address types.Address) (uint16, error)
}

type StateWriter interface {
	UpdateAccountData(address types.Address, original, account *account.StateAccount) error
	UpdateAccountCode(address types.Address, incarnation uint16, codeHash types.Hash, code []byte) error
	DeleteAccount(address types.Address, original *account.StateAccount) error
	WriteAccountStorage(address types.Address, incarnation uint16, key *types.Hash, original, value *uint256.Int) error
	CreateContract(address types.Address) error
}

type WriterWithChangeSets interface {
	StateWriter
	WriteChangeSets() error
	WriteHistory() error
}

type NoopWriter struct {
}

var noopWriter = &NoopWriter{}

func NewNoopWriter() *NoopWriter {
	return noopWriter
}

func (nw *NoopWriter) UpdateAccountData(address types.Address, original, account *account.StateAccount) error {
	return nil
}

func (nw *NoopWriter) DeleteAccount(address types.Address, original *account.StateAccount) error {
	return nil
}

func (nw *NoopWriter) UpdateAccountCode(address types.Address, incarnation uint16, codeHash types.Hash, code []byte) error {
	return nil
}

func (nw *NoopWriter) WriteAccountStorage(address types.Address, incarnation uint16, key *types.Hash, original, value *uint256.Int) error {
	return nil
}

func (nw *NoopWriter) CreateContract(address types.Address) error {
	return nil
}

func (nw *NoopWriter) WriteChangeSets() error {
	return nil
}

func (nw *NoopWriter) WriteHistory() error {
	return nil
}
