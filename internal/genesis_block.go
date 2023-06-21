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

package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/amazechain/amc/conf"
	"math/big"
	"sync"

	"github.com/amazechain/amc/modules"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/c2h5oh/datasize"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"

	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
)

var ErrGenesisNoConfig = errors.New("genesis has no chain configuration")

type GenesisBlock struct {
	Hash               string
	GenesisBlockConfig *conf.GenesisBlockConfig
	//	ChainConfig *params.ChainConfig
}

func (g *GenesisBlock) Write(tx kv.RwTx) (*block2.Block, *state.IntraBlockState, error) {
	block, statedb, err2 := g.WriteGenesisState(tx)
	if err2 != nil {
		return block, statedb, err2
	}

	if err := rawdb.WriteTd(tx, block.Hash(), block.Number64().Uint64(), uint256.NewInt(0)); err != nil {
		return nil, nil, err
	}
	if err := rawdb.WriteBlock(tx, block); err != nil {
		return nil, nil, err
	}
	//if err := rawdb.TxNums.WriteForGenesis(tx, 1); err != nil {
	//	return nil, nil, err
	//}
	if err := rawdb.WriteReceipts(tx, block.Number64().Uint64(), nil); err != nil {
		return nil, nil, err
	}

	if err := rawdb.WriteCanonicalHash(tx, block.Hash(), block.Number64().Uint64()); err != nil {
		return nil, nil, err
	}

	rawdb.WriteHeadBlockHash(tx, block.Hash())
	if err := rawdb.WriteHeadHeaderHash(tx, block.Hash()); err != nil {
		return nil, nil, err
	}
	if err := rawdb.WriteChainConfig(tx, block.Hash(), g.GenesisBlockConfig.Config); err != nil {
		return nil, nil, err
	}
	// We support ethash/serenity for issuance (for now)
	//if g.Config.Consensus != params.EtHashConsensus {
	return block, statedb, nil
	//}
}

func (g *GenesisBlock) ToBlock() (*block2.Block, *state.IntraBlockState, error) {
	_ = g.GenesisBlockConfig.Alloc //nil-check

	var root types.Hash
	var statedb *state.IntraBlockState
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { // we may run inside write tx, can't open 2nd write tx in same goroutine
		defer wg.Done()
		//TODO
		tmpDB := mdbx.NewMDBX(nil).InMem("").MapSize(2 * datasize.GB).MustOpen()
		defer tmpDB.Close()
		tx, err := tmpDB.BeginRw(context.Background())
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()
		r, w := state.NewPlainStateReader(tx), state.NewPlainStateWriter(tx, tx, 0)
		statedb = state.New(r)

		for _, a := range g.GenesisBlockConfig.Alloc {
			addr, err := types.HexToString(a.Address)
			if err != nil {
				panic(err)
			}

			b, ok := new(big.Int).SetString(a.Balance, 10)
			balance, _ := uint256.FromBig(b)
			if !ok {
				panic("overflow at genesis allocs")
			}

			statedb.AddBalance(addr, balance)
			statedb.SetCode(addr, a.Code)
			statedb.SetNonce(addr, a.Nonce)
			for key, value := range a.Storage {
				k := key
				val := uint256.NewInt(0).SetBytes(value.Bytes())
				statedb.SetState(addr, &k, *val)
			}
			if len(a.Code) > 0 || len(a.Storage) > 0 {
				statedb.SetIncarnation(addr, state.FirstContractIncarnation)
			}
		}

		if err := statedb.FinalizeTx(g.GenesisBlockConfig.Config.Rules(0), w); err != nil {
			panic(err)
		}
		root = statedb.GenerateRootHash()
	}()
	wg.Wait()

	var ExtraData []byte

	switch g.GenesisBlockConfig.Config.Engine.EngineName {
	case "APoaEngine", "APosEngine":

		var signers []types.Address

		for _, miner := range g.GenesisBlockConfig.Miners {
			addr, err := types.HexToString(miner)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid miner:  %s", miner)
			}
			signers = append(signers, addr)
		}
		// Sort the signers and embed into the extra-data section
		for i := 0; i < len(signers); i++ {
			for j := i + 1; j < len(signers); j++ {
				if bytes.Compare(signers[i][:], signers[j][:]) > 0 {
					signers[i], signers[j] = signers[j], signers[i]
				}
			}
		}
		ExtraData = make([]byte, 32+len(signers)*types.AddressLength+65)
		for i, signer := range signers {
			copy(ExtraData[32+i*types.AddressLength:], signer[:])
		}

	default:
		return nil, nil, fmt.Errorf("invalid engine name %s", g.GenesisBlockConfig.Config.Engine.Etherbase)
	}

	head := &block2.Header{
		ParentHash:  g.GenesisBlockConfig.ParentHash,
		Coinbase:    g.GenesisBlockConfig.Coinbase,
		Root:        root,
		TxHash:      types.Hash{0},
		ReceiptHash: types.Hash{0},
		Difficulty:  g.GenesisBlockConfig.Difficulty,
		Number:      uint256.NewInt(g.GenesisBlockConfig.Number),
		GasLimit:    g.GenesisBlockConfig.Config.Engine.GasCeil,
		GasUsed:     g.GenesisBlockConfig.GasUsed,
		Time:        uint64(g.GenesisBlockConfig.Timestamp),
		Extra:       g.GenesisBlockConfig.ExtraData,
		MixDigest:   g.GenesisBlockConfig.Mixhash,
		Nonce:       block2.EncodeNonce(g.GenesisBlockConfig.Nonce),
		BaseFee:     g.GenesisBlockConfig.BaseFee,
	}
	head.Extra = ExtraData

	if g.GenesisBlockConfig.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.GenesisBlockConfig.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	if g.GenesisBlockConfig.Config != nil && (g.GenesisBlockConfig.Config.IsLondon(0)) {
		if g.GenesisBlockConfig.BaseFee != nil {
			head.BaseFee = g.GenesisBlockConfig.BaseFee
		} else {
			head.BaseFee = uint256.NewInt(params.InitialBaseFee)
		}
	}

	//var withdrawals []*types.Withdrawal
	//if g.Config != nil && (g.Config.IsShanghai(g.Timestamp)) {
	//	withdrawals = []*types.Withdrawal{}
	//}
	return block2.NewBlock(head, nil).(*block2.Block), statedb, nil
}

