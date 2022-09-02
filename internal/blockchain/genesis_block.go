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

package blockchain

import (
	"encoding/json"
	"fmt"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/modules/statedb"
	"math/big"
)

type GenesisBlock struct {
	Hash string
	*conf.GenesisBlockConfig
}

func NewGenesisBlockFromConfig(config *conf.GenesisBlockConfig, db db.IDatabase, changeDB kv.RwDB) (block2.IBlock, error) {

	state := statedb.NewStateDB(types.Hash{}, db, changeDB)

	for _, a := range config.Alloc {
		addr, err := types.HexToString(a.Address)
		if err != nil {
			return nil, err
		}

		b, ok := new(big.Int).SetString(a.Balance, 10)
		balance, _ := types.FromBig(b)
		if !ok {
			return nil, fmt.Errorf("failed to get alloc[%s] balance[%s]", a.Address, a.Balance)
		}

		state.CreateAccount(addr)
		state.SetBalance(addr, balance)
	}

	root := state.IntermediateRoot()

	//todo
	engine, err := json.Marshal(config.Engine)
	if err != nil {
		return nil, err
	}

	engineHash := types.BytesToHash(engine)

	header := block2.Header{
		ParentHash:  types.Hash{0},
		Coinbase:    types.Address{0},
		Root:        root,
		TxHash:      types.Hash{0},
		ReceiptHash: types.Hash{0},
		Difficulty:  types.NewInt64(0),
		Number:      types.NewInt64(0),
		GasLimit:    500000000,
		GasUsed:     0,
		Time:        uint64(config.Timestamp),
		Extra:       engineHash[:],
		MixDigest:   types.Hash{0},
		Nonce:       block2.BlockNonce{0},
		BaseFee:     types.NewInt64(0),
	}

	block := block2.NewBlock(&header, nil)

	_, err = state.Commit(types.NewInt64(0))

	return block, err
}
