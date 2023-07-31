package initialsync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/crypto/rand"
	"github.com/amazechain/amc/internal/p2p"
	leakybucket "github.com/amazechain/amc/internal/p2p/leaky-bucket"
	amcsync "github.com/amazechain/amc/internal/sync"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

const (
	// maxPendingRequests limits how many concurrent fetch request one can initiate.
	maxPendingRequests = 64
	// peersPercentagePerRequest caps percentage of peers to be used in a request.
	peersPercentagePerRequest = 0.75
	// handshakePollingInterval is a polling interval for checking the number of received handshakes.
	handshakePollingInterval = 5 * time.Second
	// peerLocksPollingInterval is a polling interval for checking if there are stale peer locks.
	peerLocksPollingInterval = 5 * time.Minute
	// peerLockMaxAge is maximum time before stale lock is purged.
	peerLockMaxAge = 60 * time.Minute
	// peerFilterCapacityWeight defines how peer's capacity affects peer's score. Provided as
	// percentage, i.e. 0.3 means capacity will determine 30% of peer's score.
	peerFilterCapacityWeight = 0.2
)

var (
	errNoPeersAvailable      = errors.New("no peers available, waiting for reconnect")
	errFetcherCtxIsDone      = errors.New("fetcher's context is done, reinitialize")
	errBlockNrIsTooHigh      = errors.New("block number is higher than the target block number")
	errBlockAlreadyProcessed = errors.New("block is already processed")
	errParentDoesNotExist    = errors.New("beacon node doesn't have a parent in db with root")
	errNoPeersWithAltBlocks  = errors.New("no peers with alternative blocks found")
)

// blocksFetcherConfig is a config to setup the block fetcher.
type blocksFetcherConfig struct {
	chain                    common.IBlockChain
	p2p                      p2p.P2P
	peerFilterCapacityWeight float64
	mode                     syncMode
}

// blocksFetcher is a service to fetch chain data from peers.
// On an incoming requests, requested block range is evenly divided
// among available peers (for fair network load distribution).
type blocksFetcher struct {
	sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	rand            *rand.Rand
	chain           common.IBlockChain
	p2p             p2p.P2P
	blocksPerPeriod uint64
	rateLimiter     *leakybucket.Collector
	peerLocks       map[peer.ID]*peerLock
	fetchRequests   chan *fetchRequestParams
	fetchResponses  chan *fetchRequestResponse
	capacityWeight  float64       // how remaining capacity affects peer selection
	mode            syncMode      // allows to use fetcher in different sync scenarios
	quit            chan struct{} // termination notifier
}

// peerLock restricts fetcher actions on per peer basis. Currently, used for rate limiting.
type peerLock struct {
	sync.Mutex
	accessed time.Time
}

// fetchRequestParams holds parameters necessary to schedule a fetch request.
type fetchRequestParams struct {
	ctx   context.Context // if provided, it is used instead of global fetcher's context
	start *uint256.Int    // starting slot
	count uint64          // how many slots to receive (fetcher may return fewer slots)
}

// fetchRequestResponse is a combined type to hold results of both successful executions and errors.
// Valid usage pattern will be to check whether result's `err` is nil, before using `blocks`.
type fetchRequestResponse struct {
	pid    peer.ID
	start  *uint256.Int
	count  uint64
	blocks []*types_pb.Block
	err    error
}

