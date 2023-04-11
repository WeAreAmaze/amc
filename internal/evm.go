// Copyright 2016 The go-ethereum Authors
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

package internal

import (
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/vm/evmtypes"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
)

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(header *block.Header, blockHashFunc func(n uint64) types.Hash, engine consensus.Engine, author *types.Address) evmtypes.BlockContext {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	var beneficiary types.Address
	if author == nil {
		beneficiary, _ = engine.Author(header) // Ignore error, we're past header validation
	} else {
		beneficiary = *author
	}
	var baseFee uint256.Int
	if header.BaseFee != nil {
		overflow := baseFee.SetFromBig(header.BaseFee.ToBig())
		if overflow {
			panic(fmt.Errorf("header.BaseFee higher than 2^256-1"))
		}
	}

	var prevRandDao *types.Hash
	if header.Difficulty.Cmp(uint256.NewInt(0)) == 0 {
		// EIP-4399. We use SerenityDifficulty (i.e. 0) as a telltale of Proof-of-Stake blocks.
		prevRandDao = &header.MixDigest
	}

	var transferFunc evmtypes.TransferFunc
	if engine != nil && engine.Type() == params.BorConsensus {
		transferFunc = BorTransfer
	} else {
		transferFunc = Transfer
	}

	return evmtypes.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    transferFunc,
		GetHash:     blockHashFunc,
		Coinbase:    beneficiary,
		BlockNumber: header.Number.Uint64(),
		Time:        header.Time,
		Difficulty:  header.Difficulty.ToBig(),
		BaseFee:     &baseFee,
		GasLimit:    header.GasLimit,
		PrevRanDao:  prevRandDao,
	}
}

// NewEVMTxContext creates a new transaction context for a single transaction.
func NewEVMTxContext(msg Message) evmtypes.TxContext {
	return evmtypes.TxContext{
		Origin:   msg.From(),
		GasPrice: msg.GasPrice(),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *block.Header, getHeader func(hash types.Hash, number uint64) *block.Header) func(n uint64) types.Hash {
	// Cache will initially contain [refHash.parent],
	// Then fill up with [refHash.p, refHash.pp, refHash.ppp, ...]
	var cache []types.Hash

	return func(n uint64) types.Hash {
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, ref.ParentHash)
		}
		if idx := ref.Number.Uint64() - n - 1; idx < uint64(len(cache)) {
			return cache[idx]
		}
		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]
		lastKnownNumber := ref.Number.Uint64() - uint64(len(cache))

		for {
			header := getHeader(lastKnownHash, lastKnownNumber)
			if header == nil {
				break
			}
			cache = append(cache, header.ParentHash)
			lastKnownHash = header.ParentHash
			lastKnownNumber = header.Number.Uint64() - 1
			if n == lastKnownNumber {
				return lastKnownHash
			}
		}
		return types.Hash{}
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db evmtypes.IntraBlockState, addr types.Address, amount *uint256.Int) bool {
	return !db.GetBalance(addr).Lt(amount)
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db evmtypes.IntraBlockState, sender, recipient types.Address, amount *uint256.Int, bailout bool) {
	if !bailout {
		db.SubBalance(sender, amount)
	}
	db.AddBalance(recipient, amount)
}

// BorTransfer transfer in Bor
func BorTransfer(db evmtypes.IntraBlockState, sender, recipient types.Address, amount *uint256.Int, bailout bool) {
	// get inputs before
	//input1 := db.GetBalance(sender).Clone()
	//input2 := db.GetBalance(recipient).Clone()

	if !bailout {
		db.SubBalance(sender, amount)
	}
	db.AddBalance(recipient, amount)

	//// get outputs after
	//output1 := db.GetBalance(sender).Clone()
	//output2 := db.GetBalance(recipient).Clone()
	//
	//// add transfer log
	//AddTransferLog(db, sender, recipient, amount, input1, input2, output1, output2)
}

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus.Engine

	// GetHeader returns the header corresponding to the hash/number argument pair.
	GetHeader(types.Hash, uint64) *block.Header
}
