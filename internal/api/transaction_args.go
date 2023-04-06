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

package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/log"
	"github.com/holiman/uint256"
	"math/big"

	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/math"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	mvm_common "github.com/amazechain/amc/internal/avm/common"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
)

// TransactionArgs represents
type TransactionArgs struct {
	From                 *mvm_common.Address `json:"from"`
	To                   *mvm_common.Address `json:"to"`
	Gas                  *hexutil.Uint64     `json:"gas"`
	GasPrice             *hexutil.Big        `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big        `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big        `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big        `json:"value"`
	Nonce                *hexutil.Uint64     `json:"nonce"`

	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *mvm_types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big          `json:"chainId,omitempty"`
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *mvm_common.Hash      `json:"blockHash"`
	BlockNumber      *hexutil.Big          `json:"blockNumber"`
	From             mvm_common.Address    `json:"from"`
	Gas              hexutil.Uint64        `json:"gas"`
	GasPrice         *hexutil.Big          `json:"gasPrice"`
	GasFeeCap        *hexutil.Big          `json:"maxFeePerGas,omitempty"`
	GasTipCap        *hexutil.Big          `json:"maxPriorityFeePerGas,omitempty"`
	Hash             mvm_common.Hash       `json:"hash"`
	Input            hexutil.Bytes         `json:"input"`
	Nonce            hexutil.Uint64        `json:"nonce"`
	To               *mvm_common.Address   `json:"to"`
	TransactionIndex *hexutil.Uint64       `json:"transactionIndex"`
	Value            *hexutil.Big          `json:"value"`
	Type             hexutil.Uint64        `json:"type"`
	Accesses         *mvm_types.AccessList `json:"accessList,omitempty"`
	ChainID          *hexutil.Big          `json:"chainId,omitempty"`
	V                *hexutil.Big          `json:"v"`
	R                *hexutil.Big          `json:"r"`
	S                *hexutil.Big          `json:"s"`
}

// from retrieves the transaction sender address.
func (args *TransactionArgs) from() types.Address {
	if args.From == nil {
		return types.Address{}
	}
	from := *mvm_types.ToAmcAddress(args.From)
	return from
}

// data retrieves the transaction calldata. Input field is preferred.
func (args *TransactionArgs) data() []byte {
	if args.Input != nil {
		return *args.Input
	}
	if args.Data != nil {
		return *args.Data
	}
	return nil
}

// setDefaults
func (args *TransactionArgs) setDefaults(ctx context.Context, api *API) error {
	//if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
	//	return errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	//}
	if err := args.setFeeDefaults(ctx, api); err != nil {
		return err
	}

	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Nonce == nil {
		nonce := api.TxsPool().Nonce(args.from())
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`)
	}
	if args.To == nil && len(args.data()) == 0 {
		return errors.New(`contract creation without any data provided`)
	}
	if args.Gas == nil {
		data := args.data()
		callArgs := TransactionArgs{
			From:                 args.From,
			To:                   args.To,
			GasPrice:             args.GasPrice,
			MaxFeePerGas:         args.MaxFeePerGas,
			MaxPriorityFeePerGas: args.MaxPriorityFeePerGas,
			Value:                args.Value,
			Data:                 (*hexutil.Bytes)(&data),
		}
		pendingBlockNr := jsonrpc.BlockNumberOrHashWithNumber(jsonrpc.PendingBlockNumber)
		//todo gasCap
		estimated, err := DoEstimateGas(ctx, api, callArgs, pendingBlockNr, 50000000)
		if err != nil {
			return err
		}
		args.Gas = &estimated
	}
	if args.ChainID == nil {
		id := (*hexutil.Big)(api.GetChainConfig().ChainID)
		args.ChainID = id
	}
	return nil
}

