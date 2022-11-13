package api

import (
	"context"
	common2 "github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/transaction"
	types2 "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/params"
	"github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	lru "github.com/hashicorp/golang-lru"
	"github.com/holiman/uint256"
	"github.com/amazechain/amc/log"
	"math/big"
	"sort"
	"sync"
)

const sampleNumber = 3 // Number of transactions sampled in a block

var (
	DefaultMaxPrice    = big.NewInt(500 * params.GWei)
	DefaultIgnorePrice = big.NewInt(2 * params.Wei)
)

type Config struct {
	Blocks           int
	Percentile       int
	MaxHeaderHistory int
	MaxBlockHistory  int
	Default          *big.Int `toml:",omitempty"`
	MaxPrice         *big.Int `toml:",omitempty"`
	IgnorePrice      *big.Int `toml:",omitempty"`
}

// Oracle recommends gas prices based on the content of recent
// blocks. Suitable for both light and full clients.
type Oracle struct {
	backend     common2.IBlockChain
	lastHead    common.Hash
	lastPrice   *big.Int
	maxPrice    *big.Int
	ignorePrice *big.Int
	cacheLock   sync.RWMutex
	fetchLock   sync.Mutex

	checkBlocks, percentile           int
	maxHeaderHistory, maxBlockHistory int
	historyCache                      *lru.Cache
}

// NewOracle returns a new gasprice oracle which can recommend suitable
// gasprice for newly created transaction.
func NewOracle(backend common2.IBlockChain, params Config) *Oracle {
	blocks := params.Blocks
	if blocks < 1 {
		blocks = 1
		log.Warn("Sanitizing invalid gasprice oracle sample blocks", "provided", params.Blocks, "updated", blocks)
	}
	percent := params.Percentile
	if percent < 0 {
		percent = 0
		log.Warn("Sanitizing invalid gasprice oracle sample percentile", "provided", params.Percentile, "updated", percent)
	} else if percent > 100 {
		percent = 100
		log.Warn("Sanitizing invalid gasprice oracle sample percentile", "provided", params.Percentile, "updated", percent)
	}
	maxPrice := params.MaxPrice
	if maxPrice == nil || maxPrice.Int64() <= 0 {
		maxPrice = DefaultMaxPrice
		log.Warn("Sanitizing invalid gasprice oracle price cap", "provided", params.MaxPrice, "updated", maxPrice)
	}
	ignorePrice := params.IgnorePrice
	if ignorePrice == nil || ignorePrice.Int64() <= 0 {
		ignorePrice = DefaultIgnorePrice
		log.Warn("Sanitizing invalid gasprice oracle ignore price", "provided", params.IgnorePrice, "updated", ignorePrice)
	} else if ignorePrice.Int64() > 0 {
		log.Info("Gasprice oracle is ignoring threshold set", "threshold", ignorePrice)
	}
	maxHeaderHistory := params.MaxHeaderHistory
	if maxHeaderHistory < 1 {
		maxHeaderHistory = 1
		log.Warn("Sanitizing invalid gasprice oracle max header history", "provided", params.MaxHeaderHistory, "updated", maxHeaderHistory)
	}
	maxBlockHistory := params.MaxBlockHistory
	if maxBlockHistory < 1 {
		maxBlockHistory = 1
		log.Warn("Sanitizing invalid gasprice oracle max block history", "provided", params.MaxBlockHistory, "updated", maxBlockHistory)
	}

	cache, _ := lru.New(2048)
	//headEvent := make(chan core.ChainHeadEvent, 1)
	//backend.SubscribeChainHeadEvent(headEvent)
	//go func() {
	//	var lastHead common.Hash
	//	for ev := range headEvent {
	//		if ev.Block.ParentHash() != lastHead {
	//			cache.Purge()
	//		}
	//		lastHead = ev.Block.Hash()
	//	}
	//}()

	return &Oracle{
		backend:          backend,
		lastPrice:        params.Default,
		maxPrice:         maxPrice,
		ignorePrice:      ignorePrice,
		checkBlocks:      blocks,
		percentile:       percent,
		maxHeaderHistory: maxHeaderHistory,
		maxBlockHistory:  maxBlockHistory,
		historyCache:     cache,
	}
}

