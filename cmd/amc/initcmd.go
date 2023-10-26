package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/cmd/utils"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/node"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/urfave/cli/v2"
	"os"
)

var (
	initCommand = &cli.Command{
		Name:      "init",
		Usage:     "Bootstrap and initialize a new genesis block",
		ArgsUsage: "<genesisPath>",
		Action:    initGenesis,
		Flags: []cli.Flag{
			DataDirFlag,
		},
		Description: `
The init command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
	}
)

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(cliCtx *cli.Context) error {
	var (
		err          error
		genesisBlock *block.Block
	)

	// Make sure we have a valid genesis JSON
	genesisPath := cliCtx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis JSON file")
	}

	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	genesis := new(conf.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	chaindb, err := node.OpenDatabase(&DefaultConfig, nil, kv.ChainDB.String())
	if err != nil {
		utils.Fatalf("Failed to open database: %v", err)
	}
	defer chaindb.Close()

	if err := chaindb.Update(context.TODO(), func(tx kv.RwTx) error {
		storedHash, err := rawdb.ReadCanonicalHash(tx, 0)
		if err != nil {
			return err
		}

		if storedHash != (types.Hash{}) {
			return fmt.Errorf("genesis state are already exists")

		}
		genesisBlock, err = node.WriteGenesisBlock(tx, genesis)
		if nil != err {
			return err
		}
		return nil
	}); err != nil {
		utils.Fatalf("Failed to wrote genesis state to database: %w", err)
	}
	log.Info("Successfully wrote genesis state", "hash", genesisBlock.Hash())
	return nil
}
