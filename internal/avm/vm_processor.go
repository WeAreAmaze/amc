package avm

import (
	"context"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	amc_types "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/crypto"
	"github.com/amazechain/amc/internal/avm/params"
	"github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/internal/avm/vm"
	"github.com/amazechain/amc/internal/consensus"
	log "github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/statedb"
	"github.com/amazechain/amc/utils"
	"math/big"
)

type VMProcessor struct {
	bc     common.IBlockChain
	engine consensus.IEngine

	ctx    context.Context
	cancel context.CancelFunc
}

func (V VMProcessor) ID() string {
	//TODO implement me
	panic("implement me")
}

func (V VMProcessor) Name() string {
	//TODO implement me
	panic("implement me")
}

func (V VMProcessor) Version() string {
	//TODO implement me
	panic("implement me")
}

func (V VMProcessor) Amcdata() map[string]string {
	//TODO implement me
	panic("implement me")
}

func (V VMProcessor) Endpoint() []string {
	//TODO implement me
	panic("implement me")
}

func NewVMProcessor(ctx context.Context, bc common.IBlockChain, engine consensus.IEngine) *VMProcessor {
	vp := &VMProcessor{
		bc:     bc,
		engine: engine,
	}
	c, cancel := context.WithCancel(utils.NewContext(ctx, vp))
	vp.ctx = c
	vp.cancel = cancel
	return vp
}

func (p *VMProcessor) Processor(b block.IBlock, db *statedb.StateDB) (block.Receipts, []*types.Log, error) {
	var (
		header   = b.Header().(*block.Header)
		gp       = new(common.GasPool).AddGas(b.GasLimit())
		usedGas  = new(uint64)
		receipts block.Receipts
	)

	snap := db.Snapshot()
	blockContext := NewBlockContext(b.Header(), p.bc, nil)
	ethDb := NewDBStates(db)

	log.Debugf("block gas limit: %d", *gp)
	//todo blockchain info  Debug: true, Tracer: vm.NewMarkdownLogger(os.Stdout)

	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, ethDb, params.AmazeChainConfig, vm.Config{})
	for i, tx := range b.Transactions() {
		msg := types.AsMessage(tx, header.BaseFee.ToBig(), false)
		txHash, err := tx.Hash()
		if err != nil {
			db.RevertToSnapshot(snap)
			return nil, nil, err
		}
		ethDb.Prepare(types.FromAmcHash(txHash), i)

		receipt, err := applyTransaction(msg, params.AmazeChainConfig, p.bc, nil, gp, db, header.Number.ToBig(), header.Hash(), tx, usedGas, vmenv)
		if err != nil {
			db.RevertToSnapshot(snap)
			return nil, nil, err
		}
		receipts = append(receipts, receipt)
	}

	return receipts, nil, nil
}

func applyTransaction(msg types.Message, config *params.ChainConfig, bc ChainContext, author *amc_types.Address, gp *common.GasPool, db common.IStateDB, blockNumber *big.Int, blockHash amc_types.Hash, tx *transaction.Transaction, usedGas *uint64, evm *vm.EVM) (*block.Receipt, error) {
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	ethDb := NewDBStates(db)
	evm.Reset(txContext, ethDb)

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	// Update the state with pending changes.
	var root []byte
	// todo
	if config.IsByzantium(blockNumber) {
		//statedb.Finalise(true)
	} else {
		//root = statedb.IntermediateRoot(blockchain.IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &block.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = block.ReceiptStatusFailed
	} else {
		receipt.Status = block.ReceiptStatusSuccessful
	}
	receipt.TxHash, _ = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = types.ToAmcAddress(crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce()))
	}

	// Set the receipt logs and create the bloom filter.
	hash, _ := tx.Hash()
	receipt.Logs = types.ToAmcLogs(ethDb.GetLogs(types.FromAmcHash(hash), types.FromAmcHash(blockHash)))
	//todo  log bloom
	//bloom, _ := amc_types.NewBloom(100)
	//receipt.Bloom = *bloom
	receipt.BlockHash = blockHash
	receipt.BlockNumber, _ = amc_types.FromBig(blockNumber)
	receipt.TransactionIndex = uint(ethDb.TxIndex())
	return receipt, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *amc_types.Address, gp *common.GasPool, state common.IStateDB, header *block.Header, tx *transaction.Transaction, usedGas *uint64, cfg vm.Config) (*block.Receipt, error) {
	msg := types.AsMessage(tx, header.BaseFee.ToBig(), false)
	// Create a new context to be used in the EVM environment
	blockContext := NewBlockContext(header, bc, author)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, NewDBStates(state), config, cfg)
	return applyTransaction(msg, config, bc, author, gp, state, header.Number.ToBig(), header.Hash(), tx, usedGas, vmenv)
}
