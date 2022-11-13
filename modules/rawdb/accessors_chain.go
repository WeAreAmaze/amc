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

package rawdb

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/gogo/protobuf/proto"
)

const NumberLength = 8

func GetGenesis(db db.IDatabase) (block.IBlock, error) {
	gHash, err := ReadCanonicalHash(db, 0)
	if nil != err {
		return nil, err
	}

	header, _, err := GetHeader(db, gHash)
	if err != nil {
		return nil, err
	}
	body, err := GetBody(db, gHash)

	if err != nil {
		return nil, err
	}

	genesisBlock := block.NewBlock(header, body.Transactions())

	return genesisBlock, nil
}

func StoreGenesis(db db.IDatabase, genesisBlock block.IBlock) error {
	// write g td
	gtd := genesisBlock.Difficulty()
	if err := WriteTd(db, genesisBlock.Hash(), gtd); nil != err {
		return err
	}
	// write hash
	if err := WriteCanonicalHash(db, genesisBlock.Hash(), genesisBlock.Number64().Uint64()); nil != err {
		return err
	}
	return StoreBlock(db, genesisBlock)
}

func GetHeader(db db.IDatabase, hash types.Hash) (block.IHeader, types.Hash, error) {

	r, err := db.OpenReader(headerDB)
	if err != nil {
		return nil, types.Hash{}, err
	}

	v, err := r.Get(hash.Bytes())
	if err != nil {
		return nil, types.Hash{}, err
	}

	var (
		header   block.Header
		pbHeader types_pb.PBHeader
	)

	if err := proto.Unmarshal(v, &pbHeader); err != nil {
		return nil, types.Hash{}, err
	}

	if err := header.FromProtoMessage(&pbHeader); err != nil {
		return nil, types.Hash{}, err
	}

	return &header, header.Hash(), nil
}

func StoreHeader(db db.IDatabase, iheader block.IHeader) error {
	header, ok := iheader.(*block.Header)
	if !ok {
		return fmt.Errorf("failed header types")
	}

	v, err := header.Marshal()
	if err != nil {
		return err
	}

	h := header.Hash()

	w, err := db.OpenWriter(headerDB)
	if err != nil {
		return err
	}

	return w.Put(h[:], v)
}

func GetHeaderByHash(db db.IDatabase, hash types.Hash) (block.IHeader, types.Hash, error) {
	r, err := db.OpenReader(headerDB)
	if err != nil {
		return nil, types.Hash{}, err
	}

	v, err := r.Get(hash[:])
	if err != nil {
		return nil, types.Hash{}, err
	}

	var (
		header   block.Header
		pbHeader types_pb.PBHeader
	)

	if err := proto.Unmarshal(v, &pbHeader); err != nil {
		return nil, types.Hash{}, err
	}

	if err := header.FromProtoMessage(&pbHeader); err != nil {
		return nil, types.Hash{}, err
	}

	return &header, header.Hash(), nil
}

func SaveBlocks(db db.IDatabase, blocks []block.IBlock) (int, error) {

	// todo  batch?
	for i, block := range blocks {
		if err := StoreBlock(db, block); err != nil {
			return i + 1, err
		}
	}

	return 0, nil
}

func StoreBlock(db db.IDatabase, block block.IBlock) error {

	if err := StoreHeader(db, block.Header()); err != nil {
		return err
	}

	if err := StoreBody(db, block.Hash(), block.Body()); err != nil {
		return err
	}

	if err := StoreHashNumber(db, block.Header().Hash(), block.Number64()); err != nil {
		return err
	}
	// index
	for _, tx := range block.Transactions() {
		hash, _ := tx.Hash()
		if err := StoreTransactionIndex(db, block.Number64(), hash); err != nil {
			return err
		}
	}
	return nil
}

func GetBody(db db.IDatabase, hash types.Hash) (block.IBody, error) {

	r, err := db.OpenReader(bodyDB)
	if err != nil {
		return nil, err
	}

	v, err := r.Get(hash.Bytes())
	if err != nil {
		return nil, err
	}

	var (
		pBody types_pb.PBody
		body  block.Body
	)
	if err := proto.Unmarshal(v, &pBody); err != nil {
		return nil, err
	}

	err = body.FromProtoMessage(&pBody)

	return &body, err
}

