package state

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/amazechain/amc/common/crypto"
	common "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules"
	"github.com/amazechain/amc/modules/changeset"
	"github.com/amazechain/amc/modules/ethdb"
	"github.com/amazechain/amc/modules/ethdb/bitmapdb"
	"github.com/amazechain/amc/modules/prune"
	"github.com/c2h5oh/datasize"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/dbg"
	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv"
	kv2 "github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/erigon-lib/kv/temporal/historyv2"
	"github.com/stretchr/testify/require"
	"math"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"
)

type HistoryCfg struct {
	db         kv.RwDB
	bufLimit   datasize.ByteSize
	prune      prune.Mode
	flushEvery time.Duration
	tmpdir     string
}

func generateAddrs(numOfAddrs int, isPlain bool) ([][]byte, error) {
	addrs := make([][]byte, numOfAddrs)
	for i := 0; i < numOfAddrs; i++ {
		key1, err := crypto.GenerateKey()
		if err != nil {
			return nil, err
		}
		addr := crypto.PubkeyToAddress(key1.PublicKey)
		if isPlain {
			addrs[i] = addr.Bytes()
			continue
		}
		hash, err := common.HashData(addr.Bytes())
		if err != nil {
			return nil, err
		}
		addrs[i] = hash.Bytes()
	}
	return addrs, nil
}

func generateTestData(t *testing.T, tx kv.RwTx, csBucket string, numOfBlocks int) ([][]byte, map[string][]uint64) { //nolint
	csInfo, ok := changeset.Mapper[csBucket]
	if !ok {
		t.Fatal("incorrect cs bucket")
	}
	var isPlain bool
	if modules.StorageChangeSet == csBucket || modules.AccountChangeSet == csBucket {
		isPlain = true
	}
	addrs, err := generateAddrs(3, isPlain)
	require.NoError(t, err)
	if modules.StorageChangeSet == csBucket {
		keys, innerErr := generateAddrs(3, false)
		require.NoError(t, innerErr)

		defaultIncarnation := make([]byte, modules.Incarnation)
		binary.BigEndian.PutUint16(defaultIncarnation, uint16(1))
		for i := range addrs {
			addrs[i] = append(addrs[i], defaultIncarnation...)
			addrs[i] = append(addrs[i], keys[i]...)
		}
	}

	res := make([]uint64, 0)
	res2 := make([]uint64, 0)
	res3 := make([]uint64, 0)

	for i := 0; i < numOfBlocks; i++ {
		cs := csInfo.New()
		err = cs.Add(addrs[0], []byte(strconv.Itoa(i)))
		require.NoError(t, err)

		res = append(res, uint64(i))

		if i%2 == 0 {
			err = cs.Add(addrs[1], []byte(strconv.Itoa(i)))
			require.NoError(t, err)
			res2 = append(res2, uint64(i))
		}
		if i%3 == 0 {
			err = cs.Add(addrs[2], []byte(strconv.Itoa(i)))
			require.NoError(t, err)
			res3 = append(res3, uint64(i))
		}
		err = csInfo.Encode(uint64(i), cs, func(k, v []byte) error {
			return tx.Put(csBucket, k, v)
		})
		require.NoError(t, err)
	}

	return addrs, map[string][]uint64{
		string(addrs[0]): res,
		string(addrs[1]): res2,
		string(addrs[2]): res3,
	}
}

