package filters

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"sync"
	"time"
)

// Type determines the kind of filter and is used to put the filter in to
// the correct bucket when added.
type Type byte

const (
	// UnknownSubscription indicates an unknown subscription type
	UnknownSubscription Type = iota
	// LogsSubscription queries for new or removed (chain reorg) logs
	LogsSubscription
	// PendingLogsSubscription queries for logs in pending blocks
	PendingLogsSubscription
	// MinedAndPendingLogsSubscription queries for logs in mined and pending blocks.
	MinedAndPendingLogsSubscription
	// PendingTransactionsSubscription queries tx hashes for pending
	// transactions entering the pending state
	PendingTransactionsSubscription
	// BlocksSubscription queries hashes for blocks that are imported
	BlocksSubscription
	// LastSubscription keeps track of the last index
	LastIndexSubscription
)

const (
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// rmLogsChanSize is the size of channel listening to RemovedLogsEvent.
	rmLogsChanSize = 10
	// logsChanSize is the size of channel listening to LogsEvent.
	logsChanSize = 10
	// chainEvChanSize is the size of channel listening to ChainEvent.
	chainEvChanSize = 10
)

type subscription struct {
	id        jsonrpc.ID
	typ       Type
	created   time.Time
	logsCrit  FilterCriteria
	logs      chan []*block.Log
	hashes    chan []types.Hash
	headers   chan block.IHeader
	installed chan struct{} // closed when the filter is installed
	err       chan error    // closed when the filter is uninstalled
}

// EventSystem creates subscriptions, processes events and broadcasts them to the
// subscription which match the subscription criteria.
type EventSystem struct {
	api       Api
	lightMode bool
	lastHead  block.IHeader

	// Subscriptions
	txsSub         event.Subscription // Subscription for new transaction event
	logsSub        event.Subscription // Subscription for new log event
	rmLogsSub      event.Subscription // Subscription for removed log event
	pendingLogsSub event.Subscription // Subscription for pending log event
	chainSub       event.Subscription // Subscription for new chain event

	// Channels
	install       chan *subscription              // install filter for event notification
	uninstall     chan *subscription              // remove filter for event notification
	txsCh         chan common.NewTxsEvent         // Channel to receive new transactions event
	logsCh        chan common.NewLogsEvent        // Channel to receive new log event
	pendingLogsCh chan common.NewPendingLogsEvent // Channel to receive new log event
	rmLogsCh      chan common.RemovedLogsEvent    // Channel to receive removed log event
	chainCh       chan common.ChainHighestBlock   // Channel to receive new chain event
}

// NewEventSystem creates a new manager that listens for event on the given mux,
// parses and filters them. It uses the all map to retrieve filter changes. The
// work loop holds its own index that is used to forward events to filters.
//
// The returned manager has a loop that needs to be stopped with the Stop function
// or by stopping the given mux.
func NewEventSystem(api Api) *EventSystem {

	m := &EventSystem{
		api:           api,
		lightMode:     false,
		install:       make(chan *subscription),
		uninstall:     make(chan *subscription),
		txsCh:         make(chan common.NewTxsEvent),
		logsCh:        make(chan common.NewLogsEvent),
		rmLogsCh:      make(chan common.RemovedLogsEvent),
		pendingLogsCh: make(chan common.NewPendingLogsEvent),
		chainCh:       make(chan common.ChainHighestBlock),
	}

	// Subscribe events
	m.txsSub = event.GlobalEvent.Subscribe(m.txsCh)
	m.logsSub = event.GlobalEvent.Subscribe(m.logsCh)
	m.rmLogsSub = event.GlobalEvent.Subscribe(m.rmLogsCh)
	m.chainSub = event.GlobalEvent.Subscribe(m.chainCh)
	m.pendingLogsSub = event.GlobalEvent.Subscribe(m.pendingLogsCh)

	// Make sure none of the subscriptions are empty
	if m.txsSub == nil || m.logsSub == nil || m.rmLogsSub == nil || m.chainSub == nil || m.pendingLogsSub == nil {
		log.Error("Subscribe for event system failed")
	}

	go m.eventLoop()
	return m
}

