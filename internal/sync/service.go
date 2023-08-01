// Package sync includes all chain-synchronization logic for the AmazeChain node,
// including gossip-sub blocks, txs, and other p2p
// messages, as well as ability to process and respond to block requests
// by peers.
package sync

import (
	"context"
	"github.com/amazechain/amc/common"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/amazechain/amc/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
)

const rangeLimit = 1024
const seenBlockSize = 1000
const badBlockSize = 1000
const syncMetricsInterval = 10 * time.Second

// todo
const ttfbTimeout = 5 * time.Second  // TtfbTimeout is the maximum time to wait for first byte of request response (time-to-first-byte).
const respTimeout = 10 * time.Second // RespTimeout is the maximum time for complete response transfer.

// todo
const maxRequestBlocks = 1024

const maintainPeerStatusesInterval = 2 * time.Minute

const resyncInterval = 1 * time.Minute

const enableFullSSZDataLogging = false

var (
	// Seconds in one block.
	pendingBlockExpTime = 8 * time.Second
	errWrongMessage     = errors.New("wrong pubsub message")
	errNilMessage       = errors.New("nil pubsub message")
)

// Common type for functional p2p validation options.
type validationFn func(ctx context.Context) (pubsub.ValidationResult, error)

// config to hold dependencies for the sync service.
type config struct {
	p2p         p2p.P2P
	chain       common.IBlockChain
	initialSync Checker
}

// This defines the interface for interacting with block chain service
type blockchainService interface {
}

// Service is responsible for handling all run time p2p related operations as the
// main entry point for network messages.
type Service struct {
	cfg    *config
	ctx    context.Context
	cancel context.CancelFunc

	subHandler  *subTopicHandler
	rateLimiter *limiter

	seenBlockCache *lru.Cache[types.Hash, *block2.Block]
	seenBlockLock  sync.RWMutex
	badBlockLock   sync.RWMutex
	badBlockCache  *lru.Cache[types.Hash, bool]

	validateBlockLock               sync.RWMutex
	seenExitLock                    sync.RWMutex
	seenSyncMessageLock             sync.RWMutex
	seenSyncContributionLock        sync.RWMutex
	syncContributionBitsOverlapLock sync.RWMutex
}

// NewService initializes new regular sync service.
func NewService(ctx context.Context, opts ...Option) *Service {
	ctx, cancel := context.WithCancel(ctx)
	r := &Service{
		ctx:    ctx,
		cancel: cancel,
		cfg:    &config{},
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil
		}
	}

	r.subHandler = newSubTopicHandler()
	r.rateLimiter = newRateLimiter(r.cfg.p2p)
	r.initCaches()

	r.registerRPCHandlers()

	digest, err := r.currentForkDigest()
	if err != nil {
		panic("Could not retrieve current fork digest")
	}
	r.registerSubscribers(digest)
	//go r.forkWatcher()

	return r
}

// Start the regular sync service.
func (s *Service) Start() {
	s.cfg.p2p.AddConnectionHandler(s.reValidatePeer, s.sendGoodbye)
	s.cfg.p2p.AddDisconnectionHandler(func(_ context.Context, _ peer.ID) error {
		// no-op
		return nil
	})
	s.cfg.p2p.AddPingMethod(s.sendPingRequest)
	s.maintainPeerStatuses()
	s.resyncIfBehind()

	// Update sync metrics.
	utils.RunEvery(s.ctx, syncMetricsInterval, s.updateMetrics)
}

// Stop the regular sync service.
func (s *Service) Stop() error {
	defer func() {
		if s.rateLimiter != nil {
			s.rateLimiter.free()
		}
	}()
	// Removing RPC Stream handlers.
	for _, p := range s.cfg.p2p.Host().Mux().Protocols() {
		s.cfg.p2p.Host().RemoveStreamHandler(protocol.ID(p))
	}
	// Deregister Topic Subscribers.
	for _, t := range s.cfg.p2p.PubSub().GetTopics() {
		s.unSubscribeFromTopic(t)
	}
	defer s.cancel()
	return nil
}

// Status of the currently running regular sync service.
func (s *Service) Status() error {
	// If our HighestBlockNumber lower than our peers are reporting then we might be out of sync.
	if s.cfg.chain.CurrentBlock().Number64().Uint64()+1 < s.cfg.p2p.Peers().HighestBlockNumber().Uint64() {
		return errors.New("out of sync")
	}
	return nil
}

// This initializes the caches to update seen beacon objects coming in from the wire
// and prevent DoS.
func (s *Service) initCaches() {
	s.badBlockCache, _ = lru.New[types.Hash, bool](seenBlockSize)
	s.seenBlockCache, _ = lru.New[types.Hash, *block2.Block](badBlockSize)
}

// marks the chain as having started.
func (s *Service) markForChainStart() {
	//s.chainStarted.Set()
}

// Checker defines a struct which can verify whether a node is currently
// synchronizing a chain with the rest of peers in the network.
type Checker interface {
	Syncing() bool
	Synced() bool
	Status() error
	Resync() error
}
