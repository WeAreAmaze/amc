// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package txspool

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/internal"
	"github.com/amazechain/amc/internal/consensus/misc"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/state"

	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/prque"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
)

const (

	// txSlotSize is used to calculate how many data slots a single transaction
	// takes up based on its size. The slots are used as DoS protection, ensuring
	// that validating a new transaction remains a constant operation (in reality
	// O(maxslots), where max slots are 4 currently).
	txSlotSize = 32 * 1024
	// txMaxSize is the maximum size a single transaction can have. This field has
	// non-trivial consequences: larger transactions are significantly harder and
	// more expensive to propagate; larger transactions also take more resources
	// to validate whether they fit into the pool or not.
	txMaxSize = 4 * txSlotSize // 128KB
)

var (
	ErrAlreadyKnown       = fmt.Errorf("already known")
	ErrInvalidSender      = fmt.Errorf("invalid sender")
	ErrOversizedData      = fmt.Errorf("oversized data")
	ErrNegativeValue      = fmt.Errorf("negative value")
	ErrGasLimit           = fmt.Errorf("exceeds block gas limit")
	ErrUnderpriced        = fmt.Errorf("transaction underpriced")
	ErrTxPoolOverflow     = fmt.Errorf("txpool is full")
	ErrReplaceUnderpriced = fmt.Errorf("replacement transaction underpriced")

	ErrFeeCapVeryHigh = fmt.Errorf("max fee per gas higher than 2^256-1")

	ErrNonceTooLow  = fmt.Errorf("nonce too low")
	ErrNonceTooHigh = fmt.Errorf("nonce too high")

	ErrInsufficientFunds = fmt.Errorf("insufficient funds for gas * price + value")

	// ErrTipAboveFeeCap is a sanity error to ensure no one is able to specify a
	// transaction with a tip higher than the total fee cap.
	ErrTipAboveFeeCap = fmt.Errorf("max priority fee per gas higher than max fee per gas")
)

type txspoolResetRequest struct {
	oldBlock, newBlock block.IBlock
}

type TxsPoolConfig struct {
	Locals   []types.Address
	NoLocals bool

	PriceLimit uint64
	PriceBump  uint64

	AccountSlots uint64
	GlobalSlots  uint64
	AccountQueue uint64
	GlobalQueue  uint64

	Lifetime time.Duration
}

// DefaultTxPoolConfig default blockchain
var DefaultTxPoolConfig = TxsPoolConfig{

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 16,
	GlobalSlots:  4096 + 1024,
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,
}

type TxsPool struct {
	config      TxsPoolConfig
	chainconfig *params.ChainConfig

	bc common.IBlockChain

	currentState  *state.IntraBlockState
	pendingNonces *txNoncer
	currentMaxGas uint64

	ctx    context.Context //
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex // lock

	istanbul bool // Fork indicator whether we are in the istanbul stage.
	eip2718  bool // Fork indicator whether we are using EIP-2718 type transactions.
	eip1559  bool // Fork indicator whether we are using EIP-1559 type transactions.
	shanghai bool // Fork indicator whether we are in the Shanghai stage.

	locals   *accountSet
	pending  map[types.Address]*txsList
	queue    map[types.Address]*txsList
	beats    map[types.Address]time.Time
	all      *txLookup
	priced   *txPricedList
	gasPrice *uint256.Int

	// channel
	reqResetCh      chan *txspoolResetRequest
	reqPromoteCh    chan *accountSet
	queueTxEventCh  chan *transaction.Transaction
	reorgDoneCh     chan chan struct{}
	reorgShutdownCh chan struct{}

	changesSinceReorg int

	isRun uint32
}

func NewTxsPool(ctx context.Context, bc common.IBlockChain) (txs_pool.ITxsPool, error) {

	c, cancel := context.WithCancel(ctx)
	// for test
	//log.Init(nil)
	pool := &TxsPool{
		chainconfig: bc.Config(),
		config:      DefaultTxPoolConfig,
		ctx:         c,
		cancel:      cancel,

		bc: bc,
		// todo
		//currentMaxGas: bc.CurrentBlock().GasLimit(),
		//
		locals: newAccountSet(),

		pending: make(map[types.Address]*txsList),
		queue:   make(map[types.Address]*txsList),
		beats:   make(map[types.Address]time.Time),
		all:     newTxLookup(),

		//chainHeadCh:     make(chan ChainHeadEvent, chainHeadChanSize),
		reqResetCh:      make(chan *txspoolResetRequest),
		reqPromoteCh:    make(chan *accountSet),
		queueTxEventCh:  make(chan *transaction.Transaction),
		reorgDoneCh:     make(chan chan struct{}),
		reorgShutdownCh: make(chan struct{}),
		gasPrice:        uint256.NewInt(DefaultTxPoolConfig.PriceLimit),
	}

	//
	pool.pendingNonces = newTxNoncer(pool.currentState)

	pool.priced = newTxPricedList(pool.all)
	pool.reset(nil, bc.CurrentBlock())

	pool.wg.Add(1)
	go pool.scheduleLoop()

	pool.wg.Add(1)
	go pool.blockChangeLoop()

	//todo for test
	//pool.wg.Add(1)
	//go pool.ethFetchTxPoolLoop()

	//pool.wg.Add(1)
	//go pool.ethImportTxPoolLoop()

	pool.wg.Add(1)
	//go pool.ethTxPoolCheckLoop()

	return pool, nil
}

