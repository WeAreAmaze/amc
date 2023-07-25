package internal

import (
	"context"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/consensus/apos"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"os"
	"strings"
	"testing"
)

// So we can deterministically seed different blockchains
var (
	canonicalSeed = 1
	forkSeed      = 2
)

func TestReorgLongBlocks(t *testing.T) { testReorgLong(t, true) }

func testReorgLong(t *testing.T, full bool) {
	testReorg(t, []int64{0, 0, -9}, []int64{0, 0, 0, -9}, 393280+params.GenesisDifficulty.Uint64(), full)
}

func testReorg(t *testing.T, first, second []int64, td uint64, full bool) {

	db := rawdb.NewMemoryDatabase(os.TempDir())
	defer db.Close()

	// Create a pristine chain and database
	genDb, _, blockchain, err := newCanonical(db, apos.New(params.TestAposChainConfig.Engine, db, params.TestAposChainConfig), 0, full)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	//defer blockchain.Stop()

	// Insert an easy and a difficult chain afterwards
	easyBlocks, _ := GenerateChain(params.TestAposChainConfig, blockchain.CurrentBlock().(*block.Block), apos.New(params.TestAposChainConfig.Engine, db, params.TestAposChainConfig), genDb, len(first), func(i int, b *BlockGen) {
		b.OffsetTime(first[i])
	}, false)
	for i, blk := range easyBlocks.Blocks {
		header := blk.Header().(*block.Header)
		if i > 0 {
			header.ParentHash = easyBlocks.Blocks[i-1].Hash()
		}
		header.Extra = make([]byte, 32+65)
		header.Difficulty = uint256.NewInt(2)

		//sig, _ := crypto.Sign(apos.SealHash(header).Bytes(), key)
		//copy(header.Extra[len(header.Extra)-clique.ExtraSeal:], sig)
		//chain.Headers[i] = header
		//chain.Blocks[i] = block.WithSeal(header)
	}
	diffBlocks, _ := GenerateChain(params.TestAposChainConfig, blockchain.CurrentBlock().(*block.Block), apos.New(params.TestAposChainConfig.Engine, db, params.TestAposChainConfig), genDb, len(second), func(i int, b *BlockGen) {
		b.OffsetTime(second[i])
	}, false)
	//if full {
	if _, err := blockchain.InsertChain(blockToIBlock(easyBlocks.Blocks)); err != nil {
		t.Fatalf("failed to insert easy chain: %v", err)
	}
	if _, err := blockchain.InsertChain(blockToIBlock(diffBlocks.Blocks)); err != nil {
		t.Fatalf("failed to insert difficult chain: %v", err)
	}
	//} else {
	//	easyHeaders := make([]*types.Header, len(easyBlocks.))
	//	for i, block := range easyBlocks {
	//		easyHeaders[i] = block.Header()
	//	}
	//	diffHeaders := make([]*types.Header, len(diffBlocks))
	//	for i, block := range diffBlocks {
	//		diffHeaders[i] = block.Header()
	//	}
	//	if _, err := blockchain.InsertHeaderChain(easyHeaders); err != nil {
	//		t.Fatalf("failed to insert easy chain: %v", err)
	//	}
	//	if _, err := blockchain.InsertHeaderChain(diffHeaders); err != nil {
	//		t.Fatalf("failed to insert difficult chain: %v", err)
	//	}
	//}
	// Check that the chain is valid number and link wise
	//if full {
	prev := blockchain.CurrentBlock().Header().(*block.Header)
	for blk, _ := blockchain.GetBlockByNumber(new(uint256.Int).Sub(blockchain.CurrentBlock().Number64(), uint256.NewInt(1))); blk.Number64().Uint64() != 0; prev = blk.Header().(*block.Header) {
		if prev.ParentHash != blk.Hash() {
			t.Errorf("parent block hash mismatch: have %x, want %x", prev.ParentHash, blk.Hash())
		}
		blk, _ = blockchain.GetBlockByNumber(new(uint256.Int).Sub(blk.Number64(), uint256.NewInt(1)))
	}
	//} else {
	//	prev := blockchain.CurrentHeader()
	//	for header := blockchain.GetHeaderByNumber(blockchain.CurrentHeader().Number.Uint64() - 1); header.Number.Uint64() != 0; prev, header = header, blockchain.GetHeaderByNumber(header.Number.Uint64()-1) {
	//		if prev.ParentHash != header.Hash() {
	//			t.Errorf("parent header hash mismatch: have %x, want %x", prev.ParentHash, header.Hash())
	//		}
	//	}
	//}
	// Make sure the chain total difficulty is the correct one
	want := new(uint256.Int).Add(blockchain.genesisBlock.Difficulty(), uint256.NewInt(td))
	//if full {
	cur := blockchain.CurrentBlock()
	if have := blockchain.GetTd(cur.Hash(), cur.Number64()); have.Cmp(want) != 0 {
		t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
	}
	//} else {
	//	cur := blockchain.CurrentHeader()
	//	if have := blockchain.GetTd(cur.Hash(), cur.Number.Uint64()); have.Cmp(want) != 0 {
	//		t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
	//	}
	//}
}

