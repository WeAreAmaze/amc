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

package download

import (
	"github.com/amazechain/amc/api/protocol/types_pb"
	"hash"
	"sync"
	"time"
)

var (
	blockCacheMaxItems     = 8192              // Maximum number of blocks to cache before throttling the download
	blockCacheInitialItems = 2048              // Initial number of blocks to start fetching, before we know the sizes of the blocks
	blockCacheMemory       = 256 * 1024 * 1024 // Maximum amount of memory to use for block caching
	blockCacheSizeWeight   = 0.1               // Multiplier to approximate the average block size based on past ones
)

// fetchRequest
type fetchRequest struct {
}

// fetchResult
type fetchResult struct {
}

// queue
type queue struct {
	mode SyncMode

	headerHead     hash.Hash                      // Hash of the last queued header to verify order
	headerTaskPool map[uint64]*types_pb.PBHeader  // Pending header retrieval tasks, mapping starting indexes to skeleton headers
	headerPeerMiss map[string]map[uint64]struct{} // Set of per-peer header batches known to be unavailable
	headerPendPool map[string]*fetchRequest       // Currently pending header retrieval operations
	headerResults  []*types_pb.PBHeader           // Result cache accumulating the completed headers
	headerHashes   []hash.Hash                    // Result cache accumulating the completed header hashes
	headerProced   int                            // Number of headers already processed from the results
	headerOffset   uint64                         // Number of the first header in the result cache
	headerContCh   chan bool                      // Channel to notify when header download finishes

	resultCache *resultStore // Downloaded but not yet delivered fetch results

	lock   *sync.RWMutex
	active *sync.Cond
	closed bool

	lastStatLog time.Time
}

func (q *queue) Prepare(offset uint64, mode SyncMode) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// Prepare the queue for sync results
	q.resultCache.Prepare(offset)
	q.mode = mode
}
