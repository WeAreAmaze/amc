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
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
)

type SystemCall func(contract types.Address, data []byte) ([]byte, error)
type Call func(contract types.Address, data []byte) ([]byte, error)

// ChainHeaderReader defines a small collection of methods needed to access the local
// blockchain during header verification.
type ChainHeaderReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// CurrentBlock retrieves the current header from the local chain.
	CurrentBlock() block.IBlock

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash types.Hash, number *uint256.Int) block.IHeader

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number *uint256.Int) block.IHeader

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash types.Hash) (block.IHeader, error)

	// GetTd retrieves the total difficulty from the database by hash and number.
	GetTd(types.Hash, *uint256.Int) *uint256.Int

	GetBlockByNumber(number *uint256.Int) (block.IBlock, error)

	GetDepositInfo(address types.Address) (*uint256.Int, *uint256.Int)
	GetAccountRewardUnpaid(account types.Address) (*uint256.Int, error)
	RewardsOfEpoch(number *uint256.Int, lastEpoch *uint256.Int) (map[types.Address]*uint256.Int, error)
}

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {
	ChainHeaderReader

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash types.Hash, number uint64) block.IBlock
	GetBlockByNumber(number *uint256.Int) (block.IBlock, error)
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	EngineReader

	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	//Author(header block.IHeader) (types.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainHeaderReader, header block.IHeader, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainHeaderReader, headers []block.IHeader, seals []bool) (chan<- struct{}, <-chan error)

	// VerifyUncles verifies that the given block's uncles conform to the consensus
	// rules of a given engine.
	VerifyUncles(chain ChainReader, block block.IBlock) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainHeaderReader, header block.IHeader) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// but does not assemble the block.
	//
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	Finalize(chain ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction,
		uncles []block.IHeader) ([]*block.Reward, map[types.Address]*uint256.Int, error)

	// FinalizeAndAssemble runs any post-transaction state modifications (e.g. block
	// rewards) and assembles the final block.
	//
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	FinalizeAndAssemble(chain ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction,
		uncles []block.IHeader, receipts []*block.Receipt) (block.IBlock, []*block.Reward, map[types.Address]*uint256.Int, error)

	//Rewards(tx kv.RwTx, header block.IHeader, state *state.IntraBlockState, setRewards bool) ([]*block.Reward, error)

	// Seal generates a new sealing request for the given input block and pushes
	// the result into the given channel.
	//
	// Note, the method returns immediately and will send the result async. More
	// than one result may also be returned depending on the consensus algorithm.
	Seal(chain ChainHeaderReader, block block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error

	// SealHash returns the hash of a block prior to it being sealed.
	SealHash(header block.IHeader) types.Hash

	// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
	// that a new block should have.
	CalcDifficulty(chain ChainHeaderReader, time uint64, parent block.IHeader) *uint256.Int

	//Type() params.ConsensusType

	// APIs returns the RPC APIs this consensus engine provides.
	APIs(chain ChainReader) []jsonrpc.API

	// Close terminates any background threads maintained by the consensus engine.
	Close() error
}

// EngineReader are read-only methods of the consensus engine
// All of these methods should have thread-safe implementations
type EngineReader interface {
	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header block.IHeader) (types.Address, error)

	// Service transactions are free and don't pay baseFee after EIP-1559
	IsServiceTransaction(sender types.Address, syscall SystemCall) bool

	Type() params.ConsensusType
}

var (
	SystemAddress = types.HexToAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")
)
