// Package initialsync includes all initial block download and processing
// logic for the node, using a round robin strategy and a finite-state-machine
// to handle edge-cases in a beacon node's sync status.
package initialsync

import (
	"context"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/paulbellamy/ratecounter"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

// todo
const minimumSyncPeers = 1
const blockBatchLimit = 10
const blockBatchLimitBurstFactor = 2

// Config to set up the initial sync service.
type Config struct {
	P2P   p2p.P2P
	Chain common.IBlockChain
}

// Service service.
type Service struct {
	cfg     *Config
	ctx     context.Context
	cancel  context.CancelFunc
	synced  atomic.Bool
	syncing atomic.Bool
	counter *ratecounter.RateCounter
}

// NewService configures the initial sync service responsible for bringing the node up to the
// latest head of the blockchain.
func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		counter: ratecounter.NewRateCounter(counterSeconds * time.Second),
	}

	return s
}

// Start the initial sync service.
func (s *Service) Start() {
	log.Info("Starting initial chain sync...")
	highestExpectedBlockNr := s.waitForMinimumPeers()
	if err := s.roundRobinSync(highestExpectedBlockNr); err != nil {
		if errors.Is(s.ctx.Err(), context.Canceled) {
			return
		}
		panic(err)
	}
	log.Info("Synced up to blockNr: %d", s.cfg.Chain.CurrentBlock().Number64())
	s.markSynced()
}

// Stop initial sync.
func (s *Service) Stop() error {
	s.cancel()
	return nil
}

// Status of initial sync.
func (s *Service) Status() error {
	if s.syncing.Load() == true {
		return errors.New("syncing")
	}
	return nil
}

// Syncing returns true if initial sync is still running.
func (s *Service) Syncing() bool {
	return s.syncing.Load()
}

// Synced returns true if initial sync has been completed.
func (s *Service) Synced() bool {
	return s.synced.Load()
}

// Resync allows a node to start syncing again if it has fallen
// behind the current network head.
func (s *Service) Resync() error {
	// Set it to false since we are syncing again.
	s.markSyncing()
	defer func() {
		s.markSynced()
	}() // Reset it at the end of the method.
	//
	beforeBlockNr := s.cfg.Chain.CurrentBlock().Number64()
	highestExpectedBlockNr := s.waitForMinimumPeers()
	if err := s.roundRobinSync(highestExpectedBlockNr); err != nil {
		log.Error("Resync fail", "err", err, "highestExpectedBlockNr", highestExpectedBlockNr, "currentNr", s.cfg.Chain.CurrentBlock().Number64(), "beforeResyncBlockNr", beforeBlockNr)
		return err
	}
	//
	log.Info("Resync attempt complete", "highestExpectedBlockNr", highestExpectedBlockNr, "currentNr", s.cfg.Chain.CurrentBlock().Number64(), "beforeResyncBlockNr", beforeBlockNr)
	return nil
}

func (s *Service) waitForMinimumPeers() (highestExpectedBlockNr *uint256.Int) {
	required := minimumSyncPeers
	var peers []peer.ID
	for {
		cb := s.cfg.Chain.CurrentBlock()
		highestExpectedBlockNr, peers = s.cfg.P2P.Peers().BestPeers(minimumSyncPeers, cb.Number64())
		if len(peers) >= required {
			break
		}
		log.Info("Waiting for enough suitable peers before syncing",
			"suitable", len(peers),
			"required", required,
		)
		time.Sleep(handshakePollingInterval)
	}
	return
}

// markSynced marks node as synced and notifies feed listeners.
func (s *Service) markSyncing() {
	s.syncing.Swap(true)
}

// markSynced marks node as synced and notifies feed listeners.
func (s *Service) markSynced() {
	s.syncing.Swap(false)
	s.synced.Swap(true)
}
