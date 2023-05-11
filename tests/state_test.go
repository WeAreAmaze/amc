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

package tests

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/modules/state"
	"github.com/c2h5oh/datasize"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"math/big"
	"testing"
)

//go:embed  allocs
var allocs embed.FS

func ReadGenesis(filename string) *conf.GenesisBlockConfig {
	f, err := allocs.Open(filename)
	if err != nil {
		panic("%s not found, use default genesis")
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	gc := new(conf.GenesisBlockConfig)
	err = decoder.Decode(gc)
	if err != nil {
		panic(fmt.Sprintf("Could not parse genesis preallocation for %s: %v", filename, err))
	}
	return gc
}

func TestStateRoot(t *testing.T) {

	tmpDB := mdbx.NewMDBX(nil).InMem("").MapSize(2 * datasize.GB).MustOpen()
	defer tmpDB.Close()
	tx, err := tmpDB.BeginRw(context.Background())
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	r, roop := state.NewPlainStateReader(tx), state.NewNoopWriter()
	statedb := state.New(r)

	GenesisBlockConfig := ReadGenesis("allocs/genesis_online.json")
	for _, a := range GenesisBlockConfig.Alloc {
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

	if err := statedb.FinalizeTx(GenesisBlockConfig.Config.Rules(0), roop); err != nil {
		panic(err)
	}

	for i := 0; i < 1000; i++ {
		root := statedb.GenerateRootHash().Hex()
		if root != "0x1995d438cfe70662e278ed8b0d92c154f152e418b7d86c616fc538d969ba5eca" {
			t.Error("not same", root)
		}
	}
}
