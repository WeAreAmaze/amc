package types

import (
	"encoding/json"
	"errors"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/internal/avm/common"
)

type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex" gencodec:"required"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash"`
	// index of the log in the receipt
	Index uint `json:"logIndex" gencodec:"required"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed"`
}

// MarshalJSON marshals as JSON.
func (l Log) MarshalJSON() ([]byte, error) {
	type Log struct {
		Address     common.Address `json:"address" gencodec:"required"`
		Topics      []common.Hash  `json:"topics" gencodec:"required"`
		Data        hexutil.Bytes  `json:"data" gencodec:"required"`
		BlockNumber hexutil.Uint64 `json:"blockNumber"`
		TxHash      common.Hash    `json:"transactionHash" gencodec:"required"`
		TxIndex     hexutil.Uint   `json:"transactionIndex"`
		BlockHash   common.Hash    `json:"blockHash"`
		Index       hexutil.Uint   `json:"logIndex"`
		Removed     bool           `json:"removed"`
	}
	var enc Log
	enc.Address = l.Address
	enc.Topics = l.Topics
	enc.Data = l.Data
	enc.BlockNumber = hexutil.Uint64(l.BlockNumber)
	enc.TxHash = l.TxHash
	enc.TxIndex = hexutil.Uint(l.TxIndex)
	enc.BlockHash = l.BlockHash
	enc.Index = hexutil.Uint(l.Index)
	enc.Removed = l.Removed
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (l *Log) UnmarshalJSON(input []byte) error {
	type Log struct {
		Address     *common.Address `json:"address" gencodec:"required"`
		Topics      []common.Hash   `json:"topics" gencodec:"required"`
		Data        *hexutil.Bytes  `json:"data" gencodec:"required"`
		BlockNumber *hexutil.Uint64 `json:"blockNumber"`
		TxHash      *common.Hash    `json:"transactionHash" gencodec:"required"`
		TxIndex     *hexutil.Uint   `json:"transactionIndex"`
		BlockHash   *common.Hash    `json:"blockHash"`
		Index       *hexutil.Uint   `json:"logIndex"`
		Removed     *bool           `json:"removed"`
	}
	var dec Log
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Address == nil {
		return errors.New("missing required field 'address' for Log")
	}
	l.Address = *dec.Address
	if dec.Topics == nil {
		return errors.New("missing required field 'topics' for Log")
	}
	l.Topics = dec.Topics
	if dec.Data == nil {
		return errors.New("missing required field 'data' for Log")
	}
	l.Data = *dec.Data
	if dec.BlockNumber != nil {
		l.BlockNumber = uint64(*dec.BlockNumber)
	}
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionHash' for Log")
	}
	l.TxHash = *dec.TxHash
	if dec.TxIndex != nil {
		l.TxIndex = uint(*dec.TxIndex)
	}
	if dec.BlockHash != nil {
		l.BlockHash = *dec.BlockHash
	}
	if dec.Index != nil {
		l.Index = uint(*dec.Index)
	}
	if dec.Removed != nil {
		l.Removed = *dec.Removed
	}
	return nil
}
