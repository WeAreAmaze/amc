// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package kv

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/utils"
	"os"
	"time"

	"github.com/torquem-ch/mdbx-go/mdbx"
	"go.uber.org/atomic"
)

func DefaultPageSize() uint64 {
	osPageSize := os.Getpagesize()
	if osPageSize < 4096 { // reduce further may lead to errors (because some data is just big)
		osPageSize = 4096
	} else if osPageSize > mdbx.MaxPageSize {
		osPageSize = mdbx.MaxPageSize
	}
	osPageSize = osPageSize / 4096 * 4096 // ensure it's rounded
	return uint64(osPageSize)
}

// BigChunks - read `table` by big chunks - restart read transaction after each 1 minutes
func BigChunks(db RoDB, table string, from []byte, walker func(tx Tx, k, v []byte) (bool, error)) error {
	rollbackEvery := time.NewTicker(1 * time.Minute)

	var stop bool
	for !stop {
		if err := db.View(context.Background(), func(tx Tx) error {
			c, err := tx.Cursor(table)
			if err != nil {
				return err
			}
			defer c.Close()

			k, v, err := c.Seek(from)
		Loop:
			for ; k != nil; k, v, err = c.Next() {
				if err != nil {
					return err
				}

				// break loop before walker() call, to make sure all keys are received by walker() exactly once
				select {
				case <-rollbackEvery.C:

					break Loop
				default:
				}

				ok, err := walker(tx, k, v)
				if err != nil {
					return err
				}
				if !ok {
					stop = true
					break
				}
			}

			if k == nil {
				stop = true
			}

			from = utils.Copy(k) // next transaction will start from this key

			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

var (
	bytesTrue  = []byte{1}
	bytesFalse = []byte{0}
)

func bytes2bool(in []byte) bool {
	if len(in) < 1 {
		return false
	}
	return in[0] == 1
}

var ErrChanged = fmt.Errorf("key must not change")

// EnsureNotChangedBool - used to store immutable blockchain flags in db. protects from human mistakes
func EnsureNotChangedBool(tx GetPut, bucket string, k []byte, value bool) (ok, enabled bool, err error) {
	vBytes, err := tx.GetOne(bucket, k)
	if err != nil {
		return false, enabled, err
	}
	if vBytes == nil {
		if value {
			vBytes = bytesTrue
		} else {
			vBytes = bytesFalse
		}
		if err := tx.Put(bucket, k, vBytes); err != nil {
			return false, enabled, err
		}
	}

	enabled = bytes2bool(vBytes)
	return value == enabled, enabled, nil
}

func GetBool(tx Getter, bucket string, k []byte) (enabled bool, err error) {
	vBytes, err := tx.GetOne(bucket, k)
	if err != nil {
		return false, err
	}
	return bytes2bool(vBytes), nil
}

func ReadAhead(ctx context.Context, db RoDB, progress *atomic.Bool, table string, from []byte, amount uint32) {
	if progress.Load() {
		return
	}
	progress.Store(true)
	go func() {
		defer progress.Store(false)
		_ = db.View(ctx, func(tx Tx) error {
			c, err := tx.Cursor(table)
			if err != nil {
				return err
			}
			defer c.Close()

			for k, _, err := c.Seek(from); k != nil && amount > 0; k, _, err = c.Next() {
				if err != nil {
					return err
				}
				amount--
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
			return nil
		})
	}()
}