func StoreBody(db db.IDatabase, hash types.Hash, body block.IBody) error {

	if body == nil {
		return nil
	}

	pbBody := body.ToProtoMessage()

	v, err := proto.Marshal(pbBody)
	if err != nil {
		return err
	}

	w, err := db.OpenWriter(bodyDB)
	if err != nil {
		return err
	}

	return w.Put(hash.Bytes(), v)
}

func GetLatestBlock(db db.IDatabase) (block.IBlock, error) {
	key := types.StringToHash(latestBlock)
	r, err := db.OpenReader(latestStateDB)
	if err != nil {
		return nil, err
	}
	v, err := r.Get(key[:])

	if err != nil {
		return nil, err
	}
	var (
		pBlock types_pb.PBlock
		block  block.Block
	)
	if err := proto.Unmarshal(v, &pBlock); err != nil {
		return nil, err
	}
	if err := block.FromProtoMessage(&pBlock); err != nil {
		return nil, err
	}

	return &block, nil
}

func SaveLatestBlock(db db.IDatabase, block block.IBlock) error {
	key := types.StringToHash(latestBlock)
	b, err := proto.Marshal(block.ToProtoMessage())
	if err != nil {
		return err
	}

	w, err := db.OpenWriter(latestStateDB)
	if err != nil {
		return err
	}

	return w.Put(key[:], b)
}

// GetTransaction get tx by txHash
func GetTransaction(db db.IDatabase, txHash types.Hash) (*transaction.Transaction, types.Hash, types.Int256, uint64, error) {

	blockNumber, err := GetTransactionIndex(db, txHash)
	if err != nil {
		return nil, types.Hash{}, types.NewInt64(0), 0, nil
	}

	hash, err := ReadCanonicalHash(db, blockNumber.Uint64())
	if nil != err {
		return nil, types.Hash{}, types.NewInt64(0), 0, err
	}
	body, err := GetBody(db, hash)
	if err != nil {
		return nil, types.Hash{}, types.NewInt64(0), 0, nil
	}
	_, headerHash, err := GetHeader(db, hash)
	if err != nil {
		return nil, types.Hash{}, types.NewInt64(0), 0, nil
	}
	for txIndex, tx := range body.Transactions() {
		hash, _ := tx.Hash()
		if hash == txHash {
			return tx, headerHash, blockNumber, uint64(txIndex), nil
		}
	}

	return nil, types.Hash{}, types.NewInt64(0), 0, nil
}

// StoreTransactionIndex store txHash blockNumber index
func StoreTransactionIndex(db db.IDatabase, blockNumber types.Int256, txHash types.Hash) error {
	v, err := blockNumber.MarshalText()
	if err != nil {
		return err
	}
	r, err := db.OpenWriter(transactionIndex)
	if err != nil {
		return err
	}

	return r.Put(txHash.HexBytes(), v)
}

func DeleteTransactionIndex(db db.IDatabase, txHash types.Hash) error {
	r, err := db.OpenWriter(transactionIndex)
	if err != nil {
		return err
	}
	return r.Delete(txHash.Bytes())
}

// GetTransactionIndex get block number by txHash
func GetTransactionIndex(db db.IDatabase, txHash types.Hash) (types.Int256, error) {

	r, err := db.OpenReader(transactionIndex)
	if err != nil {
		return types.NewInt64(0), err
	}

	v, err := r.Get(txHash.HexBytes())

	if err != nil {
		return types.NewInt64(0), err
	}

	int256, err := types.FromHex(string(v))

	if err != nil {
		return types.NewInt64(0), err
	}
	return int256, nil
}

// StoreHashNumber store hash to number index
func StoreHashNumber(db db.IDatabase, hash types.Hash, number types.Int256) error {

	v, err := number.MarshalText()
	if err != nil {
		return err
	}
	r, err := db.OpenWriter(hashNumberDB)
	if err != nil {
		return err
	}

	return r.Put(hash.HexBytes(), v)
}

