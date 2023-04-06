package state

import (
	"bytes"
	"fmt"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"math"

	"github.com/amazechain/amc/modules/changeset"

	"github.com/RoaringBitmap/roaring/roaring64"

	"github.com/amazechain/amc/modules/ethdb/bitmapdb"
	"github.com/ledgerwatch/erigon-lib/kv"
)

func originalAccountData(original *account.StateAccount, omitHashes bool) []byte {
	var originalData []byte
	if !original.Initialised {
		originalData = []byte{}
	} else if omitHashes {
		testAcc := original.SelfCopy()
		copy(testAcc.CodeHash[:], emptyCodeHash)
		testAcc.Root = crypto.Keccak256Hash(nil)
		originalDataLen := testAcc.EncodingLengthForStorage()
		originalData = make([]byte, originalDataLen)
		testAcc.EncodeForStorage(originalData)
	} else {
		originalDataLen := original.EncodingLengthForStorage()
		originalData = make([]byte, originalDataLen)
		original.EncodeForStorage(originalData)
	}
	return originalData
}

func writeIndex(blocknum uint64, changes *changeset.ChangeSet, bucket string, changeDb kv.RwTx) error {
	buf := bytes.NewBuffer(nil)
	for _, change := range changes.Changes {
		k := modules.CompositeKeyWithoutIncarnation(change.Key)

		index, err := bitmapdb.Get64(changeDb, bucket, k, 0, math.MaxUint32)
		if err != nil {
			return fmt.Errorf("find chunk failed: %w", err)
		}
		index.Add(blocknum)
		if err = bitmapdb.WalkChunkWithKeys64(k, index, bitmapdb.ChunkLimit, func(chunkKey []byte, chunk *roaring64.Bitmap) error {
			buf.Reset()
			if _, err = chunk.WriteTo(buf); err != nil {
				return err
			}
			return changeDb.Put(bucket, chunkKey, types.CopyBytes(buf.Bytes()))
		}); err != nil {
			return err
		}
	}

	return nil
}
