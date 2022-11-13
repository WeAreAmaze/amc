package avm

import (
	"github.com/amazechain/amc/common/block"
	amc_types "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/params"
	"github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/internal/avm/vm"
	"github.com/amazechain/amc/internal/consensus"
	"math/big"
)

//
//// NewEVMContext creates a new context for use in the EVM.
//func NewEVMContext(from common.Address, blockNum, timeStamp, difficulty int64) vm.Context {
//	// If we don't have an explicit author (i.e. not mining), extract from the header
//	return vm.Context{
//		CanTransfer: CanTransfer,
//		Transfer:    Transfer,
//		GetHash:     GetHashFn(),
//		Origin:      from,
//		Coinbase:    common.Address{},
//		BlockNumber: new(big.Int).Set(big.NewInt(blockNum)),
//		Time:        new(big.Int).Set(big.NewInt(timeStamp)),
//		Difficulty:  new(big.Int).Set(big.NewInt(difficulty)),
//		GasLimit:    0xfffffffffffffff, //header.GasLimit,
//		GasPrice:    new(big.Int).Set(big.NewInt(10)),
//	}
//}
//

type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus.Engine

	// GetHeader returns the hash corresponding to their hash.
	GetHeader(amc_types.Hash, amc_types.Int256) block.IHeader
}

func NewBlockContext(header block.IHeader, chain ChainContext, author *amc_types.Address) vm.BlockContext {
	var beneficiary *common.Address
	h := header.(*block.Header)
	if author == nil {
		author, _ := chain.Engine().Author(h)
		beneficiary = types.FromAmcAddress(&author)
	} else {
		beneficiary = types.FromAmcAddress(author)
	}
	return vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(h, chain),
		Coinbase:    *beneficiary,
		GasLimit:    h.GasLimit,
		BlockNumber: h.Number.ToBig(),
		Time:        new(big.Int).SetUint64(h.Time),
		Difficulty:  h.Difficulty.ToBig(),
		BaseFee:     h.BaseFee.ToBig(),
		Random:      &common.Hash{},
	}
}

func NewEVMTxContext(msg Message) vm.TxContext {
	return vm.TxContext{
		Origin:   msg.From(),
		GasPrice: new(big.Int).Set(msg.GasPrice()),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *block.Header, chain ChainContext) func(n uint64) common.Hash {

	var cache []common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, types.FromAmcHash(ref.ParentHash))
		}
		if idx := ref.Number.Sub(amc_types.NewInt64(n + 1)); idx.Slt(amc_types.NewInt64(uint64(len(cache)))) {
			return cache[idx.Uint64()]
		}

		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]
		lastKnownNumber := ref.Number.Sub(amc_types.NewInt64(uint64(len(cache))))

		for {
			h := chain.GetHeader(types.ToAmcHash(lastKnownHash), lastKnownNumber)
			if h == nil {
				break
			}
			header := h.(*block.Header)
			cache = append(cache, types.FromAmcHash(header.ParentHash))
			lastKnownHash = types.FromAmcHash(header.ParentHash)
			lastKnownNumber = header.Number.Sub(amc_types.NewInt64(1))
			if lastKnownNumber.Equal(amc_types.NewInt64(n)) {
				return lastKnownHash
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks wether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	if db.GetBalance(addr).Cmp(amount) < 0 {
		db.AddBalance(addr, amount)
	}
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

func CalcGasLimit(parentGasLimit, desiredLimit uint64) uint64 {
	delta := parentGasLimit/params.GasLimitBoundDivisor - 1
	limit := parentGasLimit
	if desiredLimit < params.MinGasLimit {
		desiredLimit = params.MinGasLimit
	}
	// If we're outside our allowed gas range, we try to hone towards them
	if limit < desiredLimit {
		limit = parentGasLimit + delta
		if limit > desiredLimit {
			limit = desiredLimit
		}
		return limit
	}
	if limit > desiredLimit {
		limit = parentGasLimit - delta
		if limit < desiredLimit {
			limit = desiredLimit
		}
	}
	return limit
}
