package initialsync

import (
	"context"
	"github.com/amazechain/amc/internal/p2p/peers/scorers"
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.opencensus.io/trace"
)

// peerLock returns peer lock for a given peer. If lock is not found, it is created.
func (f *blocksFetcher) peerLock(pid peer.ID) *peerLock {
	f.Lock()
	defer f.Unlock()
	if lock, ok := f.peerLocks[pid]; ok && lock != nil {
		lock.accessed = time.Now()
		return lock
	}
	f.peerLocks[pid] = &peerLock{
		Mutex:    sync.Mutex{},
		accessed: time.Now(),
	}
	return f.peerLocks[pid]
}

// removeStalePeerLocks is a cleanup procedure which removes stale locks.
func (f *blocksFetcher) removeStalePeerLocks(age time.Duration) {
	f.Lock()
	defer f.Unlock()
	for peerID, lock := range f.peerLocks {
		if time.Since(lock.accessed) >= age {
			lock.Lock()
			delete(f.peerLocks, peerID)
			lock.Unlock()
		}
	}
}

// selectFailOverPeer randomly selects fail over peer from the list of available peers.
func (f *blocksFetcher) selectFailOverPeer(excludedPID peer.ID, peers []peer.ID) (peer.ID, error) {
	if len(peers) == 0 {
		return "", errNoPeersAvailable
	}
	if len(peers) == 1 && peers[0] == excludedPID {
		return "", errNoPeersAvailable
	}

	ind := f.rand.Int() % len(peers)
	if peers[ind] == excludedPID {
		return f.selectFailOverPeer(excludedPID, append(peers[:ind], peers[ind+1:]...))
	}
	return peers[ind], nil
}

// waitForMinimumPeers spins and waits up until enough peers are available.
func (f *blocksFetcher) waitForMinimumPeers(ctx context.Context) ([]peer.ID, error) {
	required := f.p2p.GetConfig().MinSyncPeers
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		_, peers := f.p2p.Peers().BestPeers(f.p2p.GetConfig().MinSyncPeers, f.chain.CurrentBlock().Number64())
		if len(peers) >= required {
			return peers, nil
		}
		log.Info("Waiting for enough suitable peers before syncing （blocksFetcher）", "suitable", len(peers), "required", required)
		time.Sleep(handshakePollingInterval)
	}
}

// filterPeers returns transformed list of peers, weight sorted by scores and capacity remaining.
// List can be further constrained using peersPercentage, where only percentage of peers are returned.
func (f *blocksFetcher) filterPeers(ctx context.Context, peers []peer.ID, peersPercentage float64) []peer.ID {
	ctx, span := trace.StartSpan(ctx, "initialsync.filterPeers")
	defer span.End()

	if len(peers) == 0 {
		return peers
	}

	// Sort peers using both block provider score and, custom, capacity based score (see
	// peerFilterCapacityWeight if you want to give different weights to provider's and capacity
	// scores).
	// Scores produced are used as weights, so peers are ordered probabilistically i.e. peer with
	// a higher score has higher chance to end up higher in the list.
	scorer := f.p2p.Peers().Scorers().BlockProviderScorer()
	peers = scorer.WeightSorted(f.rand, peers, func(peerID peer.ID, blockProviderScore float64) float64 {
		remaining, capacity := float64(f.rateLimiter.Remaining(peerID.String())), float64(f.rateLimiter.Capacity())
		// When capacity is close to exhaustion, allow less performant peer to take a chance.
		// Otherwise, there's a good chance system will be forced to wait for rate limiter.
		if remaining < float64(f.blocksPerPeriod) {
			return 0.0
		}
		capScore := remaining / capacity
		overallScore := blockProviderScore*(1.0-f.capacityWeight) + capScore*f.capacityWeight
		return math.Round(overallScore*scorers.ScoreRoundingFactor) / scorers.ScoreRoundingFactor
	})

	return trimPeers(peers, peersPercentage, f.p2p.GetConfig().MinSyncPeers)
}

// trimPeers limits peer list, returning only specified percentage of peers.
// Takes system constraints into account (min/max peers to sync).
func trimPeers(peers []peer.ID, peersPercentage float64, MinSyncPeers int) []peer.ID {
	// todo
	required := MinSyncPeers
	// Weak/slow peers will be pushed down the list and trimmed since only percentage of peers is selected.
	limit := math.Round(float64(len(peers)) * peersPercentage)
	// Limit cannot be less that minimum peers required by sync mechanism.
	limit = math.Max(limit, float64(required))
	// Limit cannot be higher than number of peers available (safe-guard).
	limit = math.Min(limit, float64(len(peers)))

	limit = math.Floor(limit)

	return peers[:uint64(limit)]
}
