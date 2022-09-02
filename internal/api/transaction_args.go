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
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	mvm_common "github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/common/hexutil"
	"github.com/amazechain/amc/internal/avm/common/math"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"math/big"
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

	ChainID *hexutil.Big `json:"chainId,omitempty"`
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
	from := mvm_types.ToAmcAddress(*args.From)
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
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
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
		estimated, err := DoEstimateGas(ctx, api, callArgs, pendingBlockNr)
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
func (args *TransactionArgs) ToMessage(api *API) (mvm_types.Message, error) {
	header := api.BlockChain().CurrentBlock().Header().(*block.Header)
	msg := mvm_types.AsMessage(args.toTransaction(baseFee, header.BaseFee.ToBig()), header.BaseFee.ToBig(), true)
	return msg, nil
}

// toTransaction assemble Transaction
func (args *TransactionArgs) toTransaction(globalGasCap uint64, baseFee *big.Int) *transaction.Transaction {
	var data transaction.TxData

	GasPrice, _ := types.FromBig((*big.Int)(args.GasPrice))
	Value, _ := types.FromBig((*big.Int)(args.Value))

	gas := globalGasCap
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}

	nonce := uint64(0)
	if args.Nonce != nil {
		nonce = uint64(*args.Nonce)
	}

	addr := args.from()

	//todo
	switch {
	// todo other type
	default:
		to := mvm_types.ToAmcAddress(*args.To)
		data = &transaction.LegacyTx{
			From:     &addr,
			To:       &to,
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: GasPrice,
			Value:    Value,
			Data:     args.data(),
		}
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
	from, _ := tx.From()
	hash, _ := tx.Hash()
	result := &RPCTransaction{
		Type:     hexutil.Uint64(tx.Type()),
		From:     *mvm_types.FromAmcAddress(from),
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice().ToBig()),
		Hash:     mvm_types.FromAmcHash(hash),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       mvm_types.FromAmcAddress(*tx.To()),
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
