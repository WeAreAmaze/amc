package txspool

import (
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"time"
)

// LazyTransaction contains a small subset of the transaction properties that is
// enough for the miner and other APIs to handle large batches of transactions;
// and supports pulling up the entire transaction when really needed.
type LazyTransaction struct {
	Pool LazyResolver             // Transaction resolver to pull the real transaction up
	Hash types.Hash               // Transaction hash to pull up if needed
	Tx   *transaction.Transaction // Transaction if already resolved

	Time      time.Time    // Time when the transaction was first seen
	GasFeeCap *uint256.Int // Maximum fee per gas the transaction may consume
	GasTipCap *uint256.Int // Maximum miner tip per gas the transaction can pay

	Gas     uint64 // Amount of gas required by the transaction
	BlobGas uint64 // Amount of blob gas required by the transaction
}

// Resolve retrieves the full transaction belonging to a lazy handle if it is still
// maintained by the transaction pool.
func (ltx *LazyTransaction) Resolve() *transaction.Transaction {
	if ltx.Tx == nil {
		ltx.Tx = ltx.Pool.Get(ltx.Hash)
	}
	return ltx.Tx
}

// LazyResolver is a minimal interface needed for a transaction pool to satisfy
// resolving lazy transactions. It's mostly a helper to avoid the entire sub-
// pool being injected into the lazy transaction.
type LazyResolver interface {
	// Get returns a transaction if it is contained in the pool, or nil otherwise.
	Get(hash types.Hash) *transaction.Transaction
}
