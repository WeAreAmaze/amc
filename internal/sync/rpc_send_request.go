package sync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"io"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

// ErrInvalidFetchedData is thrown if stream fails to provide requested blocks.
var ErrInvalidFetchedData = errors.New("invalid data returned from peer")

// BlockProcessor defines a block processing function, which allows to start utilizing
// blocks even before all blocks are ready.
type BlockProcessor func(block *types_pb.Block) error

// SendBodiesByRangeRequest sends BeaconBlocksByRange and returns fetched blocks, if any.
func SendBodiesByRangeRequest(ctx context.Context, chain common.IBlockChain, p2pProvider p2p.SenderEncoder, pid peer.ID, req *sync_pb.BodiesByRangeRequest, blockProcessor BlockProcessor) ([]*types_pb.Block, error) {
	topic, err := p2p.TopicFromMessage(p2p.BodiesByRangeMessageName)
	if err != nil {
		return nil, err
	}
	//todo
	stream, err := p2pProvider.Send(ctx, &sync_pb.BodiesByRangeRequest{
		StartBlockNumber: utils.ConvertUint256IntToH256(utils.ConvertH256ToUint256Int(req.StartBlockNumber)),
		Count:            req.Count,
		Step:             req.Step,
	}, topic, pid)

	if err != nil {
		return nil, err
	}
	defer closeStream(stream)

	// Augment block processing function, if non-nil block processor is provided.
	blocks := make([]*types_pb.Block, 0, req.Count)
	process := func(blk *types_pb.Block) error {
		blocks = append(blocks, blk)
		if blockProcessor != nil {
			return blockProcessor(blk)
		}
		return nil
	}
	var prevBlockNr *uint256.Int
	blockStart := utils.ConvertH256ToUint256Int(req.StartBlockNumber)
	for i := uint64(0); ; i++ {
		isFirstChunk := i == 0
		blk, err := ReadChunkedBlock(stream, p2pProvider, isFirstChunk)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		// The response MUST contain no more than `count` blocks, and no more than
		// MAX_REQUEST_BLOCKS blocks.
		if i >= req.Count || i >= maxRequestBlocks {
			return nil, ErrInvalidFetchedData
		}
		blockNr := utils.ConvertH256ToUint256Int(blk.Header.Number)
		// Returned blocks MUST be in the slot range [start_slot, start_slot + count * step).
		if blockNr.Cmp(blockStart) == -1 || blockNr.Cmp(new(uint256.Int).AddUint64(blockStart, req.Count*req.Step)) >= 0 {
			return nil, ErrInvalidFetchedData
		}
		// Returned blocks, where they exist, MUST be sent in a consecutive order.
		// Consecutive blocks MUST have values in `step` increments (slots may be skipped in between).
		isSlotOutOfOrder := false
		if prevBlockNr != nil && prevBlockNr.Cmp(blockNr) >= 0 {
			isSlotOutOfOrder = true
		} else if prevBlockNr != nil && req.Step != 0 && new(uint256.Int).Mod(new(uint256.Int).Sub(blockNr, prevBlockNr), uint256.NewInt(req.Step)).Uint64() != 0 {
			isSlotOutOfOrder = true
		}
		if !isFirstChunk && isSlotOutOfOrder {
			return nil, ErrInvalidFetchedData
		}
		prevBlockNr = blockNr.Clone()
		if err := process(blk); err != nil {
			return nil, err
		}
	}

	return blocks, nil
}
