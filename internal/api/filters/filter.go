package filters

import (
	"context"
	"errors"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal"
	"github.com/amazechain/amc/internal/consensus"
	vm2 "github.com/amazechain/amc/internal/vm"
	"github.com/amazechain/amc/internal/vm/evmtypes"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/libp2p/go-libp2p-core/peer"
	"math/big"
)

type Api interface {
	TxsPool() txs_pool.ITxsPool
	Downloader() common.IDownloader
	P2pServer() common.INetwork
	Peers() map[peer.ID]common.Peer
	Database() kv.RwDB
	Engine() consensus.Engine
	BlockChain() common.IBlockChain
	GetEvm(ctx context.Context, msg internal.Message, ibs evmtypes.IntraBlockState, header block.IHeader, vmConfig *vm2.Config) (*vm2.EVM, func() error, error)
}

// Filter can be used to retrieve and filter logs.
type Filter struct {
	api Api

	db        kv.RwDB
	addresses []types.Address
	topics    [][]types.Hash

	block      types.Hash // Block hash if filtering a single block
	begin, end int64      // Range interval if filtering multiple blocks

	//matcher *bloombits.Matcher
}

// NewRangeFilter creates a new filter which uses a bloom filter on blocks to
// figure out whether a particular block is interesting or not.
func NewRangeFilter(api Api, begin, end int64, addresses []types.Address, topics [][]types.Hash) *Filter {
	// Flatten the address and topic filter clauses into a single bloombits filter
	// system. Since the bloombits are not positional, nil topics are permitted,
	// which get flattened into a nil byte slice.
	var filters [][][]byte
	if len(addresses) > 0 {
		filter := make([][]byte, len(addresses))
		for i, address := range addresses {
			filter[i] = address.Bytes()
		}
		filters = append(filters, filter)
	}
	for _, topicList := range topics {
		filter := make([][]byte, len(topicList))
		for i, topic := range topicList {
			filter[i] = topic.Bytes()
		}
		filters = append(filters, filter)
	}
	//size, _ := api.BloomStatus()

	// Create a generic filter and convert it into a range filter
	filter := newFilter(api, addresses, topics)

	//filter.matcher = bloombits.NewMatcher(size, filters)
	filter.begin = begin
	filter.end = end

	return filter
}

// NewBlockFilter creates a new filter which directly inspects the contents of
// a block to figure out whether it is interesting or not.
func NewBlockFilter(api Api, block types.Hash, addresses []types.Address, topics [][]types.Hash) *Filter {
	// Create a generic filter and convert it into a block filter
	filter := newFilter(api, addresses, topics)
	filter.block = block
	return filter
}

// newFilter creates a generic filter that can either filter based on a block hash,
// or based on range queries. The search criteria needs to be explicitly set.
func newFilter(api Api, addresses []types.Address, topics [][]types.Hash) *Filter {
	return &Filter{
		api:       api,
		addresses: addresses,
		topics:    topics,
		db:        api.Database(),
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *Filter) Logs(ctx context.Context) ([]*block.Log, error) {
	// If we're doing singleton block filtering, execute and return
	if f.block != (types.Hash{}) {
		header, err := f.api.BlockChain().GetHeaderByHash(f.block)
		if err != nil {
			return nil, err
		}
		if header == nil {
			return nil, errors.New("unknown block")
		}
		return f.blockLogs(ctx, header)
	}
	// Short-cut if all we care about is pending logs
	if f.begin == jsonrpc.PendingBlockNumber.Int64() {
		if f.end != jsonrpc.PendingBlockNumber.Int64() {
			return nil, errors.New("invalid block range")
		}
		return f.pendingLogs()
	}
	// Figure out the limits of the filter range
	header := f.api.BlockChain().CurrentBlock().Header()
	if header == nil {
		return nil, nil
	}
	var (
		head    = header.Number64().Uint64()
		end     = uint64(f.end)
		pending = f.end == jsonrpc.PendingBlockNumber.Int64()
	)
	if f.begin == jsonrpc.LatestBlockNumber.Int64() {
		f.begin = int64(head)
	}
	if f.end == jsonrpc.LatestBlockNumber.Int64() || f.end == jsonrpc.PendingBlockNumber.Int64() {
		end = head
	}
	// Gather all indexed logs, and finish with non indexed ones
	var (
		logs           []*block.Log
		err            error
		size, sections = uint64(4096), uint64(10)
		//todo
	)
	if indexed := sections * size; indexed > uint64(f.begin) {
		if indexed > end {
			logs, err = f.indexedLogs(ctx, end)
		} else {
			logs, err = f.indexedLogs(ctx, indexed-1)
		}
		if err != nil {
			return logs, err
		}
	}
	rest, err := f.unindexedLogs(ctx, end)
	logs = append(logs, rest...)
	if pending {
		pendingLogs, err := f.pendingLogs()
		if err != nil {
			return nil, err
		}
		logs = append(logs, pendingLogs...)
	}
	return logs, err
}

// indexedLogs returns the logs matching the filter criteria based on the bloom
// bits indexed available locally or via the network.
func (f *Filter) indexedLogs(ctx context.Context, end uint64) ([]*block.Log, error) {
	// Create a matcher session and request servicing from the backend
	matches := make(chan uint64, 64)

	//session, err := f.matcher.Start(ctx, uint64(f.begin), end, matches)
	//if err != nil {
	//	return nil, err
	//}
	//defer session.Close()

	//f.api.ServiceFilter(ctx, session)

	// Iterate over the matches until exhausted or context closed
	var logs []*block.Log

	for {
		select {
		case number, ok := <-matches:
			// Abort if all matches have been fulfilled
			if !ok {
				//err := session.Error()
				//if err == nil {
				//	f.begin = int64(end) + 1
				//}
				return logs, nil
			}
			f.begin = int64(number) + 1

			// Retrieve the suggested block and pull any truly matching logs
			header := f.api.BlockChain().GetHeaderByNumber(uint256.NewInt(number))
			if header == nil {
				return logs, nil
			}
			found, err := f.checkMatches(ctx, header)
			if err != nil {
				return logs, err
			}
			logs = append(logs, found...)

		case <-ctx.Done():
			return logs, ctx.Err()
		}
	}
}

// unindexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *Filter) unindexedLogs(ctx context.Context, end uint64) ([]*block.Log, error) {
	var logs []*block.Log

	for ; f.begin <= int64(end); f.begin++ {
		header := f.api.BlockChain().GetHeaderByNumber(uint256.NewInt(uint64(f.begin)))
		if header == nil {
			return logs, nil
		}
		found, err := f.blockLogs(ctx, header)
		if err != nil {
			return logs, err
		}
		logs = append(logs, found...)
	}
	return logs, nil
}