// promoteTx adds a transaction to the pending (processable) list of transactions
// and returns whether it was inserted or an older was better.
//
// Note, this method assumes the pool lock is held!
func (pool *TxsPool) promoteTx(addr types.Address, hash types.Hash, tx *transaction.Transaction) bool {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxsList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		pool.all.Remove(hash)
		pool.priced.Removed(1)
		return false
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		hash := old.Hash()
		pool.all.Remove(hash)
		pool.priced.Removed(1)
	} else {
		// Nothing was replaced, bump the pending counter
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.pendingNonces.set(addr, tx.Nonce()+1)

	// Successful promotion, bump the heartbeat
	pool.beats[addr] = time.Now()
	return true
}

// AddLocals
func (pool *TxsPool) AddLocals(txs []*transaction.Transaction) []error {
	return pool.addTxs(txs, !pool.config.NoLocals, true)
}

// AddLocal
func (pool *TxsPool) AddLocal(tx *transaction.Transaction) error {
	errs := pool.AddLocals([]*transaction.Transaction{tx})
	return errs[0]
}

// AddRemotes
func (pool *TxsPool) AddRemotes(txs []*transaction.Transaction) []error {
	return pool.addTxs(txs, false, false)
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxsPool) addTxs(txs []*transaction.Transaction, local, sync bool) []error {
	// Filter out known ones without obtaining the pool lock or recovering signatures
	var (
		errs = make([]error, len(txs))
		news = make([]*transaction.Transaction, 0, len(txs))
	)
	for i, tx := range txs {
		// If the transaction is known, pre-set the error slot
		hash := tx.Hash()
		if pool.all.Get(hash) != nil {
			errs[i] = ErrAlreadyKnown
			//knownTxMeter.Mark(1)
			continue
		}
		if pool.validateSender(tx) == false {
			errs[i] = ErrInvalidSender
			continue
		}
		// Accumulate all unknown transactions for deeper processing
		news = append(news, tx)
	}
	if len(news) == 0 {
		return errs
	}

	// Process all the new transaction and merge any errors into the original slice
	pool.mu.Lock()
	newErrs, dirtyAddrs := pool.addTxsLocked(news, local)
	pool.mu.Unlock()

	var nilSlot = 0
	for _, err := range newErrs {
		for errs[nilSlot] != nil {
			nilSlot++
		}
		errs[nilSlot] = err
		nilSlot++
	}
	if local {
		var localTxs = make([]*transaction.Transaction, 0, len(txs))
		for i, err := range errs {
			if err == nil {
				localTxs = append(localTxs, txs[i])
			}
		}
		//log.Infof("event new local txs : %v", localTxs)
		event.GlobalEvent.Send(&common.NewLocalTxsEvent{Txs: localTxs})
	}
	// Reorg the pool internals if needed and return
	done := pool.requestPromoteExecutables(dirtyAddrs)
	if sync {
		<-done
	}
	return errs
}

// addTxsLocked attempts to queue a batch of transactions if they are valid.
// The transaction pool lock must be held.
func (pool *TxsPool) addTxsLocked(txs []*transaction.Transaction, local bool) ([]error, *accountSet) {
	dirty := newAccountSet()
	errs := make([]error, len(txs))
	for i, tx := range txs {
		replaced, err := pool.add(tx, local)
		errs[i] = err
		if err == nil && !replaced {
			dirty.addTx(tx)
		}
	}
	//validTxMeter.Mark(int64(len(dirty.accounts)))
	return errs, dirty
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (pool *TxsPool) removeTx(hash types.Hash, outofbound bool) {
	// Fetch the transaction we wish to delete
	tx := pool.all.Get(hash)
	if tx == nil {
		return
	}

	addr := *tx.From() // already verify

	// Remove it from the list of known transactions
	pool.all.Remove(hash)
	if outofbound {
		pool.priced.Removed(1)
	}
	if pool.locals.contains(addr) {
		// todo
		//localGauge.Dec(1)
	}
	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// If no more pending transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
			}
			// Postpone any invalidated transactions
			for _, tx := range invalids {
				// Internal shuffle shouldn't touch the lookup set.
				hash := tx.Hash()
				pool.enqueueTx(hash, tx, false, false)
			}
			// Update the account nonce if needed
			pool.pendingNonces.setIfLower(addr, tx.Nonce())
			// Reduce the pending counter
			//pendingGauge.Dec(int64(1 + len(invalids)))
			return
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		if removed, _ := future.Remove(tx); removed {
			// Reduce the queued counter
			//queuedGauge.Dec(1)
		}
		if future.Empty() {
			delete(pool.queue, addr)
			delete(pool.beats, addr)
		}
	}
}

