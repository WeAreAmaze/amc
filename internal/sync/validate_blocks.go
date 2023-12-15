package sync

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"go.opencensus.io/trace"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

var (
	ErrOptimisticParent = errors.New("parent of the block is optimistic")
)

// validateBlockPubSub checks that the incoming block has a valid BLS signature.
// Blocks that have already been seen are ignored. If the BLS signature is any valid signature,
// this method rebroadcasts the message.
func (s *Service) validateBlockPubSub(ctx context.Context, pid peer.ID, msg *pubsub.Message) (pubsub.ValidationResult, error) {
	receivedTime := time.Now()
	// Validation runs on publish (not just subscriptions), so we should approve any message from
	// ourselves.
	if pid == s.cfg.p2p.PeerID() {
		return pubsub.ValidationAccept, nil
	}

	// We should not attempt to process blocks until fully synced, but propagation is OK.
	if s.cfg.initialSync.Syncing() {
		return pubsub.ValidationIgnore, nil
	}

	ctx, span := trace.StartSpan(ctx, "sync.validateBlockPubSub")
	defer span.End()

	m, err := s.decodePubsubMessage(msg)
	if err != nil {
		//tracing.AnnotateError(span, err)
		return pubsub.ValidationReject, errors.Wrap(err, "Could not decode message")
	}

	s.validateBlockLock.Lock()
	defer s.validateBlockLock.Unlock()

	blk, ok := m.(*types_pb.Block)
	if !ok {
		return pubsub.ValidationReject, errors.New("msg is not types_pb.Block")
	}

	iBlock := new(block.Block)
	if err = iBlock.FromProtoMessage(blk); err != nil {
		return pubsub.ValidationReject, errors.New("block.Block is nil")
	}

	iHeader, iBody := iBlock.Header(), iBlock.Body()
	header, ok := iHeader.(*block.Header)
	if !ok {
		return pubsub.ValidationReject, errors.New("msg.header is not block.Header")
	}
	_, ok = iBody.(*block.Body)
	if !ok {
		return pubsub.ValidationReject, errors.New("msg.body is not types_pb.Block")
	}

	//todo
	// Broadcast the block on a feed to notify other services in the beacon node
	// of a received block (even if it does not process correctly through a state transition).

	if s.cfg.chain.HasBlock(header.Root, header.Number.Uint64()) {
		return pubsub.ValidationIgnore, nil
	}

	// Check if parent is a bad block and then reject the block.
	if s.hasBadBlock(header.ParentHash) {
		s.setBadBlock(ctx, header.Root)
		err := fmt.Errorf("received block with root %#x that has an invalid parent %#x", header.Root, header.ParentHash)
		log.Debug("Received block with an invalid parent", "err", err)
		return pubsub.ValidationReject, err
	}

	// Be lenient in handling early blocks. Instead of discarding blocks arriving later than
	// MAXIMUM_GOSSIP_CLOCK_DISPARITY in future, we tolerate blocks arriving at max two slots
	// earlier (SECONDS_PER_SLOT * 2 seconds). Queue such blocks and process them at the right slot.
	//genesisTime := uint64(s.cfg.chain.GenesisTime().Unix())
	//if header.Time > time.Now() {
	//	log.Error("Ignored block: could not verify header time")
	//	return pubsub.ValidationIgnore, nil
	//}

	// Add metrics for block arrival time subtracts slot start time.
	if err := captureArrivalTimeMetric(header.Time); err != nil {
		log.Debug("Ignored block: could not capture arrival time metric", "err", err)
		return pubsub.ValidationIgnore, nil
	}

	// Handle block when the parent is unknown.
	if !s.cfg.chain.HasBlock(header.ParentHash, header.Number.Uint64()-1) {
		// todo feature?
		return pubsub.ValidationIgnore, nil
	}

	msg.ValidatorData = iBlock.ToProtoMessage() // Used in downstream subscriber
	//
	//log.Debug("Received block")

	blockVerificationGossipSummary.Observe(float64(time.Since(receivedTime).Milliseconds()))
	return pubsub.ValidationAccept, nil
}

// Returns true if the block is marked as a bad block.
func (s *Service) hasBadBlock(root types.Hash) bool {
	s.badBlockLock.RLock()
	defer s.badBlockLock.RUnlock()
	_, seen := s.badBlockCache.Get(root)
	return seen
}

// Set bad block in the cache.
func (s *Service) setBadBlock(ctx context.Context, root types.Hash) {
	s.badBlockLock.Lock()
	defer s.badBlockLock.Unlock()
	if ctx.Err() != nil { // Do not mark block as bad if it was due to context error.
		return
	}
	log.Debug("Inserting in invalid block cache", "root", root)
	s.badBlockCache.Add(root, true)
}

// This captures metrics for block arrival time.
func captureArrivalTimeMetric(headerTime uint64) error {
	startTime := time.Unix(int64(headerTime), 0)
	//todo future block
	if time.Now().Sub(startTime) < 0 {
		return fmt.Errorf("the block is future block time is %s", startTime.Format(time.RFC3339))
	}
	ms := time.Now().Sub(startTime) / time.Millisecond
	arrivalBlockPropagationHistogram.Observe(float64(ms))
	arrivalBlockPropagationGauge.Set(float64(ms))

	return nil
}

// isBlockQueueable checks if the slot_time in the block is greater than
// current_time +  MAXIMUM_GOSSIP_CLOCK_DISPARITY. in short, this function
// returns true if the corresponding block should be queued and false if
// the block should be processed immediately.
//func isBlockQueueable(genesisTime uint64, slot primitives.Slot, receivedTime time.Time) bool {
//	slotTime, err := slots.ToTime(genesisTime, slot)
//	if err != nil {
//		return false
//	}
//
//	currentTimeWithDisparity := receivedTime.Add(params.BeaconNetworkConfig().MaximumGossipClockDisparity)
//	return currentTimeWithDisparity.Unix() < slotTime.Unix()
//}

//func getBlockFields(b interfaces.ReadOnlySignedBeaconBlock) logrus.Fields {
//	if consensusblocks.BeaconBlockIsNil(b) != nil {
//		return logrus.Fields{}
//	}
//	graffiti := b.Block().Body().Graffiti()
//	return logrus.Fields{
//		"slot":          b.Block().Slot(),
//		"proposerIndex": b.Block().ProposerIndex(),
//		"graffiti":      string(graffiti[:]),
//		"version":       b.Block().Version(),
//	}
//}