// newBlocksFetcher creates ready to use fetcher.
func newBlocksFetcher(ctx context.Context, cfg *blocksFetcherConfig) *blocksFetcher {

	// Initialize block limits.
	allowedBlocksPerSecond := float64(cfg.p2p.GetConfig().P2PLimit.BlockBatchLimit)
	allowedBlocksBurst := int64(cfg.p2p.GetConfig().P2PLimit.BlockBatchLimitBurstFactor * cfg.p2p.GetConfig().P2PLimit.BlockBatchLimit)

	blockLimiterPeriod := time.Duration(cfg.p2p.GetConfig().P2PLimit.BlockBatchLimiterPeriod) * time.Second

	// Allow fetcher to go almost to the full burst capacity (less a single batch).
	//rateLimiter := leakybucket.NewCollector(allowedBlocksPerSecond, allowedBlocksBurst-allowedBlocksBurst, blockLimiterPeriod, false /* deleteEmptyBuckets */)
	rateLimiter := leakybucket.NewCollector(allowedBlocksPerSecond, allowedBlocksBurst, blockLimiterPeriod, false /* deleteEmptyBuckets */)

	capacityWeight := cfg.peerFilterCapacityWeight
	if capacityWeight >= 1 {
		capacityWeight = peerFilterCapacityWeight
	}

	ctx, cancel := context.WithCancel(ctx)
	return &blocksFetcher{
		ctx:             ctx,
		cancel:          cancel,
		rand:            rand.NewGenerator(),
		chain:           cfg.chain,
		p2p:             cfg.p2p,
		blocksPerPeriod: uint64(allowedBlocksPerSecond),
		rateLimiter:     rateLimiter,
		peerLocks:       make(map[peer.ID]*peerLock),
		fetchRequests:   make(chan *fetchRequestParams, maxPendingRequests),
		fetchResponses:  make(chan *fetchRequestResponse, maxPendingRequests),
		capacityWeight:  capacityWeight,
		mode:            cfg.mode,
		quit:            make(chan struct{}),
	}
}

// start boots up the fetcher, which starts listening for incoming fetch requests.
func (f *blocksFetcher) start() error {
	select {
	case <-f.ctx.Done():
		return errFetcherCtxIsDone
	default:
		go f.loop()
		return nil
	}
}

// stop terminates all fetcher operations.
func (f *blocksFetcher) stop() {
	defer func() {
		if f.rateLimiter != nil {
			f.rateLimiter.Free()
			f.rateLimiter = nil
		}
	}()
	f.cancel()
	<-f.quit // make sure that loop() is done
}

// requestResponses exposes a channel into which fetcher pushes generated request responses.
func (f *blocksFetcher) requestResponses() <-chan *fetchRequestResponse {
	return f.fetchResponses
}

// loop is a main fetcher loop, listens for incoming requests/cancellations, forwards outgoing responses.
func (f *blocksFetcher) loop() {
	defer close(f.quit)

	// Wait for all loop's goroutines to finish, and safely release resources.
	wg := &sync.WaitGroup{}
	defer func() {
		wg.Wait()
		close(f.fetchResponses)
	}()

	// Periodically remove stale peer locks.
	go func() {
		ticker := time.NewTicker(peerLocksPollingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				f.removeStalePeerLocks(peerLockMaxAge)
			case <-f.ctx.Done():
				return
			}
		}
	}()

	// Main loop.
	for {
		// Make sure there is are available peers before processing requests.
		if _, err := f.waitForMinimumPeers(f.ctx); err != nil {
			log.Error("cannot wait peers", "err", err)
		}

		select {
		case <-f.ctx.Done():
			log.Debug("Context closed, exiting goroutine (blocks fetcher)")
			return
		case req := <-f.fetchRequests:
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case <-f.ctx.Done():
				case f.fetchResponses <- f.handleRequest(req.ctx, req.start, req.count):
				}
			}()
		}
	}
}

// scheduleRequest adds request to incoming queue.
func (f *blocksFetcher) scheduleRequest(ctx context.Context, start *uint256.Int, count uint64) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	request := &fetchRequestParams{
		ctx:   ctx,
		start: start,
		count: count,
	}
	select {
	case <-f.ctx.Done():
		return errFetcherCtxIsDone
	case f.fetchRequests <- request:
	}
	return nil
}

// handleRequest parses fetch request and forwards it to response builder.
func (f *blocksFetcher) handleRequest(ctx context.Context, start *uint256.Int, count uint64) *fetchRequestResponse {
	ctx, span := trace.StartSpan(ctx, "initialsync.handleRequest")
	defer span.End()

	response := &fetchRequestResponse{
		start:  start,
		count:  count,
		blocks: []*types_pb.Block{},
		err:    nil,
	}

	if ctx.Err() != nil {
		response.err = ctx.Err()
		return response
	}

	_, peers := f.p2p.Peers().BestPeers(f.p2p.GetConfig().MinSyncPeers, f.chain.CurrentBlock().Number64())
	if len(peers) == 0 {
		response.err = errNoPeersAvailable
		return response
	}

	response.blocks, response.pid, response.err = f.fetchBlocksFromPeer(ctx, start, count, peers)
	return response
}

