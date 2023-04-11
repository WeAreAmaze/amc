package api

import (
	"context"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"testing"
	"time"
)

func TestGetBlockByNumber(t *testing.T) {
	var (
		db      = rawdb.NewMemoryDatabase()
		engine  = clique.New(params.AllCliqueProtocolChanges.Clique, db)
		ctx, _  = context.WithTimeout(context.Background(), 2*time.Second)
		blockNr = 335
		//minerAddress = common.HexToAddress("0xa2142AB3f25eAa9985F22C3f5B1fF9FA378dac21")
	)
	//
	client, err := ethclient.Dial("ws://127.0.0.1:20013")
	if err != nil {
		t.Error("cannot connect to node")
	}
	//
	block, err := client.BlockByNumber(ctx, new(big.Int).SetInt64(int64(blockNr)))
	if err != nil {
		t.Errorf("cannot get block: %v", err)
	}
	//
	address, err := engine.Author(block.Header())
	if err != nil {
		t.Errorf("cannot ecrecover address: %v", err)
	}

	t.Logf("header size %s", block.Header().Size())
	t.Logf("ecrecover address %s", address.String())
	t.Logf("eth header hash %s", block.Header().Hash())
}

func TestBeanchMark(t *testing.T) {

	var (
		key, _  = crypto.HexToECDSA("d6d8d19bd786d6676819b806694b1100a4414a94e51e9a82a351bd8f7f3f3658")
		addr    = crypto.PubkeyToAddress(key.PublicKey)
		signer  = new(types.HomesteadSigner)
		content = context.Background()
	)
	//
	client, err := ethclient.Dial("ws://127.0.0.1:20013")
	if err != nil {
		t.Error("cannot connect to node")
	}

	for i := 0; i < 1000000; i++ {
		tx, _ := types.SignTx(types.NewTransaction(uint64(i), addr, big.NewInt(params.Wei), params.TxGas, big.NewInt(params.GWei), nil), signer, key)
		client.SendTransaction(content, tx)
	}

}
