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

package apos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/turbo/rpchelper"
	"github.com/holiman/uint256"
	"strconv"

	"github.com/amazechain/amc/contracts/deposit"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
)

const maxSearchBlock = 1000

type MinedBlock struct {
	BlockNumber *uint256.Int `json:"blockNumber"`
	Timestamp   uint64       `json:"timestamp"`
	Reward      *uint256.Int `json:"reward"`
}
type MinedBlockResponse struct {
	MinedBlocks        []MinedBlock `json:"minedBlocks"`
	CurrentBlockNumber *uint256.Int `json:"currentBlockNumber"`
}

// API is a user facing jsonrpc API to allow controlling the signer and voting
// mechanisms of the proof-of-authority scheme.
type API struct {
	chain consensus.ChainReader
	apos  *APos
}

// GetSnapshot retrieves the state snapshot at a given block.
func (api *API) GetSnapshot(number *jsonrpc.BlockNumber) (*Snapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	var header block.IHeader
	if number == nil || *number == jsonrpc.LatestBlockNumber {
		header = api.chain.CurrentBlock().Header()
	} else {
		header = api.chain.GetHeaderByNumber(uint256.NewInt(uint64(number.Int64())))
	}
	// Ensure we have an actually valid block and return its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.apos.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash types.Hash) (*Snapshot, error) {
	header, _ := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.apos.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
}

// GetSigners retrieves the list of authorized signers at the specified block.
func (api *API) GetSigners(number *jsonrpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header block.IHeader
	if number == nil || *number == jsonrpc.LatestBlockNumber {
		header = api.chain.CurrentBlock().Header()
	} else {
		header = api.chain.GetHeaderByNumber(uint256.NewInt(uint64(number.Int64())))
	}
	// Ensure we have an actually valid block and return the signers from its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.apos.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}

	signers := snap.signers()
	ethSigners := make([]common.Address, len(signers))
	for i, signer := range signers {
		ethSigners[i] = *mvm_types.FromAmcAddress(&signer)
	}
	return ethSigners, nil
}

// GetSignersAtHash retrieves the list of authorized signers at the specified block.
func (api *API) GetSignersAtHash(hash types.Hash) ([]common.Address, error) {
	header, _ := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.apos.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	signers := snap.signers()
	ethSigners := make([]common.Address, len(signers))
	for i, signer := range signers {
		ethSigners[i] = *mvm_types.FromAmcAddress(&signer)
	}
	return ethSigners, nil
}

// Proposals returns the current proposals the node tries to uphold and vote on.
func (api *API) Proposals() map[common.Address]bool {
	api.apos.lock.RLock()
	defer api.apos.lock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.apos.proposals {
		proposals[*mvm_types.FromAmcAddress(&address)] = auth
	}
	return proposals
}

// Propose injects a new authorization proposal that the signer will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.apos.lock.Lock()
	defer api.apos.lock.Unlock()

	api.apos.proposals[*mvm_types.ToAmcAddress(&address)] = auth
}

// Discard drops a currently running proposal, stopping the signer from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.apos.lock.Lock()
	defer api.apos.lock.Unlock()

	delete(api.apos.proposals, *mvm_types.ToAmcAddress(&address))
}

type status struct {
	InturnPercent float64               `json:"inturnPercent"`
	SigningStatus map[types.Address]int `json:"sealerActivity"`
	NumBlocks     uint64                `json:"numBlocks"`
}

