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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules"
	"google.golang.org/protobuf/proto"

	"github.com/holiman/uint256"
	"math"
	"time"

	common2 "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db kv.Getter, number uint64) (types.Hash, error) {
	data, err := db.GetOne(modules.HeaderCanonical, modules.EncodeBlockNumber(number))
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed ReadCanonicalHash: %w, number=%d", err, number)
	}
	if len(data) == 0 {
		return types.Hash{}, nil
	}
	return types.BytesToHash(data), nil
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db kv.Putter, hash types.Hash, number uint64) error {
	if err := db.Put(modules.HeaderCanonical, modules.EncodeBlockNumber(number), hash.Bytes()); err != nil {
		return fmt.Errorf("failed to store number to hash mapping: %w", err)
	}
	return nil
}

// TruncateCanonicalHash removes all the number to hash canonical mapping from block number N
func TruncateCanonicalHash(tx kv.RwTx, blockFrom uint64, deleteHeaders bool) error {
	if err := tx.ForEach(modules.HeaderCanonical, modules.EncodeBlockNumber(blockFrom), func(k, v []byte) error {
		if deleteHeaders {
			deleteHeader(tx, types.BytesToHash(v), blockFrom)
		}
		return tx.Delete(modules.HeaderCanonical, k)
	}); err != nil {
		return fmt.Errorf("TruncateCanonicalHash: %w", err)
	}
	return nil
}

// IsCanonicalHash determines whether a header with the given hash is on the canonical chain.
func IsCanonicalHash(db kv.Getter, hash types.Hash) (bool, error) {
	number := ReadHeaderNumber(db, hash)
	if number == nil {
		return false, nil
	}
	canonicalHash, err := ReadCanonicalHash(db, *number)
	if err != nil {
		return false, err
	}
	return canonicalHash != (types.Hash{}) && canonicalHash == hash, nil
}

