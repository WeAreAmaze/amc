// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package blockchain

import (
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/modules/statedb"
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type BlockValidator struct {
	bc     *BlockChain      // Canonical block chain
	engine consensus.Engine // Consensus engine used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(blockchain *BlockChain, engine consensus.Engine) *BlockValidator {
	validator := &BlockValidator{
		engine: engine,
		bc:     blockchain,
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
func (v *BlockValidator) ValidateBody(block block.IBlock) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash()) {
		return ErrKnownBlock
	}

	if hash := types.DeriveSha(transaction.Transactions(block.Transactions())); hash != block.TxHash() {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, block.TxHash())
	}

	if !v.bc.HasBlockAndState(block.ParentHash()) {
		if !v.bc.HasBlock(block.ParentHash()) {
			return ErrUnknownAncestor
		}
		return ErrPrunedAncestor
	}
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *BlockValidator) ValidateState(iBlock block.IBlock, statedb *statedb.StateDB, receipts block.Receipts, usedGas uint64) error {

	if iBlock.GasUsed() != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", iBlock.GasUsed(), usedGas)
	}
	receiptSha := types.DeriveSha(receipts)
	if receiptSha != iBlock.Header().(*block.Header).ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", iBlock.Header().(*block.Header).ReceiptHash, receiptSha)
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(); iBlock.Header().(*block.Header).Root != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", iBlock.Header().(*block.Header).Root, root)
	}
	return nil
}
