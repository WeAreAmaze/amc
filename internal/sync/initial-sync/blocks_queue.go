package initialsync

import (
	"context"
	"errors"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/internal/p2p"
	amcsync "github.com/amazechain/amc/internal/sync"
	"github.com/holiman/uint256"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// queueStopCallTimeout is time allowed for queue to release resources when quitting.
	queueStopCallTimeout = 1 * time.Second
	// pollingInterval defines how often state machine needs to check for new events.
	pollingInterval = 200 * time.Millisecond
	// staleEpochTimeout is an period after which epoch's state is considered stale.
	staleEpochTimeout = 1 * time.Second
	// skippedMachineTimeout is a period after which skipped machine is considered as stuck
	// and is reset (if machine is the last one, then all machines are reset and search for
	// skipped slot or backtracking takes place).
	skippedMachineTimeout = 10 * staleEpochTimeout
	// lookaheadSteps is a limit on how many forward steps are loaded into queue.
	// Each step is managed by assigned finite state machine. Must be >= 2.
	lookaheadSteps = 8
	// noRequiredPeersErrMaxRetries defines number of retries when no required peers are found.
	noRequiredPeersErrMaxRetries = 1000
	// noRequiredPeersErrRefreshInterval defines interval for which queue will be paused before
	// making the next attempt to obtain data.
	noRequiredPeersErrRefreshInterval = 15 * time.Second
	// maxResetAttempts number of times stale FSM is reset, before backtracking is triggered.
	maxResetAttempts = 4
	// startBackSlots defines number of slots before the current head, which defines a start position
	// of the initial machine. This allows more robustness in case of normal sync sets head to some
	// orphaned block: in that case starting earlier and re-fetching blocks allows to reorganize chain.
	//startBackSlots = 32
)

var (
	errQueueCtxIsDone             = errors.New("queue's context is done, reinitialize")
	errQueueTakesTooLongToStop    = errors.New("queue takes too long to stop")
	errInvalidInitialState        = errors.New("invalid initial state")
	errInputNotFetchRequestParams = errors.New("input data is not type *fetchRequestParams")
	errNoRequiredPeers            = errors.New("no peers with required blocks are found")
)

const (
	modeStopOnFinalizedEpoch syncMode = iota
	modeNonConstrained
)

// syncMode specifies sync mod type.
type syncMode uint8

// blocksQueueConfig is a config to setup block queue service.
type blocksQueueConfig struct {
	blocksFetcher          *blocksFetcher
	chain                  common.IBlockChain
	highestExpectedBlockNr *uint256.Int
	p2p                    p2p.P2P
	mode                   syncMode
}

// blocksQueue is a priority queue that serves as a intermediary between block fetchers (producers)
// and block processing goroutine (consumer). Consumer can rely on order of incoming blocks.
type blocksQueue struct {
	ctx                    context.Context
	cancel                 context.CancelFunc
	smm                    *stateMachineManager
	blocksFetcher          *blocksFetcher
	chain                  common.IBlockChain
	highestExpectedBlockNr *uint256.Int
	mode                   syncMode
	exitConditions         struct {
		noRequiredPeersErrRetries int
	}
	fetchedData chan *blocksQueueFetchedData // output channel for ready blocks
	quit        chan struct{}                // termination notifier
}

// blocksQueueFetchedData is a data container that is returned from a queue on each step.
type blocksQueueFetchedData struct {
	pid    peer.ID
	blocks []*types_pb.Block
}