// add validates a transaction and inserts it into the non-executable queue for later
// pending promotion and execution. If the transaction is a replacement for an already
// pending or queued one, it overwrites the previous transaction if its price is higher.
//
// If a newly added transaction is marked as local, its sending account will be
// be added to the allowlist, preventing any associated transaction from being dropped
// out of the pool due to pricing constraints.
func (pool *TxsPool) add(tx *transaction.Transaction, local bool) (replaced bool, err error) {
	// if exist
	hash := tx.Hash()
	gasPrice := tx.GasPrice()

	if pool.all.Get(hash) != nil {
		log.Debug("Discarding already known transaction", "hash", hash)
		return false, ErrAlreadyKnown
	}

	// Make the local flag. If it's from local source or it's from the network but
	// the sender is marked as local previously, treat it as the local transaction.
	isLocal := local || pool.locals.containsTx(tx)

	// If the transaction fails basic validation, discard it
	if err := pool.validateTx(tx, isLocal); err != nil {
		//log.Debug("Discarding invalid transaction", "hash", hash, "err", err)
		return false, err
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(pool.all.Slots()+numSlots(tx)) > pool.config.GlobalSlots+pool.config.GlobalQueue {
		// If the new transaction is underpriced, don't accept it
		if !isLocal && pool.priced.Underpriced(tx) {
			log.Debug("Discarding underpriced transaction", "hash", hash, "gasTipCap", gasPrice, "gasFeeCap", gasPrice)
			return false, ErrUnderpriced
		}
		// We're about to replace a transaction. The reorg does a more thorough
		// analysis of what to remove and how, but it runs async. We don't want to
		// do too many replacements between reorg-runs, so we cap the number of
		// replacements to 25% of the slots
		if pool.changesSinceReorg > int(pool.config.GlobalSlots/4) {
			return false, ErrTxPoolOverflow
		}

		// New transaction is better than our worse ones, make room for it.
		// If it's a local transaction, forcibly discard all available transactions.
		// Otherwise if we can't make enough room for new one, abort the operation.
		drop, success := pool.priced.Discard(pool.all.Slots()-int(pool.config.GlobalSlots+pool.config.GlobalQueue)+numSlots(tx), isLocal)

		// Special case, we still can't make the room for the new remote one.
		if !isLocal && !success {
			log.Debug("Discarding overflown transaction", "hash", hash)
			return false, ErrTxPoolOverflow
		}
		// Bump the counter of rejections-since-reorg
		pool.changesSinceReorg += len(drop)
		// Kick out the underpriced remote transactions.
		for _, tx := range drop {
			log.Debug("Discarding freshly underpriced transaction", "hash", hash, "gasTipCap", gasPrice, "gasFeeCap", gasPrice)
			hash := tx.Hash()
			pool.removeTx(hash, false)
		}
	}
	// Try to replace an existing transaction in the pending pool
	from := *tx.From() //
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		// Nonce already pending, check if required price bump is met
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			return false, ErrReplaceUnderpriced
		}
		// New transaction is better, replace old one
		if old != nil {
			hash := old.Hash()
			pool.all.Remove(hash)
			pool.priced.Removed(1)
		}
		pool.all.Add(tx, isLocal)
		pool.priced.Put(tx, isLocal)
		pool.queueTxEvent(tx)
		log.Debug("Pooled new executable transaction", "hash", hash, "from", from, "to", tx.To)

		// Successful promotion, bump the heartbeat
		pool.beats[from] = time.Now()
		return old != nil, nil
	}
	// New transaction isn't replacing a pending one, push into queue
	replaced, err = pool.enqueueTx(hash, tx, isLocal, true)
	if err != nil {
		return false, err
	}
	// Mark local addresses and journal local transactions
	if local && !pool.locals.contains(from) {
		log.Infof("Setting new local account address %v", from)
		pool.locals.add(from)
		pool.priced.Removed(pool.all.RemoteToLocals(pool.locals)) // Migrate the remotes if it's marked as local first time.
	}

	//log.Debug("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To)
	return replaced, nil
}