// Subscription is created when the client registers itself for a particular event.
type Subscription struct {
	ID        jsonrpc.ID
	f         *subscription
	es        *EventSystem
	unsubOnce sync.Once
}

// Err returns a channel that is closed when unsubscribed.
func (sub *Subscription) Err() <-chan error {
	return sub.f.err
}

// Unsubscribe uninstalls the subscription from the event broadcast loop.
func (sub *Subscription) Unsubscribe() {
	sub.unsubOnce.Do(func() {
	uninstallLoop:
		for {
			// write uninstall request and consume logs/hashes. This prevents
			// the eventLoop broadcast method to deadlock when writing to the
			// filter event channel while the subscription loop is waiting for
			// this method to return (and thus not reading these events).
			select {
			case sub.es.uninstall <- sub.f:
				break uninstallLoop
			case <-sub.f.logs:
			case <-sub.f.hashes:
			case <-sub.f.headers:
			}
		}

		// wait for filter to be uninstalled in work loop before returning
		// this ensures that the manager won't use the event channel which
		// will probably be closed by the client asap after this method returns.
		<-sub.Err()
	})
}

// subscribe installs the subscription in the event broadcast loop.
func (es *EventSystem) subscribe(sub *subscription) *Subscription {
	es.install <- sub
	<-sub.installed
	return &Subscription{ID: sub.id, f: sub, es: es}
}

// SubscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel. Default value for the from and to
// block is "latest". If the fromBlock > toBlock an error is returned.
func (es *EventSystem) SubscribeLogs(crit FilterCriteria, logs chan []*block.Log) (*Subscription, error) {
	var from, to jsonrpc.BlockNumber
	if crit.FromBlock == nil {
		from = jsonrpc.LatestBlockNumber
	} else {
		from = jsonrpc.BlockNumber(crit.FromBlock.Int64())
	}
	if crit.ToBlock == nil {
		to = jsonrpc.LatestBlockNumber
	} else {
		to = jsonrpc.BlockNumber(crit.ToBlock.Int64())
	}

	// only interested in pending logs
	if from == jsonrpc.PendingBlockNumber && to == jsonrpc.PendingBlockNumber {
		return es.subscribePendingLogs(crit, logs), nil
	}
	// only interested in new mined logs
	if from == jsonrpc.LatestBlockNumber && to == jsonrpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	// only interested in mined logs within a specific block range
	if from >= 0 && to >= 0 && to >= from {
		return es.subscribeLogs(crit, logs), nil
	}
	// interested in mined logs from a specific block number, new logs and pending logs
	if from >= jsonrpc.LatestBlockNumber && to == jsonrpc.PendingBlockNumber {
		return es.subscribeMinedPendingLogs(crit, logs), nil
	}
	// interested in logs from a specific block number to new mined blocks
	if from >= 0 && to == jsonrpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	return nil, fmt.Errorf("invalid from and to block combination: from > to")
}