// Status returns the status of the last N blocks,
// - the number of active signers,
// - the number of signers,
// - the percentage of in-turn blocks
func (api *API) Status() (*status, error) {
	var (
		numBlocks = uint64(64)
		header    = api.chain.CurrentBlock().Header()
		diff      = uint64(0)
		optimals  = 0
	)
	snap, err := api.apos.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	var (
		signers = snap.signers()
		end     = header.Number64().Uint64()
		start   = end - numBlocks
	)
	if numBlocks > end {
		start = 1
		numBlocks = end - start
	}
	signStatus := make(map[types.Address]int)
	for _, s := range signers {
		signStatus[s] = 0
	}
	for n := start; n < end; n++ {
		h := api.chain.GetHeaderByNumber(uint256.NewInt(n))
		block := api.chain.GetBlock(h.Hash(), n)
		if h == nil {
			return nil, fmt.Errorf("missing block %d", n)
		}
		if block.Difficulty().Cmp(diffInTurn) == 0 {
			optimals++
		}
		diff += block.Difficulty().Uint64()
		sealer, err := api.apos.Author(h)
		if err != nil {
			return nil, err
		}
		signStatus[sealer]++
	}
	return &status{
		InturnPercent: float64(100*optimals) / float64(numBlocks),
		SigningStatus: signStatus,
		NumBlocks:     numBlocks,
	}, nil
}

type blockNumberOrHashOrRLP struct {
	*jsonrpc.BlockNumberOrHash
	RLP hexutil.Bytes `json:"rlp,omitempty"`
}

func (sb *blockNumberOrHashOrRLP) UnmarshalJSON(data []byte) error {
	bnOrHash := new(jsonrpc.BlockNumberOrHash)
	// Try to unmarshal bNrOrHash
	if err := bnOrHash.UnmarshalJSON(data); err == nil {
		sb.BlockNumberOrHash = bnOrHash
		return nil
	}
	// Try to unmarshal RLP
	var input string
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	blob, err := hexutil.Decode(input)
	if err != nil {
		return err
	}
	sb.RLP = blob
	return nil
}

// GetSigner returns the signer for a specific Apos block.
// Can be called with either a blocknumber, blockhash or an rlp encoded blob.
// The RLP encoded blob can either be a block or a header.
func (api *API) GetSigner(rlpOrBlockNr *blockNumberOrHashOrRLP) (types.Address, error) {
	if len(rlpOrBlockNr.RLP) == 0 {
		blockNrOrHash := rlpOrBlockNr.BlockNumberOrHash
		var header block.IHeader
		if blockNrOrHash == nil {
			header = api.chain.CurrentBlock().Header()
		} else if hash, ok := blockNrOrHash.Hash(); ok {
			header, _ = api.chain.GetHeaderByHash(hash)
		} else if number, ok := blockNrOrHash.Number(); ok {
			header = api.chain.GetHeaderByNumber(uint256.NewInt(uint64(number.Int64())))
		}
		if header == nil {
			return types.Address{}, fmt.Errorf("missing block %v", blockNrOrHash.String())
		}
		return api.apos.Author(header)
	}

	return types.Address{}, fmt.Errorf("do not support rlp")
}

