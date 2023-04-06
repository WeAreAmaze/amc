package apos

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type Faker struct{}

func (f Faker) Author(header block.IHeader) (types.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (f Faker) VerifyHeader(chain consensus.ChainHeaderReader, header block.IHeader, seal bool) error {
	//TODO implement me
	panic("implement me")
}

func (f Faker) VerifyHeaders(chain consensus.ChainHeaderReader, headers []block.IHeader, seals []bool) (chan<- struct{}, <-chan error) {
	//TODO implement me
	panic("implement me")
}

func (f Faker) VerifyUncles(chain consensus.ChainReader, block block.IBlock) error {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Prepare(chain consensus.ChainHeaderReader, header block.IHeader) error {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Finalize(chain consensus.ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction, uncles []block.IHeader) {
	//TODO implement me
	panic("implement me")
}

func (f Faker) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction, uncles []block.IHeader, receipts []*block.Receipt, reward []*block.Reward) (block.IBlock, error) {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Rewards(tx kv.RwTx, header block.IHeader, state *state.IntraBlockState, setRewards bool) ([]*block.Reward, error) {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Seal(chain consensus.ChainHeaderReader, block block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error {
	//TODO implement me
	panic("implement me")
}

func (f Faker) SealHash(header block.IHeader) types.Hash {
	//TODO implement me
	panic("implement me")
}

func (f Faker) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent block.IHeader) *uint256.Int {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Type() params.ConsensusType {
	return params.Faker
}

func (f Faker) APIs(chain consensus.ChainReader) []jsonrpc.API {
	//TODO implement me
	panic("implement me")
}

func (f Faker) Close() error {
	//TODO implement me
	panic("implement me")
}

func NewFaker() consensus.Engine {
	return &Faker{}
}

func (f Faker) IsServiceTransaction(sender types.Address, syscall consensus.SystemCall) bool {
	return false
}
