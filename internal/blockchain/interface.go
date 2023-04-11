package blockchain

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/modules/statedb"
)

// Validator is an interface which defines the standard for block validation. It
// is only responsible for validating block contents, as the header validation is
// done by the specific consensus engines.
type Validator interface {
	// ValidateBody validates the given block's content.
	ValidateBody(block block.IBlock) error

	// ValidateState validates the given statedb and optionally the receipts and
	// gas used.
	ValidateState(block block.IBlock, state *statedb.StateDB, receipts block.Receipts, usedGas uint64) error
}
