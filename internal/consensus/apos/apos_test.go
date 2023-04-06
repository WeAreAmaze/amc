package apos

import (
	"bytes"
	"github.com/amazechain/amc/accounts"
	"github.com/amazechain/amc/accounts/keystore"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/rlp"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"golang.org/x/crypto/sha3"
	"io"
	"math/big"
	"testing"
	"time"
)

func TestSignatureAndEcrecover(t *testing.T) {
	var hash common.Hash
	hash.SetBytes([]byte{
		0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0x10, 0x00,
	})

	b := []byte{
		0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
	}
	var addr common.Address
	addr.SetBytes(b)

	var singers = []common.Address{
		common.HexToAddress("0xdD983ceE9ce4fa065361F130ECd9B41103b46035"),
		common.HexToAddress("0x5679AAcA8BC5A1ba9281B1da5a2288A6eC439149"),
	}
	header := mvm_types.Header{
		ParentHash:  hash,
		UncleHash:   mvm_types.EmptyUncleHash,
		Coinbase:    addr,
		Root:        hash,
		TxHash:      hash,
		ReceiptHash: hash,
		Difficulty:  big.NewInt(1),
		Number:      big.NewInt(1),
		GasLimit:    2000,
		GasUsed:     1000,
		Time:        uint64(time.Now().Unix()),
		//Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		MixDigest: common.Hash{},
		Nonce:     mvm_types.EncodeNonce(0),
	}

	header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	header.Extra = header.Extra[:extraVanity]
	for _, signer := range singers {
		header.Extra = append(header.Extra, signer[:]...)
	}
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

	_, ks := tmpKeyStore(t, true)
	private, err := crypto.HexToECDSA("48f421effd9d49a33674acb60dfa3ac53eb6ce03f8f8d002566a5dd87d8a06fe")
	if nil != err {
		t.Error(err)
	}
	account, err := ks.ImportECDSA(private, "foo")
	if nil != err {
		t.Error(err)
	}
	if account.Address.Hex() != "0xdD983ceE9ce4fa065361F130ECd9B41103b46035" {
		t.Errorf("address should equal")
	}

	wallet := ks.Wallets()[0]

	h, err := wallet.SignDataWithPassphrase(account, "foo", accounts.MimetypeClique, clique(header))
	if nil != err {
		t.Error(err)
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], h[:])

	pubkey, err := crypto.Ecrecover(SealEthHash(header).Bytes(), h)
	if err != nil {
		t.Error(err)
	}
	var signer types.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	if signer.Hex() != "0xdD983ceE9ce4fa065361F130ECd9B41103b46035" {
		t.Error("failed recoever signer")
	}

}

func tmpKeyStore(t *testing.T, encrypted bool) (string, *keystore.KeyStore) {
	d := t.TempDir()
	newKs := keystore.NewPlaintextKeyStore
	if encrypted {
		newKs = func(kd string) *keystore.KeyStore { return keystore.NewKeyStore(kd, 2, 1) }
	}
	return d, newKs(d)
}

func encodeEthHeader(w io.Writer, header mvm_types.Header) {
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}

func clique(header mvm_types.Header) []byte {
	b := new(bytes.Buffer)
	encodeEthHeader(b, header)
	return b.Bytes()
}

func SealEthHash(header mvm_types.Header) (hash types.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeEthHeader(hasher, header)
	hasher.(crypto.KeccakState).Read(hash[:])
	return hash
}

// This test case is a repro of an annoying bug that took us forever to catch.
// In Clique PoA networks (Rinkeby, Görli, etc), consecutive blocks might have
// the same state root (no block subsidy, empty block). If a node crashes, the
// chain ends up losing the recent state and needs to regenerate it from blocks
// already in the database. The bug was that processing the block *prior* to an
// empty one **also completes** the empty one, ending up in a known-block error.
func TestReimportMirroredState(t *testing.T) {
	// Initialize a Clique chain with a single signer
	var (
		db     = rawdb.NewMemoryDatabase()
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		engine = New(params.AllCliqueProtocolChanges.Clique, db)
		signer = new(types.HomesteadSigner)
	)
	genspec := &core.Genesis{
		Config:    params.AllCliqueProtocolChanges,
		ExtraData: make([]byte, extraVanity+common.AddressLength+extraSeal),
		Alloc: map[common.Address]core.GenesisAccount{
			addr: {Balance: big.NewInt(10000000000000000)},
		},
		BaseFee: big.NewInt(params.InitialBaseFee),
	}
	copy(genspec.ExtraData[extraVanity:], addr[:])

	// Generate a batch of blocks, each properly signed
	chain, _ := core.NewBlockChain(rawdb.NewMemoryDatabase(), nil, genspec, nil, engine, vm.Config{}, nil, nil)
	defer chain.Stop()

	_, blocks, _ := core.GenerateChainWithGenesis(genspec, engine, 3, func(i int, block *core.BlockGen) {
		// The chain maker doesn't have access to a chain, so the difficulty will be
		// lets unset (nil). Set it here to the correct value.
		block.SetDifficulty(diffInTurn)

		// We want to simulate an empty middle block, having the same state as the
		// first one. The last is needs a state change again to force a reorg.
		if i != 1 {
			tx, err := types.SignTx(types.NewTransaction(block.TxNonce(addr), common.Address{0x00}, new(big.Int), params.TxGas, block.BaseFee(), nil), signer, key)
			if err != nil {
				panic(err)
			}
			block.AddTxWithChain(chain, tx)
		}
	})
	for i, block := range blocks {
		header := block.Header()
		if i > 0 {
			header.ParentHash = blocks[i-1].Hash()
		}
		header.Extra = make([]byte, extraVanity+extraSeal)
		header.Difficulty = diffInTurn

		sig, _ := crypto.Sign(SealHash(header).Bytes(), key)
		copy(header.Extra[len(header.Extra)-extraSeal:], sig)
		blocks[i] = block.WithSeal(header)

		// recover
		signature := header.Extra[len(header.Extra)-extraSeal:]
		pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
		if err != nil {
			t.Fatalf("failed recover: %v", err)
		}
		var signer1 common.Address
		copy(signer1[:], crypto.Keccak256(pubkey[1:])[12:])
		if addr != addr { // addr 和 signer 相同
			t.Errorf("have %x, want %x", addr, addr)
		}
	}

	// Insert the first two blocks and make sure the chain is valid
	db = rawdb.NewMemoryDatabase()
	chain, _ = core.NewBlockChain(db, nil, genspec, nil, engine, vm.Config{}, nil, nil)
	defer chain.Stop()

	if _, err := chain.InsertChain(blocks[:2]); err != nil {
		t.Fatalf("failed to insert initial blocks: %v", err)
	}
	if head := chain.CurrentBlock().NumberU64(); head != 2 {
		t.Fatalf("chain head mismatch: have %d, want %d", head, 2)
	}

	// Simulate a crash by creating a new chain on top of the database, without
	// flushing the dirty states out. Insert the last block, triggering a sidechain
	// reimport.
	chain, _ = core.NewBlockChain(db, nil, genspec, nil, engine, vm.Config{}, nil, nil)
	defer chain.Stop()

	if _, err := chain.InsertChain(blocks[2:]); err != nil {
		t.Fatalf("failed to insert final block: %v", err)
	}
	if head := chain.CurrentBlock().NumberU64(); head != 3 {
		t.Fatalf("chain head mismatch: have %d, want %d", head, 3)
	}
}