func TestIndexGenerator_Truncate(t *testing.T) {
	buckets := []string{modules.AccountChangeSet, modules.StorageChangeSet}
	//tmpDir, ctx := t.TempDir(), context.Background()
	kv := kv2.NewTestDB(t)
	cfg := HistoryCfg{kv, 0, prune.DefaultMode, 0, t.TempDir()}
	for i := range buckets {
		csbucket := buckets[i]

		tx, err := kv.BeginRw(context.Background())
		require.NoError(t, err)
		defer tx.Rollback()

		hashes, expected := generateTestData(t, tx, csbucket, 2100)
		mp := historyv2.Mapper[csbucket]
		indexBucket := mp.IndexBucket
		cfgCopy := cfg
		cfgCopy.bufLimit = 10
		cfgCopy.flushEvery = time.Microsecond
		err = promoteHistory("logPrefix", tx, csbucket, 0, uint64(2100), cfgCopy, nil)
		require.NoError(t, err)

		reduceSlice := func(arr []uint64, timestamtTo uint64) []uint64 {
			pos := sort.Search(len(arr), func(i int) bool {
				return arr[i] > timestamtTo
			})
			return arr[:pos]
		}

		//t.Run("truncate to 2050 "+csbucket, func(t *testing.T) {
		expected[string(hashes[0])] = reduceSlice(expected[string(hashes[0])], 2050)
		expected[string(hashes[1])] = reduceSlice(expected[string(hashes[1])], 2050)
		expected[string(hashes[2])] = reduceSlice(expected[string(hashes[2])], 2050)

		err = UnwindHistory(tx, csbucket, 2050, nil)
		require.NoError(t, err)

		checkIndex(t, tx, indexBucket, hashes[0], expected[string(hashes[0])])
		checkIndex(t, tx, indexBucket, hashes[1], expected[string(hashes[1])])
		checkIndex(t, tx, indexBucket, hashes[2], expected[string(hashes[2])])
		//})

		//t.Run("truncate to 2000 "+csbucket, func(t *testing.T) {
		expected[string(hashes[0])] = reduceSlice(expected[string(hashes[0])], 2000)
		expected[string(hashes[1])] = reduceSlice(expected[string(hashes[1])], 2000)
		expected[string(hashes[2])] = reduceSlice(expected[string(hashes[2])], 2000)

		err = UnwindHistory(tx, csbucket, 2000, nil)
		require.NoError(t, err)

		checkIndex(t, tx, indexBucket, hashes[0], expected[string(hashes[0])])
		checkIndex(t, tx, indexBucket, hashes[1], expected[string(hashes[1])])
		checkIndex(t, tx, indexBucket, hashes[2], expected[string(hashes[2])])
		//})

		//t.Run("truncate to 1999 "+csbucket, func(t *testing.T) {
		err = UnwindHistory(tx, csbucket, 1999, nil)
		require.NoError(t, err)
		expected[string(hashes[0])] = reduceSlice(expected[string(hashes[0])], 1999)
		expected[string(hashes[1])] = reduceSlice(expected[string(hashes[1])], 1999)
		expected[string(hashes[2])] = reduceSlice(expected[string(hashes[2])], 1999)

		checkIndex(t, tx, indexBucket, hashes[0], expected[string(hashes[0])])
		checkIndex(t, tx, indexBucket, hashes[1], expected[string(hashes[1])])
		checkIndex(t, tx, indexBucket, hashes[2], expected[string(hashes[2])])
		bm, err := bitmapdb.Get64(tx, indexBucket, hashes[0], 1999, math.MaxUint32)
		require.NoError(t, err)
		if bm.GetCardinality() > 0 && bm.Maximum() > 1999 {
			t.Fatal(bm.Maximum())
		}
		bm, err = bitmapdb.Get64(tx, indexBucket, hashes[1], 1999, math.MaxUint32)
		require.NoError(t, err)
		if bm.GetCardinality() > 0 && bm.Maximum() > 1999 {
			t.Fatal()
		}
		//})

		//t.Run("truncate to 999 "+csbucket, func(t *testing.T) {
		expected[string(hashes[0])] = reduceSlice(expected[string(hashes[0])], 999)
		expected[string(hashes[1])] = reduceSlice(expected[string(hashes[1])], 999)
		expected[string(hashes[2])] = reduceSlice(expected[string(hashes[2])], 999)

		err = UnwindHistory(tx, csbucket, 999, nil)
		if err != nil {
			t.Fatal(err)
		}
		bm, err = bitmapdb.Get64(tx, indexBucket, hashes[0], 999, math.MaxUint32)
		require.NoError(t, err)
		if bm.GetCardinality() > 0 && bm.Maximum() > 999 {
			t.Fatal()
		}
		bm, err = bitmapdb.Get64(tx, indexBucket, hashes[1], 999, math.MaxUint32)
		require.NoError(t, err)
		if bm.GetCardinality() > 0 && bm.Maximum() > 999 {
			t.Fatal()
		}

		checkIndex(t, tx, indexBucket, hashes[0], expected[string(hashes[0])])
		checkIndex(t, tx, indexBucket, hashes[1], expected[string(hashes[1])])
		checkIndex(t, tx, indexBucket, hashes[2], expected[string(hashes[2])])

		////})
		//err = pruneHistoryIndex(tx, csbucket, "", tmpDir, 128, ctx, logger)
		//assert.NoError(t, err)
		//expectNoHistoryBefore(t, tx, csbucket, 128)
		//
		//// double prune is safe
		//err = pruneHistoryIndex(tx, csbucket, "", tmpDir, 128, ctx, logger)
		//assert.NoError(t, err)
		//expectNoHistoryBefore(t, tx, csbucket, 128)
		tx.Rollback()
	}
}

