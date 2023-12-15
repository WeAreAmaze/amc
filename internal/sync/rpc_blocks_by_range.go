package sync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	types "github.com/amazechain/amc/common/block"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"time"

	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

// bodiesByRangeRPCHandler looks up the request blocks from the database from a given start block.
func (s *Service) bodiesByRangeRPCHandler(ctx context.Context, msg interface{}, stream libp2pcore.Stream) error {
	ctx, span := trace.StartSpan(ctx, "sync.BodiesByRangeHandler")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()
	SetRPCStreamDeadlines(stream)

	// Ticker to stagger out large requests.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	m, ok := msg.(*sync_pb.BodiesByRangeRequest)
	if !ok {
		return errors.New("message is not type *pb.BeaconBlockByRangeRequest")
	}
	if err := s.validateRangeRequest(m); err != nil {
		s.writeErrorResponseToStream(responseCodeInvalidRequest, err.Error(), stream)
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		//tracing.AnnotateError(span, err)
		return err
	}
	// Only have range requests with a step of 1 being processed.
	if m.Step > 1 {
		m.Step = 1
	}
	// The initial count for the first batch to be returned back.
	count := m.Count
	allowedBlocksPerSecond := uint64(s.cfg.p2p.GetConfig().P2PLimit.BlockBatchLimit)
	if count > allowedBlocksPerSecond {
		count = allowedBlocksPerSecond
	}
	// initial batch start and end slots to be returned to remote peer.
	startBlockNumber := utils.ConvertH256ToUint256Int(m.StartBlockNumber)
	endBlockNumber := new(uint256.Int).AddUint64(startBlockNumber, m.Step*(count-1))

	// The final requested slot from remote peer.
	endReqBlockNumber := new(uint256.Int).AddUint64(startBlockNumber, m.Step*(m.Count-1))

	blockLimiter, err := s.rateLimiter.topicCollector(string(stream.Protocol()))
	if err != nil {
		return err
	}
	remainingBucketCapacity := blockLimiter.Remaining(stream.Conn().RemotePeer().String())
	span.AddAttributes(
		trace.Int64Attribute("start", int64(startBlockNumber.Uint64())), // lint:ignore uintcast -- This conversion is OK for tracing.
		trace.Int64Attribute("end", int64(endReqBlockNumber.Uint64())),  // lint:ignore uintcast -- This conversion is OK for tracing.
		trace.Int64Attribute("step", int64(m.Step)),
		trace.Int64Attribute("count", int64(m.Count)),
		trace.StringAttribute("peer", stream.Conn().RemotePeer().Pretty()),
		trace.Int64Attribute("remaining_capacity", remainingBucketCapacity),
	)
	for startBlockNumber.Cmp(endReqBlockNumber) <= 0 {
		if err := s.rateLimiter.validateRequest(stream, allowedBlocksPerSecond); err != nil {
			//tracing.AnnotateError(span, err)
			return err
		}

		if new(uint256.Int).Sub(endBlockNumber, startBlockNumber).Uint64() > rangeLimit {
			s.writeErrorResponseToStream(responseCodeInvalidRequest, p2ptypes.ErrInvalidRequest.Error(), stream)
			err := p2ptypes.ErrInvalidRequest
			//tracing.AnnotateError(span, err)
			return err
		}

		err := s.writeBodiesRangeToStream(ctx, startBlockNumber, endBlockNumber, m.Step, stream)
		if err != nil && !errors.Is(err, p2ptypes.ErrInvalidParent) {
			return err
		}
		// Reduce capacity of peer in the rate limiter first.
		// Decrease allowed blocks capacity by the number of streamed blocks.
		if startBlockNumber.Cmp(endBlockNumber) <= 0 {
			s.rateLimiter.add(stream, int64(1+new(uint256.Int).Div(new(uint256.Int).Sub(endBlockNumber, startBlockNumber), new(uint256.Int).SetUint64(m.Step)).Uint64()))
		}
		// Exit in the event we have a disjoint chain to
		// return.
		if errors.Is(err, p2ptypes.ErrInvalidParent) {
			break
		}

		// Recalculate start and end slots for the next batch to be returned to the remote peer.
		startBlockNumber = new(uint256.Int).AddUint64(endBlockNumber, m.Step)

		endBlockNumber = new(uint256.Int).AddUint64(startBlockNumber, m.Step*(allowedBlocksPerSecond-1))
		if endBlockNumber.Cmp(endReqBlockNumber) == 1 {
			endBlockNumber = endReqBlockNumber
		}

		// do not wait if all blocks have already been sent.
		if startBlockNumber.Cmp(endReqBlockNumber) == 1 {
			break
		}

		// wait for ticker before resuming streaming blocks to remote peer.
		<-ticker.C
	}
	closeStream(stream)
	return nil
}