// enqueueTx inserts a new transaction into the non-executable transaction queue.
//
// Note, this method assumes the pool lock is held!
func (pool *TxsPool) enqueueTx(hash types.Hash, tx *transaction.Transaction, local bool, addAll bool) (bool, error) {
	// Try to insert the transaction into the future queue
	from := *tx.From() // already validated
	if pool.queue[from] == nil {
		pool.queue[from] = newTxsList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		return false, ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		hash := old.Hash()
		pool.all.Remove(hash)
		pool.priced.Removed(1)
	}

	// If the transaction isn't in lookup set but it's expected to be there,
	// show the error log.
	if pool.all.Get(hash) == nil && !addAll {
		log.Error("Missing transaction in lookup set, please report the issue", "hash", hash)
	}
	if addAll {
		pool.all.Add(tx, local)
		pool.priced.Put(tx, local)
	}
	// If we never record the heartbeat, do it right now.
	if _, exist := pool.beats[from]; !exist {
		pool.beats[from] = time.Now()
	}
	return old != nil, nil
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxsPool) validateTx(tx *transaction.Transaction, local bool) error {
	// Accept only legacy transactions until EIP-2718/2930 activates.
	// todo
	if !pool.eip2718 && tx.Type() != transaction.LegacyTxType {
		return internal.ErrTxTypeNotSupported
	}
	// Reject dynamic fee transactions until EIP-1559 activates.
	if !pool.eip1559 && tx.Type() == transaction.DynamicFeeTxType {
		return internal.ErrTxTypeNotSupported
	}
	// Reject transactions over defined size to prevent DOS attacks
	//if tx.Size() > txMaxSize {
	//	return ErrOversizedData
	//}

	gasPrice := tx.GasPrice()
	addr := *tx.From()

	if uint64(unsafe.Sizeof(tx)) > txMaxSize {
		return ErrOversizedData
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}

	// Ensure the transaction doesn't exceed the current block limit gas.
	if pool.currentMaxGas < tx.Gas() {
		return ErrGasLimit
	}

	// Sanity check for extremely large numbers
	if gasPrice.BitLen() > 256 {
		return ErrFeeCapVeryHigh
	}
	if tx.GasTipCap().BitLen() > 256 {
		return internal.ErrTipVeryHigh
	}
	// Ensure gasFeeCap is greater than or equal to gasTipCap.
	if tx.GasFeeCapIntCmp(tx.GasTipCap()) < 0 {
		return ErrTipAboveFeeCap
	}
	// Make sure the transaction is signed properly.

	// Drop non-local transactions under our own minimal accepted gas price or tip
	if !local && gasPrice.Cmp(pool.gasPrice) < 0 {
		return ErrUnderpriced
	}
	// Ensure the transaction adheres to nonce ordering
	if pool.currentState.GetNonce(addr) > tx.Nonce() {
		return ErrNonceTooLow
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	if pool.currentState.GetBalance(addr).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}

	// Ensure the transaction has more gas than the basic tx fee.
	intrGas, err := internal.IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil, true, pool.istanbul, pool.shanghai)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return internal.ErrIntrinsicGas
	}
	return nil
}

// validateSender verify todo
func (pool *TxsPool) validateSender(tx *transaction.Transaction) bool {

	return true
}

// requestReset requests a pool reset to the new head block.
// The returned channel is closed when the reset has occurred.
func (pool *TxsPool) requestReset(oldBlock block.IBlock, newBlock block.IBlock) <-chan struct{} {
	select {
	case pool.reqResetCh <- &txspoolResetRequest{oldBlock, newBlock}:
		return <-pool.reorgDoneCh
	//case <-pool.reorgShutdownCh:
	//	return pool.reorgShutdownCh
	case <-pool.ctx.Done():
		return pool.ctx.Done()
	}
}

// requestPromoteExecutables requests transaction promotion checks for the given addresses.
// The returned channel is closed when the promotion checks have occurred.
func (pool *TxsPool) requestPromoteExecutables(set *accountSet) <-chan struct{} {
	select {
	case pool.reqPromoteCh <- set:
		return <-pool.reorgDoneCh
	case <-pool.ctx.Done():
		return pool.ctx.Done()
	}
}

// queueTxEvent enqueues a transaction event to be sent in the next reorg run.
func (pool *TxsPool) queueTxEvent(tx *transaction.Transaction) {
	select {
	case pool.queueTxEventCh <- tx:
	//case <-pool.reorgShutdownCh:
	case <-pool.ctx.Done():
	}
}