// fetchBlocksFromPeer fetches blocks from a single randomly selected peer.
func (f *blocksFetcher) fetchBlocksFromPeer(ctx context.Context, start *uint256.Int, count uint64, peers []peer.ID) ([]*types_pb.Block, peer.ID, error) {
	ctx, span := trace.StartSpan(ctx, "initialsync.fetchBlocksFromPeer")
	defer span.End()

	peers = f.filterPeers(ctx, peers, peersPercentagePerRequest)
	req := &sync_pb.BodiesByRangeRequest{
		StartBlockNumber: utils.ConvertUint256IntToH256(start),
		Count:            count,
		Step:             1,
	}
	for i := 0; i < len(peers); i++ {
		blocks, err := f.requestBlocks(ctx, req, peers[i])
		if err == nil {
			f.p2p.Peers().Scorers().BlockProviderScorer().Touch(peers[i])
			return blocks, peers[i], err
		} else {
			log.Warn("Could not request blocks by range", "err", err)
		}
	}
	return nil, "", errNoPeersAvailable
}

// requestBlocks is a wrapper for handling BeaconBlocksByRangeRequest requests/streams.
func (f *blocksFetcher) requestBlocks(ctx context.Context, req *sync_pb.BodiesByRangeRequest, pid peer.ID) ([]*types_pb.Block, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	l := f.peerLock(pid)
	l.Lock()
	log.Debug("Requesting blocks",
		"peer", pid,
		"start", utils.ConvertH256ToUint256Int(req.StartBlockNumber).Uint64(),
		"count", req.Count,
		"step", req.Step,
		"capacity", f.rateLimiter.Remaining(pid.String()),
		"score", f.p2p.Peers().Scorers().BlockProviderScorer().FormatScorePretty(pid),
	)

	if f.rateLimiter.Remaining(pid.String()) < int64(req.Count) {
		if err := f.waitForBandwidth(pid, req.Count); err != nil {
			l.Unlock()
			return nil, err
		}
	}
	f.rateLimiter.Add(pid.String(), int64(req.Count))
	l.Unlock()
	return amcsync.SendBodiesByRangeRequest(ctx, f.chain, f.p2p, pid, req, nil)
}

// waitForBandwidth blocks up until peer's bandwidth is restored.
func (f *blocksFetcher) waitForBandwidth(pid peer.ID, count uint64) error {

	rem := f.rateLimiter.Remaining(pid.String())
	if uint64(rem) >= count {
		// Exit early if we have sufficient capacity
		return nil
	}
	//todo
	//intCount, err := math.Int(count)
	//if err != nil {
	//	return err
	//}
	toWait := timeToWait(int64(count), rem, f.rateLimiter.Capacity(), f.rateLimiter.TillEmpty(pid.String()))
	timer := time.NewTimer(toWait)

	log.Debug("Slowing down for rate limit", "peer", pid, "timeToWait", common.PrettyDuration(toWait))
	defer timer.Stop()
	select {
	case <-f.ctx.Done():
		return errFetcherCtxIsDone
	case <-timer.C:
		// Peer has gathered enough capacity to be polled again.
	}
	return nil
}

// Determine how long it will take for us to have the required number of blocks allowed by our rate limiter.
// We do this by calculating the duration till the rate limiter can request these blocks without exceeding
// the provided bandwidth limits per peer.
func timeToWait(wanted, rem, capacity int64, timeTillEmpty time.Duration) time.Duration {
	// Defensive check if we have more than enough blocks
	// to request from the peer.
	if rem >= wanted {
		return 0
	}
	// Handle edge case where capacity is equal to the remaining amount
	// of blocks. This also handles the impossible case in where remaining blocks
	// exceed the limiter's capacity.
	if capacity <= rem {
		return 0
	}
	blocksNeeded := wanted - rem
	currentNumBlks := capacity - rem
	expectedTime := int64(timeTillEmpty) * blocksNeeded / currentNumBlks
	return time.Duration(expectedTime)
}
