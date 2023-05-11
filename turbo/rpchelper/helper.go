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

package rpchelper

import (
	"fmt"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// unable to decode supplied params, or an invalid number of parameters
type nonCanonocalHashError struct{ hash types.Hash }

func (e nonCanonocalHashError) ErrorCode() int { return -32603 }

func (e nonCanonocalHashError) Error() string {
	return fmt.Sprintf("hash %x is not currently canonical", e.hash)
}

func GetBlockNumber(blockNrOrHash jsonrpc.BlockNumberOrHash, tx kv.Tx) (*uint256.Int, types.Hash, error) {
	if tx == nil {

	}
	return _GetBlockNumber(blockNrOrHash.RequireCanonical, blockNrOrHash, tx)
}

func GetCanonicalBlockNumber(blockNrOrHash jsonrpc.BlockNumberOrHash, tx kv.Tx) (*uint256.Int, types.Hash, error) {
	return _GetBlockNumber(true, blockNrOrHash, tx)
}

func _GetBlockNumber(requireCanonical bool, blockNrOrHash jsonrpc.BlockNumberOrHash, tx kv.Tx) (blockNumber *uint256.Int, hash types.Hash, err error) {

	var ok bool
	hash, ok = blockNrOrHash.Hash()
	if !ok {
		number := *blockNrOrHash.BlockNumber
		switch number {
		case jsonrpc.LatestBlockNumber:
			if blockNumber, err = GetLatestBlockNumber(tx); err != nil {
				return nil, types.Hash{}, err
			}
		case jsonrpc.EarliestBlockNumber:
			blockNumber = uint256.NewInt(0)
		case jsonrpc.FinalizedBlockNumber:
			blockNumber, err = GetFinalizedBlockNumber(tx)
			if err != nil {
				return nil, types.Hash{}, err
			}
		case jsonrpc.SafeBlockNumber:
			blockNumber, err = GetSafeBlockNumber(tx)
			if err != nil {
				return nil, types.Hash{}, err
			}
		case jsonrpc.PendingBlockNumber:
			//todo
			if blockNumber, err = GetLatestBlockNumber(tx); err != nil {
				return nil, types.Hash{}, err
			}
		default:
			blockNumber = uint256.NewInt(uint64(number.Int64()))
		}
		hash, err = rawdb.ReadCanonicalHash(tx, blockNumber.Uint64())
		if err != nil {
			return nil, types.Hash{}, err
		}
	} else {
		number := rawdb.ReadHeaderNumber(tx, hash)
		if number == nil {
			return nil, types.Hash{}, fmt.Errorf("block %x not found", hash)
		}
		blockNumber = uint256.NewInt(*number)

		ch, err := rawdb.ReadCanonicalHash(tx, blockNumber.Uint64())
		if err != nil {
			return nil, types.Hash{}, err
		}
		if requireCanonical && ch != hash {
			return nil, types.Hash{}, nonCanonocalHashError{hash}
		}
	}
	return blockNumber, hash, nil
}