func (s *Service) writeBodiesRangeToStream(ctx context.Context, startSlot, endSlot *uint256.Int, step uint64, stream libp2pcore.Stream) error {
	ctx, span := trace.StartSpan(ctx, "sync.WriteBodiesRangeToStream")
	defer span.End()

	var blks = make([]types.IBlock, 0)
	for ; startSlot.Cmp(endSlot) <= 0; startSlot = startSlot.AddUint64(startSlot, step) {
		b, err := s.cfg.chain.GetBlockByNumber(startSlot)
		if err != nil {
			//tracing.AnnotateError(span, err)
			log.Warn("Could not retrieve blocks", "err", err)
			s.writeErrorResponseToStream(responseCodeServerError, p2ptypes.ErrGeneric.Error(), stream)
			return err
		}
		if b == nil {
			break
			// I don't think this an error

			//log.Warn("Could not retrieve blocks", "err", fmt.Errorf("block #%d not found", startSlot.Uint64()))
			//s.writeErrorResponseToStream(responseCodeServerError, p2ptypes.ErrInvalidBlockNr.Error(), stream)
			//return err
		}

		blks = append(blks, b)
	}

	start := time.Now()

	for _, b := range blks {
		if chunkErr := s.chunkBlockWriter(stream, b); chunkErr != nil {
			log.Debug("Could not send a chunked response", "err", chunkErr)
			s.writeErrorResponseToStream(responseCodeServerError, p2ptypes.ErrGeneric.Error(), stream)
			//tracing.AnnotateError(span, chunkErr)
			return chunkErr
		}
	}

	rpcBlocksByRangeResponseLatency.Observe(float64(time.Since(start).Milliseconds()))
	// Return error in the event we have an invalid parent.
	return nil
}

func (s *Service) validateRangeRequest(r *sync_pb.BodiesByRangeRequest) error {
	startSlot := utils.ConvertH256ToUint256Int(r.StartBlockNumber)
	count := r.Count
	step := r.Step

	// Add a buffer for possible large range requests from nodes syncing close to the
	// head of the chain.
	buffer := rangeLimit * 2
	highestExpectedBlockNumber := new(uint256.Int).AddUint64(s.cfg.chain.CurrentBlock().Number64(), uint64(buffer))

	// Ensure all request params are within appropriate bounds
	if count == 0 || count > maxRequestBlocks {
		return p2ptypes.ErrInvalidRequest
	}

	if step == 0 || step > rangeLimit {
		return p2ptypes.ErrInvalidRequest
	}

	if startSlot.Cmp(highestExpectedBlockNumber) == 1 {
		return p2ptypes.ErrInvalidRequest
	}

	endSlot := new(uint256.Int).AddUint64(startSlot, step*(count-1))
	if endSlot.Uint64()-startSlot.Uint64() > rangeLimit {
		return p2ptypes.ErrInvalidRequest
	}
	return nil
}

func (s *Service) writeErrorResponseToStream(responseCode byte, reason string, stream libp2pcore.Stream) {
	writeErrorResponseToStream(responseCode, reason, stream, s.cfg.p2p)
}