// ToMessage to evm message
func (args *TransactionArgs) ToMessage(globalGasCap uint64, baseFee *big.Int) (transaction.Message, error) {
	//header := api.BlockChain().CurrentBlock().Header().(*block.Header)
	//signer := transaction.MakeSigner(api.chainConfig, header.Number.ToBig())
	//args.setDefaults(context.Background(), api)
	//return args.toTransaction().AsMessage(signer, header.BaseFee)
	// msg := mvm_types.AsMessage(, header.BaseFee.ToBig(), true)

	// Reject invalid combinations of pre- and post-1559 fee styles
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return transaction.Message{}, errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	}
	// Set sender address or use zero address if none specified.
	addr := args.from()

	// Set default gas & gas price if none were set
	gas := globalGasCap
	if gas == 0 {
		gas = uint64(math.MaxUint64 / 2)
	}
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != 0 && globalGasCap < gas {
		log.Warn("Caller gas above allowance, capping", "requested", gas, "cap", globalGasCap)
		gas = globalGasCap
	}
	var (
		gasPrice  *big.Int
		gasFeeCap *big.Int
		gasTipCap *big.Int
	)
	if baseFee == nil {
		// If there's no basefee, then it must be a non-1559 execution
		gasPrice = new(big.Int)
		if args.GasPrice != nil {
			gasPrice = args.GasPrice.ToInt()
		}
		gasFeeCap, gasTipCap = gasPrice, gasPrice
	} else {
		// A basefee is provided, necessitating 1559-type execution
		if args.GasPrice != nil {
			// User specified the legacy gas field, convert to 1559 gas typing
			gasPrice = args.GasPrice.ToInt()
			gasFeeCap, gasTipCap = gasPrice, gasPrice
		} else {
			// User specified 1559 gas fields (or none), use those
			gasFeeCap = new(big.Int)
			if args.MaxFeePerGas != nil {
				gasFeeCap = args.MaxFeePerGas.ToInt()
			}
			gasTipCap = new(big.Int)
			if args.MaxPriorityFeePerGas != nil {
				gasTipCap = args.MaxPriorityFeePerGas.ToInt()
			}
			// Backfill the legacy gasPrice for EVM execution, unless we're all zeroes
			gasPrice = new(big.Int)
			if gasFeeCap.BitLen() > 0 || gasTipCap.BitLen() > 0 {
				gasPrice = math.BigMin(new(big.Int).Add(gasTipCap, baseFee), gasFeeCap)
			}
		}
	}
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	data := args.data()
	var accessList transaction.AccessList
	if args.AccessList != nil {
		accessList = mvm_types.ToAmcAccessList(*args.AccessList)
	}
	val, is1 := uint256.FromBig(value)
	gp, is2 := uint256.FromBig(gasPrice)
	gfc, is3 := uint256.FromBig(gasFeeCap)
	gtc, is4 := uint256.FromBig(gasTipCap)
	if is1 || is2 || is3 || is4 {
		return transaction.Message{}, fmt.Errorf("args.Value higher than 2^256-1")
	}
	msg := transaction.NewMessage(addr, mvm_types.ToAmcAddress(args.To), 0, val, gas, gp, gfc, gtc, data, accessList, false, true)
	return msg, nil

}