func (pool *TxsPool) reset(oldBlock, newBlock block.IBlock) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject []*transaction.Transaction

	if oldBlock != nil && oldBlock.Header().Hash().String() != newBlock.ParentHash().String() {
		// If the reorg is too deep, avoid doing it (will happen during fast sync)
		oldNum := oldBlock.Number64()
		newNum := newBlock.Number64()

		if depth := uint64(math.Abs(float64(oldNum.Uint64()) - float64(newNum.Uint64()))); depth > 64 {
			log.Debug("Skipping deep transaction reorg", "depth", depth)
		} else {
			// Reorg seems shallow enough to pull in all transactions into memorynewHash
			var discarded, included []*transaction.Transaction
			var (
				rem = oldBlock
				add = newBlock
			)
			if rem == nil {
				// This can happen if a setHead is performed, where we simply discard the old
				// head from the chain.
				// If that is the case, we don't have the lost transactions any more, and
				// there's nothing to add
				if newNum.Cmp(oldNum) >= 0 {
					// If we reorged to a same or higher number, then it's not a case of setHead
					log.Warn("Transaction pool reset with missing oldhead",
						"old", oldBlock.Header().Hash(), "oldnum", oldNum, "new", newBlock.Hash(), "newnum", newNum)
					return
				}
				// If the reorg ended up on a lower number, it's indicative of setHead being the cause
				log.Debug("Skipping transaction reset caused by setHead",
					"old", oldBlock.Header().Hash(), "oldnum", oldNum, "new", newBlock.Hash(), "newnum", newNum)
				// We still need to update the current state s.th. the lost transactions can be readded by the user
			} else {
				for rem.Number64().Cmp(add.Number64()) == 1 {
					discarded = append(discarded, rem.Body().Transactions()...)
					if rem, _ = pool.bc.GetBlockByHash(rem.ParentHash()); rem == nil {
						log.Error("Unrooted old chain seen by tx pool", "block", oldBlock.Number64(), "hash", oldBlock.Hash())
						return
					}
				}
				for add.Number64().Cmp(rem.Number64()) == 1 {
					included = append(included, add.Body().Transactions()...)
					if add, _ = pool.bc.GetBlockByHash(add.ParentHash()); add == nil {
						log.Error("Unrooted new chain seen by tx pool", "block", newBlock.Number64(), "hash", newBlock.Hash())
						return
					}
				}
				for rem.Hash() != add.Hash() {
					discarded = append(discarded, rem.Body().Transactions()...)
					if rem, _ = pool.bc.GetBlockByHash(rem.ParentHash()); rem == nil {
						log.Error("Unrooted old chain seen by tx pool", "block", oldBlock.Number64(), "hash", oldBlock.Header().Hash())
						return
					}
					included = append(included, add.Body().Transactions()...)
					if add, _ = pool.bc.GetBlockByHash(add.ParentHash()); add == nil {
						log.Error("Unrooted new chain seen by tx pool", "block", newBlock.Number64(), "hash", newBlock.Hash())
						return
					}
				}
				log.Debugf("start to reset txs pool4 discarded %d , included %d, old number :%d ,new number:%d",
					len(discarded), len(included), oldNum.Uint64(), newNum.Uint64())
				reinject = pool.txDifference(discarded, included)
			}
		}
	}
	// Initialize the internal state to the current head
	if newBlock == nil {
		newBlock = pool.bc.CurrentBlock() // Special case during testing
	}

	if err := pool.ResetState(newBlock.Header().Hash()); nil != err {
		log.Errorf("reset current state faild, %v", err)
		return
	}
	pool.pendingNonces = newTxNoncer(pool.currentState) //newTxNoncer(statedb)
	pool.currentMaxGas = newBlock.GasLimit()

	// Inject any transactions discarded due to reorgs
	if len(reinject) > 0 {
		log.Debugf("Reinjecting stale transactions count %d", len(reinject))
	}

	//senderCacher.recover(pool.signer, reinject)
	pool.addTxsLocked(reinject, false)

	// Update all fork indicator by next pending block number.
	next := new(big.Int).Add(newBlock.Number64().ToBig(), big.NewInt(1))
	pool.istanbul = pool.chainconfig.IsIstanbul(next.Uint64())
	pool.eip2718 = pool.chainconfig.IsBerlin(next.Uint64())
	pool.eip1559 = pool.chainconfig.IsLondon(next.Uint64())
}

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (pool *TxsPool) promoteExecutables(accounts []types.Address) []*transaction.Transaction {
	// Track the promoted transactions to broadcast them at once
	var promoted []*transaction.Transaction

	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			continue // Just in case someone calls with a non existing account
		}
		// Drop all transactions that are deemed too old (low nonce)
		forwards := list.Forward(pool.currentState.GetNonce(addr))
		for _, tx := range forwards {
			hash := tx.Hash()
			pool.all.Remove(hash)
		}
		//log.Debug("Removed old queued transactions", "count", len(forwards))
		// Drop all transactions that are too costly (low balance or out of gas)
		//todo remove GetBalance check
		drops, _ := list.Filter(*pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			pool.all.Remove(hash)
		}
		//log.Debug("Removed unpayable queued transactions", "count", len(drops))

		// Gather all executable transactions and promote them
		readies := list.Ready(pool.pendingNonces.get(addr))
		for _, tx := range readies {
			hash := tx.Hash()
			if pool.promoteTx(addr, hash, tx) {
				promoted = append(promoted, tx)
			}
		}
		//log.Debug("Promoted queued transactions", "count", len(promoted))

		// Drop all transactions over the allowed limit
		var caps []*transaction.Transaction
		if !pool.locals.contains(addr) {
			caps = list.Cap(int(pool.config.AccountQueue))
			for _, tx := range caps {
				hash := tx.Hash()
				pool.all.Remove(hash)
				//log.Debug("Removed cap-exceeding queued transaction", "hash", hash)
			}
		}
		// Mark all the items dropped as removed
		//todo pool.priced.Removed(len(forwards) + len(drops) + len(caps))
		pool.priced.Removed(len(caps))
		if pool.locals.contains(addr) {
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
			delete(pool.beats, addr)
		}
	}
	return promoted
}