// GetRewards
func (api *API) GetRewards(address common.Address, from jsonrpc.BlockNumberOrHash, to jsonrpc.BlockNumberOrHash) (resp *RewardResponse, err error) {

	var (
		resolvedFromBlock *uint256.Int
		resolvedToBlock   *uint256.Int
	)

	api.apos.db.View(context.Background(), func(tx kv.Tx) error {
		resolvedFromBlock, _, err = rpchelper.GetCanonicalBlockNumber(from, tx)
		resolvedToBlock, _, err = rpchelper.GetCanonicalBlockNumber(to, tx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	rewardService := newReward(api.apos.config, api.apos.chainConfig)
	resp, err = rewardService.GetRewards(*mvm_types.ToAmcAddress(&address), resolvedFromBlock, resolvedToBlock, api.chain.GetBlockByNumber)

	return resp, err
}

// GetRewards
func (api *API) GetDepositInfo(address common.Address) (*deposit.Info, error) {

	addr := *mvm_types.ToAmcAddress(&address)

	info := new(deposit.Info)
	var err error

	api.apos.db.View(context.Background(), func(tx kv.Tx) error {
		info = deposit.GetDepositInfo(tx, addr)
		return nil
	})

	return info, err
}

// GetRewards todo:needs check
func (api *API) GetBlockRewards(blockNr jsonrpc.BlockNumberOrHash) (resp []*block.Reward, err error) {
	var (
		resolvedBlockNr *uint256.Int
		hash            types.Hash
	)
	api.apos.db.View(context.Background(), func(tx kv.Tx) error {
		resolvedBlockNr, hash, err = rpchelper.GetCanonicalBlockNumber(blockNr, tx)
		if err != nil {
			return err
		}
		//header := rawdb.ReadHeader(tx, hash, resolvedBlockNr.Uint64())
		body := rawdb.ReadBlock(tx, hash, resolvedBlockNr.Uint64())
		if body == nil {
			err = errors.New("cannot find block body")
			return err
		}
		resp = body.Body().Reward()
		return nil
	})
	return resp, err
}

// getHeader search header by BlockNumberOrHash
func (api *API) getHeader(from jsonrpc.BlockNumberOrHash) (currentHeader block.IHeader) {
	//
	if blockNr, ok := from.Number(); ok {
		if blockNr == jsonrpc.LatestBlockNumber || blockNr == jsonrpc.PendingBlockNumber {
			currentHeader = api.chain.CurrentBlock().Header()
		} else if blockNr < jsonrpc.EarliestBlockNumber {
			currentHeader = api.chain.GetHeaderByNumber(uint256.NewInt(0))
		} else {
			currentHeader = api.chain.GetHeaderByNumber(uint256.NewInt(uint64(blockNr.Int64())))
		}
	} else if hash, ok := from.Hash(); ok {
		currentHeader, _ = api.chain.GetHeaderByHash(hash)
	} else {
		currentHeader = api.chain.CurrentBlock().Header()
	}
	return
}

// GetTasks
func (api *API) GetMinedBlock(address common.Address, from jsonrpc.BlockNumberOrHash, wantCount uint64) (*MinedBlockResponse, error) {

	addr := *mvm_types.ToAmcAddress(&address)
	var (
		err           error
		searchCount   int
		findCount     uint64
		currentHeader block.IHeader
		currentBlock  block.IBlock
		depositInfo   *deposit.Info
	)

	api.apos.db.View(context.Background(), func(tx kv.Tx) error {
		depositInfo = deposit.GetDepositInfo(tx, addr)
		return nil
	})
	if depositInfo == nil {
		return nil, fmt.Errorf("address do not have depositInfo")
	}
	//
	currentHeader = api.getHeader(from)
	if currentHeader == nil {
		return nil, err
	}

	//
	currentBlock = api.chain.GetBlock(currentHeader.Hash(), currentHeader.Number64().Uint64())
	searchCount = 0
	findCount = 0
	minedBlocks := make([]MinedBlock, 0)

	for i := currentHeader.Number64().Uint64(); i >= 0; i-- {
		verifier := currentBlock.Body().Verifier()
		for _, verify := range verifier {
			if addr == verify.Address {
				minedBlocks = append(minedBlocks, MinedBlock{
					BlockNumber: currentBlock.Number64(),
					Timestamp:   currentBlock.Time(),
					Reward:      depositInfo.RewardPerBlock,
				})
				findCount++
				if findCount >= wantCount {
					goto Finish
				}
			}
		}
		searchCount++
		if searchCount >= maxSearchBlock {
			break
		}
		currentHeader = api.chain.GetHeaderByNumber(new(uint256.Int).SubUint64(currentBlock.Number64(), 1))
		if currentHeader == nil {
			break
		}
		currentBlock = api.chain.GetBlock(currentHeader.Hash(), currentHeader.Number64().Uint64())
	}

Finish:
	return &MinedBlockResponse{
		MinedBlocks:        minedBlocks,
		CurrentBlockNumber: currentBlock.Number64(),
	}, nil
}

func (api *API) DebugDBString(dbname string, key string) (string, error) {
	tx, err := api.apos.db.BeginRo(context.TODO())
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	val, err := tx.GetOne(dbname, []byte(key))
	if err != nil {
		return "", err
	}
	return strconv.Quote(string(val)), nil
}