// newBlocksQueue creates initialized priority queue.
func newBlocksQueue(ctx context.Context, cfg *blocksQueueConfig) *blocksQueue {
	ctx, cancel := context.WithCancel(ctx)

	blocksFetcher := cfg.blocksFetcher
	if blocksFetcher == nil {
		blocksFetcher = newBlocksFetcher(ctx, &blocksFetcherConfig{
			chain: cfg.chain,
			p2p:   cfg.p2p,
		})
	}
	highestExpectedBlockNr := cfg.highestExpectedBlockNr

	// Override fetcher's sync mode.
	blocksFetcher.mode = cfg.mode

	queue := &blocksQueue{
		ctx:                    ctx,
		cancel:                 cancel,
		highestExpectedBlockNr: highestExpectedBlockNr,
		blocksFetcher:          blocksFetcher,
		chain:                  cfg.chain,
		mode:                   cfg.mode,
		fetchedData:            make(chan *blocksQueueFetchedData, 1),
		quit:                   make(chan struct{}),
	}

	// Configure state machines.
	queue.smm = newStateMachineManager()
	queue.smm.addEventHandler(eventTick, stateNew, queue.onScheduleEvent(ctx))
	queue.smm.addEventHandler(eventDataReceived, stateScheduled, queue.onDataReceivedEvent(ctx))
	queue.smm.addEventHandler(eventTick, stateDataParsed, queue.onReadyToSendEvent(ctx))
	queue.smm.addEventHandler(eventTick, stateSkipped, queue.onProcessSkippedEvent(ctx))
	queue.smm.addEventHandler(eventTick, stateSent, queue.onCheckStaleEvent(ctx))

	return queue
}

// start boots up the queue processing.
func (q *blocksQueue) start() error {
	select {
	case <-q.ctx.Done():
		return errQueueCtxIsDone
	default:
		go q.loop()
		return nil
	}
}

// stop terminates all queue operations.
func (q *blocksQueue) stop() error {
	q.cancel()
	select {
	case <-q.quit:
		return nil
	case <-time.After(queueStopCallTimeout):
		return errQueueTakesTooLongToStop
	}
}

// loop is a main queue loop.
func (q *blocksQueue) loop() {
	defer close(q.quit)

	defer func() {
		q.blocksFetcher.stop()
		close(q.fetchedData)
	}()

	if err := q.blocksFetcher.start(); err != nil {
		log.Debug("Can not start blocks provider", "err", err)
	}

	// Define initial state machines.
	var startBlockNr *uint256.Int
	if q.chain.CurrentBlock().Number64().Cmp(uint256.NewInt(0)) == 0 {
		startBlockNr = new(uint256.Int).AddUint64(q.chain.CurrentBlock().Number64(), 1)
	} else {
		startBlockNr = q.chain.CurrentBlock().Number64()
	}
	blocksPerRequest := q.blocksFetcher.blocksPerPeriod
	for i := startBlockNr.Clone(); i.Cmp(new(uint256.Int).AddUint64(startBlockNr, blocksPerRequest*lookaheadSteps)) == -1; i = i.AddUint64(i, blocksPerRequest) {
		q.smm.addStateMachine(i)
	}

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()
	for {
		if waitHighestExpectedBlockNr(q) {
			continue
		}

		log.Debug("tick",
			"highestExpectedBlockNr", q.highestExpectedBlockNr,
			"ourBlockNr", q.chain.CurrentBlock().Number64(),
			"state", q.smm.String(),
		)

		select {
		case <-ticker.C:
			for _, key := range q.smm.keys {
				fsm := q.smm.machines[key.Uint64()]
				if err := fsm.trigger(eventTick, nil); err != nil {
					log.Debug("Can not trigger event",
						"highestExpectedBlockNr", q.highestExpectedBlockNr,
						"noRequiredPeersErrRetries", q.exitConditions.noRequiredPeersErrRetries,
						"event", eventTick,
						"start", fsm.start,
						"error", err.Error(),
					)
					if errors.Is(err, errNoRequiredPeers) {
						forceExit := q.exitConditions.noRequiredPeersErrRetries > noRequiredPeersErrMaxRetries
						if forceExit {
							q.cancel()
						} else {
							q.exitConditions.noRequiredPeersErrRetries++
							log.Debug("Waiting for finalized peers")
							time.Sleep(noRequiredPeersErrRefreshInterval)
						}
						continue
					}
				}
				if fsm.start.Cmp(q.highestExpectedBlockNr) == 1 {
					if err := q.smm.removeStateMachine(fsm.start); err != nil {
						log.Debug("Can not remove state machine", "err", err)
					}
					continue
				}
				// Do garbage collection, and advance sliding window forward.
				if q.chain.CurrentBlock().Number64().Cmp(new(uint256.Int).AddUint64(fsm.start, blocksPerRequest-1)) >= 0 {
					highestStartSlot, err := q.smm.highestStartSlot()
					if err != nil {
						log.Debug("Cannot obtain highest epoch state number", "err", err)
						continue
					}
					if err := q.smm.removeStateMachine(fsm.start); err != nil {
						log.Debug("Can not remove state machine", "err", err)
					}
					if len(q.smm.machines) < lookaheadSteps {
						q.smm.addStateMachine(new(uint256.Int).AddUint64(highestStartSlot, blocksPerRequest))
					}
				}
			}
		case response, ok := <-q.blocksFetcher.requestResponses():
			if !ok {
				log.Debug("Fetcher closed output channel")
				q.cancel()
				return
			}
			// Update state of an epoch for which data is received.
			if fsm, ok := q.smm.findStateMachine(response.start); ok {
				if err := fsm.trigger(eventDataReceived, response); err != nil {
					log.Debug("Can not process event",
						"event", eventDataReceived,
						"start", fsm.start,
						"error", err.Error(),
					)
					fsm.setState(stateNew)
					continue
				}
			}
		case <-q.ctx.Done():
			log.Debug("Context closed, exiting goroutine (blocks queue)")
			return
		}
	}
}