// SuggestTipCap returns a tip cap so that newly created transaction can have a
// very high chance to be included in the following blocks.
//
// Note, for legacy transactions and the legacy eth_gasPrice RPC call, it will be
// necessary to add the basefee to the returned number to fall back to the legacy
// behavior.
func (oracle *Oracle) SuggestTipCap(ctx context.Context, chainConfig *params.ChainConfig) (*big.Int, error) {
	var latestNumber jsonrpc.BlockNumber
	latestNumber = jsonrpc.LatestBlockNumber
	head, _ := oracle.backend.GetHeaderByNumber(types2.NewInt64(uint64(latestNumber)))
	var headHash common.Hash
	if head == nil {
		headHash = common.Hash{}
	} else {
		headHash = common.Hash(head.Hash())
	}

	// If the latest gasprice is still available, return it.
	oracle.cacheLock.RLock()
	lastHead, lastPrice := oracle.lastHead, oracle.lastPrice
	oracle.cacheLock.RUnlock()
	if headHash == lastHead {
		return new(big.Int).Set(lastPrice), nil
	}
	oracle.fetchLock.Lock()
	defer oracle.fetchLock.Unlock()

	// Try checking the cache again, maybe the last fetch fetched what we need
	oracle.cacheLock.RLock()
	lastHead, lastPrice = oracle.lastHead, oracle.lastPrice
	oracle.cacheLock.RUnlock()
	if headHash == lastHead {
		return new(big.Int).Set(lastPrice), nil
	}
	var (
		sent, exp int
		number    = head.Number64().Uint64()
		result    = make(chan results, oracle.checkBlocks)
		quit      = make(chan struct{})
		results   []*big.Int
	)
	for sent < oracle.checkBlocks && number > 0 {
		go oracle.getBlockValues(ctx, types.MakeSigner(chainConfig, big.NewInt(int64(number))), number, sampleNumber, oracle.ignorePrice, result, quit)
		sent++
		exp++
		number--
	}
	for exp > 0 {
		res := <-result
		if res.err != nil {
			close(quit)
			return new(big.Int).Set(lastPrice), res.err
		}
		exp--
		// Nothing returned. There are two special cases here:
		// - The block is empty
		// - All the transactions included are sent by the miner itself.
		// In these cases, use the latest calculated price for sampling.
		if len(res.values) == 0 {
			res.values = []*big.Int{lastPrice}
		}
		// Besides, in order to collect enough data for sampling, if nothing
		// meaningful returned, try to query more blocks. But the maximum
		// is 2*checkBlocks.
		if len(res.values) == 1 && len(results)+1+exp < oracle.checkBlocks*2 && number > 0 {
			go oracle.getBlockValues(ctx, types.MakeSigner(chainConfig, big.NewInt(int64(number))), number, sampleNumber, oracle.ignorePrice, result, quit)
			sent++
			exp++
			number--
		}
		results = append(results, res.values...)
	}
	price := lastPrice
	if len(results) > 0 {
		sort.Sort(bigIntArray(results))
		price = results[(len(results)-1)*oracle.percentile/100]
	}
	if price.Cmp(oracle.maxPrice) > 0 {
		price = new(big.Int).Set(oracle.maxPrice)
	}
	oracle.cacheLock.Lock()
	oracle.lastHead = headHash
	oracle.lastPrice = price
	oracle.cacheLock.Unlock()

	return new(big.Int).Set(price), nil
}

type results struct {
	values []*big.Int
	err    error
}

// getBlockPrices calculates the lowest transaction gas price in a given block
// and sends it to the result channel. If the block is empty or all transactions
// are sent by the miner itself(it doesn't make any sense to include this kind of
// transaction prices for sampling), nil gasprice is returned.
func (oracle *Oracle) getBlockValues(ctx context.Context, signer types.Signer, blockNum uint64, limit int, ignoreUnder *big.Int, result chan results, quit chan struct{}) {
	block, err := oracle.backend.GetBlockByNumber(types2.NewInt64(uint64(jsonrpc.BlockNumber(blockNum))))
	if block == nil {
		select {
		case result <- results{nil, err}:
		case <-quit:
		}
		return
	}
	// Sort the transaction by effective tip in ascending sort.
	txs := make([]*transaction.Transaction, len(block.Transactions()))
	copy(txs, block.Transactions())
	sorter := newSorter(txs, block.BaseFee64())
	sort.Sort(sorter)

	var prices []*big.Int
	for _, tx := range sorter.txs {
		tip, _ := tx.EffectiveGasTip(block.BaseFee64())
		ignoreUnderx, _ := uint256.FromBig(ignoreUnder)
		if ignoreUnder != nil && tip.Cmp(ignoreUnderx) == -1 {
			continue
		}
		sender, err := tx.From()
		if err == nil && sender != block.Coinbase() {
			prices = append(prices, tip.ToBig())
			if len(prices) >= limit {
				break
			}
		}
	}
	select {
	case result <- results{prices, nil}:
	case <-quit:
	}
}

type txSorter struct {
	txs     []*transaction.Transaction
	baseFee types2.Int256
}

func newSorter(txs []*transaction.Transaction, baseFee types2.Int256) *txSorter {
	return &txSorter{
		txs:     txs,
		baseFee: baseFee,
	}
}

func (s *txSorter) Len() int { return len(s.txs) }
func (s *txSorter) Swap(i, j int) {
	s.txs[i], s.txs[j] = s.txs[j], s.txs[i]
}
func (s *txSorter) Less(i, j int) bool {
	// It's okay to discard the error because a tx would never be
	// accepted into a block with an invalid effective tip.
	tip1, _ := s.txs[i].EffectiveGasTip(s.baseFee)
	tip2, _ := s.txs[j].EffectiveGasTip(s.baseFee)
	x, _ := uint256.FromBig(tip2.ToBig())
	return tip1.Cmp(x) < 0
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// FullNodeGPO contains default gasprice oracle settings for full node.
var FullNodeGPO = Config{
	Blocks:           20,
	Percentile:       60,
	MaxHeaderHistory: 1024,
	MaxBlockHistory:  1024,
	MaxPrice:         DefaultMaxPrice,
	IgnorePrice:      DefaultIgnorePrice,
}

// LightClientGPO contains default gasprice oracle settings for light client.
var LightClientGPO = Config{
	Blocks:           2,
	Percentile:       60,
	MaxHeaderHistory: 300,
	MaxBlockHistory:  5,
	MaxPrice:         DefaultMaxPrice,
	IgnorePrice:      DefaultIgnorePrice,
}
