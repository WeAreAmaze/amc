package internal

import (
	"context"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/paths"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
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
	var (
		chainConfig = params.TestAposChainConfig
		db          = rawdb.NewMemoryDatabase(paths.RandomTmpPath())
		aposEngine  = apos.New(chainConfig.Engine, db, chainConfig)
	)
	defer db.Close()

	gBlock, gb, err := newGenesisBlockConfig(db, chainConfig, []types.Address{addr}, []conf.Allocate{conf.Allocate{Address: strings.Replace(addr.Hex(), "0x", "AMC", 1), Balance: "100000000000000000000000000"}})
	if err != nil {
		t.Fatalf("failed to create genesis block: %v", err)
	}

	// Create a pristine chain and database
	genDb, _, blockchain, err := newCanonical(db, chainConfig, aposEngine, gBlock, gb, 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer genDb.Close()

	// Insert an easy and a difficult chain afterwards
	easyBlocks, _ := GenerateChain(chainConfig, blockchain.CurrentBlock().(*block.Block), aposEngine, genDb, len(first), func(i int, b *BlockGen) {
		b.OffsetTime(first[i])
	})

	diffBlocks, _ := GenerateChain(chainConfig, blockchain.CurrentBlock().(*block.Block), aposEngine, genDb, len(second), func(i int, b *BlockGen) {
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

func newGenesisBlockConfig(db kv.RwDB, chainConfig *params.ChainConfig, miners []types.Address, allocate []conf.Allocate) (*block.Block, *GenesisBlock, error) {
	var (
		gBlock *block.Block
	)
	gb := &GenesisBlock{
		GenesisBlockConfig: &conf.GenesisBlockConfig{
			ExtraData: make([]byte, 32+20*len(miners)+65),
			BaseFee:   uint256.NewInt(params.InitialBaseFee),
			Config:    chainConfig,
			Alloc:     allocate,
			Miners:    make([]string, 0),
		},
	}
	for i, miner := range miners {
		gb.GenesisBlockConfig.Miners = append(gb.GenesisBlockConfig.Miners, strings.Replace(miner.Hex(), "0x", "AMC", 1))
		copy(gb.GenesisBlockConfig.ExtraData[32+i*20:32+i*20+20], miner[:])
	}

	if err := db.Update(context.Background(), func(tx kv.RwTx) error {
		gBlock, _, _ = gb.Write(tx)
		//todo remove
		//if chainConfig.Engine.EngineName == "APosEngine" {
		//	minersPB := consensus_pb.PBSigners{}
		//	for _, miner := range miners {
		//
		//		minersPB.Signer = append(minersPB.Signer, &consensus_pb.PBSigner{
		//			Public:  miner,
		//			Address: utils.ConvertAddressToH160(miner),
		//		})
		//	}
		//	data, err := proto.Marshal(&miners)
		//	if err != nil {
		//		return err
		//	}
		//
		//	if err := rawdb.StoreSigners(tx, data); err != nil {
		//		return err
		//	}
		//}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return gBlock, gb, nil
}

// newCanonical creates a chain database, and injects a deterministic canonical
// chain. Depending on the full flag, if creates either a full block chain or a
// header only chain. The database and genesis specification for block generation
// are also returned in case more test blocks are needed later.
func newCanonical(db kv.RwDB, chainConfig *params.ChainConfig, engine consensus.Engine, gBlock *block.Block, gb *GenesisBlock, n int) (kv.RwDB, *conf.GenesisBlockConfig, *BlockChain, error) {

	// Initialize a fresh chain with only a genesis block
	blockchain, _ := NewBlockChain(context.Background(), gBlock, engine, nil, db, nil, chainConfig, chainConfig.Engine)

	// Create and inject the requested chain
	if n == 0 {
		return rawdb.NewMemoryDatabase(paths.RandomTmpPath()), gb.GenesisBlockConfig, blockchain.(*BlockChain), nil
	}
	//if full {
	// Full block-chain requested
	genDb, blocks, _ := GenerateChainWithGenesis(gb, engine, n, func(i int, block *BlockGen) {
		block.SetDifficulty(uint256.NewInt(2))
	})

	_, err := blockchain.InsertChain(blockToIBlock(blocks))
	return genDb, gb.GenesisBlockConfig, blockchain.(*BlockChain), err
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

// Tests
func TestReward(t *testing.T) {
	var (
		chainConfig = params.TestAposChainConfig
		db          = rawdb.NewMemoryDatabase(paths.RandomTmpPath())
		aposEngine  = apos.New(chainConfig.Engine, db, chainConfig)
		miners      = []types.Address{addr}

		key1, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _  = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		key3, _  = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1    = crypto.PubkeyToAddress(key1.PublicKey)
		addr2    = crypto.PubkeyToAddress(key2.PublicKey)
		addr3    = crypto.PubkeyToAddress(key3.PublicKey)
		allocate = []conf.Allocate{
			{
				Address: strings.Replace(addr.Hex(), "0x", "AMC", 1),
				Balance: "100000000000000000000000000",
			},
			{
				Address: strings.Replace(addr1.Hex(), "0x", "AMC", 1),
				Balance: "100000000000000000000000000",
			},
			{
				Address: strings.Replace(addr2.Hex(), "0x", "AMC", 1),
				Balance: "100000000000000000000000000",
			},
			{
				Address: strings.Replace(addr3.Hex(), "0x", "AMC", 1),
				Balance: "100000000000000000000000000",
			},
		}
		signer = transaction.LatestSigner(chainConfig)
	)
	defer db.Close()

	gBlock, gb, err := newGenesisBlockConfig(db, chainConfig, miners, allocate)
	if err != nil {
		t.Fatalf("failed to create genesis block: %v", err)
	}

	// Create a pristine chain and database
	genDb, _, blockchain, err := newCanonical(db, chainConfig, aposEngine, gBlock, gb, 1)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer genDb.Close()

	chain, _ := GenerateChain(chainConfig, blockchain.CurrentBlock().(*block.Block), aposEngine, genDb, 1, func(i int, gen *BlockGen) {
		switch i {
		case 0:
			pastDrop, _ := transaction.SignTx(transaction.NewTransaction(gen.TxNonce(addr2), addr2, &addr1, uint256.NewInt(1000), params.TxGas, uint256.NewInt(params.GWei), nil), signer, key2)
			gen.AddTx(*pastDrop)

		case 2:
		}
	})

	// Import the chain. This runs all block validation rules.
	if _, err1 := blockchain.InsertChain(blockToIBlock(chain.Blocks)); err1 != nil {
		t.Fatalf("failed to insert original chain: %v", err1)
	}

}
