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

package transaction

import (
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
)

type SakuragiTx struct {
	Nonce    uint64      // nonce of sender account
	GasPrice uint256.Int // wei per gas
	Gas      uint64      // gas limit
	To       *types.Address
	From     *types.Address
	Value    uint256.Int // wei amount
	Data     []byte      // contract invocation input data
	Sign     []byte      // signature values
}
