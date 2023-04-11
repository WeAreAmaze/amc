package rawdb

import (
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"testing"
)

// Tests block total difficulty storage and retrieval operations.
func TestTdStorage(t *testing.T) {
	_, tx := memdb.NewTestTx(t)

	// Create a test TD to move around the database and make sure it's really new
	hash, td := types.Hash{}, uint256.NewInt(314)
	entry, err := ReadTd(tx, hash, 0)
	if err != nil {
		t.Fatalf("ReadTd failed: %v", err)
	}
	if entry != nil {
		t.Fatalf("Non existent TD returned: %v", entry)
	}
	// Write and verify the TD in the database
	err = WriteTd(tx, hash, 0, td)
	if err != nil {
		t.Fatalf("WriteTd failed: %v", err)
	}
	entry, err = ReadTd(tx, hash, 0)
	if err != nil {
		t.Fatalf("ReadTd failed: %v", err)
	}
	if entry == nil {
		t.Fatalf("Stored TD not found")
	} else if entry.Cmp(td) != 0 {
		t.Fatalf("Retrieved TD mismatch: have %v, want %v", entry, td)
	}
	// Delete the TD and verify the execution
	err = TruncateTd(tx, 0)
	if err != nil {
		t.Fatalf("DeleteTd failed: %v", err)
	}
	entry, err = ReadTd(tx, hash, 0)
	if err != nil {
		t.Fatalf("ReadTd failed: %v", err)
	}
	if entry != nil {
		t.Fatalf("Deleted TD returned: %v", entry)
	}

	gHash, gTd := types.Hash{}, uint256.NewInt(0)
	if err := WriteTd(tx, gHash, 100, gTd); nil != err {
		t.Fatalf("WriteTd failed: %v", err)
	}

	entryE, errE := ReadTd(tx, gHash, 101)
	if nil != errE {
		t.Fatalf("ReadTd failed: %v", err)
	}

	if entryE != nil {
		t.Fatal("ReadTd returned nil")
	}

	entryg, errg := ReadTd(tx, gHash, 100)
	if nil != errg {
		t.Fatalf("ReadTd failed: %v", err)
	}

	if entryg.Cmp(gTd) != 0 {
		t.Fatal("ReadTd returned nil")
	}
}
