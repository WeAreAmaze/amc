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

package rawdb

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// TxLookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxLookupEntry struct {
	BlockHash  types.Hash
	BlockIndex uint64
	Index      uint64
}

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadTxLookupEntry(db kv.Getter, txnHash types.Hash) (*uint64, error) {
	data, err := db.GetOne(modules.TxLookup, txnHash.Bytes())
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	// number := new(big.Int).SetBytes(data).Uint64()
	number := uint256.NewInt(0).SetBytes(data).Uint64()
	return &number, nil
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteTxLookupEntries(db kv.Putter, block *block.Block) {
	for _, tx := range block.Transactions() {
		data := block.Number64().Bytes()
		h := tx.Hash()
		if err := db.Put(modules.TxLookup, h.Bytes(), data); err != nil {
			log.Crit("Failed to store transaction lookup entry", "err", err)
		}
	}
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func DeleteTxLookupEntry(db kv.Deleter, hash types.Hash) error {
	return db.Delete(modules.TxLookup, hash.Bytes())
}

// ReadTransactionByHash retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransactionByHash(db kv.Tx, hash types.Hash) (*transaction.Transaction, types.Hash, uint64, uint64, error) {
	blockNumber, err := ReadTxLookupEntry(db, hash)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	if blockNumber == nil {
		return nil, types.Hash{}, 0, 0, nil
	}
	blockHash, err := ReadCanonicalHash(db, *blockNumber)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	if blockHash == (types.Hash{}) {
		return nil, types.Hash{}, 0, 0, nil
	}
	body := ReadCanonicalBodyWithTransactions(db, blockHash, *blockNumber)
	if body == nil {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash)
		return nil, types.Hash{}, 0, 0, nil
	}
	senders, err1 := ReadSenders(db, blockHash, *blockNumber)
	if err1 != nil {
		return nil, types.Hash{}, 0, 0, err1
	}
	body.SendersToTxs(senders)
	for txIndex, tx := range body.Txs {
		h := tx.Hash()
		if h == hash {
			return tx, blockHash, *blockNumber, uint64(txIndex), nil
		}
	}
	log.Error("Transaction not found", "number", blockNumber, "hash", blockHash, "txhash", hash)
	return nil, types.Hash{}, 0, 0, nil
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db kv.Tx, hash types.Hash, blockNumber uint64) (*transaction.Transaction, types.Hash, uint64, uint64, error) {
	blockHash, err := ReadCanonicalHash(db, blockNumber)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	if blockHash == (types.Hash{}) {
		return nil, types.Hash{}, 0, 0, nil
	}
	body := ReadCanonicalBodyWithTransactions(db, blockHash, blockNumber)
	if body == nil {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash)
		return nil, types.Hash{}, 0, 0, nil
	}
	senders, err1 := ReadSenders(db, blockHash, blockNumber)
	if err1 != nil {
		return nil, types.Hash{}, 0, 0, err1
	}
	body.SendersToTxs(senders)
	for txIndex, tx := range body.Txs {
		h := tx.Hash()
		if h == hash {
			return tx, blockHash, blockNumber, uint64(txIndex), nil
		}
	}
	log.Error("Transaction not found", "number", blockNumber, "hash", blockHash, "txhash", hash)
	return nil, types.Hash{}, 0, 0, nil
}

func ReadReceipt(db kv.Tx, txHash types.Hash) (*block.Receipt, types.Hash, uint64, uint64, error) {
	// Retrieve the context of the receipt based on the transaction hash
	blockNumber, err := ReadTxLookupEntry(db, txHash)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	if blockNumber == nil {
		return nil, types.Hash{}, 0, 0, nil
	}
	blockHash, err := ReadCanonicalHash(db, *blockNumber)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	if blockHash == (types.Hash{}) {
		return nil, types.Hash{}, 0, 0, nil
	}
	b, senders, err := ReadBlockWithSenders(db, blockHash, *blockNumber)
	if err != nil {
		return nil, types.Hash{}, 0, 0, err
	}
	// Read all the receipts from the block and return the one with the matching hash
	receipts := ReadReceipts(db, b, senders)
	for receiptIndex, receipt := range receipts {
		if receipt.TxHash == txHash {
			return receipt, blockHash, *blockNumber, uint64(receiptIndex), nil
		}
	}
	log.Error("Receipt not found", "number", blockNumber, "hash", blockHash, "txhash", txHash)
	return nil, types.Hash{}, 0, 0, nil
}