// GetHashNumber get hash to number index
func GetHashNumber(db db.IDatabase, hash types.Hash) (types.Int256, error) {

	r, err := db.OpenReader(hashNumberDB)
	if err != nil {
		return types.NewInt64(0), err
	}

	v, err := r.Get(hash.HexBytes())
	if err != nil {
		return types.NewInt64(0), err
	}

	int256, err := types.FromHex(string(v))
	if err != nil {
		return types.NewInt64(0), err
	}

	return int256, nil
}

func GetReceipts(db db.IDatabase, hash types.Hash) (block.Receipts, error) {

	r, err := db.OpenReader(receiptsDB)
	if err != nil {
		return nil, err
	}

	v, err := r.Get(hash.HexBytes())
	if err != nil {
		return nil, err
	}

	var (
		pReceipts types_pb.Receipts
		receipts  block.Receipts
	)
	if err := proto.Unmarshal(v, &pReceipts); err != nil {
		return nil, err
	}

	err = receipts.FromProtoMessage(&pReceipts)

	return receipts, err
}

func StoreReceipts(db db.IDatabase, hash types.Hash, receipts block.Receipts) error {
	if receipts == nil {
		return nil
	}

	pReceipts := receipts.ToProtoMessage()

	v, err := proto.Marshal(pReceipts)
	if err != nil {
		return err
	}

	w, err := db.OpenWriter(receiptsDB)
	if err != nil {
		return err
	}

	return w.Put(hash.HexBytes(), v)
}

func WriteTD(db db.IDatabase, hash types.Hash, td types.Int256) error {
	return nil
}

func DeleteTD(db db.IDatabase, hash types.Hash) {
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db db.IDatabase, number uint64) (types.Hash, error) {
	r, err := db.OpenReader(HeaderCanonical)
	if err != nil {
		return types.Hash{}, err
	}

	data, err := r.Get(EncodeBlockNumber(number))
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed ReadCanonicalHash: %w, number=%d", err, number)
	}
	if len(data) == 0 {
		return types.Hash{}, nil
	}
	var hash types.Hash
	copy(hash[:], data[:])
	return hash, nil
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db db.IDatabase, hash types.Hash, number uint64) error {
	w, err := db.OpenWriter(HeaderCanonical)
	if err != nil {
		return err
	}

	return w.Put(EncodeBlockNumber(number), hash.Bytes())
}

// EncodeBlockNumber encodes a block number as big endian uint64
func EncodeBlockNumber(number uint64) []byte {
	enc := make([]byte, NumberLength)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

var ErrInvalidSize = errors.New("bit endian number has an invalid size")

func DecodeBlockNumber(number []byte) (uint64, error) {
	if len(number) != NumberLength {
		return 0, fmt.Errorf("%w: %d", ErrInvalidSize, len(number))
	}
	return binary.BigEndian.Uint64(number), nil
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db db.IDatabase, number uint64) error {
	w, err := db.OpenWriter(HeaderCanonical)
	if err != nil {
		return err
	}
	return w.Delete(EncodeBlockNumber(number))
}

// ReadTd retrieves a block's total difficulty corresponding to the hash.
func ReadTd(db db.IDatabase, hash types.Hash) (types.Int256, error) {
	r, err := db.OpenReader(HeaderTD)
	if nil != err {
		return types.Int256{}, err
	}
	data, err := r.Get(hash[:])
	if err != nil {
		return types.Int256{}, fmt.Errorf("failed ReadTd: %w", err)
	}
	if len(data) == 0 {
		return types.Int256{}, nil
	}
	td := new(types.Int256)
	if err := td.UnmarshalText(data); nil != err {
		return types.Int256{}, err
	}
	return *td, nil
}

//// HeaderKey = num (uint64 big endian) + hash
//func HeaderKey(hash types.Hash) []byte {
//	k := make([]byte, types.HashLength)
//	copy(k[:], hash[:])
//	return k
//}

// WriteTd stores the total difficulty of a block into the database.
func WriteTd(db db.IDatabase, hash types.Hash, td types.Int256) error {
	data, err := td.Marshal()
	if err != nil {
		return fmt.Errorf("failed to RLP encode block total difficulty: %w", err)
	}

	w, err := db.OpenWriter(HeaderTD)
	if nil != err {
		return err
	}
	if err := w.Put(hash[:], data); err != nil {
		return fmt.Errorf("failed to store block total difficulty: %w", err)
	}
	return nil
}
