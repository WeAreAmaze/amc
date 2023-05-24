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
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"

	"github.com/amazechain/amc/modules"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// PutAccountReward
func PutAccountReward(db kv.Putter, account types.Address, val *uint256.Int) error {
	key := fmt.Sprintf("account:%s", account.String())
	return db.Put(modules.Reward, []byte(key), val.Bytes())
}

// GetAccountReward
func GetAccountReward(db kv.Getter, account types.Address) (*uint256.Int, error) {
	key := fmt.Sprintf("account:%s", account.String())
	val, err := db.GetOne(modules.Reward, []byte(key))
	if err != nil {
		return uint256.NewInt(0), err
	}
	return uint256.NewInt(0).SetBytes(val), nil
}