// blockLogs returns the logs matching the filter criteria within a single block.
func (f *Filter) blockLogs(ctx context.Context, header block.IHeader) (logs []*block.Log, err error) {
	//todo header.Bloom
	bloom, _ := types.NewBloom(100)
	if bloomFilter(bloom, f.addresses, f.topics) {
		found, err := f.checkMatches(ctx, header)
		if err != nil {
			return logs, err
		}
		logs = append(logs, found...)
	}
	return logs, nil
}

// checkMatches checks if the receipts belonging to the given header contain any log events that
// match the filter criteria. This function is called when the bloom filter signals a potential match.
func (f *Filter) checkMatches(ctx context.Context, header block.IHeader) (logs []*block.Log, err error) {
	// Get the logs of the block
	logsList, err := f.api.BlockChain().GetLogs(header.Hash())
	if err != nil {
		return nil, err
	}
	var unfiltered []*block.Log
	for _, logs := range logsList {
		unfiltered = append(unfiltered, logs...)
	}
	logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)
	if len(logs) > 0 {
		// We have matching logs, check if we need to resolve full logs via the light client
		if logs[0].TxHash == (types.Hash{}) {
			receipts, err := f.api.BlockChain().GetReceipts(header.Hash())
			if err != nil {
				return nil, err
			}
			unfiltered = unfiltered[:0]
			for _, receipt := range receipts {
				unfiltered = append(unfiltered, receipt.Logs...)
			}
			logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)
		}
		return logs, nil
	}
	return nil, nil
}

// pendingLogs returns the logs matching the filter criteria within the pending block.
func (f *Filter) pendingLogs() ([]*block.Log, error) {
	//todo
	//pendingBlock, receipts := f.api.PendingBlockAndReceipts()
	//if bloomFilter(pendingBlock.Bloom(), f.addresses, f.topics) {
	//	var unfiltered []*block.Log
	//	for _, r := range receipts {
	//		unfiltered = append(unfiltered, r.Logs...)
	//	}
	//	return filterLogs(unfiltered, nil, nil, f.addresses, f.topics), nil
	//}
	return nil, nil
}

func includes(addresses []types.Address, a types.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}

// filterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []*block.Log, fromBlock, toBlock *big.Int, addresses []types.Address, topics [][]types.Hash) []*block.Log {
	var ret []*block.Log
Logs:
	for _, log := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > log.BlockNumber.Uint64() {
			continue
		}
		if toBlock != nil && toBlock.Int64() >= 0 && toBlock.Uint64() < log.BlockNumber.Uint64() {
			continue
		}

		if len(addresses) > 0 && !includes(addresses, log.Address) {
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue
		}
		for i, sub := range topics {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func bloomFilter(bloom *types.Bloom, addresses []types.Address, topics [][]types.Hash) bool {
	if len(addresses) > 0 {
		var included bool
		for _, addr := range addresses {
			if bloom.Contain(addr.Bytes()) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topics {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if bloom.Contain(topic.Bytes()) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}
