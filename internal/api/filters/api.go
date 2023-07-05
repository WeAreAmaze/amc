package filters

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"sync"
	"time"
)

// filter is a helper struct that holds metadata information over the filter type
// and associated subscription in the event system.
type filter struct {
	typ      Type
	deadline *time.Timer // filter is inactiv when deadline triggers
	hashes   []types.Hash
	crit     FilterCriteria
	logs     []*block.Log
	s        *Subscription // associated subscription in event system
}

// FilterAPI offers support to create and manage filters. This will allow external clients to retrieve various
// information related to the Ethereum protocol such als blocks, transactions and logs.
type FilterAPI struct {
	api       Api
	events    *EventSystem
	filtersMu sync.Mutex
	filters   map[jsonrpc.ID]*filter
	timeout   time.Duration
}

// NewFilterAPI returns a new FilterAPI instance.
func NewFilterAPI(api Api, timeout time.Duration) *FilterAPI {
	filterAPI := &FilterAPI{
		api:     api,
		events:  NewEventSystem(api),
		filters: make(map[jsonrpc.ID]*filter),
		timeout: timeout,
	}
	go filterAPI.timeoutLoop(timeout)

	return filterAPI
}

// timeoutLoop runs at the interval set by 'timeout' and deletes filters
// that have not been recently used. It is started when the API is created.
func (filterApi *FilterAPI) timeoutLoop(timeout time.Duration) {
	var toUninstall []*Subscription
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	for {
		<-ticker.C
		filterApi.filtersMu.Lock()
		for id, f := range filterApi.filters {
			select {
			case <-f.deadline.C:
				toUninstall = append(toUninstall, f.s)
				delete(filterApi.filters, id)
			default:
				continue
			}
		}
		filterApi.filtersMu.Unlock()

		// Unsubscribes are processed outside the lock to avoid the following scenario:
		// event loop attempts broadcasting events to still active filters while
		// Unsubscribe is waiting for it to process the uninstall request.
		for _, s := range toUninstall {
			s.Unsubscribe()
		}
		toUninstall = nil
	}
}

// NewPendingTransactionFilter creates a filter that fetches pending transaction hashes
// as transactions enter the pending state.
//
// It is part of the filter package because this filter can be used through the
// `eth_getFilterChanges` polling method that is also used for log filters.
func (filterApi *FilterAPI) NewPendingTransactionFilter() jsonrpc.ID {
	var (
		pendingTxs   = make(chan []types.Hash)
		pendingTxSub = filterApi.events.SubscribePendingTxs(pendingTxs)
	)

	filterApi.filtersMu.Lock()
	filterApi.filters[pendingTxSub.ID] = &filter{typ: PendingTransactionsSubscription, deadline: time.NewTimer(filterApi.timeout), hashes: make([]types.Hash, 0), s: pendingTxSub}
	filterApi.filtersMu.Unlock()

	go func() {
		for {
			select {
			case ph := <-pendingTxs:
				filterApi.filtersMu.Lock()
				if f, found := filterApi.filters[pendingTxSub.ID]; found {
					f.hashes = append(f.hashes, ph...)
				}
				filterApi.filtersMu.Unlock()
			case <-pendingTxSub.Err():
				filterApi.filtersMu.Lock()
				delete(filterApi.filters, pendingTxSub.ID)
				filterApi.filtersMu.Unlock()
				return
			}
		}
	}()

	return pendingTxSub.ID
}