// truncatePending removes transactions from the pending queue if the pool is above the
// pending limit. The algorithm tries to reduce transaction counts by an approximately
// equal number for all for accounts with many pending transactions.
func (pool *TxsPool) truncatePending() {
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending <= pool.config.GlobalSlots {
		return
	}

	// Assemble a spam order to penalize large transactors first
	spammers := prque.New(nil)
	for addr, list := range pool.pending {
		// Only evict transactions from high rollers
		if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
			spammers.Push(addr, int64(list.Len()))
		}
	}
	// Gradually drop transactions from offenders
	offenders := []types.Address{}
	for pending > pool.config.GlobalSlots && !spammers.Empty() {
		// Retrieve the next offender if not local address
		offender, _ := spammers.Pop()
		offenders = append(offenders, offender.(types.Address))

		// Equalize balances until all the same or below threshold
		if len(offenders) > 1 {
			// Calculate the equalization threshold for all current offenders
			threshold := pool.pending[offender.(types.Address)].Len()

			// Iteratively reduce all offenders until below limit or threshold reached
			for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
				for i := 0; i < len(offenders)-1; i++ {
					list := pool.pending[offenders[i]]

					caps := list.Cap(list.Len() - 1)
					for _, tx := range caps {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						pool.all.Remove(hash)

						// Update the account nonce to the dropped transaction
						pool.pendingNonces.setIfLower(offenders[i], tx.Nonce())
						log.Debug("Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pool.priced.Removed(len(caps))
					if pool.locals.contains(offenders[i]) {
					}
					pending--
				}
			}
		}
	}

	// If still above threshold, reduce to limit or min allowance
	if pending > pool.config.GlobalSlots && len(offenders) > 0 {
		for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
			for _, addr := range offenders {
				list := pool.pending[addr]

				caps := list.Cap(list.Len() - 1)
				for _, tx := range caps {
					// Drop the transaction from the global pools too
					hash := tx.Hash()
					pool.all.Remove(hash)

					// Update the account nonce to the dropped transaction
					pool.pendingNonces.setIfLower(addr, tx.Nonce())
					log.Debug("Removed fairness-exceeding pending transaction", "hash", hash)
				}
				pool.priced.Removed(len(caps))
				if pool.locals.contains(addr) {
				}
				pending--
			}
		}
	}
}

// truncateQueue drops the oldes transactions in the queue if the pool is above the global queue limit.
func (pool *TxsPool) truncateQueue() {
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued <= pool.config.GlobalQueue {
		return
	}

	// Sort all accounts with queued transactions by heartbeat
	addresses := make(addressesByHeartbeat, 0, len(pool.queue))
	for addr := range pool.queue {
		if !pool.locals.contains(addr) { // don't drop locals
			addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
		}
	}
	sort.Sort(addresses)

	// Drop transactions until the total is below the limit or only locals remain
	for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
		addr := addresses[len(addresses)-1]
		list := pool.queue[addr.address]

		addresses = addresses[:len(addresses)-1]

		// Drop all transactions if they are less than the overflow
		if size := uint64(list.Len()); size <= drop {
			for _, tx := range list.Flatten() {
				hash := tx.Hash()
				pool.removeTx(hash, true)
			}
			drop -= size
			continue
		}
		// Otherwise drop only last few transactions
		txs := list.Flatten()
		for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
			hash := txs[i].Hash()
			pool.removeTx(hash, true)
			drop--
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
//
// Note: transactions are not marked as removed in the priced list because re-heaping
// is always explicitly triggered by SetBaseFee and it would be unnecessary and wasteful
// to trigger a re-heap is this function
func (pool *TxsPool) demoteUnexecutables() {
	// Iterate over all accounts and demote any non-executable transactions
	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

		// Drop all transactions that are deemed too old (low nonce)
		olds := list.Forward(nonce)
		for _, tx := range olds {
			hash := tx.Hash()
			pool.all.Remove(hash)
			//log.Debug("Removed old pending transaction", "hash", hash)
		}
		// Drop all transactions that are too costly (low balance or out of gas), and queue any invalids back for later
		drops, invalids := list.Filter(*pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			//log.Debug("Removed unpayable pending transaction", "hash", hash)
			pool.all.Remove(hash)
		}

		for _, tx := range invalids {
			hash := tx.Hash()
			//log.Debug("Demoting pending transaction", "hash", hash)

			// Internal shuffle shouldn't touch the lookup set.
			pool.enqueueTx(hash, tx, false, false)
		}
		if pool.locals.contains(addr) {
		}
		// If there's a gap in front, alert (should never happen) and postpone all transactions
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			gapped := list.Cap(0)
			for _, tx := range gapped {
				hash := tx.Hash()
				log.Error("Demoting invalidated transaction", "hash", hash)

				// Internal shuffle shouldn't touch the lookup set.
				pool.enqueueTx(hash, tx, false, false)
			}
			// This might happen in a reorg, so log it to the metering
		}
		// Delete the entire pending entry if it became empty.
		if list.Empty() {
			delete(pool.pending, addr)
		}
	}
}

