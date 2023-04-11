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
	"sync"
)

// resultStore implements a structure for maintaining fetchResults, tracking their
// download-progress and delivering (finished) results.
type resultStore struct {
	items        []*fetchResult // Downloaded but not yet delivered fetch results
	resultOffset uint64         // Offset of the first cached fetch result in the block chain

	// Internal index of first non-completed entry, updated atomically when needed.
	// If all items are complete, this will equal length(items), so
	// *important* : is not safe to use for indexing without checking against length
	indexIncomplete int32 // atomic access

	// throttleThreshold is the limit up to which we _want_ to fill the
	// results. If blocks are large, we want to limit the results to less
	// than the number of available slots, and maybe only fill 1024 out of
	// 8192 possible places. The queue will, at certain times, recalibrate
	// this index.
	throttleThreshold uint64

	lock sync.RWMutex
}

func newResultStore(size int) *resultStore {
	return &resultStore{
		resultOffset:      0,
		items:             make([]*fetchResult, size),
		throttleThreshold: uint64(size),
	}
}

// SetThrottleThreshold updates the throttling threshold based on the requested
// limit and the total queue capacity. It returns the (possibly capped) threshold
func (r *resultStore) SetThrottleThreshold(threshold uint64) uint64 {
	r.lock.Lock()
	defer r.lock.Unlock()

	limit := uint64(len(r.items))
	if threshold >= limit {
		threshold = limit
	}
	r.throttleThreshold = threshold
	return r.throttleThreshold
}

// Prepare initialises the offset with the given block number
func (r *resultStore) Prepare(offset uint64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.resultOffset < offset {
		r.resultOffset = offset
	}
}