func promoteHistory(logPrefix string, tx kv.RwTx, changesetBucket string, start, stop uint64, cfg HistoryCfg, quit <-chan struct{}) error {
	logEvery := time.NewTicker(30 * time.Second)
	defer logEvery.Stop()

	updates := map[string]*roaring64.Bitmap{}
	checkFlushEvery := time.NewTicker(cfg.flushEvery)
	defer checkFlushEvery.Stop()

	collectorUpdates := etl.NewCollector(logPrefix, cfg.tmpdir, etl.NewSortableBuffer(etl.BufferOptimalSize))
	defer collectorUpdates.Close()

	if err := changeset.ForRange(tx, changesetBucket, start, stop, func(blockN uint64, k, v []byte) error {
		if err := libcommon.Stopped(quit); err != nil {
			return err
		}

		k = modules.CompositeKeyWithoutIncarnation(k)

		select {
		default:
		case <-logEvery.C:
			var m runtime.MemStats
			dbg.ReadMemStats(&m)
			log.Info(fmt.Sprintf("[%s] Progress", logPrefix), "number", blockN, "alloc", libcommon.ByteCount(m.Alloc), "sys", libcommon.ByteCount(m.Sys))
		case <-checkFlushEvery.C:
			if needFlush64(updates, cfg.bufLimit) {
				if err := flushBitmaps64(collectorUpdates, updates); err != nil {
					return err
				}
				updates = map[string]*roaring64.Bitmap{}
			}
		}

		m, ok := updates[string(k)]
		if !ok {
			m = roaring64.New()
			updates[string(k)] = m
		}
		m.Add(blockN)

		return nil
	}); err != nil {
		return err
	}

	if err := flushBitmaps64(collectorUpdates, updates); err != nil {
		return err
	}

	var currentBitmap = roaring64.New()
	var buf = bytes.NewBuffer(nil)

	lastChunkKey := make([]byte, 128)
	var loaderFunc = func(k []byte, v []byte, table etl.CurrentTableReader, next etl.LoadNextFunc) error {
		if _, err := currentBitmap.ReadFrom(bytes.NewReader(v)); err != nil {
			return err
		}

		lastChunkKey = lastChunkKey[:len(k)+8]
		copy(lastChunkKey, k)
		binary.BigEndian.PutUint64(lastChunkKey[len(k):], ^uint64(0))
		lastChunkBytes, err := table.Get(lastChunkKey)
		if err != nil && !errors.Is(err, ethdb.ErrKeyNotFound) {
			return fmt.Errorf("find last chunk failed: %w", err)
		}
		if len(lastChunkBytes) > 0 {
			lastChunk := roaring64.New()
			_, err = lastChunk.ReadFrom(bytes.NewReader(lastChunkBytes))
			if err != nil {
				return fmt.Errorf("couldn't read last log index chunk: %w, len(lastChunkBytes)=%d", err, len(lastChunkBytes))
			}

			currentBitmap.Or(lastChunk) // merge last existing chunk from tx - next loop will overwrite it
		}
		if err = bitmapdb.WalkChunkWithKeys64(k, currentBitmap, bitmapdb.ChunkLimit, func(chunkKey []byte, chunk *roaring64.Bitmap) error {
			buf.Reset()
			if _, err = chunk.WriteTo(buf); err != nil {
				return err
			}
			return next(k, chunkKey, buf.Bytes())
		}); err != nil {
			return err
		}
		currentBitmap.Clear()
		return nil
	}

	if err := collectorUpdates.Load(tx, historyv2.Mapper[changesetBucket].IndexBucket, loaderFunc, etl.TransformArgs{Quit: quit}); err != nil {
		return err
	}
	return nil
}

func checkIndex(t *testing.T, db kv.Tx, bucket string, k []byte, expected []uint64) {
	t.Helper()
	k = modules.CompositeKeyWithoutIncarnation(k)
	m, err := bitmapdb.Get64(db, bucket, k, 0, math.MaxUint32)
	if err != nil {
		t.Fatal(err, common.Bytes2Hex(k))
	}
	val := m.ToArray()
	if !reflect.DeepEqual(val, expected) {
		fmt.Printf("get     : %v\n", val)
		fmt.Printf("expected: %v\n", toU32(expected))
		t.Fatal()
	}
}

func toU32(in []uint64) []uint32 {
	out := make([]uint32, len(in))
	for i := range in {
		out[i] = uint32(in[i])
	}
	return out
}

func needFlush64(bitmaps map[string]*roaring64.Bitmap, memLimit datasize.ByteSize) bool {
	sz := uint64(0)
	for _, m := range bitmaps {
		sz += m.GetSizeInBytes() * 2 // for golang's overhead
	}
	const memoryNeedsForKey = 32 * 2 * 2 //  len(key) * (string and bytes) overhead * go's map overhead
	return uint64(len(bitmaps)*memoryNeedsForKey)+sz > uint64(memLimit)
}

func flushBitmaps64(c *etl.Collector, inMem map[string]*roaring64.Bitmap) error {
	for k, v := range inMem {
		v.RunOptimize()
		if v.GetCardinality() == 0 {
			continue
		}
		newV := bytes.NewBuffer(make([]byte, 0, v.GetSerializedSizeInBytes()))
		if _, err := v.WriteTo(newV); err != nil {
			return err
		}
		if err := c.Collect([]byte(k), newV.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
