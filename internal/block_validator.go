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
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type BlockValidator struct {
	bc     *BlockChain      // Canonical block chain
	engine consensus.Engine // Consensus engine used for validating
	config *params.ChainConfig
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(config *params.ChainConfig, blockchain *BlockChain, engine consensus.Engine) *BlockValidator {
	validator := &BlockValidator{
		engine: engine,
		bc:     blockchain,
		config: config,
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
func (v *BlockValidator) ValidateBody(b block.IBlock) error {
	// Check Signature valid
	vfs := b.Body().Verifier()
	addrs := make([]types.Address, len(vfs))
	ss := make([]bls.PublicKey, len(vfs))
	for i, p := range vfs {
		addrs[i] = p.Address
		blsP, err := bls.PublicKeyFromBytes(p.PublicKey[:])
		if nil != err {
			return err
		}
		ss[i] = blsP
	}

	if v.config.IsBeijing(b.Number64().Uint64()) {
		header := b.Header().(*block.Header)
		sig, err := bls.SignatureFromBytes(header.Signature[:])
		if nil != err {
			return err
		}
		if !sig.FastAggregateVerify(ss, header.Root) {
			log.Warn("AggSignature verify falied", "blockNr", b.Number64().Uint64(), "Signature", hexutil.Encode(header.Signature[:]), "Root", hexutil.Encode(header.Root[:]))
			for i, addr := range addrs {
				log.Warn("", "address", addr.String(), "publicKey", hexutil.Encode(ss[i].Marshal()))
			}
			return fmt.Errorf("AggSignature verify falied")
		}
	}

	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(b.Hash(), b.Number64().Uint64()) {
		return ErrKnownBlock
	}

	if hash := DeriveSha(transaction.Transactions(b.Transactions())); hash != b.TxHash() {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, b.TxHash())
	}

	if !v.bc.HasBlockAndState(b.ParentHash(), b.Number64().Uint64()-1) {
		if !v.bc.HasBlock(b.ParentHash(), b.Number64().Uint64()-1) {
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
func (v *BlockValidator) ValidateState(iBlock block.IBlock, statedb *state.IntraBlockState, receipts block.Receipts, usedGas uint64) error {
	header := iBlock.Header().(*block.Header)
	if header.GasUsed != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", header.GasUsed, usedGas)
	}

	rbloom := block.CreateBloom(receipts)
	if rbloom != header.Bloom {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", header.Bloom, rbloom)
	}

	receiptSha := DeriveSha(receipts)
	if receiptSha != header.ReceiptHash {
		for i, tx := range iBlock.Body().Transactions() {
			log.Warn("tx", "index", i, "from", tx.From(), "GasUsed", receipts[i].GasUsed)
			for index2, l := range receipts[i].Logs {
				log.Warn("tx logs", "index", index2, "address", l.Address, "topic", l.Topics[0], "data", hexutil.Encode(l.Data))
			}

		}
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", header.ReceiptHash, receiptSha)
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	// TODO 替换 emptyroot
	if root := statedb.IntermediateRoot(); header.StateRoot() != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Root, root)
	}
	return nil
}