// runReorg runs reset and promoteExecutables on behalf of scheduleReorgLoop.
func (pool *TxsPool) runReorg(done chan struct{}, reset *txspoolResetRequest, dirtyAccounts *accountSet, events map[types.Address]*txsSortedMap) {
	defer close(done)

	//go_logger.Logger.Sugar().Infof("start reset txs pool %v", reset)
	var promoteAddrs []types.Address
	if dirtyAccounts != nil && reset == nil {
		// Only dirty accounts need to be promoted, unless we're resetting.
		// For resets, all addresses in the tx queue will be promoted and
		// the flatten operation can be avoided.
		promoteAddrs = dirtyAccounts.flatten()
	}
	pool.mu.Lock()
	if reset != nil {
		// Reset from the old head to the new, rescheduling any reorged transactions
		pool.reset(reset.oldBlock, reset.newBlock)

		// Nonces were reset, discard any events that became stale
		for addr := range events {
			events[addr].Forward(pool.pendingNonces.get(addr))
			if events[addr].Len() == 0 {
				delete(events, addr)
			}
		}
		// Reset needs promote for all addresses
		promoteAddrs = make([]types.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			promoteAddrs = append(promoteAddrs, addr)
		}
	}
	// Check for pending transactions for every account that sent new ones
	promoted := pool.promoteExecutables(promoteAddrs)

	// If a new block appeared, validate the pool of pending transactions. This will
	// remove any transaction that has been included in the block or was invalidated
	// because of another transaction (e.g. higher gas price).
	if reset != nil {
		pool.demoteUnexecutables()

		if reset.newBlock != nil && pool.chainconfig.IsLondon(reset.newBlock.Number64().Uint64()+1) {
			pendingBaseFee, _ := uint256.FromBig(misc.CalcBaseFee(pool.chainconfig, reset.newBlock.Header().(*block.Header)))
			pool.priced.SetBaseFee(pendingBaseFee)
		}
		// Update all accounts to the latest known pending nonce
		nonces := make(map[types.Address]uint64, len(pool.pending))
		for addr, list := range pool.pending {
			highestPending := list.LastElement()
			nonces[addr] = highestPending.Nonce() + 1
		}
		pool.pendingNonces.setAll(nonces)
	}
	// Ensure pool.queue and pool.pending sizes stay within the configured limits.
	pool.truncatePending()
	pool.truncateQueue()

	pool.changesSinceReorg = 0 // Reset change counter
	pool.mu.Unlock()

	// Notify subsystems for newly added transactions
	for _, tx := range promoted {
		addr := *tx.From()
		if _, ok := events[addr]; !ok {
			events[addr] = newTxSortedMap()
		}
		events[addr].Put(tx)
	}
	// todo
	if len(events) > 0 {
		var txs []*transaction.Transaction
		for _, set := range events {
			txs = append(txs, set.Flatten()...)
		}
		event.GlobalEvent.Send(&common.NewTxsEvent{Txs: txs})
	}
}

// scheduleLoop
func (pool *TxsPool) scheduleLoop() {
	defer pool.wg.Done()

	var (
		curDone       chan struct{} // non-nil while runReorg is active
		nextDone      = make(chan struct{})
		launchNextRun bool
		reset         *txspoolResetRequest
		dirtyAccounts *accountSet
		queuedEvents  = make(map[types.Address]*txsSortedMap)
	)

	for {
		// Launch next background reorg if needed
		if curDone == nil && launchNextRun {
			// Run the background reorg and announcements
			go pool.runReorg(nextDone, reset, dirtyAccounts, queuedEvents)

			curDone, nextDone = nextDone, make(chan struct{})
			launchNextRun = false

			reset, dirtyAccounts = nil, nil
			queuedEvents = make(map[types.Address]*txsSortedMap)
		}

		select {
		case req := <-pool.reqResetCh:
			// Reset request: update head if request is already pending.
			if reset == nil {
				reset = req
			} else {
				reset.newBlock = req.newBlock
			}
			launchNextRun = true
			pool.reorgDoneCh <- nextDone

		case req := <-pool.reqPromoteCh:
			// Promote request: update address set if request is already pending.
			if dirtyAccounts == nil {
				dirtyAccounts = req
			} else {
				dirtyAccounts.merge(req)
			}
			launchNextRun = true
			pool.reorgDoneCh <- nextDone

		case tx := <-pool.queueTxEventCh:
			// Queue up the event, but don't schedule a reorg. It's up to the caller to
			// request one later if they want the events sent.
			addr := *tx.From()
			if _, ok := queuedEvents[addr]; !ok {
				queuedEvents[addr] = newTxSortedMap()
			}
			queuedEvents[addr].Put(tx)

		case <-curDone:
			curDone = nil

		//case <-pool.reorgShutdownCh:
		//	// Wait for current run to finish.
		//	if curDone != nil {
		//		<-curDone
		//	}
		//	close(nextDone)
		//	return
		case <-pool.ctx.Done():
			// Wait for current run to finish.
			if curDone != nil {
				<-curDone
			}
			close(nextDone)
			return
		}

	}
}