func (g *GenesisBlock) WriteGenesisState(tx kv.RwTx) (*block2.Block, *state.IntraBlockState, error) {
	block, statedb, err := g.ToBlock()
	if err != nil {
		return nil, nil, err
	}
	for _, account := range g.GenesisBlockConfig.Alloc {
		if len(account.Code) > 0 || len(account.Storage) > 0 {
			// Special case for weird tests - inaccessible storage
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], state.FirstContractIncarnation)
			addr, err := types.HexToString(account.Address)
			if err != nil {
				panic(err)
			}
			if err := tx.Put(modules.IncarnationMap, addr[:], b[:]); err != nil {
				return nil, nil, err
			}
		}
	}
	if block.Number64().Uint64() != 0 {
		return nil, statedb, fmt.Errorf("can't commit genesis block with number > 0")
	}

	blockWriter := state.NewPlainStateWriter(tx, tx, g.GenesisBlockConfig.Number)
	if err := statedb.CommitBlock(&params.Rules{}, blockWriter); err != nil {
		return nil, statedb, fmt.Errorf("cannot write state: %w", err)
	}
	if err := blockWriter.WriteChangeSets(); err != nil {
		return nil, statedb, fmt.Errorf("cannot write change sets: %w", err)
	}
	if err := blockWriter.WriteHistory(); err != nil {
		return nil, statedb, fmt.Errorf("cannot write history: %w", err)
	}

	return block, statedb, nil
}

//func NewGenesisBlockFromConfig(config *conf.GenesisBlockConfig, engineName string, tx kv.RwTx) (*block2.Block, error) {
//
//	state := statedb.NewStateDB(types.Hash{}, tx)
//
//	for _, a := range config.Alloc {
//		addr, err := types.HexToString(a.Address)
//		if err != nil {
//			return nil, err
//		}
//
//		b, ok := new(big.Int).SetString(a.Balance, 10)
//		balance, _ := types.FromBig(b)
//		if !ok {
//			return nil, fmt.Errorf("failed to get alloc[%s] balance[%s]", a.Address, a.Balance)
//		}
//
//		state.CreateAccount(addr)
//		state.SetBalance(addr, balance)
//	}
//
//	root := state.IntermediateRoot()
//
//	var ExtraData []byte
//
//	switch engineName {
//	case "APoaEngine":
//
//		var signers []types.Address
//
//		for _, miner := range config.Miners {
//			addr, err := types.HexToString(miner)
//			if err != nil {
//				return nil, fmt.Errorf("invalid miner:  %s", miner)
//			}
//			signers = append(signers, addr)
//		}
//		// Sort the signers and embed into the extra-data section
//		for i := 0; i < len(signers); i++ {
//			for j := i + 1; j < len(signers); j++ {
//				if bytes.Compare(signers[i][:], signers[j][:]) > 0 {
//					signers[i], signers[j] = signers[j], signers[i]
//				}
//			}
//		}
//		ExtraData = make([]byte, 32+len(signers)*types.AddressLength+65)
//		for i, signer := range signers {
//			copy(ExtraData[32+i*types.AddressLength:], signer[:])
//		}
//
//	default:
//		return nil, fmt.Errorf("invalid engine name %s", engineName)
//	}
//
//	header := block2.Header{
//		ParentHash:  types.Hash{0},
//		Coinbase:    types.Address{0},
//		Root:        root,
//		TxHash:      types.Hash{0},
//		ReceiptHash: types.Hash{0},
//		Difficulty:  uint256.NewInt(0),
//		Number:      uint256.NewInt(0),
//		GasLimit:    500000000,
//		GasUsed:     0,
//		Time:        uint64(config.Timestamp),
//		Extra:       ExtraData,
//		MixDigest:   types.Hash{0},
//		Nonce:       block2.BlockNonce{0},
//		BaseFee:     uint256.NewInt(0),
//	}
//
//	block := block2.NewBlock(&header, nil)
//
//	_, err := state.Commit(uint256.NewInt(0))
//
//	return block, err
//}
