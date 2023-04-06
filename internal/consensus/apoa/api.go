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

package apoa

import (
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/holiman/uint256"
)

// API is a user facing jsonrpc API to allow controlling the signer and voting
// mechanisms of the proof-of-authority scheme.
type API struct {
	chain consensus.ChainReader
	apoa  *Apoa
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
	return api.apoa.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash types.Hash) (*Snapshot, error) {
	header, _ := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.apoa.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
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
	snap, err := api.apoa.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
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
	snap, err := api.apoa.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
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
	api.apoa.lock.RLock()
	defer api.apoa.lock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.apoa.proposals {
		proposals[*mvm_types.FromAmcAddress(&address)] = auth
	}
	return proposals
}

// Propose injects a new authorization proposal that the signer will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.apoa.lock.Lock()
	defer api.apoa.lock.Unlock()

	api.apoa.proposals[*mvm_types.ToAmcAddress(&address)] = auth
}

// Discard drops a currently running proposal, stopping the signer from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.apoa.lock.Lock()
	defer api.apoa.lock.Unlock()

	delete(api.apoa.proposals, *mvm_types.ToAmcAddress(&address))
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
	snap, err := api.apoa.snapshot(api.chain, header.Number64().Uint64(), header.Hash(), nil)
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
		sealer, err := api.apoa.Author(h)
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

// GetSigner returns the signer for a specific apoa block.
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
		return api.apoa.Author(header)
	}

	return types.Address{}, fmt.Errorf("do not support rlp")
}