// blockChangeLoop
func (pool *TxsPool) blockChangeLoop() {
	defer pool.wg.Done()

	highestBlockCh := make(chan common.ChainHighestBlock)
	defer close(highestBlockCh)
	highestSub := event.GlobalEvent.Subscribe(highestBlockCh)
	defer highestSub.Unsubscribe()

	oldBlock := pool.bc.CurrentBlock()

	for {
		select {
		case <-highestSub.Err():
			return
		case <-pool.ctx.Done():
			return
		case highestBlock, ok := <-highestBlockCh:
			if ok && highestBlock.Inserted {
				pool.requestReset(oldBlock, pool.bc.CurrentBlock())
				oldBlock = pool.bc.CurrentBlock()
			}
		}
	}
}

// Stop terminates the transaction pool.
func (pool *TxsPool) Stop() {
	pool.cancel()
	pool.wg.Wait()
	log.Info("Transaction pool stopped")
}

// txDifference
func (pool *TxsPool) txDifference(a, b []*transaction.Transaction) []*transaction.Transaction {
	keep := make([]*transaction.Transaction, 0, len(a))

	remove := make(map[types.Hash]struct{})
	for _, tx := range b {
		hash := tx.Hash()
		remove[hash] = struct{}{}
	}

	for _, tx := range a {
		hash := tx.Hash()
		if _, ok := remove[hash]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
//
// The enforceTips parameter can be used to do an extra filtering on the pending
// transactions and only return those whose **effective** tip is large enough in
// the next pending execution environment.
func (pool *TxsPool) Pending(enforceTips bool) map[types.Address][]*transaction.Transaction {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[types.Address][]*transaction.Transaction)
	for addr, list := range pool.pending {
		txs := list.Flatten()

		// If the miner requests tip enforcement, cap the lists now
		if enforceTips && !pool.locals.contains(addr) {
			for i, tx := range txs {
				if tx.EffectiveGasTipIntCmp(pool.gasPrice, pool.priced.urgent.baseFee) < 0 {
					txs = txs[:i]
					break
				}
			}
		}
		if len(txs) > 0 {
			pending[addr] = txs
		}
	}
	return pending
}

// Has
func (pool *TxsPool) Has(hash types.Hash) bool {
	return pool.all.Get(hash) != nil
}

// GetTransaction
func (pool *TxsPool) GetTransaction() (txs []*transaction.Transaction, err error) {
	//
	pending := pool.Pending(false)
	heads := make([]*transaction.Transaction, 0, len(txs))
	for _, accTxs := range pending {
		//heads = append(heads, accTxs[0])
		heads = append(heads, accTxs...)
	}
	return heads, nil
}

// GetTx
func (pool *TxsPool) GetTx(hash types.Hash) *transaction.Transaction {
	return pool.all.Get(hash)
}

// Content
func (pool *TxsPool) Content() (map[types.Address][]*transaction.Transaction, map[types.Address][]*transaction.Transaction) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[types.Address][]*transaction.Transaction)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[types.Address][]*transaction.Transaction)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

func (pool *TxsPool) Nonce(addr types.Address) uint64 {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingNonces.get(addr)
}

// StatsPrint
func (pool *TxsPool) StatsPrint() {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	pendingAddresses, pendingTxs, queuedAddresses, queuedTxs := pool.Stats()

	log.Debugf("txs pool： pendingAddresses count: %d pendingTxs count: %d ", pendingAddresses, pendingTxs)
	log.Debugf("txs pool： queuedAddresses count: %d queuedTxs count: %d", queuedAddresses, queuedTxs)
}

func (pool *TxsPool) Stats() (int, int, int, int) {
	pendingTxs := 0
	pendingAddresses := len(pool.pending)
	for _, list := range pool.pending {
		pendingTxs += list.Len()
	}
	queuedTxs := 0
	queuedAddresses := len(pool.queue)
	for _, list := range pool.queue {
		queuedTxs += list.Len()
	}
	return pendingAddresses, pendingTxs, queuedAddresses, queuedTxs
}

func (pool *TxsPool) ResetState(blockHash types.Hash) error {
	if pool.currentState != nil {
		reader := pool.currentState.GetStateReader()
		if reader != nil {
			if hreader, ok := reader.(*state.HistoryStateReader); ok {
				hreader.Rollback()
			}
		}
	}

	tx, err := pool.bc.DB().BeginRo(pool.ctx)
	if nil != err {
		return err
	}
	blockNr := rawdb.ReadHeaderNumber(tx, blockHash)
	if nil == blockNr {
		return fmt.Errorf("invaild block hash")
	}
	stateReader := state.NewStateHistoryReader(tx, tx, *blockNr)
	pool.currentState = state.New(stateReader)
	return nil
}
