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

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/account"
	common "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/node"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules"
	"github.com/amazechain/amc/modules/ethdb/bitmapdb"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/amazechain/amc/turbo/backup"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/urfave/cli/v2"
	"math"
	"math/big"
	"os"
	"time"
)

var (
	exportCommand = &cli.Command{
		Name:        "export",
		Usage:       "Export AmazeChain data",
		ArgsUsage:   "",
		Description: ``,
		Subcommands: []*cli.Command{
			{
				Name:      "txs",
				Usage:     "Export All AmazeChain Transactions",
				ArgsUsage: "",
				Action:    exportTransactions,
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: ``,
			},
			{
				Name:      "balance",
				Usage:     "Export All AmazeChain account balance",
				ArgsUsage: "",
				Action:    exportBalance,
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: ``,
			},
			{
				Name:      "dbState",
				Usage:     "Export All MDBX Buckets disk space",
				ArgsUsage: "",
				Action:    exportDBState,
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: ``,
			},
			{
				Name:      "dbCopy",
				Usage:     "copy data from '--chaindata' to '--chaindata.to'",
				ArgsUsage: "",
				Action:    dbCopy,
				Flags: []cli.Flag{
					FromDataDirFlag,
					ToDataDirFlag,
				},
				Description: ``,
			},
			{
				Name:      "history",
				Usage:     "Export address history",
				ArgsUsage: "",
				Action:    exportHistory,
				Flags: []cli.Flag{
					DataDirFlag,
					AddressFlag,
				},
				Description: ``,
			},
		},
	}
)

func exportTransactions(ctx *cli.Context) error {

	stack, err := node.NewNode(ctx, &DefaultConfig)
	if err != nil {
		return err
	}

	blockChain := stack.BlockChain()
	defer stack.Close()

	currentBlock := blockChain.CurrentBlock()

	for i := uint64(0); i < currentBlock.Number64().Uint64(); i++ {

		block, err := blockChain.GetBlockByNumber(uint256.NewInt(i + 1))
		if err != nil {
			panic("cannot get block")
		}
		for _, transaction := range block.Transactions() {
			if transaction.To() == nil {
				continue
			}
			fmt.Printf("%d,%s,%s,%d,%s,%.2f\n",
				block.Number64().Uint64(),
				time.Unix(int64(block.Time()), 0).Format(time.RFC3339),
				transaction.From().Hex(),
				transaction.Nonce(),
				transaction.To().Hex(),
				new(big.Float).Quo(new(big.Float).SetInt(transaction.Value().ToBig()), new(big.Float).SetInt(big.NewInt(params.AMT))),
			)
		}
	}

	return nil
}

func exportBalance(ctx *cli.Context) error {

	stack, err := node.NewNode(ctx, &DefaultConfig)
	if err != nil {
		return err
	}
	db := stack.Database()
	defer stack.Close()

	roTX, err := db.BeginRo(ctx.Context)
	if err != nil {
		return err
	}
	defer roTX.Rollback()
	//kv.ReadAhead(ctx.Context, roTX.(kv.RoDB), atomic.NewBool(false), name, nil, 1<<32-1) // MaxUint32
	//
	srcC, err := roTX.Cursor("Account")
	if err != nil {
		return err
	}

	for k, v, err := srcC.First(); k != nil; k, v, err = srcC.Next() {
		if err != nil {
			return err
		}
		var acc account.StateAccount
		if err = acc.DecodeForStorage(v); err != nil {
			return err
		}

		fmt.Printf("%x, %d, %s, %.2f\n",
			k,
			acc.Nonce,
			acc.Balance.Hex(),
			new(big.Float).Quo(new(big.Float).SetInt(acc.Balance.ToBig()), new(big.Float).SetInt(big.NewInt(params.AMT))),
		)
	}

	return nil
}

func exportHistory(ctx *cli.Context) error {
	modules.AmcInit()
	kv.ChaindataTablesCfg = modules.AmcTableCfg

	exportAddress := common.HexToAddress(ctx.String(AddressFlag.Name))

	stack, err := node.NewNode(ctx, &DefaultConfig)
	if err != nil {
		return err
	}
	db := stack.Database()
	defer stack.Close()

	roTX, err := db.BeginRo(ctx.Context)
	if err != nil {
		return err
	}
	defer roTX.Rollback()

	index, err := bitmapdb.Get64(roTX, modules.AccountsHistory, exportAddress.Bytes(), 0, math.MaxUint32)
	if err != nil {
		return err
	}

	it := index.Iterator()

	for it.HasNext() {
		blockNr := it.Next()

		stateReader := state.NewPlainState(roTX, blockNr+1)
		ibs := state.New(stateReader)

		fmt.Printf("%d, %s, %d\n",
			blockNr,
			ibs.GetBalance(exportAddress).Hex(),
			ibs.GetNonce(exportAddress),
		)

	}

	return nil
}

func exportDBState(ctx *cli.Context) error {

	stack, err := node.NewNode(ctx, &DefaultConfig)
	if err != nil {
		return err
	}
	db := stack.Database()
	defer stack.Close()

	var tsize uint64

	roTX, err := db.BeginRo(ctx.Context)
	if err != nil {
		return err
	}
	defer roTX.Rollback()

	migrator, ok := roTX.(kv.BucketMigrator)
	if !ok {
		return fmt.Errorf("cannot open db as BucketMigrator")
	}
	Buckets, err := migrator.ListBuckets()
	for _, Bucket := range Buckets {
		size, _ := roTX.BucketSize(Bucket)
		tsize += size
		Cursor, _ := roTX.Cursor(Bucket)
		count, _ := Cursor.Count()
		Cursor.Close()
		if count != 0 {
			fmt.Printf("%30v count %10d size: %s \r\n", Bucket, count, common.StorageSize(size))
		}
	}
	fmt.Printf("total %s \n", common.StorageSize(tsize))
	return nil
}

func dbCopy(ctx *cli.Context) error {

	modules.AmcInit()
	kv.ChaindataTablesCfg = modules.AmcTableCfg

	fromChaindata := ctx.String(FromDataDirFlag.Name)
	toChaindata := ctx.String(ToDataDirFlag.Name)

	if f, err := os.Stat(fromChaindata); err != nil || !f.IsDir() {
		log.Errorf("fromChaindata do not exists or is not a dir, err: %s", err)
		return err
	}
	if f, err := os.Stat(toChaindata); err != nil || !f.IsDir() {
		log.Errorf("toChaindata do not exists or is not a dir, err: %s", err)
		return err
	}

	from, to := backup.OpenPair(fromChaindata, toChaindata, kv.ChainDB, 0)
	err := backup.Kv2kv(ctx.Context, from, to, nil, backup.ReadAheadThreads)
	if err != nil && !errors.Is(err, context.Canceled) {
		if !errors.Is(err, context.Canceled) {
			log.Error(err.Error())
		}
		return nil
	}

	return nil
}