func waitHighestExpectedBlockNr(q *blocksQueue) bool {
	// Check highest expected blockNr when we approach chain's head slot.
	if q.chain.CurrentBlock().Number64().Cmp(q.highestExpectedBlockNr) >= 0 {
		// By the time initial sync is complete, highest slot may increase, re-check.
		targetBlockNr := q.blocksFetcher.bestFinalizedBlockNr()
		if q.highestExpectedBlockNr.Cmp(targetBlockNr) == -1 {
			q.highestExpectedBlockNr = targetBlockNr
			return true
		}
		log.Debug("Highest expected blockNr reached", "blockNr", targetBlockNr)
		q.cancel()
	}
	return false
}

// onScheduleEvent is an event called on newly arrived epochs. Transforms state to scheduled.
func (q *blocksQueue) onScheduleEvent(ctx context.Context) eventHandlerFn {
	return func(m *stateMachine, in interface{}) (stateID, error) {
		if m.state != stateNew {
			return m.state, errInvalidInitialState
		}
		if m.start.Cmp(q.highestExpectedBlockNr) == 1 {
			m.setState(stateSkipped)
			return m.state, errBlockNrIsTooHigh
		}
		blocksPerRequest := q.blocksFetcher.blocksPerPeriod
		if q.highestExpectedBlockNr.Cmp(new(uint256.Int).AddUint64(m.start, blocksPerRequest)) < 0 {
			blocksPerRequest = new(uint256.Int).Sub(q.highestExpectedBlockNr, m.start).Uint64() + 1
		}
		if err := q.blocksFetcher.scheduleRequest(ctx, m.start, blocksPerRequest); err != nil {
			return m.state, err
		}
		return stateScheduled, nil
	}
}

// onDataReceivedEvent is an event called when data is received from fetcher.
func (q *blocksQueue) onDataReceivedEvent(ctx context.Context) eventHandlerFn {
	return func(m *stateMachine, in interface{}) (stateID, error) {
		if ctx.Err() != nil {
			return m.state, ctx.Err()
		}
		if m.state != stateScheduled {
			return m.state, errInvalidInitialState
		}
		response, ok := in.(*fetchRequestResponse)
		if !ok {
			return m.state, errInputNotFetchRequestParams
		}
		if response.err != nil {
			switch response.err {
			//todo
			case errBlockNrIsTooHigh:
				// Current window is already too big, re-request previous epochs.
				for _, fsm := range q.smm.machines {
					if fsm.start.Cmp(response.start) == -1 && fsm.state == stateSkipped {
						fsm.setState(stateNew)
					}
				}
			case amcsync.ErrInvalidFetchedData:
				// Peer returned invalid data, penalize.
				q.blocksFetcher.p2p.Peers().Scorers().BadResponsesScorer().Increment(m.pid)
				log.Debug("Peer is penalized for invalid blocks", "pid", response.pid)
			}
			return m.state, response.err
		}
		m.pid = response.pid
		m.blocks = response.blocks
		return stateDataParsed, nil
	}
}

