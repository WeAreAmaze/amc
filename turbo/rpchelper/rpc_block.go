package rpchelper

import (
	"fmt"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

func GetLatestBlockNumber(tx kv.Tx) (*uint256.Int, error) {
	current := rawdb.ReadCurrentBlock(tx)
	if current == nil {
		return nil, fmt.Errorf("cannot get current block")
	}
	return current.Number64(), nil
}

func GetFinalizedBlockNumber(tx kv.Tx) (*uint256.Int, error) {
	//todo
	return GetLatestBlockNumber(tx)
}

func GetSafeBlockNumber(tx kv.Tx) (*uint256.Int, error) {
	//todo
	return GetLatestBlockNumber(tx)
}
