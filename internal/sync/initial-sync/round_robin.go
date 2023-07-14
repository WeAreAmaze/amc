package initialsync

import (
	"context"
	"errors"
	"github.com/amazechain/amc/api/protocol/types_pb"
	block2 "github.com/amazechain/amc/common/block"
	"github.com/holiman/uint256"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/paulbellamy/ratecounter"
)

const (
	// counterSeconds is an interval over which an average rate will be calculated.
	counterSeconds = 20
)

// batchBlockReceiverFn defines batch receiving function.
type batchBlockReceiverFn func(chain []block2.IBlock) (int, error)

// Round Robin sync looks at the latest peer statuses and syncs up to the highest known epoch.
//
// Step 1 - Sync to finalized epoch.
// Sync with peers having the majority on best finalized epoch greater than node's head state.
func (s *Service) roundRobinSync(highestExpectedBlockNr *uint256.Int) error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	s.counter = ratecounter.NewRateCounter(counterSeconds * time.Second)

	// Step 1 - Sync to end of finalized BlockNr.
	if err := s.syncToFinalizedBlockNr(ctx, highestExpectedBlockNr); err != nil {
		return err
	}
	return nil
}

// syncToFinalizedBlockNr sync from head to best known finalized epoch.
func (s *Service) syncToFinalizedBlockNr(ctx context.Context, highestExpectedBlockNr *uint256.Int) error {

	if s.cfg.Chain.CurrentBlock().Number64().Cmp(highestExpectedBlockNr) >= 0 {
		// No need to sync, already synced to the finalized slot.
		log.Debug("Already synced to finalized epoch")
		return nil
	}
	queue := newBlocksQueue(ctx, &blocksQueueConfig{
		p2p:                    s.cfg.P2P,
		chain:                  s.cfg.Chain,
		highestExpectedBlockNr: highestExpectedBlockNr,
		mode:                   modeStopOnFinalizedEpoch,
	})
	if err := queue.start(); err != nil {
		return err
	}

	for data := range queue.fetchedData {
		s.processFetchedData(ctx, s.cfg.Chain.CurrentBlock().Number64(), data)
	}

	log.Info("Synced to finalized epoch - now syncing blocks up to current head", "syncedBlockNr", s.cfg.Chain.CurrentBlock().Number64(), "highestExpectedBlockNr", highestExpectedBlockNr)
	if err := queue.stop(); err != nil {
		log.Debug("Error stopping queue", "err", err)
	}

	return nil
}

// processFetchedData processes data received from queue.
func (s *Service) processFetchedData(ctx context.Context, startBlockNr *uint256.Int, data *blocksQueueFetchedData) {
	defer s.updatePeerScorerStats(data.pid, startBlockNr)

	// Use Batch Block Verify to process and verify batches directly.
	if _, err := s.processBatchedBlocks(ctx, data.blocks, s.cfg.Chain.InsertChain); err != nil {
		log.Warn("Skip processing batched blocks", "err", err)
	}
}

func (s *Service) processBatchedBlocks(ctx context.Context, blks []*types_pb.Block, bFunc batchBlockReceiverFn) (int, error) {
	if len(blks) == 0 {
		return 0, errors.New("0 blocks provided into method")
	}
	blocks := make([]block2.IBlock, len(blks))

	for _, blk := range blks {
		var block block2.IBlock
		if err := block.FromProtoMessage(blk); err != nil {
			return 0, err
		}
		blocks = append(blocks, block)
	}
	return bFunc(blocks)
}

// updatePeerScorerStats adjusts monitored metrics for a peer.
func (s *Service) updatePeerScorerStats(pid peer.ID, startBlockNr *uint256.Int) {
	if pid == "" {
		return
	}
	headBlockNr := s.cfg.Chain.CurrentBlock().Number64()
	if startBlockNr.Uint64() >= headBlockNr.Uint64() {
		return
	}
	if diff := headBlockNr.Uint64() - startBlockNr.Uint64(); diff > 0 {
		scorer := s.cfg.P2P.Peers().Scorers().BlockProviderScorer()
		scorer.IncrementProcessedBlocks(pid, diff)
	}
}