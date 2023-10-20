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
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/params/networkname"
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
	"github.com/amazechain/amc/conf"
)

var ErrGenesisNoConfig = errors.New("genesis has no chain configuration")

//go:embed allocs
var chainspecs embed.FS

func readGenesisAlloc(filename string) conf.GenesisAlloc {
	f, err := chainspecs.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("Could not open GenesisAlloc for %s: %v", filename, err))
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	spec := conf.GenesisAlloc{}
	err = decoder.Decode(&spec)
	if err != nil {
		panic(fmt.Sprintf("Could not parse GenesisAlloc for %s: %v", filename, err))
	}
	return spec
}

type GenesisBlock struct {
	Hash          string
	GenesisConfig *conf.Genesis
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
	if err := rawdb.WriteChainConfig(tx, block.Hash(), g.GenesisConfig.Config); err != nil {
		return nil, nil, err
	}
	// We support ethash/serenity for issuance (for now)
	//if g.Config.Consensus != params.EtHashConsensus {
	return block, statedb, nil
	//}
}

func (g *GenesisBlock) ToBlock() (*block2.Block, *state.IntraBlockState, error) {
	_ = g.GenesisConfig.Alloc //nil-check

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

		for address, account := range g.GenesisConfig.Alloc {
			b, ok := new(big.Int).SetString(account.Balance, 10)
			balance, _ := uint256.FromBig(b)
			if !ok {
				panic("overflow at genesis allocs")
			}
			statedb.AddBalance(address, balance)
			statedb.SetCode(address, account.Code)
			statedb.SetNonce(address, account.Nonce)
			for key, value := range account.Storage {
				k := key
				val := uint256.NewInt(0).SetBytes(value.Bytes())
				statedb.SetState(address, &k, *val)
			}
			if len(account.Code) > 0 || len(account.Storage) > 0 {
				statedb.SetIncarnation(address, state.FirstContractIncarnation)
			}
		}

		if err := statedb.FinalizeTx(g.GenesisConfig.Config.Rules(0), w); err != nil {
			panic(err)
		}
		root = statedb.GenerateRootHash()
	}()
	wg.Wait()

	var ExtraData []byte

	switch g.GenesisConfig.Config.Consensus {
	case params.CliqueConsensus, params.AposConsensu:

		var signers []types.Address

		for _, miner := range g.GenesisConfig.Miners {
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
	}

	head := &block2.Header{
		ParentHash:  g.GenesisConfig.ParentHash,
		Coinbase:    g.GenesisConfig.Coinbase,
		Root:        root,
		TxHash:      types.Hash{0},
		ReceiptHash: types.Hash{0},
		Difficulty:  g.GenesisConfig.Difficulty,
		Number:      uint256.NewInt(g.GenesisConfig.Number),
		GasLimit:    g.GenesisConfig.GasLimit,
		GasUsed:     g.GenesisConfig.GasUsed,
		Time:        uint64(g.GenesisConfig.Timestamp),
		Extra:       g.GenesisConfig.ExtraData,
		MixDigest:   g.GenesisConfig.Mixhash,
		Nonce:       block2.EncodeNonce(g.GenesisConfig.Nonce),
		BaseFee:     g.GenesisConfig.BaseFee,
	}
	head.Extra = ExtraData

	if g.GenesisConfig.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.GenesisConfig.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	if g.GenesisConfig.Config != nil && (g.GenesisConfig.Config.IsLondon(0)) {
		if g.GenesisConfig.BaseFee != nil {
			head.BaseFee = g.GenesisConfig.BaseFee
		} else {
			head.BaseFee = uint256.NewInt(params.InitialBaseFee)
		}
	}

	return block2.NewBlock(head, nil).(*block2.Block), statedb, nil
}

func (g *GenesisBlock) WriteGenesisState(tx kv.RwTx) (*block2.Block, *state.IntraBlockState, error) {
	block, statedb, err := g.ToBlock()
	if err != nil {
		return nil, nil, err
	}
	for address, account := range g.GenesisConfig.Alloc {
		if len(account.Code) > 0 || len(account.Storage) > 0 {
			// Special case for weird tests - inaccessible storage
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], state.FirstContractIncarnation)
			if err := tx.Put(modules.IncarnationMap, address[:], b[:]); err != nil {
				return nil, nil, err
			}
		}
	}
	if block.Number64().Uint64() != 0 {
		return nil, statedb, fmt.Errorf("can't commit genesis block with number > 0")
	}

	blockWriter := state.NewPlainStateWriter(tx, tx, g.GenesisConfig.Number)
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

func GenesisByChainName(chain string) *conf.Genesis {
	switch chain {
	case networkname.MainnetChainName:
		return mainnetGenesisBlock()
	case networkname.TestnetChainName:
		return testnetGenesisBlock()
	default:
		return nil
	}
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func mainnetGenesisBlock() *conf.Genesis {
	return &conf.Genesis{
		Config: params.MainnetChainConfig,
		Nonce:  0,
		Alloc:  readGenesisAlloc("mainnet.json"),
	}
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func testnetGenesisBlock() *conf.Genesis {
	return &conf.Genesis{
		Config: params.MainnetChainConfig,
		Nonce:  0,
		Alloc:  readGenesisAlloc("testnet.json"),
	}
}