// toTransaction assemble Transaction
func (args *TransactionArgs) toTransaction() *transaction.Transaction {
	var data transaction.TxData
	switch {
	case args.MaxFeePerGas != nil:
		al := transaction.AccessList{}
		if args.AccessList != nil {
			al = mvm_types.ToAmcAccessList(*args.AccessList)
		}
		dy := &transaction.DynamicFeeTx{
			To:         mvm_types.ToAmcAddress(args.To),
			Nonce:      uint64(*args.Nonce),
			Gas:        uint64(*args.Gas),
			Data:       args.data(),
			AccessList: al,
		}
		var is bool
		dy.GasFeeCap, is = uint256.FromBig((*big.Int)(args.MaxFeePerGas))
		if is {
			log.Error("GasFeeCap to uint256 failed")
		}
		dy.ChainID, is = uint256.FromBig((*big.Int)(args.ChainID))
		if is {
			log.Error("ChainID to uint256 failed")
		}
		dy.GasTipCap, is = uint256.FromBig((*big.Int)(args.MaxPriorityFeePerGas))
		if is {
			log.Error("GasTipCap to uint256 failed")
		}
		dy.Value, is = uint256.FromBig((*big.Int)(args.Value))
		if is {
			log.Error("Value to uint256 failed")
		}
		data = dy
	case args.AccessList != nil:
		alt := &transaction.AccessListTx{
			To:    mvm_types.ToAmcAddress(args.To),
			Nonce: uint64(*args.Nonce),
			Gas:   uint64(*args.Gas),
			Data:  args.data(),
		}
		alt.AccessList = mvm_types.ToAmcAccessList(*args.AccessList)
		var is bool
		alt.GasPrice, is = uint256.FromBig((*big.Int)(args.GasPrice))
		if is {
			log.Error("GasPrice to uint256 failed")
		}
		alt.ChainID, is = uint256.FromBig((*big.Int)(args.ChainID))
		if is {
			log.Error("ChainID to uint256 failed")
		}
		alt.Value, is = uint256.FromBig((*big.Int)(args.Value))
		if is {
			log.Error("Value to uint256 failed")
		}
		data = alt
	default:
		lt := &transaction.LegacyTx{
			To:    mvm_types.ToAmcAddress(args.To),
			Nonce: uint64(*args.Nonce),
			Gas:   uint64(*args.Gas),
			Data:  args.data(),
		}
		var is bool
		lt.GasPrice, is = uint256.FromBig((*big.Int)(args.GasPrice))
		if is {
			log.Error("GasPrice to uint256 failed")
		}
		lt.Value, is = uint256.FromBig((*big.Int)(args.Value))
		if is {
			log.Error("Value to uint256 failed")
		}
		data = lt
	}
	return transaction.NewTx(data)
}

// ToTransaction
//func (args *TransactionArgs) ToTransaction() *transaction.Transaction {
//	return args.toTransaction()
//}

// LrpLegacyTx is the transaction data of regular Ethereum transactions.

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *transaction.Transaction, current block.IHeader) *RPCTransaction {
	blockNumber := uint64(0)
	//todo baseFee
	return newRPCTransaction(tx, types.Hash{}, blockNumber, 0, big.NewInt(baseFee))
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx *transaction.Transaction, blockHash types.Hash, blockNumber uint64, index uint64, baseFee *big.Int) *RPCTransaction {

	v, r, s := tx.RawSignatureValues()
	from := tx.From()
	hash := tx.Hash()
	result := &RPCTransaction{
		Type:     hexutil.Uint64(tx.Type()),
		From:     *mvm_types.FromAmcAddress(from),
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice().ToBig()),
		Hash:     mvm_types.FromAmcHash(hash),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       mvm_types.FromAmcAddress(tx.To()),
		Value:    (*hexutil.Big)(tx.Value().ToBig()),
		V:        (*hexutil.Big)(v.ToBig()),
		R:        (*hexutil.Big)(r.ToBig()),
		S:        (*hexutil.Big)(s.ToBig()),
	}
	if blockHash != (types.Hash{}) {
		hash := mvm_types.FromAmcHash(blockHash)
		result.BlockHash = &hash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = (*hexutil.Uint64)(&index)
	}
	switch tx.Type() {
	case transaction.LegacyTxType:
		// if a legacy transaction has an EIP-155 chain id, include it explicitly
		if id := tx.ChainId(); id.Sign() == 0 {
			result.ChainID = (*hexutil.Big)(id.ToBig())
		}
	case transaction.AccessListTxType:
		// todo copy al
		//al := tx.AccessList()
		//result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId().ToBig())
	case transaction.DynamicFeeTxType:
		// todo copy al
		//al := tx.AccessList()
		//result.Accesses = &al
		result.ChainID = (*hexutil.Big)(tx.ChainId().ToBig())
		result.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap().ToBig())
		result.GasTipCap = (*hexutil.Big)(tx.GasTipCap().ToBig())
		// if the transaction has been mined, compute the effective gas price
		if baseFee != nil && blockHash != (types.Hash{}) {
			// price = min(tip, gasFeeCap - baseFee) + baseFee
			price := math.BigMin(new(big.Int).Add(tx.GasTipCap().ToBig(), baseFee), tx.GasFeeCap().ToBig())
			result.GasPrice = (*hexutil.Big)(price)
		} else {
			result.GasPrice = (*hexutil.Big)(tx.GasFeeCap().ToBig())
		}
	}
	return result
}