// NewPendingTransactions creates a subscription that is triggered each time a transaction
// enters the transaction pool and was signed from one of the transactions this nodes manages.
func (filterApi *FilterAPI) NewPendingTransactions(ctx context.Context) (*jsonrpc.Subscription, error) {
	notifier, supported := jsonrpc.NotifierFromContext(ctx)
	if !supported {
		return &jsonrpc.Subscription{}, jsonrpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		txHashes := make(chan []types.Hash, 128)
		pendingTxSub := filterApi.events.SubscribePendingTxs(txHashes)

		for {
			select {
			case hashes := <-txHashes:
				// To keep the original behaviour, send a single tx hash in one notification.
				// TODO(rjl493456442) Send a batch of tx hashes in one notification
				for _, h := range hashes {
					notifier.Notify(rpcSub.ID, h)
				}
			case <-rpcSub.Err():
				pendingTxSub.Unsubscribe()
				return
			case <-notifier.Closed():
				pendingTxSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewBlockFilter creates a filter that fetches blocks that are imported into the chain.
// It is part of the filter package since polling goes with eth_getFilterChanges.
func (filterApi *FilterAPI) NewBlockFilter() jsonrpc.ID {
	var (
		headers   = make(chan block.IHeader)
		headerSub = filterApi.events.SubscribeNewHeads(headers)
	)

	filterApi.filtersMu.Lock()
	filterApi.filters[headerSub.ID] = &filter{typ: BlocksSubscription, deadline: time.NewTimer(filterApi.timeout), hashes: make([]types.Hash, 0), s: headerSub}
	filterApi.filtersMu.Unlock()

	go func() {
		for {
			select {
			case h := <-headers:
				filterApi.filtersMu.Lock()
				if f, found := filterApi.filters[headerSub.ID]; found {
					f.hashes = append(f.hashes, h.Hash())
				}
				filterApi.filtersMu.Unlock()
			case <-headerSub.Err():
				filterApi.filtersMu.Lock()
				delete(filterApi.filters, headerSub.ID)
				filterApi.filtersMu.Unlock()
				return
			}
		}
	}()

	return headerSub.ID
}

// NewHeads send a notification each time a new (header) block is appended to the chain.
func (filterApi *FilterAPI) NewHeads(ctx context.Context) (*jsonrpc.Subscription, error) {
	notifier, supported := jsonrpc.NotifierFromContext(ctx)
	if !supported {
		return &jsonrpc.Subscription{}, jsonrpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan block.IHeader)
		headersSub := filterApi.events.SubscribeNewHeads(headers)
		for {
			select {
			case h := <-headers:
				notifier.Notify(rpcSub.ID, mvm_types.FromAmcHeader(h))
			case <-rpcSub.Err():
				headersSub.Unsubscribe()
				return
			case <-notifier.Closed():
				headersSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (filterApi *FilterAPI) Logs(ctx context.Context, crit FilterCriteria) (*jsonrpc.Subscription, error) {
	notifier, supported := jsonrpc.NotifierFromContext(ctx)
	if !supported {
		return &jsonrpc.Subscription{}, jsonrpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan []*block.Log)
	)

	logsSub, err := filterApi.events.SubscribeLogs(crit, matchedLogs)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case logs := <-matchedLogs:
				for _, log := range logs {
					log := log
					notifier.Notify(rpcSub.ID, &log)
				}
			case <-rpcSub.Err(): // client send an unsubscribe request
				logsSub.Unsubscribe()
				return
			case <-notifier.Closed(): // connection dropped
				logsSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewFilter creates a new filter and returns the filter id. It can be
// used to retrieve logs when the state changes. This method cannot be
// used to fetch logs that are already stored in the state.
//
// Default criteria for the from and to block are "latest".
// Using "latest" as block number will return logs for mined blocks.
// Using "pending" as block number returns logs for not yet mined (pending) blocks.
// In case logs are removed (chain reorg) previously returned logs are returned
// again but with the removed property set to true.
//
// In case "fromBlock" > "toBlock" an error is returned.
func (filterApi *FilterAPI) NewFilter(crit FilterCriteria) (jsonrpc.ID, error) {
	logs := make(chan []*block.Log)
	logsSub, err := filterApi.events.SubscribeLogs(crit, logs)
	if err != nil {
		return "", err
	}

	filterApi.filtersMu.Lock()
	filterApi.filters[logsSub.ID] = &filter{typ: LogsSubscription, crit: crit, deadline: time.NewTimer(filterApi.timeout), logs: make([]*block.Log, 0), s: logsSub}
	filterApi.filtersMu.Unlock()

	go func() {
		for {
			select {
			case l := <-logs:
				filterApi.filtersMu.Lock()
				if f, found := filterApi.filters[logsSub.ID]; found {
					f.logs = append(f.logs, l...)
				}
				filterApi.filtersMu.Unlock()
			case <-logsSub.Err():
				filterApi.filtersMu.Lock()
				delete(filterApi.filters, logsSub.ID)
				filterApi.filtersMu.Unlock()
				return
			}
		}
	}()

	return logsSub.ID, nil
}

// GetLogs returns logs matching the given argument that are stored within the state.
func (filterApi *FilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*block.Log, error) {
	var filter *Filter
	if crit.BlockHash != (types.Hash{}) {
		// Block filter requested, construct a single-shot filter
		filter = NewBlockFilter(filterApi.api, crit.BlockHash, crit.Addresses, crit.Topics)
	} else {
		// Convert the RPC block numbers into internal representations
		begin := jsonrpc.LatestBlockNumber.Int64()
		if crit.FromBlock != nil {
			begin = crit.FromBlock.Int64()
		}
		end := jsonrpc.LatestBlockNumber.Int64()
		if crit.ToBlock != nil {
			end = crit.ToBlock.Int64()
		}
		// Construct the range filter
		filter = NewRangeFilter(filterApi.api, begin, end, crit.Addresses, crit.Topics)
	}
	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx)
	if err != nil {
		return nil, err
	}
	return returnLogs(logs), err
}

// UninstallFilter removes the filter with the given filter id.
func (filterApi *FilterAPI) UninstallFilter(id jsonrpc.ID) bool {
	filterApi.filtersMu.Lock()
	f, found := filterApi.filters[id]
	if found {
		delete(filterApi.filters, id)
	}
	filterApi.filtersMu.Unlock()
	if found {
		f.s.Unsubscribe()
	}

	return found
}

// GetFilterLogs returns the logs for the filter with the given id.
// If the filter could not be found an empty array of logs is returned.
func (filterApi *FilterAPI) GetFilterLogs(ctx context.Context, id jsonrpc.ID) ([]*block.Log, error) {
	filterApi.filtersMu.Lock()
	f, found := filterApi.filters[id]
	filterApi.filtersMu.Unlock()

	if !found || f.typ != LogsSubscription {
		return nil, fmt.Errorf("filter not found")
	}

	var filter *Filter
	if f.crit.BlockHash != (types.Hash{}) {
		// Block filter requested, construct a single-shot filter
		filter = NewBlockFilter(filterApi.api, f.crit.BlockHash, f.crit.Addresses, f.crit.Topics)
	} else {
		// Convert the RPC block numbers into internal representations
		begin := jsonrpc.LatestBlockNumber.Int64()
		if f.crit.FromBlock != nil {
			begin = f.crit.FromBlock.Int64()
		}
		end := jsonrpc.LatestBlockNumber.Int64()
		if f.crit.ToBlock != nil {
			end = f.crit.ToBlock.Int64()
		}
		// Construct the range filter
		filter = NewRangeFilter(filterApi.api, begin, end, f.crit.Addresses, f.crit.Topics)
	}
	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx)
	if err != nil {
		return nil, err
	}
	return returnLogs(logs), nil
}

// GetFilterChanges returns the logs for the filter with the given id since
// last time it was called. This can be used for polling.
//
// For pending transaction and block filters the result is []types.Hash.
// (pending)Log filters return []Log.
func (filterApi *FilterAPI) GetFilterChanges(id jsonrpc.ID) (interface{}, error) {
	filterApi.filtersMu.Lock()
	defer filterApi.filtersMu.Unlock()

	if f, found := filterApi.filters[id]; found {
		if !f.deadline.Stop() {
			// timer expired but filter is not yet removed in timeout loop
			// receive timer value and reset timer
			<-f.deadline.C
		}
		f.deadline.Reset(filterApi.timeout)

		switch f.typ {
		case PendingTransactionsSubscription, BlocksSubscription:
			hashes := f.hashes
			f.hashes = nil
			return returnHashes(hashes), nil
		case LogsSubscription, MinedAndPendingLogsSubscription:
			logs := f.logs
			f.logs = nil
			return returnLogs(logs), nil
		}
	}

	return []interface{}{}, fmt.Errorf("filter not found")
}

// returnHashes is a helper that will return an empty hash array case the given hash array is nil,
// otherwise the given hashes array is returned.
func returnHashes(hashes []types.Hash) []types.Hash {
	if hashes == nil {
		return []types.Hash{}
	}
	return hashes
}

// returnLogs is a helper that will return an empty log array in case the given logs array is nil,
// otherwise the given logs array is returned.
func returnLogs(logs []*block.Log) []*block.Log {
	if logs == nil {
		return []*block.Log{}
	}
	return logs
}
