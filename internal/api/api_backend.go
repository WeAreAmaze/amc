// Copyright 2023 The AmazeChain Authors
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

package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/accounts"
	common2 "github.com/amazechain/amc/common"
	types "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	common "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal"
	"github.com/amazechain/amc/internal/vm"
	"github.com/amazechain/amc/internal/vm/evmtypes"
	"github.com/amazechain/amc/modules/rawdb"
	rpc "github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// API implements ethapi.Backend for full nodes
//type API struct {
//	extRPCEnabled       bool
//	allowUnprotectedTxs bool
//	eth                 *node.Node
//	//gpo                 *gasprice.Oracle
//}

// ChainConfig returns the active chain configuration.
func (b *API) ChainConfig() *params.ChainConfig {
	return b.chainConfig
}

func (b *API) CurrentBlock() *types.Header {
	return b.bc.CurrentBlock().Header().(*types.Header)
}

//func (b *API) SetHead(number uint64) {
//	b.eth.handler.downloader.Cancel()
//	b.bc.SetHead(number)
//}

func (b *API) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	//if number == rpc.PendingBlockNumber {
	//	block := b.eth.Miner().PendingBlock()
	//	return block.Header(), nil
	//}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.bc.CurrentBlock().Header().(*types.Header), nil
	}
	//if number == rpc.FinalizedBlockNumber {
	//	if !b.eth.Merger().TDDReached() {
	//		return nil, errors.New("'finalized' tag not supported on pre-merge network")
	//	}
	//	block := b.eth.blockchain.CurrentFinalBlock()
	//	if block != nil {
	//		return block, nil
	//	}
	//	return nil, errors.New("finalized block not found")
	//}
	//if number == rpc.SafeBlockNumber {
	//	if !b.eth.Merger().TDDReached() {
	//		return nil, errors.New("'safe' tag not supported on pre-merge network")
	//	}
	//	block := b.eth.blockchain.CurrentSafeBlock()
	//	if block != nil {
	//		return block, nil
	//	}
	//	return nil, errors.New("safe block not found")
	//}
	return b.bc.GetHeaderByNumber(uint256.NewInt(uint64(number.Int64()))).(*types.Header), nil
}

func (b *API) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, _ := b.bc.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.bc.(*internal.BlockChain).GetCanonicalHash(header.Number64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header.(*types.Header), nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *API) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	iHeader, _ := b.bc.GetHeaderByHash(hash)
	return iHeader.(*types.Header), nil
}

func (b *API) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	//if number == rpc.PendingBlockNumber {
	//	block := b.minPendingBlock()
	//	return block, nil
	//}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		header := b.bc.CurrentBlock()
		return b.bc.GetBlock(header.Hash(), header.Number64().Uint64()).(*types.Block), nil
	}
	//if number == rpc.FinalizedBlockNumber {
	//	if !b.eth.Merger().TDDReached() {
	//		return nil, errors.New("'finalized' tag not supported on pre-merge network")
	//	}
	//	header := b.eth.blockchain.CurrentFinalBlock()
	//	return b.eth.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	//}
	//if number == rpc.SafeBlockNumber {
	//	//if !b.eth.Merger().TDDReached() {
	//	//	return nil, errors.New("'safe' tag not supported on pre-merge network")
	//	//}
	//	header := b.BlockChain().CurrentBlock()
	//	return b.eth.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	//}
	iBlock, err := b.bc.GetBlockByNumber(uint256.NewInt(uint64(number)))
	if nil != err {
		return nil, err
	}
	return iBlock.(*types.Block), nil
}

func (b *API) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	iBlock, _ := b.bc.GetBlockByHash(hash)
	return iBlock.(*types.Block), nil
}

// GetBody returns body of a block. It does not resolve special block numbers.
//func (b *API) GetBody(ctx context.Context, hash common.Hash, number rpc.BlockNumber) (*types.Body, error) {
//	if number < 0 || hash == (common.Hash{}) {
//		return nil, errors.New("invalid arguments; expect hash and no special block numbers")
//	}
//	if body := b.bc.GetBody(hash); body != nil {
//		return body, nil
//	}
//	return nil, errors.New("block body not found")
//}