// setFeeDefaults fills in default fee values for unspecified tx fields.
func (args *TransactionArgs) setFeeDefaults(ctx context.Context, b *API) error {
	// If both gasPrice and at least one of the EIP-1559 fee parameters are specified, error.
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	}
	// If the tx has completely specified a fee mechanism, no default is needed. This allows users
	// who are not yet synced past London to get defaults for other tx values. See
	// https://github.com/ethereum/go-ethereum/pull/23274 for more information.
	eip1559ParamsSet := args.MaxFeePerGas != nil && args.MaxPriorityFeePerGas != nil
	if (args.GasPrice != nil && !eip1559ParamsSet) || (args.GasPrice == nil && eip1559ParamsSet) {
		// Sanity check the EIP-1559 fee parameters if present.
		if args.GasPrice == nil && args.MaxFeePerGas.ToInt().Cmp(args.MaxPriorityFeePerGas.ToInt()) < 0 {
			return fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", args.MaxFeePerGas, args.MaxPriorityFeePerGas)
		}
		return nil
	}
	// Now attempt to fill in default value depending on whether London is active or not.
	head := b.BlockChain().CurrentBlock()
	if b.chainConfig.IsLondon(head.Number64().Uint64()) {
		// London is active, set maxPriorityFeePerGas and maxFeePerGas.
		if err := args.setLondonFeeDefaults(ctx, head.Header().(*block.Header), b); err != nil {
			return err
		}
	} else {
		if args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil {
			return fmt.Errorf("maxFeePerGas and maxPriorityFeePerGas are not valid before London is active")
		}
		// London not active, set gas price.
		price, err := b.gpo.SuggestTipCap(ctx, b.chainConfig)
		if err != nil {
			return err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	return nil
}

// setLondonFeeDefaults fills in reasonable default fee values for unspecified fields.
func (args *TransactionArgs) setLondonFeeDefaults(ctx context.Context, head *block.Header, b *API) error {
	// Set maxPriorityFeePerGas if it is missing.
	if args.MaxPriorityFeePerGas == nil {
		tip, err := b.gpo.SuggestTipCap(ctx, b.chainConfig)
		if err != nil {
			return err
		}
		args.MaxPriorityFeePerGas = (*hexutil.Big)(tip)
	}
	// Set maxFeePerGas if it is missing.
	if args.MaxFeePerGas == nil {
		// Set the max fee to be 2 times larger than the previous block's base fee.
		// The additional slack allows the tx to not become invalidated if the base
		// fee is rising.
		val := new(big.Int).Add(
			args.MaxPriorityFeePerGas.ToInt(),
			new(big.Int).Mul(head.BaseFee.ToBig(), big.NewInt(2)),
		)
		args.MaxFeePerGas = (*hexutil.Big)(val)
	}
	// Both EIP-1559 fee parameters are now set; sanity check them.
	if args.MaxFeePerGas.ToInt().Cmp(args.MaxPriorityFeePerGas.ToInt()) < 0 {
		return fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", args.MaxFeePerGas, args.MaxPriorityFeePerGas)
	}
	return nil
}
