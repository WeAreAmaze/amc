package main

import (
	"fmt"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/modules"
	mdbx2 "github.com/erigontech/mdbx-go"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/cmp"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"
	"runtime"
	"time"
)

func restore(bucket, ReadPath, WritePath string) {
	modules.AmcInit()
	kv.ChaindataTablesCfg = modules.AmcTableCfg

	roTxsLimiter := semaphore.NewWeighted(int64(cmp.Max(32, runtime.GOMAXPROCS(-1)*8)))
	rOpts := mdbx.NewMDBX(log.New()).
		WriteMergeThreshold(4 * 8192).
		Path(ReadPath).Label(kv.ChainDB).
		DBVerbosity(kv.DBVerbosityLvl(2)).RoTxsLimiter(roTxsLimiter)
	rOpts = rOpts.Flags(func(f uint) uint { return f | mdbx2.Accede })
	rdb, err := rOpts.Open()
	if nil != err {
		panic(err)
	}
	defer rdb.Close()

	ctx, _ := common.RootContext()

	tx, err := rdb.BeginRo(ctx)
	if nil != err {
		panic(err)
	}
	defer tx.Rollback()

	rc, err := tx.Cursor(bucket)
	if nil != err {
		panic(err)
	}
	defer rc.Close()
	bucketSize, _ := rc.Count()
	fmt.Printf("%s: have data %d \n", bucket, bucketSize)
	// -----------------------write tx-------------------//
	const ThreadsLimit = 9_000
	limiterB := semaphore.NewWeighted(ThreadsLimit)
	wOpts := mdbx.NewMDBX(log.New()).
		Path(WritePath).Label(kv.ChainDB).
		DBVerbosity(kv.DBVerbosityLvl(2)).RoTxsLimiter(limiterB)

	wdb, err := wOpts.Open()
	if nil != err {
		panic(err)
	}
	defer wdb.Close()

	wTx, err1 := wdb.BeginRw(ctx)
	if err1 != nil {
		panic(err1)
	}
	defer wTx.Rollback()

	if err := wTx.ClearBucket(bucket); nil != err {
		fmt.Println("clear bucket failed")
		panic(err)
	}
	fmt.Println("clear bucket ->", bucket)

	wc, err := wTx.RwCursor(bucket)
	if nil != err {
		panic(err)
	}
	defer wc.Close()

	commitEvery := time.NewTicker(1 * time.Second)
	defer commitEvery.Stop()

	var count uint64
	for k, v, err := rc.First(); k != nil; k, v, err = rc.Next() {
		if err != nil {
			panic(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-commitEvery.C:
			fmt.Printf("finished: %d, total size: %d, reset progress: %.2f %%\n", count, bucketSize, float64(count)/float64(bucketSize)*100)
		default:
		}

		var a account.StateAccount
		if err = a.DecodeForStorageProto(v); nil != err {
			panic(err)
		}

		buffer := make([]byte, a.EncodingLengthForStorage())
		a.EncodeForStorage(buffer)

		if casted, ok := wc.(kv.RwCursorDupSort); ok {
			if err = casted.AppendDup(k, buffer); err != nil {
				panic(err)
			}
		} else {
			if err = wc.Append(k, buffer); err != nil {
				panic(err)
			}
		}
		count++
	}
	curretSize, _ := wc.Count()
	wTx.Commit()

	fmt.Println("restore Account data finished!", "current bucket size:", curretSize)

}