// newCanonical creates a chain database, and injects a deterministic canonical
// chain. Depending on the full flag, if creates either a full block chain or a
// header only chain. The database and genesis specification for block generation
// are also returned in case more test blocks are needed later.
func newCanonical(db kv.RwDB, engine consensus.Engine, n int, full bool) (kv.RwDB, *conf.GenesisBlockConfig, *BlockChain, error) {
	var (
		key, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr    = crypto.PubkeyToAddress(key.PublicKey)
		genesis = &conf.GenesisBlockConfig{
			ExtraData: make([]byte, 32+20+65),
			BaseFee:   uint256.NewInt(params.InitialBaseFee),
			Config:    params.TestAposChainConfig,
			Alloc:     make([]conf.Allocate, 1),
		}

		//signer = transaction.LatestSignerForChainID(nil)
	)
	copy(genesis.ExtraData[32:], addr[:])
	genesis.Alloc[0] = conf.Allocate{
		Address: strings.Replace(addr.Hex(), "0x", "AMC", 1),
		Balance: "100000000000000000000000000",
	}

	gb := GenesisBlock{
		GenesisBlockConfig: genesis,
	}
	gBlock, _, _ := gb.ToBlock()
	// Initialize a fresh chain with only a genesis block
	blockchain, _ := NewBlockChain(context.Background(), gBlock, engine, nil, db, nil, params.TestAposChainConfig, params.TestAposChainConfig.Engine)

	//getHeader := func(hash types.Hash, number uint64) {
	//
	//}

	// Create and inject the requested chain
	if n == 0 {
		return db, genesis, blockchain.(*BlockChain), nil
	}
	//if full {
	// Full block-chain requested
	genDb, blocks := makeBlockChainWithGenesis(db, &gb, n, engine, func(i int, b *BlockGen) {
		// The chain maker doesn't have access to a chain, so the difficulty will be
		// lets unset (nil). Set it here to the correct value.
		b.SetDifficulty(uint256.NewInt(2))

		// We want to simulate an empty middle block, having the same state as the
		// first one. The last is needs a state change again to force a reorg.
		if i != 1 {
			//baseFee := b.GetHeader().BaseFee
			//tx, err := transaction.SignTx(transaction.NewTransaction(b.TxNonce(addr), types.Address{0x00}, new(uint256.Int), params.TxGas, baseFee, nil), *signer, key)
			//if err != nil {
			//	panic(err)
			//}
			//b.AddTxWithChain(getHeader, engine, *tx)
		}
	})
	ib := make([]block.IBlock, len(blocks))
	for i, b := range blocks {
		ib[i] = b
	}
	_, err := blockchain.InsertChain(ib)
	return genDb, genesis, blockchain.(*BlockChain), err
	//}
	//// Header-only chain requested
	//genDb, headers := makeHeaderChainWithGenesis(genesis, n, engine, canonicalSeed)
	//_, err := blockchain.InsertHeaderChain(headers)
	//return genDb, genesis, blockchain.(*BlockChain), err
}

func blockToIBlock(in []*block.Block) []block.IBlock {
	out := make([]block.IBlock, len(in))
	for i, b := range in {
		out[i] = b
	}
	return out
}
