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

package internal

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules/state"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// Validator is an interface which defines the standard for block validation. It
// is only responsible for validating block contents, as the header validation is
// done by the specific consensus engines.
//type Validator interface {
//	// ValidateBody validates the given block's content.
//	ValidateBody(block *block.Block) error
//
//	// ValidateState validates the given statedb and optionally the receipts and
//	// gas used.
//	ValidateState(block *block.Block, state *state.StateDB, receipts block.Receipts, usedGas uint64) error
//}
//
//// Prefetcher is an interface for pre-caching transaction signatures and state.
//type Prefetcher interface {
//	// Prefetch processes the state changes according to the Ethereum rules by running
//	// the transaction messages using the statedb, but any changes are discarded. The
//	// only goal is to pre-cache transaction signatures and state trie nodes.
//	Prefetch(block *block.Block, statedb *state.StateDB, cfg vm.Config, interrupt *uint32)
//}

// Processor is an interface for processing blocks using a given initial state.
type Processor interface {
	// Process processes the state changes according to the Ethereum rules by running
	// the transaction messages using the statedb and applying any rewards to both
	// the processor (coinbase) and any included uncles.
	Process(tx kv.RwTx, b *block.Block, ibs *state.IntraBlockState, stateReader state.StateReader, stateWriter state.WriterWithChangeSets, blockHashFunc func(n uint64) types.Hash) (block.Receipts, []*block.Log, uint64, error)
}