func (b *API) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, _ := b.bc.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.bc.(*internal.BlockChain).GetCanonicalHash(header.Number64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.bc.GetBlock(hash, header.Number64().Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block.(*types.Block), nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

//func (b *API) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
//	return b.eth.miner.PendingBlockAndReceipts()
//}

//func (b *API) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
//	// Pending state is only known by the miner
//	if number == rpc.PendingBlockNumber {
//		block, state := b.eth.miner.Pending()
//		return state, block.Header(), nil
//	}
//	// Otherwise resolve the block number and return its state
//	header, err := b.HeaderByNumber(ctx, number)
//	if err != nil {
//		return nil, nil, err
//	}
//	if header == nil {
//		return nil, nil, errors.New("header not found")
//	}
//	stateDb, err := b.bc.StateAt(header.Root)
//	return stateDb, header, err
//}

//func (b *API) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
//	if blockNr, ok := blockNrOrHash.Number(); ok {
//		return b.StateAndHeaderByNumber(ctx, blockNr)
//	}
//	if hash, ok := blockNrOrHash.Hash(); ok {
//		header, err := b.HeaderByHash(ctx, hash)
//		if err != nil {
//			return nil, nil, err
//		}
//		if header == nil {
//			return nil, nil, errors.New("header for hash not found")
//		}
//		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
//			return nil, nil, errors.New("hash is not currently canonical")
//		}
//		stateDb, err := b.bc.StateAt(header.Root)
//		return stateDb, header, err
//	}
//	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
//}

func (b *API) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.bc.GetReceipts(hash)

}

//func (b *API) GetLogs(ctx context.Context, hash common.Hash, number uint64) ([][]*types.Log, error) {
//	return rawdb.ReadLogs(b.eth.chainDb, hash, number, b.ChainConfig()), nil
//}

func (b *API) GetTd(ctx context.Context, hash common.Hash) *uint256.Int {
	if header, _ := b.bc.GetHeaderByHash(hash); header != nil {
		return b.bc.GetTd(hash, header.Number64())
	}
	return nil
}

func (b *API) GetEVM(ctx context.Context, msg *transaction.Message, state *state.IntraBlockState, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error) {
	if vmConfig == nil {
		//vmConfig = b.bc.GetVMConfig()
		vmConfig = &vm.Config{}
	}
	txContext := internal.NewEVMTxContext(msg)
	getHeader := func(hash common.Hash, n uint64) *types.Header {
		h := b.bc.GetHeader(hash, uint256.NewInt(n))
		return h.(*types.Header)
	}
	context := internal.NewEVMBlockContext(header, internal.GetHashFn(header, getHeader), b.bc.Engine(), nil)
	return vm.NewEVM(context, txContext, state, b.bc.Config(), *vmConfig), state.Error, nil
}

//func (b *API) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
//	return b.bc.SubscribeRemovedLogsEvent(ch)
//}

//func (b *API) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
//	return b.eth.miner.SubscribePendingLogs(ch)
//}
//
//func (b *API) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
//	return b.bc.SubscribeChainEvent(ch)
//}
//
//func (b *API) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
//	return b.bc.SubscribeChainHeadEvent(ch)
//}
//
//func (b *API) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
//	return b.bc.SubscribeChainSideEvent(ch)
//}

//func (b *API) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
//	return b.bc.SubscribeLogsEvent(ch)
//}

//func (b *API) SendTx(ctx context.Context, signedTx *types.Transaction) error {
//	return b.eth.txPool.AddLocal(signedTx)
//}
//
//func (b *API) GetPoolTransactions() (types.Transactions, error) {
//	pending := b.eth.txPool.Pending(false)
//	var txs types.Transactions
//	for _, batch := range pending {
//		txs = append(txs, batch...)
//	}
//	return txs, nil
//}
//
//func (b *API) GetPoolTransaction(hash common.Hash) *types.Transaction {
//	return b.eth.txPool.Get(hash)
//}

func (b *API) GetTransaction(ctx context.Context, txHash common.Hash) (*transaction.Transaction, common.Hash, uint64, uint64, error) {
	var t *transaction.Transaction
	var blockHash common.Hash
	var index uint64
	var err error
	var blockNumber uint64
	b.db.View(ctx, func(tx kv.Tx) error {
		t, blockHash, blockNumber, index, err = rawdb.ReadTransactionByHash(tx, txHash)
		return err
	})
	return t, blockHash, blockNumber, index, nil
}

//func (b *API) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
//	return b.eth.txPool.Nonce(addr), nil
//}
//
//func (b *API) Stats() (pending int, queued int) {
//	return b.eth.txPool.Stats()
//}
//
//func (b *API) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
//	return b.eth.TxPool().Content()
//}
//
//func (b *API) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
//	return b.eth.TxPool().ContentFrom(addr)
//}
//
//func (b *API) TxPool() *txpool.TxPool {
//	return b.eth.TxPool()
//}

//func (b *API) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
//	return b.eth.TxPool().SubscribeNewTxsEvent(ch)
//}
//
//func (b *API) SyncProgress() ethereum.SyncProgress {
//	return b.eth.Downloader().Progress()
//}
//
//func (b *API) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
//	return b.gpo.SuggestTipCap(ctx)
//}
//
//func (b *API) FeeHistory(ctx context.Context, blockCount uint64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (firstBlock *big.Int, reward [][]*big.Int, baseFee []*big.Int, gasUsedRatio []float64, err error) {
//	return b.gpo.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
//}

func (b *API) ChainDb() kv.RwDB {
	return b.db
}

//func (b *API) EventMux() *event.TypeMux {
//	return b.eth.EventMux()
//}

func (b *API) AccountManager() *accounts.Manager {
	return b.accountManager
}

//func (b *API) ExtRPCEnabled() bool {
//	return b.extRPCEnabled
//}

//func (b *API) UnprotectedAllowed() bool {
//	return b.allowUnprotectedTxs
//}

//func (b *API) RPCGasCap() uint64 {
//	return b.eth.config.RPCGasCap
//}
//
//func (b *API) RPCEVMTimeout() time.Duration {
//	return b.eth.config.RPCEVMTimeout
//}
//
//func (b *API) RPCTxFeeCap() float64 {
//	return b.eth.config.RPCTxFeeCap
//}

//func (b *API) BloomStatus() (uint64, uint64) {
//	sections, _, _ := b.eth.bloomIndexer.Sections()
//	return params.BloomBitsBlocks, sections
//}
//
//func (b *API) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
//	for i := 0; i < bloomFilterThreads; i++ {
//		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.eth.bloomRequests)
//	}
//}

func (b *API) CurrentHeader() *types.Header {
	return b.bc.CurrentBlock().Header().(*types.Header)
}

//func (b *API) Miner() *miner.Miner {
//	return b..Miner().(*miner.Miner)
//}

//func (b *API) StateAtBlock(ctx context.Context, tx kv.Tx, block *types.Block /*reexec uint64, base *state.IntraBlockState, readOnly bool, preferDisk bool*/) (*state.IntraBlockState, tracers.StateReleaseFunc, error) {
//	return b.bc.StateAt(tx, block.Number64().Uint64()), nil, nil
//}
//
//func (b *API) StateAtTransaction(ctx context.Context, tx kv.Tx, block *types.Block, txIndex int /*, reexec uint64*/) (*transaction.Message, evmtypes.BlockContext, *state.IntraBlockState, tracers.StateReleaseFunc, error) {
//	return .StateAtTransaction(ctx, tx, block, txIndex /*, reexec*/)
//}

// stateAtTransaction returns the execution environment of a certain transaction.
func (eth *API) StateAtTransaction(ctx context.Context, dbTx kv.Tx, blk *types.Block, txIndex int) (*transaction.Message, evmtypes.BlockContext, *state.IntraBlockState, error) {
	// Short circuit if it's genesis block.
	if blk.Number64().Uint64() == 0 {
		return nil, evmtypes.BlockContext{}, nil, errors.New("no transaction in genesis")
	}
	// Create the parent state database
	parent := eth.BlockChain().GetBlock(blk.ParentHash(), blk.Number64().Uint64()-1).(*types.Block)
	if parent == nil {
		return nil, evmtypes.BlockContext{}, nil, fmt.Errorf("parent %#x not found", blk.ParentHash())
	}
	// Lookup the statedb of parent block from the live database,
	// otherwise regenerate it on the flight.

	statedb, err := eth.StateAtBlock(ctx, dbTx, parent)
	if err != nil {
		return nil, evmtypes.BlockContext{}, nil, err
	}
	if txIndex == 0 && len(blk.Transactions()) == 0 {
		return nil, evmtypes.BlockContext{}, statedb, nil
	}
	// Recompute transactions up to the target index.
	signer := transaction.MakeSigner(eth.BlockChain().Config(), blk.Number64().ToBig())
	getHeader := func(hash common.Hash, number uint64) *types.Header {
		return rawdb.ReadHeader(dbTx, hash, number)
	}
	blockContext := internal.NewEVMBlockContext(blk.Header().(*types.Header), internal.GetHashFn(blk.Header().(*types.Header), getHeader), eth.Engine(), nil)
	vmenv := vm.NewEVM(blockContext, evmtypes.TxContext{}, statedb, eth.BlockChain().Config(), vm.Config{})
	rules := vmenv.ChainRules()

	for idx, tx := range blk.Transactions() {
		//// Assemble the transaction call message and return if the requested offset
		//msg, _ := tx.AsMessage(signer, blk.BaseFee64())
		//txContext := internal.NewEVMTxContext(msg)
		//
		//if idx == txIndex {
		//	return msg, blockContext, statedb, release, nil
		//}
		//vmenv.Reset()
		//// Not yet the searched for transaction, execute on top of the current state
		//statedb.Prepare(tx.Hash(), blk.Hash(), idx)
		//if _, err := internal.ApplyMessage(vmenv, msg, new(common.GasPool).AddGas(tx.Gas()), true, false); err != nil {
		//	return nil, evmtypes.BlockContext{}, nil, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		//}
		//// Ensure any modifications are committed to the state
		//// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		//statedb.FinalizeTx(rules, )
		statedb.Prepare(tx.Hash(), blk.Hash(), idx, *tx.From())

		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer, blk.BaseFee64())
		if msg.FeeCap().IsZero() && eth.Engine() != nil {
			syscall := func(contract common.Address, data []byte) ([]byte, error) {
				return internal.SysCallContract(contract, data, *eth.BlockChain().Config(), statedb, blk.Header().(*types.Header), eth.Engine() /* constCall */)
			}
			msg.SetIsFree(eth.Engine().IsServiceTransaction(msg.From(), syscall))
		}

		TxContext := internal.NewEVMTxContext(msg)
		if idx == txIndex {
			return &msg, blockContext, statedb, nil
		}
		vmenv.Reset(TxContext, statedb)
		// Not yet the searched for transaction, execute on top of the current state
		if _, err := internal.ApplyMessage(vmenv, msg, new(common2.GasPool).AddGas(tx.Gas()), true /* refunds */, false /* gasBailout */); err != nil {
			return nil, evmtypes.BlockContext{}, nil, fmt.Errorf("transaction %x failed: %w", tx.Hash(), err)
		}
		// Ensure any modifications are committed to the state
		// Only delete empty objects if EIP161 (part of Spurious Dragon) is in effect
		_ = statedb.FinalizeTx(rules, statedb.GetStateReader().(*state.PlainState))

		if idx+1 == len(blk.Transactions()) {
			// Return the state from evaluating all txs in the block, note no msg or TxContext in this case
			return nil, blockContext, statedb, nil
		}
	}
	return nil, evmtypes.BlockContext{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, blk.Hash())
}

