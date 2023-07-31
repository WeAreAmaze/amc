package initialsync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

// forkData represents alternative chain path supported by a given peer.
// Blocks are stored in an ascending slot order. The first block is guaranteed to have parent
// either in DB or initial sync cache.
type forkData struct {
	peer   peer.ID
	blocks []*types_pb.Block
}

// nonSkippedSlotAfter checks slots after the given one in an attempt to find a non-empty future slot.
// For efficiency only one random slot is checked per epoch, so returned slot might not be the first
// non-skipped slot. This shouldn't be a problem, as in case of adversary peer, we might get incorrect
// data anyway, so code that relies on this function must be robust enough to re-request, if no progress
// is possible with a returned value.
func (f *blocksFetcher) nonSkippedSlotAfter(ctx context.Context, blockNr *uint256.Int) (*uint256.Int, error) {
	ctx, span := trace.StartSpan(ctx, "initialsync.nonSkippedSlotAfter")
	defer span.End()
	//todo
	return nil, nil
}

// findFork queries all peers that have higher head slot, in an attempt to find
// ones that feature blocks from alternative branches. Once found, peer is further queried
// to find common ancestor slot. On success, all obtained blocks and peer is returned.
func (f *blocksFetcher) findFork(ctx context.Context, blockNr *uint256.Int) (*forkData, error) {
	ctx, span := trace.StartSpan(ctx, "initialsync.findFork")
	defer span.End()

	return nil, errNoPeersWithAltBlocks
}

// findForkWithPeer loads some blocks from a peer in an attempt to find alternative blocks.
func (f *blocksFetcher) findForkWithPeer(ctx context.Context, pid peer.ID, blockNr *uint256.Int) (*forkData, error) {

	//fork, err := f.findAncestor(ctx, pid, blockNr)
	return nil, errors.New("no alternative blocks exist within scanned range")
}

// findAncestor tries to figure out common ancestor slot that connects a given root to known block.
func (f *blocksFetcher) findAncestor(ctx context.Context, pid peer.ID, b *types_pb.Block) (*forkData, error) {

	return nil, errors.New("no common ancestor found")
}

// bestFinalizedSlot returns the highest finalized slot of the majority of connected peers.
func (f *blocksFetcher) bestFinalizedBlockNr() *uint256.Int {
	finalizedBlockNr, _ := f.p2p.Peers().BestPeers(f.p2p.GetConfig().MinSyncPeers, f.chain.CurrentBlock().Number64())
	return finalizedBlockNr
}
