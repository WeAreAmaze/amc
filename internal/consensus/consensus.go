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

package consensus

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
)

type IEngine interface {
	VerifyHeader(header block.IHeader) error
	VerifyBlock(block block.IBlock) error
	GenerateBlock(parentBlock block.IBlock, txs []*transaction.Transaction) (block.IBlock, error)
	Seal(block block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error
	Prepare(header *block.Header) error
	EngineName() string
	Start() error
	Stop() error
	Author(header *block.Header) types.Address
	TxPool() txs_pool.ITxsPool

	GetMiner(iBlock block.IBlock) types.Address
}