// onReadyToSendEvent is an event called to allow epochs with available blocks to send them downstream.
func (q *blocksQueue) onReadyToSendEvent(ctx context.Context) eventHandlerFn {
	return func(m *stateMachine, in interface{}) (stateID, error) {
		if ctx.Err() != nil {
			return m.state, ctx.Err()
		}
		if m.state != stateDataParsed {
			return m.state, errInvalidInitialState
		}

		if len(m.blocks) == 0 {
			return stateSkipped, nil
		}

		send := func() (stateID, error) {
			data := &blocksQueueFetchedData{
				pid:    m.pid,
				blocks: m.blocks,
			}
			select {
			case <-ctx.Done():
				return m.state, ctx.Err()
			case q.fetchedData <- data:
			}
			return stateSent, nil
		}

		// Make sure that we send epochs in a correct order.
		// If machine is the first (has lowest start block), send.
		if m.isFirst() {
			return send()
		}

		// Make sure that previous epoch is already processed.
		for _, fsm := range q.smm.machines {
			// Review only previous slots.
			if fsm.start.Cmp(m.start) == -1 {
				switch fsm.state {
				case stateNew, stateScheduled, stateDataParsed:
					return m.state, nil
				}
			}
		}

		return send()
	}
}

// onProcessSkippedEvent is an event triggered on skipped machines, allowing handlers to
// extend lookahead window, in case where progress is not possible otherwise.
func (q *blocksQueue) onProcessSkippedEvent(ctx context.Context) eventHandlerFn {
	return func(m *stateMachine, in interface{}) (stateID, error) {
		if ctx.Err() != nil {
			return m.state, ctx.Err()
		}
		if m.state != stateSkipped {
			return m.state, errInvalidInitialState
		}

		// Only the highest epoch with skipped state can trigger extension.
		if !m.isLast() {
			// When a state machine stays in skipped state for too long - reset it.
			if time.Since(m.updated) > skippedMachineTimeout {
				return stateNew, nil
			}
			return m.state, nil
		}

		// Make sure that all machines are in skipped state i.e. manager cannot progress without reset or
		// moving the last machine's start block forward (in an attempt to find next non-skipped block).
		if !q.smm.allMachinesInState(stateSkipped) {
			return m.state, nil
		}

		// Check if we have enough peers to progress, or sync needs to halt (due to no peers available).
		//bestFinalizedSlot := q.blocksFetcher.bestFinalizedBlockNr()
		if q.blocksFetcher.bestFinalizedBlockNr().Cmp(q.chain.CurrentBlock().Number64()) >= 0 {
			return stateSkipped, errNoRequiredPeers
		}

		// All machines are skipped, FSMs need reset.
		startBlockNr := new(uint256.Int).AddUint64(q.chain.CurrentBlock().Number64(), 1)

		//todo q.blocksFetcher.findFork(ctx, startSlot)

		return stateSkipped, q.resetFromBlockNr(ctx, startBlockNr)
	}
}

// onCheckStaleEvent is an event that allows to mark stale epochs,
// so that they can be re-processed.
func (_ *blocksQueue) onCheckStaleEvent(ctx context.Context) eventHandlerFn {
	return func(m *stateMachine, in interface{}) (stateID, error) {
		if ctx.Err() != nil {
			return m.state, ctx.Err()
		}
		if m.state != stateSent {
			return m.state, errInvalidInitialState
		}

		// Break out immediately if bucket is not stale.
		if time.Since(m.updated) < staleEpochTimeout {
			return m.state, nil
		}

		return stateSkipped, nil
	}
}
