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

package rawdb

import (
	"fmt"
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/common/u256"
	"github.com/amazechain/amc/modules"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// GetDeposit
func GetDeposit(db kv.Getter, addr types.Address) (types.PublicKey, *uint256.Int, error) {
	valBytes, err := db.GetOne(modules.Deposit, addr[:])
	if err != nil {
		return types.PublicKey{}, nil, err
	}
	if len(valBytes) < types.PublicKeyLength {
		return types.PublicKey{}, nil, fmt.Errorf("the data length wrong")
	}
	_, err = bls.PublicKeyFromBytes(valBytes[:types.PublicKeyLength])
	if err != nil {
		return types.PublicKey{}, nil, fmt.Errorf("cannot unmarshal pubkey from bytes")
	}
	pubkey := new(types.PublicKey)
	if err = pubkey.SetBytes(valBytes[:types.PublicKeyLength]); err != nil {
		return types.PublicKey{}, nil, fmt.Errorf("cannot unmarshal pubkey from bytes")
	}
	amount := uint256.NewInt(0).SetBytes(valBytes[types.PublicKeyLength:])

	return *pubkey, amount, nil
}

func DoDeposit(db kv.Putter, addr types.Address, pub types.PublicKey, amount *uint256.Int) error {

	data := make([]byte, types.PublicKeyLength+amount.ByteLen())
	copy(data[:types.PublicKeyLength], pub.Bytes())
	copy(data[types.PublicKeyLength:], amount.Bytes())
	//
	if err := db.Put(modules.Deposit, addr[:], data); err != nil {
		return fmt.Errorf("failed to store address Deposit: %w", err)
	}
	return nil
}

func UndoDeposit(db kv.Putter, addr types.Address, amount *uint256.Int) error {
	pub, dbAmount, err := GetDeposit(db.(kv.Getter), addr)
	if err != nil {
		return fmt.Errorf("cannot get deposit")
	}
	if dbAmount.Cmp(amount) != 0 {
		return fmt.Errorf("dbAmount != withdrawnAmount")
	}
	return DoDeposit(db, addr, pub, new(uint256.Int))
}

// DoWithdrawn removes Deposit data associated with an address.
func DoWithdrawn(db kv.RwTx, addr types.Address, withdrawnAmount *uint256.Int) error {
	pub, dbAmount, err := GetDeposit(db, addr)
	if err != nil {
		return fmt.Errorf("cannot get deposit")
	}
	if dbAmount.Cmp(withdrawnAmount) != 0 {
		return fmt.Errorf("dbAmount != withdrawnAmount")
	}
	return DoDeposit(db, addr, pub, new(uint256.Int))
}

func UndoWithdrawn(db kv.RwTx, addr types.Address, unDoWithdrawnAmount *uint256.Int) error {
	pub, dbAmount, err := GetDeposit(db, addr)
	if err != nil {
		return fmt.Errorf("cannot get deposit")
	}
	if dbAmount.Cmp(u256.Num0) != 0 {
		return fmt.Errorf("db deposit != 0 when undo withdrawn")
	}
	return DoDeposit(db, addr, pub, unDoWithdrawnAmount)
}

// IsDeposit is deposit account
func IsDeposit(db kv.Getter, addr types.Address) bool {
	//
	//is, _ := db.Has(modules.Deposit, addr[:])
	_, amount, err := GetDeposit(db, addr)
	if err != nil {
		return false
	}

	if amount.Cmp(u256.Num0) == 0 {
		return false
	}
	//
	return true
}

func DepositNum(tx kv.Tx) (uint64, error) {
	cur, err := tx.Cursor(modules.Deposit)
	if nil != err {
		return 0, err
	}
	defer cur.Close()
	return cur.Count()
}