// ReadHeaderNumber returns the header number assigned to a hash.
func ReadHeaderNumber(db kv.Getter, hash types.Hash) *uint64 {
	data, err := db.GetOne(modules.HeaderNumber, hash.Bytes())
	if err != nil {
		log.Error("ReadHeaderNumber failed", "err", err)
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) != 8 {
		log.Error("ReadHeaderNumber got wrong data len", "len", len(data))
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// WriteHeaderNumber stores the hash->number mapping.
func WriteHeaderNumber(db kv.Putter, hash types.Hash, number uint64) error {
	if err := db.Put(modules.HeaderNumber, hash[:], modules.EncodeBlockNumber(number)); err != nil {
		return err
	}
	return nil
}

// DeleteHeaderNumber removes hash->number mapping.
func DeleteHeaderNumber(db kv.Deleter, hash types.Hash) {
	if err := db.Delete(modules.HeaderNumber, hash[:]); err != nil {
		log.Crit("Failed to delete hash mapping", "err", err)
	}
}

// ReadHeaderRAW retrieves a block header in its raw database encoding.
func ReadHeaderRAW(db kv.Getter, hash types.Hash, number uint64) []byte {
	data, err := db.GetOne(modules.Headers, modules.HeaderKey(number, hash))
	if err != nil {
		log.Error("ReadHeaderRAW failed", "err", err)
	}
	return data
}

// HasHeader verifies the existence of a block header corresponding to the hash.
func HasHeader(db kv.Has, hash types.Hash, number uint64) bool {
	if has, err := db.Has(modules.Headers, modules.HeaderKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadHeader(db kv.Getter, hash types.Hash, number uint64) *block.Header {
	data := ReadHeaderRAW(db, hash, number)
	if len(data) == 0 {
		return nil
	}

	header := new(block.Header)
	pbHeader := new(types_pb.Header)

	if err := proto.Unmarshal(data, pbHeader); err != nil {
		log.Error("Invalid block header RAW", "hash", hash, "err", err)
		return nil
	}

	if err := header.FromProtoMessage(pbHeader); err != nil {
		log.Error("header FromProtoMessage failed", "err", err)
		return nil
	}
	return header
}

//func ReadCurrentBlockNumber(db kv.Getter) *uint64 {
//	headHash := ReadHeadHeaderHash(db)
//	return ReadHeaderNumber(db, headHash)
//}
//
//func ReadCurrentHeader(db kv.Getter) *types.Header {
//	headHash := ReadHeadHeaderHash(db)
//	headNumber := ReadHeaderNumber(db, headHash)
//	if headNumber == nil {
//		return nil
//	}
//	return ReadHeader(db, headHash, *headNumber)
//}

//func ReadCurrentBlock(db kv.Tx) *types.Block {
//	headHash := ReadHeadBlockHash(db)
//	headNumber := ReadHeaderNumber(db, headHash)
//	if headNumber == nil {
//		return nil
//	}
//	return ReadBlock(db, headHash, *headNumber)
//}

//func ReadLastBlockSynced(db kv.Tx) (*types.Block, error) {
//	headNumber, err := stages.GetStageProgress(db, stages.Execution)
//	if err != nil {
//		return nil, err
//	}
//	headHash, err := ReadCanonicalHash(db, headNumber)
//	if err != nil {
//		return nil, err
//	}
//	return ReadBlock(db, headHash, headNumber), nil
//}

func ReadHeadersByNumber(db kv.Tx, number uint64) ([]*block.Header, error) {
	var res []*block.Header
	c, err := db.Cursor(modules.Headers)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	prefix := modules.EncodeBlockNumber(number)
	for k, v, err := c.Seek(prefix); k != nil; k, v, err = c.Next() {
		if err != nil {
			return nil, err
		}
		if !bytes.HasPrefix(k, prefix) {
			break
		}

		header := new(block.Header)
		pbHeader := new(types_pb.Header)
		if err := proto.Unmarshal(v, pbHeader); err != nil {
			return nil, fmt.Errorf("invalid block header RAW: hash=%x, err=%w", k[8:], err)
		}
		if err := header.FromProtoMessage(pbHeader); nil != err {
			return nil, fmt.Errorf("invalid block pbHeader: hash=%x, err =%w", k[8:], err)
		}
		res = append(res, header)
	}
	return res, nil
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-number mapping.
func WriteHeader(db kv.Putter, header *block.Header) {
	var (
		hash   = header.Hash()
		number = header.Number.Uint64()
	)

	if err := WriteHeaderNumber(db, hash, number); nil != err {
		log.Crit("Failed to store hash to number mapping", "err", err)
	}

	// Write the encoded header
	data, err := header.Marshal()
	if nil != err {
		log.Crit("failed to Marshal header", "err", err)
	}
	if err := db.Put(modules.Headers, modules.HeaderKey(number, hash), data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// deleteHeader - dangerous, use DeleteAncientBlocks/TruncateBlocks methods
func deleteHeader(db kv.Deleter, hash types.Hash, number uint64) {
	if err := db.Delete(modules.Headers, modules.HeaderKey(number, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(modules.HeaderNumber, hash.Bytes()); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

// ReadBodyRAW retrieves the block body (transactions and uncles) in encoding.
func ReadBodyRAW(db kv.Tx, hash types.Hash, number uint64) []byte {
	body := ReadCanonicalBodyWithTransactions(db, hash, number)
	pbBody := body.ToProtoMessage()

	bodyRaw, err := proto.Marshal(pbBody)
	if err != nil {
		log.Error("ReadBodyRAW failed", "err", err)
	}
	return bodyRaw
}

func ReadStorageBodyRAW(db kv.Getter, hash types.Hash, number uint64) []byte {
	bodyRaw, err := db.GetOne(modules.BlockBody, modules.BlockBodyKey(number, hash))
	if err != nil {
		log.Error("ReadBodyRAW failed", "err", err)
	}
	return bodyRaw
}

func ReadStorageBody(db kv.Getter, hash types.Hash, number uint64) (block.BodyForStorage, error) {
	bodyRaw, err := db.GetOne(modules.BlockBody, modules.BlockBodyKey(number, hash))
	if err != nil {
		log.Error("ReadBodyRAW failed", "err", err)
	}
	if len(bodyRaw) != 8+4 {
		return block.BodyForStorage{}, fmt.Errorf("invalid body raw")
	}
	bodyForStorage := new(block.BodyForStorage)
	bodyForStorage.BaseTxId = binary.BigEndian.Uint64(bodyRaw[:8])
	bodyForStorage.TxAmount = binary.BigEndian.Uint32(bodyRaw[8:])
	return *bodyForStorage, nil
}

func CanonicalTxnByID(db kv.Getter, id uint64) (*transaction.Transaction, error) {
	txIdKey := make([]byte, 8)
	binary.BigEndian.PutUint64(txIdKey, id)
	v, err := db.GetOne(modules.BlockTx, txIdKey)
	if err != nil {
		return nil, err
	}

	tx := new(transaction.Transaction)
	if err := tx.Unmarshal(v); nil != err {
		return nil, err
	}

	return tx, nil
}

func CanonicalTransactions(db kv.Getter, baseTxId uint64, amount uint32) ([]*transaction.Transaction, error) {
	if amount == 0 {
		return []*transaction.Transaction{}, nil
	}
	txIdKey := make([]byte, 8)
	txs := make([]*transaction.Transaction, amount)
	binary.BigEndian.PutUint64(txIdKey, baseTxId)
	i := uint32(0)

	if err := db.ForAmount(modules.BlockTx, txIdKey, amount, func(k, v []byte) error {
		var decodeErr error
		tx := new(transaction.Transaction)
		if decodeErr = tx.Unmarshal(v); nil != decodeErr {
			return decodeErr
		}
		txs[i] = tx
		i++
		return nil
	}); err != nil {
		return nil, err
	}
	txs = txs[:i] // user may request big "amount", but db can return small "amount". Return as much as we found.
	return txs, nil
}

func WriteTransactions(db kv.RwTx, txs []*transaction.Transaction, baseTxId uint64) error {
	txId := baseTxId
	for _, tx := range txs {
		txIdKey := make([]byte, 8)
		binary.BigEndian.PutUint64(txIdKey, txId)
		txId++
		var data []byte

		data, err := tx.Marshal()
		if err != nil {
			return err
		}

		//if _, err := tx.MarshalTo(data); nil != err {
		//	return fmt.Errorf("broken tx marshal: %w", err)
		//}

		// If next Append returns KeyExists error - it means you need to open transaction in App code before calling this func. Batch is also fine.
		if err := db.Append(modules.BlockTx, txIdKey, types.CopyBytes(data)); err != nil {
			return err
		}
	}
	return nil
}

func WriteRawTransactions(tx kv.RwTx, txs [][]byte, baseTxId uint64) error {
	txId := baseTxId
	for _, txn := range txs {
		txIdKey := make([]byte, 8)
		binary.BigEndian.PutUint64(txIdKey, txId)
		// If next Append returns KeyExists error - it means you need to open transaction in App code before calling this func. Batch is also fine.
		if err := tx.Append(modules.BlockTx, txIdKey, txn); err != nil {
			return fmt.Errorf("txId=%d, baseTxId=%d, %w", txId, baseTxId, err)
		}
		txId++
	}
	return nil
}

// WriteBodyForStorage stores an encoded block body into the database.
func WriteBodyForStorage(db kv.Putter, hash types.Hash, number uint64, body *block.BodyForStorage) error {
	v := modules.BodyStorageValue(body.BaseTxId, body.TxAmount)
	return db.Put(modules.BlockBody, modules.BlockBodyKey(number, hash), v)
}

// ReadBodyByNumber - returns canonical block body
func ReadBodyByNumber(db kv.Tx, number uint64) (*block.Body, uint64, uint32, error) {
	hash, err := ReadCanonicalHash(db, number)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed ReadCanonicalHash: %w", err)
	}
	if hash == (types.Hash{}) {
		return nil, 0, 0, nil
	}
	body, baseTxId, txAmount := ReadBody(db, hash, number)
	return body, baseTxId, txAmount, nil
}

func ReadBodyWithTransactions(db kv.Getter, hash types.Hash, number uint64) (*block.Body, error) {
	body, baseTxId, txAmount := ReadBody(db, hash, number)
	if body == nil {
		return nil, nil
	}
	var err error
	body.Txs, err = CanonicalTransactions(db, baseTxId, txAmount)
	if err != nil {
		return nil, err
	}
	return body, err
}

func ReadCanonicalBodyWithTransactions(db kv.Getter, hash types.Hash, number uint64) *block.Body {
	body, baseTxId, txAmount := ReadBody(db, hash, number)
	if body == nil {
		return nil
	}
	var err error
	body.Txs, err = CanonicalTransactions(db, baseTxId, txAmount)
	if err != nil {
		log.Error("failed ReadTransactionByHash", "hash", hash, "block", number, "err", err)
		return nil
	}

	verifies, err := ReadVerifies(db, hash, number)
	if nil != err {
		log.Error("read verifier failed", err)
		return nil
	}
	body.Verifiers = verifies

	rewards, err := ReadRewards(db, hash, number)
	if nil != err {
		log.Error("read reward failed", err)
		return nil
	}
	body.Rewards = rewards
	return body
}

func RawTransactionsRange(db kv.Getter, from, to uint64) (res [][]byte, err error) {
	blockKey := make([]byte, modules.NumberLength+types.HashLength)
	encNum := make([]byte, 8)
	for i := from; i < to+1; i++ {
		binary.BigEndian.PutUint64(encNum, i)
		hash, err := db.GetOne(modules.HeaderCanonical, encNum)
		if err != nil {
			return nil, err
		}
		if len(hash) == 0 {
			continue
		}

		binary.BigEndian.PutUint64(blockKey, i)
		copy(blockKey[modules.NumberLength:], hash)
		bodyRaw, err := db.GetOne(modules.BlockBody, blockKey)
		if err != nil {
			return nil, err
		}
		if len(bodyRaw) == 0 {
			continue
		}

		baseTxId := binary.BigEndian.Uint64(bodyRaw[:8])
		txAmount := binary.BigEndian.Uint32(bodyRaw[8:])

		binary.BigEndian.PutUint64(encNum, baseTxId)
		if err = db.ForAmount(modules.BlockTx, encNum, txAmount, func(k, v []byte) error {
			res = append(res, v)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return
}

func ReadBodyForStorageByKey(db kv.Getter, k []byte) (*block.BodyForStorage, error) {
	bodyRaw, err := db.GetOne(modules.BlockBody, k)
	if err != nil {
		return nil, err
	}
	if len(bodyRaw) == 0 {
		return nil, nil
	}
	bodyForStorage := new(block.BodyForStorage)
	bodyForStorage.BaseTxId = binary.BigEndian.Uint64(bodyRaw[:8])
	bodyForStorage.TxAmount = binary.BigEndian.Uint32(bodyRaw[8:])
	return bodyForStorage, nil
}

func ReadBody(db kv.Getter, hash types.Hash, number uint64) (*block.Body, uint64, uint32) {
	data := ReadStorageBodyRAW(db, hash, number)
	if len(data) == 0 {
		return nil, 0, 0
	}
	bodyForStorage := new(block.BodyForStorage)
	bodyForStorage.BaseTxId = binary.BigEndian.Uint64(data[:8])
	bodyForStorage.TxAmount = binary.BigEndian.Uint32(data[8:])

	body := new(block.Body)
	if bodyForStorage.TxAmount < 2 {
		panic(fmt.Sprintf("block body hash too few txs amount: %d, %d", number, bodyForStorage.TxAmount))
	}
	return body, bodyForStorage.BaseTxId + 1, bodyForStorage.TxAmount - 2 // 1 system txn in the begining of block, and 1 at the end
}

func ReadSenders(db kv.Getter, hash types.Hash, number uint64) ([]types.Address, error) {
	data, err := db.GetOne(modules.Senders, modules.BlockBodyKey(number, hash))
	if err != nil {
		return nil, fmt.Errorf("readSenders failed: %w", err)
	}
	senders := make([]types.Address, len(data)/types.AddressLength)
	for i := 0; i < len(senders); i++ {
		copy(senders[i][:], data[i*types.AddressLength:])
	}
	return senders, nil
}

func WriteRawBodyIfNotExists(db kv.RwTx, hash types.Hash, number uint64, body *block.RawBody) (ok bool, lastTxnNum uint64, err error) {
	exists, err := db.Has(modules.BlockBody, modules.BlockBodyKey(number, hash))
	if err != nil {
		return false, 0, err
	}
	if exists {
		return false, 0, nil
	}
	return WriteRawBody(db, hash, number, body)
}

func WriteRawBody(db kv.RwTx, hash types.Hash, number uint64, body *block.RawBody) (ok bool, lastTxnNum uint64, err error) {
	baseTxId, err := db.IncrementSequence(modules.BlockTx, uint64(len(body.Transactions))+2)
	if err != nil {
		return false, 0, err
	}
	data := block.BodyForStorage{
		BaseTxId: baseTxId,
		TxAmount: uint32(len(body.Transactions)) + 2,
	}
	if err = WriteBodyForStorage(db, hash, number, &data); err != nil {
		return false, 0, fmt.Errorf("WriteBodyForStorage: %w", err)
	}
	lastTxnNum = baseTxId + uint64(len(body.Transactions)) + 2
	if err = WriteRawTransactions(db, body.Transactions, baseTxId+1); err != nil {
		return false, 0, fmt.Errorf("WriteRawTransactions: %w", err)
	}
	return true, lastTxnNum, nil
}

func WriteBody(db kv.RwTx, hash types.Hash, number uint64, body *block.Body) error {
	// Pre-processing
	body.SendersFromTxs()
	baseTxId, err := db.IncrementSequence(modules.BlockTx, uint64(len(body.Txs))+2)
	if err != nil {
		return err
	}
	data := block.BodyForStorage{
		BaseTxId: baseTxId,
		TxAmount: uint32(len(body.Txs)) + 2,
	}
	if err := WriteBodyForStorage(db, hash, number, &data); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}
	err = WriteTransactions(db, body.Transactions(), baseTxId+1)
	if err != nil {
		return fmt.Errorf("failed to WriteTransactions: %w", err)
	}

	if len(body.Verifiers) > 0 {
		if err := WriteVerifies(db, hash, number, body.Verifiers); nil != err {
			return err
		}
	}
	if len(body.Rewards) > 0 {
		if err := WriteRewards(db, hash, number, body.Rewards); nil != err {
			return err
		}
	}

	return nil
}

func ReadVerifies(db kv.Getter, hash types.Hash, number uint64) ([]*block.Verify, error) {
	data, err := db.GetOne(modules.BlockVerify, modules.BlockBodyKey(number, hash))
	if err != nil {
		return nil, fmt.Errorf("ReadVerifies failed: %w", err)
	}
	verifies := make([]*block.Verify, len(data)/(types.PublicKeyLength+types.AddressLength))
	for i := 0; i < len(verifies); i++ {
		verifies[i] = &block.Verify{
			Address:   types.Address{},
			PublicKey: types.PublicKey{},
		}
		copy(verifies[i].PublicKey[:], data[i*(types.PublicKeyLength+types.AddressLength):])
		copy(verifies[i].Address[:], data[i*(types.PublicKeyLength+types.AddressLength)+types.PublicKeyLength:])
	}
	return verifies, nil
}

func WriteVerifies(db kv.Putter, hash types.Hash, number uint64, verifies []*block.Verify) error {
	data := make([]byte, (types.AddressLength+types.PublicKeyLength)*len(verifies))
	for i, v := range verifies {
		copy(data[i*(types.AddressLength+types.PublicKeyLength):], v.PublicKey[:])
		copy(data[i*(types.AddressLength+types.PublicKeyLength)+types.PublicKeyLength:], v.Address[:])
	}
	if err := db.Put(modules.BlockVerify, modules.BlockBodyKey(number, hash), data); err != nil {
		return fmt.Errorf("failed to store block senders: %w", err)
	}
	return nil
}

func ReadRewards(db kv.Getter, hash types.Hash, number uint64) ([]*block.Reward, error) {
	data, err := db.GetOne(modules.BlockRewards, modules.BlockBodyKey(number, hash))
	if err != nil {
		return nil, fmt.Errorf("ReadBlockRewards failed: %w", err)
	}

	reward := make([]*block.Reward, len(data)/(32+types.AddressLength))
	for i := 0; i < len(reward); i++ {
		reward[i] = &block.Reward{
			Address: *new(types.Address).SetBytes(data[i*(32+types.AddressLength) : i*(32+types.AddressLength)+types.AddressLength]),
			Amount:  new(uint256.Int).SetBytes(data[i*(32+types.AddressLength)+types.AddressLength : i*(32+types.AddressLength)+types.AddressLength+32]),
		}
	}
	return reward, nil
}

func WriteRewards(db kv.Putter, hash types.Hash, number uint64, rewards []*block.Reward) error {

	data := make([]byte, (types.AddressLength+32)*len(rewards))
	for i, reward := range rewards {
		byte32 := reward.Amount.Bytes32()
		copy(data[i*(types.AddressLength+32):], reward.Address[:])
		copy(data[i*(types.AddressLength+32)+types.AddressLength:], byte32[:])
	}
	if err := db.Put(modules.BlockRewards, modules.BlockBodyKey(number, hash), data); err != nil {
		return fmt.Errorf("failed to store block rewards: %w", err)
	}
	return nil
}

// deleteBody removes all block body data associated with a hash.
func deleteBody(db kv.Deleter, hash types.Hash, number uint64) {
	if err := db.Delete(modules.BlockBody, modules.BlockBodyKey(number, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// ReadTd retrieves a block's total difficulty corresponding to the hash.
func ReadTd(db kv.Getter, hash types.Hash, number uint64) (*uint256.Int, error) {
	data, err := db.GetOne(modules.HeaderTD, modules.HeaderKey(number, hash))
	if err != nil {
		return nil, fmt.Errorf("failed ReadTd: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	td := uint256.NewInt(0).SetBytes(data)
	log.Trace("readTD", "hash", hash, "number", number, "td", td.Uint64())
	return td, nil
}

func ReadTdByHash(db kv.Getter, hash types.Hash) (*uint256.Int, error) {
	headNumber := ReadHeaderNumber(db, hash)
	if headNumber == nil {
		return nil, nil
	}
	return ReadTd(db, hash, *headNumber)
}

// WriteTd stores the total difficulty of a block into the database.
func WriteTd(db kv.Putter, hash types.Hash, number uint64, td *uint256.Int) error {
	data := td.Bytes()
	if err := db.Put(modules.HeaderTD, modules.HeaderKey(number, hash), data); err != nil {
		return fmt.Errorf("failed to store block total difficulty: %w", err)
	}
	return nil
}

// TruncateTd removes all block total difficulty from block number N
func TruncateTd(tx kv.RwTx, blockFrom uint64) error {
	if err := tx.ForEach(modules.HeaderTD, modules.EncodeBlockNumber(blockFrom), func(k, _ []byte) error {
		return tx.Delete(modules.HeaderTD, k)
	}); err != nil {
		return fmt.Errorf("TruncateTd: %w", err)
	}
	return nil
}

// HasReceipts verifies the existence of all the transaction receipts belonging
// to a block.
func HasReceipts(db kv.Has, number uint64) bool {
	if has, err := db.Has(modules.Receipts, modules.EncodeBlockNumber(number)); !has || err != nil {
		return false
	}
	return true
}

// ReadRawReceipts retrieves all the transaction receipts belonging to a block.
// The receipt metadata fields are not guaranteed to be populated, so they
// should not be used. Use ReadReceipts instead if the metadata is needed.
func ReadRawReceipts(db kv.Tx, blockNum uint64) block.Receipts {
	// Retrieve the flattened receipt slice
	data, err := db.GetOne(modules.Receipts, modules.EncodeBlockNumber(blockNum))
	if err != nil {
		log.Error("ReadRawReceipts failed", "err", err)
	}
	if len(data) == 0 {
		return nil
	}
	var receipts block.Receipts

	if err := receipts.Unmarshal(data); nil != err {
		log.Error("ReadRawReceipts failed", "err", err)
		return nil
	}

	//prefix := make([]byte, 8)
	//binary.BigEndian.PutUint64(prefix, blockNum)
	//if err := db.ForPrefix(Log, prefix, func(k, v []byte) error {
	//	var logs block.Logs
	//	if err := logs.Unmarshal(v); err != nil {
	//		return fmt.Errorf("receipt unmarshal failed:  %w", err)
	//	}
	//
	//	receipts[binary.BigEndian.Uint32(k[8:])].Logs = logs
	//	return nil
	//}); err != nil {
	//	log.Error("logs fetching failed", "err", err)
	//	return nil
	//}

	return receipts
}

// ReadReceipts retrieves all the transaction receipts belonging to a block, including
// its corresponding metadata fields. If it is unable to populate these metadata
// fields then nil is returned.
//
// The current implementation populates these metadata fields by reading the receipts'
// corresponding block body, so if the block body is not found it will return nil even
// if the receipt itself is stored.
func ReadReceipts(db kv.Tx, block *block.Block, senders []types.Address) block.Receipts {
	if block == nil {
		return nil
	}
	// We're deriving many fields from the block body, retrieve beside the receipt
	receipts := ReadRawReceipts(db, block.Number64().Uint64())
	if receipts == nil {
		return nil
	}
	if len(senders) > 0 {
		block.SendersToTxs(senders)
	}
	//if err := receipts.DeriveFields(block.Hash(), block.NumberU64(), block.Transactions(), senders); err != nil {
	//	log.Error("Failed to derive block receipts fields", "hash", block.Hash(), "number", block.NumberU64(), "err", err, "stack", dbg.Stack())
	//	return nil
	//}
	return receipts
}

func ReadReceiptsByHash(db kv.Tx, hash types.Hash) (block.Receipts, error) {
	number := ReadHeaderNumber(db, hash)
	if number == nil {
		return nil, nil
	}
	canonicalHash, err := ReadCanonicalHash(db, *number)
	if err != nil {
		return nil, fmt.Errorf("requested non-canonical hash %x. canonical=%x", hash, canonicalHash)
	}
	b, s, err := ReadBlockWithSenders(db, hash, *number)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	receipts := ReadReceipts(db, b, s)
	if receipts == nil {
		return nil, nil
	}
	return receipts, nil
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func WriteReceipts(tx kv.Putter, number uint64, receipts block.Receipts) error {
	for txId, r := range receipts {
		if len(r.Logs) == 0 {
			continue
		}
		var logs block.Logs
		logs = r.Logs
		v, err := logs.Marshal()
		if err != nil {
			return fmt.Errorf("encode block logs for block %d: %w", number, err)
		}

		if err = tx.Put(modules.Log, modules.LogKey(number, uint32(txId)), v); err != nil {
			return fmt.Errorf("writing logs for block %d: %w", number, err)
		}
	}

	v, err := receipts.Marshal()
	if err != nil {
		return fmt.Errorf("encode block receipts for block %d: %w", number, err)
	}

	if err = tx.Put(modules.Receipts, modules.EncodeBlockNumber(number), v); err != nil {
		return fmt.Errorf("writing receipts for block %d: %w", number, err)
	}
	return nil
}

// AppendReceipts stores all the transaction receipts belonging to a block.
func AppendReceipts(tx kv.StatelessWriteTx, blockNumber uint64, receipts block.Receipts) error {
	for txId, r := range receipts {
		if len(r.Logs) == 0 {
			continue
		}

		var logs block.Logs
		logs = r.Logs
		v, err := logs.Marshal()
		if nil != err {
			return err
		}

		if err = tx.Append(modules.Log, modules.LogKey(blockNumber, uint32(txId)), v); err != nil {
			return fmt.Errorf("writing receipts for block %d: %w", blockNumber, err)
		}
	}

	rv, err := receipts.Marshal()
	if err != nil {
		return fmt.Errorf("encode block receipts for block %d: %w", blockNumber, err)
	}

	if err = tx.Append(modules.Receipts, modules.EncodeBlockNumber(blockNumber), rv); err != nil {
		return fmt.Errorf("writing receipts for block %d: %w", blockNumber, err)
	}
	return nil
}

// TruncateReceipts removes all receipt for given block number or newer
func TruncateReceipts(db kv.RwTx, number uint64) error {
	if err := db.ForEach(modules.Receipts, modules.EncodeBlockNumber(number), func(k, _ []byte) error {
		return db.Delete(modules.Receipts, k)
	}); err != nil {
		return err
	}

	from := make([]byte, 8)
	binary.BigEndian.PutUint64(from, number)
	if err := db.ForEach(modules.Log, from, func(k, _ []byte) error {
		return db.Delete(modules.Log, k)
	}); err != nil {
		return err
	}
	return nil
}

func ReceiptsAvailableFrom(tx kv.Tx) (uint64, error) {
	c, err := tx.Cursor(modules.Receipts)
	if err != nil {
		return math.MaxUint64, err
	}
	defer c.Close()
	k, _, err := c.First()
	if err != nil {
		return math.MaxUint64, err
	}
	if len(k) == 0 {
		return math.MaxUint64, nil
	}
	return binary.BigEndian.Uint64(k), nil
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func ReadBlock(tx kv.Getter, hash types.Hash, number uint64) *block.Block {
	header := ReadHeader(tx, hash, number)
	if header == nil {
		return nil
	}
	body := ReadCanonicalBodyWithTransactions(tx, hash, number)
	if body == nil {
		return nil
	}
	return block.NewBlockFromStorage(hash, header, body)
}

// HasBlock - is more efficient than ReadBlock because doesn't read transactions.
// It's is not equivalent of HasHeader because headers and bodies written by different stages
func HasBlock(db kv.Getter, hash types.Hash, number uint64) bool {
	body := ReadStorageBodyRAW(db, hash, number)
	return len(body) > 0
}

func ReadBlockWithSenders(db kv.Getter, hash types.Hash, number uint64) (*block.Block, []types.Address, error) {
	block := ReadBlock(db, hash, number)
	if block == nil {
		return nil, nil, nil
	}
	senders, err := ReadSenders(db, hash, number)
	if err != nil {
		return nil, nil, err
	}
	if len(senders) != len(block.Transactions()) {
		return block, senders, nil // no senders is fine - will recover them on the fly
	}
	block.SendersToTxs(senders)
	return block, senders, nil
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteBlock(db kv.RwTx, b *block.Block) error {
	iBody := b.Body()
	if err := WriteBody(db, b.Hash(), b.Number64().Uint64(), iBody.(*block.Body)); err != nil {
		return err
	}
	iHeader := b.Header()
	header, ok := iHeader.(*block.Header)
	if !ok {
		return fmt.Errorf("illegal: assert header")
	}
	WriteHeader(db, header)
	return nil
}

// DeleteAncientBlocks - delete [1, to) old blocks after moving it to snapshots.
// keeps genesis in db: [1, to)
// doesn't change sequences of kv.EthTx and kv.NonCanonicalTxs
// doesn't delete Receipts, Senders, Canonical markers, TotalDifficulty
// returns [deletedFrom, deletedTo)
//func DeleteAncientBlocks(tx kv.RwTx, blockTo uint64, blocksDeleteLimit int) (deletedFrom, deletedTo uint64, err error) {
//	c, err := tx.Cursor(Headers)
//	if err != nil {
//		return
//	}
//	defer c.Close()
//
//	// find first non-genesis block
//	firstK, _, err := c.Seek(dbutils.EncodeBlockNumber(1))
//	if err != nil {
//		return
//	}
//	if firstK == nil { //nothing to delete
//		return
//	}
//	blockFrom := binary.BigEndian.Uint64(firstK)
//	stopAtBlock := libcommon.Min(blockTo, blockFrom+uint64(blocksDeleteLimit))
//	k, _, _ := c.Current()
//	deletedFrom = binary.BigEndian.Uint64(k)
//
//	var canonicalHash types.Hash
//	var b *types.BodyForStorage
//
//	for k, _, err = c.Current(); k != nil; k, _, err = c.Next() {
//		if err != nil {
//			return
//		}
//
//		n := binary.BigEndian.Uint64(k)
//		if n >= stopAtBlock { // [from, to)
//			break
//		}
//
//		canonicalHash, err = ReadCanonicalHash(tx, n)
//		if err != nil {
//			return
//		}
//		isCanonical := bytes.Equal(k[8:], canonicalHash[:])
//
//		b, err = ReadBodyForStorageByKey(tx, k)
//		if err != nil {
//			return
//		}
//		if b == nil {
//			log.Debug("DeleteAncientBlocks: block body not found", "height", n)
//		} else {
//			txIDBytes := make([]byte, 8)
//			for txID := b.BaseTxId; txID < b.BaseTxId+uint64(b.TxAmount); txID++ {
//				binary.BigEndian.PutUint64(txIDBytes, txID)
//				bucket := kv.EthTx
//				if !isCanonical {
//					bucket = kv.NonCanonicalTxs
//				}
//				if err = tx.Delete(bucket, txIDBytes); err != nil {
//					return
//				}
//			}
//		}
//		// Copying k because otherwise the same memory will be reused
//		// for the next key and Delete below will end up deleting 1 more record than required
//		kCopy := common.CopyBytes(k)
//		if err = tx.Delete(kv.Headers, kCopy); err != nil {
//			return
//		}
//		if err = tx.Delete(kv.BlockBody, kCopy); err != nil {
//			return
//		}
//	}
//
//	k, _, _ = c.Current()
//	deletedTo = binary.BigEndian.Uint64(k)
//
//	return
//}

// LastKey - candidate on move to kv.Tx interface
func LastKey(tx kv.Tx, table string) ([]byte, error) {
	c, err := tx.Cursor(table)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	k, _, err := c.Last()
	if err != nil {
		return nil, err
	}
	return k, nil
}

// FirstKey - candidate on move to kv.Tx interface
func FirstKey(tx kv.Tx, table string) ([]byte, error) {
	c, err := tx.Cursor(table)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	k, _, err := c.First()
	if err != nil {
		return nil, err
	}
	return k, nil
}

// SecondKey - useful if table always has zero-key (for example genesis block)
func SecondKey(tx kv.Tx, table string) ([]byte, error) {
	c, err := tx.Cursor(table)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	_, _, err = c.First()
	if err != nil {
		return nil, err
	}
	k, _, err := c.Next()
	if err != nil {
		return nil, err
	}
	return k, nil
}

// TruncateBlocks - delete block >= blockFrom
// does decrement sequences of kv.EthTx and kv.NonCanonicalTxs
// doesn't delete Receipts, Senders, Canonical markers, TotalDifficulty
func TruncateBlocks(ctx context.Context, tx kv.RwTx, blockFrom uint64) error {
	logEvery := time.NewTicker(20 * time.Second)
	defer logEvery.Stop()

	c, err := tx.Cursor(modules.Headers)
	if err != nil {
		return err
	}
	defer c.Close()
	if blockFrom < 1 { //protect genesis
		blockFrom = 1
	}
	sequenceTo := map[string]uint64{}
	for k, _, err := c.Last(); k != nil; k, _, err = c.Prev() {
		if err != nil {
			return err
		}
		n := binary.BigEndian.Uint64(k)
		if n < blockFrom { // [from, to)
			break
		}
		//canonicalHash, err := ReadCanonicalHash(tx, n)
		//if err != nil {
		//	return err
		//}
		//isCanonical := bytes.Equal(k[8:], canonicalHash[:])

		b, err := ReadBodyForStorageByKey(tx, k)
		if err != nil {
			return err
		}
		if b != nil {
			bucket := modules.BlockTx
			if err := tx.ForEach(bucket, modules.EncodeBlockNumber(b.BaseTxId), func(k, _ []byte) error {
				if err := tx.Delete(bucket, k); err != nil {
					return err
				}
				return nil
			}); err != nil {
				return err
			}
			sequenceTo[bucket] = b.BaseTxId
		}
		// Copying k because otherwise the same memory will be reused
		// for the next key and Delete below will end up deleting 1 more record than required
		kCopy := types.CopyBytes(k)
		if err := tx.Delete(modules.Headers, kCopy); err != nil {
			return err
		}
		if err := tx.Delete(modules.BlockBody, kCopy); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-logEvery.C:
			log.Info("TruncateBlocks", "block", n)
		default:
		}
	}
	return nil
}

func ReadBlockByNumber(db kv.Getter, number uint64) (*block.Block, error) {
	hash, err := ReadCanonicalHash(db, number)
	if err != nil {
		return nil, fmt.Errorf("failed ReadCanonicalHash: %w", err)
	}
	if hash == (types.Hash{}) {
		return nil, nil
	}

	return ReadBlock(db, hash, number), nil
}

func CanonicalBlockByNumberWithSenders(db kv.Tx, number uint64) (*block.Block, []types.Address, error) {
	hash, err := ReadCanonicalHash(db, number)
	if err != nil {
		return nil, nil, fmt.Errorf("failed ReadCanonicalHash: %w", err)
	}
	if hash == (types.Hash{}) {
		return nil, nil, nil
	}

	return ReadBlockWithSenders(db, hash, number)
}

func ReadBlockByHash(db kv.Tx, hash types.Hash) (*block.Block, error) {
	number := ReadHeaderNumber(db, hash)
	if number == nil {
		return nil, nil
	}
	return ReadBlock(db, hash, *number), nil
}

func ReadHeaderByNumber(db kv.Getter, number uint64) *block.Header {
	hash, err := ReadCanonicalHash(db, number)
	if err != nil {
		log.Error("ReadCanonicalHash failed", "err", err)
		return nil
	}
	if hash == (types.Hash{}) {
		return nil
	}

	return ReadHeader(db, hash, number)
}

func ReadHeaderByHash(db kv.Getter, hash types.Hash) (*block.Header, error) {
	number := ReadHeaderNumber(db, hash)
	if number == nil {
		return nil, nil
	}
	return ReadHeader(db, hash, *number), nil
}

//func ReadAncestor(db kv.Getter, hash types.Hash, number, ancestor uint64, maxNonCanonical *uint64, blockReader services.HeaderAndCanonicalReader) (types.Hash, uint64) {
//	if ancestor > number {
//		return types.Hash{}, 0
//	}
//	if ancestor == 1 {
//		header, err := blockReader.Header(context.Background(), db, hash, number)
//		if err != nil {
//			panic(err)
//		}
//		// in this case it is cheaper to just read the header
//		if header != nil {
//			return header.ParentHash, number - 1
//		}
//		return types.Hash{}, 0
//	}
//	for ancestor != 0 {
//		h, err := blockReader.CanonicalHash(context.Background(), db, number)
//		if err != nil {
//			panic(err)
//		}
//		if h == hash {
//			ancestorHash, err := blockReader.CanonicalHash(context.Background(), db, number-ancestor)
//			if err != nil {
//				panic(err)
//			}
//			h, err := blockReader.CanonicalHash(context.Background(), db, number)
//			if err != nil {
//				panic(err)
//			}
//			if h == hash {
//				number -= ancestor
//				return ancestorHash, number
//			}
//		}
//		if *maxNonCanonical == 0 {
//			return types.Hash{}, 0
//		}
//		*maxNonCanonical--
//		ancestor--
//		header, err := blockReader.Header(context.Background(), db, hash, number)
//		if err != nil {
//			panic(err)
//		}
//		if header == nil {
//			return types.Hash{}, 0
//		}
//		hash = header.ParentHash
//		number--
//	}
//	return hash, number
//}

// PruneTable has `limit` parameter to avoid too large data deletes per one sync cycle - better delete by small portions to reduce db.FreeList size
func PruneTable(tx kv.RwTx, table string, pruneTo uint64, ctx context.Context, limit int) error {
	c, err := tx.RwCursor(table)

	if err != nil {
		return fmt.Errorf("failed to create cursor for pruning %w", err)
	}
	defer c.Close()

	i := 0
	for k, _, err := c.First(); k != nil; k, _, err = c.Next() {
		if err != nil {
			return err
		}
		i++
		if i > limit {
			break
		}

		blockNum := binary.BigEndian.Uint64(k)
		if blockNum >= pruneTo {
			break
		}
		select {
		case <-ctx.Done():
			return common2.ErrStopped
		default:
		}
		if err = c.DeleteCurrent(); err != nil {
			return fmt.Errorf("failed to remove for block %d: %w", blockNum, err)
		}
	}
	return nil
}

func PruneTableDupSort(tx kv.RwTx, table string, logPrefix string, pruneTo uint64, logEvery *time.Ticker, ctx context.Context) error {
	c, err := tx.RwCursorDupSort(table)
	if err != nil {
		return fmt.Errorf("failed to create cursor for pruning %w", err)
	}
	defer c.Close()

	for k, _, err := c.First(); k != nil; k, _, err = c.NextNoDup() {
		if err != nil {
			return fmt.Errorf("failed to move %s cleanup cursor: %w", table, err)
		}
		blockNum := binary.BigEndian.Uint64(k)
		if blockNum >= pruneTo {
			break
		}
		select {
		case <-logEvery.C:
			log.Info(fmt.Sprintf("[%s]", logPrefix), "table", table, "block", blockNum)
		case <-ctx.Done():
			return common2.ErrStopped
		default:
		}
		if err = c.DeleteCurrentDuplicates(); err != nil {
			return fmt.Errorf("failed to remove for block %d: %w", blockNum, err)
		}
	}
	return nil
}

func ReadCurrentBlockNumber(db kv.Getter) *uint64 {
	headHash := ReadHeadHeaderHash(db)
	return ReadHeaderNumber(db, headHash)
}

func ReadCurrentHeader(db kv.Getter) *block.Header {
	headHash := ReadHeadHeaderHash(db)
	headNumber := ReadHeaderNumber(db, headHash)
	if headNumber == nil {
		return nil
	}
	return ReadHeader(db, headHash, *headNumber)
}

func ReadCurrentBlock(db kv.Tx) *block.Block {
	headHash := ReadHeadBlockHash(db)
	headNumber := ReadHeaderNumber(db, headHash)
	if headNumber == nil {
		return nil
	}
	return ReadBlock(db, headHash, *headNumber)
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadHeadHeaderHash(db kv.Getter) types.Hash {
	data, err := db.GetOne(modules.HeadHeaderKey, []byte(modules.HeadHeaderKey))
	if err != nil {
		log.Error("ReadHeadHeaderHash failed", "err", err)
	}
	if len(data) == 0 {
		return types.Hash{}
	}
	return types.BytesToHash(data)
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db kv.Getter) types.Hash {
	data, err := db.GetOne(modules.HeadBlockKey, []byte(modules.HeadBlockKey))
	if err != nil {
		log.Error("ReadHeadBlockHash failed", "err", err)
	}
	if len(data) == 0 {
		return types.Hash{}
	}
	return types.BytesToHash(data)
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db kv.Putter, hash types.Hash) {
	if err := db.Put(modules.HeadBlockKey, []byte(modules.HeadBlockKey), hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func WriteHeadHeaderHash(db kv.Putter, hash types.Hash) error {
	if err := db.Put(modules.HeadHeaderKey, []byte(modules.HeadHeaderKey), hash.Bytes()); err != nil {
		return fmt.Errorf("failed to store last header's hash: %w", err)
	}
	return nil
}

func GetPoaSnapshot(db kv.Getter, hash types.Hash) ([]byte, error) {

	return db.GetOne(modules.PoaSnapshot, hash.Bytes())
}

func StorePoaSnapshot(db kv.Putter, hash types.Hash, data []byte) error {

	return db.Put(modules.PoaSnapshot, hash.Bytes(), data)
}

func StoreSigners(db kv.Putter, data []byte) error {
	return db.Put(modules.SignersDB, []byte(modules.SignersDB), data)
}
