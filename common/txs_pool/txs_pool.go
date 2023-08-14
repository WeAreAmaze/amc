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

package txs_pool

import (
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/contracts/deposit"
)

type ITxsPool interface {
	Has(hash types.Hash) bool
	Pending(enforceTips bool) map[types.Address][]*transaction.Transaction
	GetTransaction() ([]*transaction.Transaction, error)
	GetTx(hash types.Hash) *transaction.Transaction
	AddRemotes(txs []*transaction.Transaction) []error
	AddLocal(tx *transaction.Transaction) error
	Stats() (int, int, int, int)
	Nonce(addr types.Address) uint64
	Content() (map[types.Address][]*transaction.Transaction, map[types.Address][]*transaction.Transaction)
	SetDeposit(deposit *deposit.Deposit)
}
