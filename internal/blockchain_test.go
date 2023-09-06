package internal

import (
	"context"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/paths"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/consensus/apos"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"strings"
	"testing"
)

// So we can deterministically seed different blockchains
var (
	canonicalSeed = 1
	forkSeed      = 2
	//key, _        = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr = crypto.PubkeyToAddress(key.PublicKey)
)

func TestReorgLongBlocks(t *testing.T) { testReorgLong(t, true) }

func testReorgLong(t *testing.T, full bool) {
	testReorg(t, []int64{0, 0, -9}, []int64{0, 0, 0, -9}, 8)
}

func testReorg(t *testing.T, first, second []int64, td uint64) {
	db := rawdb.NewMemoryDatabase(paths.RandomTmpPath())
	defer db.Close()

	// Create a pristine chain and database
	genDb, _, blockchain, err := newCanonical(db, apos.New(params.TestAposChainConfig.Engine, db, params.TestAposChainConfig), 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer genDb.Close()

	// Insert an easy and a difficult chain afterwards
	easyBlocks, _ := GenerateChain(params.TestAposChainConfig, blockchain.CurrentBlock().(*block.Block), apos.New(params.TestAposChainConfig.Engine, genDb, params.TestAposChainConfig), genDb, len(first), func(i int, b *BlockGen) {
		b.OffsetTime(first[i])
	})

	diffBlocks, _ := GenerateChain(params.TestAposChainConfig, blockchain.CurrentBlock().(*block.Block), apos.New(params.TestAposChainConfig.Engine, genDb, params.TestAposChainConfig), genDb, len(second), func(i int, b *BlockGen) {
		b.OffsetTime(second[i])
	})
	//if full {
	if _, err := blockchain.InsertChain(blockToIBlock(easyBlocks.Blocks)); err != nil {
		t.Fatalf("failed to insert easy chain: %v", err)
	}
	if _, err := blockchain.InsertChain(blockToIBlock(diffBlocks.Blocks)); err != nil {
		t.Fatalf("failed to insert difficult chain: %v", err)
	}

	prev := blockchain.CurrentBlock().Header().(*block.Header)
	for blk, _ := blockchain.GetBlockByNumber(new(uint256.Int).Sub(blockchain.CurrentBlock().Number64(), uint256.NewInt(1))); blk.Number64().Uint64() != 0; blk, _ = blockchain.GetBlockByNumber(new(uint256.Int).Sub(blk.Number64(), uint256.NewInt(1))) {
		//t.Logf("%d, cur: %x,pre: %x", prev.Number.Uint64(), prev.Hash(), prev.ParentHash)
		if prev.ParentHash != blk.Hash() {
			t.Errorf("parent block hash mismatch: have %x, want %x", prev.ParentHash, blk.Hash())
		}
		prev = blk.Header().(*block.Header)
	}

	// Make sure the chain total difficulty is the correct one
	want := uint256.NewInt(td)
	//if full {
	cur := blockchain.CurrentBlock()
	if have := blockchain.GetTd(cur.Hash(), cur.Number64()); have.Cmp(want) != 0 {
		t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
	}
}

// newCanonical creates a chain database, and injects a deterministic canonical
// chain. Depending on the full flag, if creates either a full block chain or a
// header only chain. The database and genesis specification for block generation
// are also returned in case more test blocks are needed later.
func newCanonical(db kv.RwDB, engine consensus.Engine, n int) (kv.RwDB, *conf.GenesisBlockConfig, *BlockChain, error) {
	var (
		genesis = &conf.GenesisBlockConfig{
			ExtraData: make([]byte, 32+20+65),
			BaseFee:   uint256.NewInt(params.InitialBaseFee),
			Config:    params.TestAposChainConfig,
			Alloc:     make([]conf.Allocate, 1),
			Miners:    []string{strings.Replace(addr.Hex(), "0x", "AMC", 1)},
		}
	)
	copy(genesis.ExtraData[32:32+20], addr[:])
	genesis.Alloc[0] = conf.Allocate{
		Address: strings.Replace(addr.Hex(), "0x", "AMC", 1),
		Balance: "100000000000000000000000000",
	}

	gb := GenesisBlock{
		GenesisBlockConfig: genesis,
	}
	var gBlock *block.Block
	db.Update(context.Background(), func(tx kv.RwTx) error {
		gBlock, _, _ = gb.Write(tx)
		return nil
	})

	// Initialize a fresh chain with only a genesis block
	blockchain, _ := NewBlockChain(context.Background(), gBlock, engine, nil, db, nil, params.TestAposChainConfig, params.TestAposChainConfig.Engine)

	//getHeader := func(hash types.Hash, number uint64) {
	//
	//}

	// Create and inject the requested chain
	if n == 0 {
		return rawdb.NewMemoryDatabase(paths.RandomTmpPath()), genesis, blockchain.(*BlockChain), nil
	}
	//if full {
	// Full block-chain requested
	genDb, blocks, _ := GenerateChainWithGenesis(&gb, engine, n, func(i int, block *BlockGen) {
		block.SetDifficulty(uint256.NewInt(2))
	})

	ib := make([]block.IBlock, len(blocks))
	for i, b := range blocks {
		ib[i] = b
	}
	_, err := blockchain.InsertChain(ib)
	return genDb, genesis, blockchain.(*BlockChain), err
}

func blockToIBlock(in []*block.Block) []block.IBlock {
	out := make([]block.IBlock, len(in))
	for i, b := range in {
		header := b.Header().(*block.Header)
		if i > 0 {
			header.ParentHash = in[i-1].Hash()
		}
		header.Extra = make([]byte, 32+65)
		header.Difficulty = uint256.NewInt(2)

		sig, _ := crypto.Sign(apos.SealHash(header).Bytes(), key)
		copy(header.Extra[len(header.Extra)-65:], sig)
		out[i] = b.WithSeal(header)
	}
	return out
}