// subscribeMinedPendingLogs creates a subscription that returned mined and
// pending logs that match the given criteria.
func (es *EventSystem) subscribeMinedPendingLogs(crit FilterCriteria, logs chan []*block.Log) *Subscription {
	sub := &subscription{
		id:        jsonrpc.NewID(),
		typ:       MinedAndPendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []types.Hash),
		headers:   make(chan block.IHeader),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// subscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel.
func (es *EventSystem) subscribeLogs(crit FilterCriteria, logs chan []*block.Log) *Subscription {
	sub := &subscription{
		id:        jsonrpc.NewID(),
		typ:       LogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []types.Hash),
		headers:   make(chan block.IHeader),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// subscribePendingLogs creates a subscription that writes contract event logs for
// transactions that enter the transaction pool.
func (es *EventSystem) subscribePendingLogs(crit FilterCriteria, logs chan []*block.Log) *Subscription {
	sub := &subscription{
		id:        jsonrpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []types.Hash),
		headers:   make(chan block.IHeader),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribeNewHeads creates a subscription that writes the header of a block that is
// imported in the chain.
func (es *EventSystem) SubscribeNewHeads(headers chan block.IHeader) *Subscription {
	sub := &subscription{
		id:        jsonrpc.NewID(),
		typ:       BlocksSubscription,
		created:   time.Now(),
		logs:      make(chan []*block.Log),
		hashes:    make(chan []types.Hash),
		headers:   headers,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribePendingTxs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) SubscribePendingTxs(hashes chan []types.Hash) *Subscription {
	sub := &subscription{
		id:        jsonrpc.NewID(),
		typ:       PendingTransactionsSubscription,
		created:   time.Now(),
		logs:      make(chan []*block.Log),
		hashes:    hashes,
		headers:   make(chan block.IHeader),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

type filterIndex map[Type]map[jsonrpc.ID]*subscription

func (es *EventSystem) handleLogs(filters filterIndex, ev common.NewLogsEvent) {
	if len(ev.Logs) == 0 {
		return
	}
	for _, f := range filters[LogsSubscription] {
		matchedLogs := filterLogs(ev.Logs, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
		if len(matchedLogs) > 0 {
			f.logs <- matchedLogs
		}
	}
}

func (es *EventSystem) handlePendingLogs(filters filterIndex, ev common.NewPendingLogsEvent) {
	if len(ev.Logs) == 0 {
		return
	}
	for _, f := range filters[PendingLogsSubscription] {
		matchedLogs := filterLogs(ev.Logs, nil, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
		if len(matchedLogs) > 0 {
			f.logs <- matchedLogs
		}
	}
}

func (es *EventSystem) handleRemovedLogs(filters filterIndex, ev common.RemovedLogsEvent) {
	for _, f := range filters[LogsSubscription] {
		matchedLogs := filterLogs(ev.Logs, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
		if len(matchedLogs) > 0 {
			f.logs <- matchedLogs
		}
	}
}

func (es *EventSystem) handleTxsEvent(filters filterIndex, ev common.NewTxsEvent) {
	hashes := make([]types.Hash, 0, len(ev.Txs))
	for _, tx := range ev.Txs {
		hash, _ := tx.Hash()
		hashes = append(hashes, hash)
	}
	for _, f := range filters[PendingTransactionsSubscription] {
		f.hashes <- hashes
	}
}

func (es *EventSystem) handleChainEvent(filters filterIndex, ev common.ChainHighestBlock) {
	for _, f := range filters[BlocksSubscription] {
		f.headers <- ev.Block.Header()
	}
	if es.lightMode && len(filters[LogsSubscription]) > 0 {
		es.lightFilterNewHead(ev.Block.Header(), func(header block.IHeader, remove bool) {
			for _, f := range filters[LogsSubscription] {
				if matchedLogs := es.lightFilterLogs(header, f.logsCrit.Addresses, f.logsCrit.Topics, remove); len(matchedLogs) > 0 {
					f.logs <- matchedLogs
				}
			}
		})
	}
}

func (es *EventSystem) lightFilterNewHead(newHeader block.IHeader, callBack func(block.IHeader, bool)) {
	oldh := es.lastHead
	es.lastHead = newHeader
	if oldh == nil {
		return
	}
	newh := newHeader
	// find common ancestor, create list of rolled back and new block hashes
	var oldHeaders, newHeaders []block.IHeader
	for oldh.Hash() != newh.Hash() {
		if oldh.Number64().Uint64() >= newh.Number64().Uint64() {
			oldHeaders = append(oldHeaders, oldh)
			pHash := oldh.(*block.Header).ParentHash
			oldh, _, _ = rawdb.GetHeader(es.api.Database(), pHash)
		}
		if oldh.Number64().Uint64() < newh.Number64().Uint64() {
			newHeaders = append(newHeaders, newh)
			pHash := newh.(*block.Header).ParentHash
			newh, _, _ = rawdb.GetHeader(es.api.Database(), pHash)
			if newh == nil {
				// happens when CHT syncing, nothing to do
				newh = oldh
			}
		}
	}
	// roll back old blocks
	for _, h := range oldHeaders {
		callBack(h, true)
	}
	// check new blocks (array is in reverse order)
	for i := len(newHeaders) - 1; i >= 0; i-- {
		callBack(newHeaders[i], false)
	}
}

// filter logs of a single header in light client mode
func (es *EventSystem) lightFilterLogs(header block.IHeader, addresses []types.Address, topics [][]types.Hash, remove bool) []*block.Log {
	//todo header.Bloom
	bloom, _ := types.NewBloom(100)
	if bloomFilter(bloom, addresses, topics) {
		// Get the logs of the block
		_, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		logsList, err := es.api.BlockChain().GetLogs(header.Hash())
		if err != nil {
			return nil
		}
		var unfiltered []*block.Log
		for _, logs := range logsList {
			for _, log := range logs {
				logcopy := *log
				logcopy.Removed = remove
				unfiltered = append(unfiltered, &logcopy)
			}
		}
		logs := filterLogs(unfiltered, nil, nil, addresses, topics)
		if len(logs) > 0 && logs[0].TxHash == (types.Hash{}) {
			// We have matching but non-derived logs
			receipts, err := es.api.BlockChain().GetReceipts(header.Hash())
			if err != nil {
				return nil
			}
			unfiltered = unfiltered[:0]
			for _, receipt := range receipts {
				for _, log := range receipt.Logs {
					logcopy := *log
					logcopy.Removed = remove
					unfiltered = append(unfiltered, &logcopy)
				}
			}
			logs = filterLogs(unfiltered, nil, nil, addresses, topics)
		}
		return logs
	}
	return nil
}

// eventLoop (un)installs filters and processes mux events.
func (es *EventSystem) eventLoop() {
	// Ensure all subscriptions get cleaned up
	defer func() {
		es.txsSub.Unsubscribe()
		es.logsSub.Unsubscribe()
		es.rmLogsSub.Unsubscribe()
		es.pendingLogsSub.Unsubscribe()
		es.chainSub.Unsubscribe()
	}()

	index := make(filterIndex)
	for i := UnknownSubscription; i < LastIndexSubscription; i++ {
		index[i] = make(map[jsonrpc.ID]*subscription)
	}

	for {
		select {
		case ev := <-es.txsCh:
			es.handleTxsEvent(index, ev)
		case ev := <-es.logsCh:
			es.handleLogs(index, ev)
		case ev := <-es.rmLogsCh:
			es.handleRemovedLogs(index, ev)
		case ev := <-es.pendingLogsCh:
			es.handlePendingLogs(index, ev)
		case ev := <-es.chainCh:
			if ev.Inserted {
				es.handleChainEvent(index, ev)
			}

		case f := <-es.install:
			if f.typ == MinedAndPendingLogsSubscription {
				// the type are logs and pending logs subscriptions
				index[LogsSubscription][f.id] = f
				index[PendingLogsSubscription][f.id] = f
			} else {
				index[f.typ][f.id] = f
			}
			close(f.installed)

		case f := <-es.uninstall:
			if f.typ == MinedAndPendingLogsSubscription {
				// the type are logs and pending logs subscriptions
				delete(index[LogsSubscription], f.id)
				delete(index[PendingLogsSubscription], f.id)
			} else {
				delete(index[f.typ], f.id)
			}
			close(f.err)

		// System stopped
		case <-es.txsSub.Err():
			return
		case <-es.logsSub.Err():
			return
		case <-es.rmLogsSub.Err():
			return
		case <-es.chainSub.Err():
			return
		}
	}
}