// StateAtBlock retrieves the state database associated with a certain block.
// If no state is locally available for the given block, a number of blocks
// are attempted to be reexecuted to generate the desired state. The optional
// base layer statedb can be provided which is regarded as the statedb of the
// parent block.
//
// An additional release function will be returned if the requested state is
// available. Release is expected to be invoked when the returned state is no longer needed.
// Its purpose is to prevent resource leaking. Though it can be noop in some cases.
//
// Parameters:
//   - block:      The block for which we want the state(state = block.Root)
//   - reexec:     The maximum number of blocks to reprocess trying to obtain the desired state
//   - base:       If the caller is tracing multiple blocks, the caller can provide the parent
//     state continuously from the callsite.
//   - readOnly:   If true, then the live 'blockchain' state database is used. No mutation should
//     be made from caller, e.g. perform Commit or other 'save-to-disk' changes.
//     Otherwise, the trash generated by caller may be persisted permanently.
//   - preferDisk: this arg can be used by the caller to signal that even though the 'base' is
//     provided, it would be preferable to start from a fresh state, if we have it
//     on disk.
func (eth *API) StateAtBlock(ctx context.Context, tx kv.Tx, blk *types.Block) (statedb *state.IntraBlockState, err error) {
	var (
		//current *block.Block
		//// database state.Database
		//report = true
		origin = blk.Number64().Uint64()
	)
	// The state is only for reading purposes, check the state presence in
	// live database.
	//if readOnly {
	// The state is available in live database, create a reference
	// on top to prevent garbage collection and return a release
	// function to deref it.
	statedb = eth.BlockChain().StateAt(tx, origin)
	//statedb.Database().TrieDB().Reference(block.Root(), common.Hash{})
	return statedb, nil
	//}
	//}
	// The state is both for reading and writing, or it's unavailable in disk,
	// try to construct/recover the state over an ephemeral trie.Database for
	// isolating the live one.
	//if base != nil {
	//	if preferDisk {
	//		// Create an ephemeral trie.Database for isolating the live one. Otherwise
	//		// the internal junks created by tracing will be persisted into the disk.
	//		database = state.NewDatabaseWithConfig(eth.chainDb, &trie.Config{Cache: 16})
	//		if statedb, err = state.New(block.Root(), database, nil); err == nil {
	//			log.Info("Found disk backend for state trie", "root", block.Root(), "number", block.Number())
	//			return statedb, noopReleaser, nil
	//		}
	//	}
	//	// The optional base statedb is given, mark the start point as parent block
	//	statedb, database, report = base, base.Database(), false
	//	current = eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	//} else {
	// Otherwise, try to reexec blocks until we find a state or reach our limit
	//current = block

	// Create an ephemeral trie.Database for isolating the live one. Otherwise
	// the internal junks created by tracing will be persisted into the disk.
	// database = state.NewDatabaseWithConfig(eth.chainDb, &trie.Config{Cache: 16})

	// If we didn't check the live database, do check state over ephemeral database,
	// otherwise we would rewind past a persisted block (specific corner case is
	// chain tracing from the genesis).
	//if !readOnly {
	//	statedb, err = state.New(current.Root(), database, nil)
	//	if err == nil {
	//		return statedb, noopReleaser, nil
	//	}
	//}
	//// Database does not have the state for the given block, try to regenerate
	//for i := uint64(0); i < reexec; i++ {
	//	if err := ctx.Err(); err != nil {
	//		return nil, nil, err
	//	}
	//	if current.NumberU64() == 0 {
	//		return nil, nil, errors.New("genesis state is missing")
	//	}
	//	parent := eth.blockchain.GetBlock(current.ParentHash(), current.NumberU64()-1)
	//	if parent == nil {
	//		return nil, nil, fmt.Errorf("missing block %v %d", current.ParentHash(), current.NumberU64()-1)
	//	}
	//	current = parent
	//
	//	statedb, err = state.New(current.Root(), database, nil)
	//	if err == nil {
	//		break
	//	}
	//}
	//if err != nil {
	//	switch err.(type) {
	//	case *trie.MissingNodeError:
	//		return nil, nil, fmt.Errorf("required historical state unavailable (reexec=%d)", reexec)
	//	default:
	//		return nil, nil, err
	//	}
	//}
	//}
	// State is available at historical point, re-execute the blocks on top for
	// the desired state.
	//var (
	//	start  = time.Now()
	//	logged time.Time
	//	parent common.Hash
	//)
	//for current.NumberU64() < origin {
	//	if err := ctx.Err(); err != nil {
	//		return nil, nil, err
	//	}
	//	// Print progress logs if long enough time elapsed
	//	if time.Since(logged) > 8*time.Second && report {
	//		log.Info("Regenerating historical state", "block", current.NumberU64()+1, "target", origin, "remaining", origin-current.NumberU64()-1, "elapsed", time.Since(start))
	//		logged = time.Now()
	//	}
	//	// Retrieve the next block to regenerate and process it
	//	next := current.NumberU64() + 1
	//	if current = eth.blockchain.GetBlockByNumber(next); current == nil {
	//		return nil, nil, fmt.Errorf("block #%d not found", next)
	//	}
	//	_, _, _, err := eth.blockchain.Processor().Process(current, statedb, vm.Config{})
	//	if err != nil {
	//		return nil, nil, fmt.Errorf("processing block %d failed: %v", current.NumberU64(), err)
	//	}
	//	// Finalize the state so any modifications are written to the trie
	//	root, err := statedb.Commit(eth.blockchain.Config().IsEIP158(current.Number()))
	//	if err != nil {
	//		return nil, nil, fmt.Errorf("stateAtBlock commit failed, number %d root %v: %w",
	//			current.NumberU64(), current.Root().Hex(), err)
	//	}
	//	statedb, err = state.New(root, database, nil)
	//	if err != nil {
	//		return nil, nil, fmt.Errorf("state reset after block %d failed: %v", current.NumberU64(), err)
	//	}
	//	// Hold the state reference and also drop the parent state
	//	// to prevent accumulating too many nodes in memory.
	//	database.TrieDB().Reference(root, common.Hash{})
	//	if parent != (common.Hash{}) {
	//		database.TrieDB().Dereference(parent)
	//	}
	//	parent = root
	//}
	//if report {
	//	nodes, imgs := database.TrieDB().Size()
	//	log.Info("Historical state regenerated", "block", current.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	//}
	//return statedb, func() { database.TrieDB().Dereference(block.Root()) }, nil
}
