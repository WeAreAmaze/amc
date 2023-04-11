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

package kvcache

import (
	"context"
	"github.com/amazechain/amc/internal/kv"
)

// DummyCache - doesn't remember anything - can be used when service is not remote
type DummyCache struct{}

var _ Cache = (*DummyCache)(nil)    // compile-time interface check
var _ CacheView = (*DummyView)(nil) // compile-time interface check

func NewDummy() *DummyCache { return &DummyCache{} }
func (c *DummyCache) View(_ context.Context, tx kv.Tx) (CacheView, error) {
	return &DummyView{cache: c, tx: tx}, nil
}
func (c *DummyCache) Evict() int { return 0 }
func (c *DummyCache) Len() int   { return 0 }
func (c *DummyCache) Get(k []byte, tx kv.Tx, id ViewID) ([]byte, error) {
	return tx.GetOne(kv.PlainState, k)
}
func (c *DummyCache) GetCode(k []byte, tx kv.Tx, id ViewID) ([]byte, error) {
	return tx.GetOne(kv.Code, k)
}

type DummyView struct {
	cache *DummyCache
	tx    kv.Tx
}

func (c *DummyView) Get(k []byte) ([]byte, error)     { return c.cache.Get(k, c.tx, 0) }
func (c *DummyView) GetCode(k []byte) ([]byte, error) { return c.cache.GetCode(k, c.tx, 0) }
