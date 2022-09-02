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

package block

import (
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/types"
	"github.com/gogo/protobuf/proto"
)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

type Receipts []*Receipt

func (rs *Receipts) FromProtoMessage(receipts *types_pb.Receipts) error {
	for _, receipt := range receipts.Receipts {
		var rec Receipt
		err := rec.fromProtoMessage(receipt)
		if err == nil {
			*rs = append(*rs, &rec)
		}
	}
	return nil
}

func (rs *Receipts) ToProtoMessage() proto.Message {
	var receipts []*types_pb.Receipt
	for _, receipt := range *rs {
		pReceipt := receipt.toProtoMessage()
		receipts = append(receipts, pReceipt.(*types_pb.Receipt))
	}
	return &types_pb.Receipts{
		Receipts: receipts,
	}
}

type Receipt struct {
	// Consensus fields: These fields are defined by the Yellow Paper
	Type              uint8       `json:"type,omitempty"`
	PostState         []byte      `json:"root"`
	Status            uint64      `json:"status"`
	CumulativeGasUsed uint64      `json:"cumulativeGasUsed" gencodec:"required"`
	Bloom             types.Bloom `json:"logsBloom"         gencodec:"required"`
	Logs              []*Log      `json:"logs"              gencodec:"required"`

	// Implementation fields: These fields are added by geth when processing a transaction.
	// They are stored in the chain database.
	TxHash          types.Hash    `json:"transactionHash" gencodec:"required"`
	ContractAddress types.Address `json:"contractAddress"`
	GasUsed         uint64        `json:"gasUsed" gencodec:"required"`

	// Inclusion information: These fields provide information about the inclusion of the
	// transaction corresponding to this receipt.
	BlockHash        types.Hash   `json:"blockHash,omitempty"`
	BlockNumber      types.Int256 `json:"blockNumber,omitempty"`
	TransactionIndex uint         `json:"transactionIndex"`
}

func (r *Receipt) Marshal() ([]byte, error) {
	bpBlock := r.toProtoMessage()
	return proto.Marshal(bpBlock)
}

func (r *Receipt) Unmarshal(data []byte) error {
	var pReceipt types_pb.Receipt
	if err := proto.Unmarshal(data, &pReceipt); err != nil {
		return err
	}
	if err := r.fromProtoMessage(&pReceipt); err != nil {
		return err
	}
	return nil
}

func (r *Receipt) toProtoMessage() proto.Message {
	//bloom, _ := r.Bloom.Marshal()

	var logs []*types_pb.Log
	for _, log := range r.Logs {
		logs = append(logs, log.toProtoMessage().(*types_pb.Log))
	}
	return &types_pb.Receipt{
		Type:              uint64(r.Type),
		PostState:         r.PostState,
		Status:            r.Status,
		CumulativeGasUsed: r.CumulativeGasUsed,
		//Bloom:             bloom,
		Logs:             logs,
		TxHash:           r.TxHash,
		ContractAddress:  r.ContractAddress,
		GasUsed:          r.GasUsed,
		BlockHash:        r.BlockHash,
		BlockNumber:      r.BlockNumber,
		TransactionIndex: uint64(r.TransactionIndex),
	}
}

func (r *Receipt) fromProtoMessage(message proto.Message) error {
	var (
		pReceipt *types_pb.Receipt
		ok       bool
	)

	if pReceipt, ok = message.(*types_pb.Receipt); !ok {
		return fmt.Errorf("type conversion failure")
	}

	//bloom := new(types.Bloom)
	//err := bloom.UnMarshalBloom(pReceipt.Bloom)
	//if err != nil {
	//	return fmt.Errorf("type conversion failure bloom")
	//}

	var logs []*Log
	for _, logMessage := range pReceipt.Logs {
		log := new(Log)

		if err := log.fromProtoMessage(logMessage); err != nil {
			return fmt.Errorf("type conversion failure log %s", err)
		}
		logs = append(logs, log)
	}

	r.Type = uint8(pReceipt.Type)
	r.PostState = pReceipt.PostState
	r.Status = pReceipt.Status
	r.CumulativeGasUsed = pReceipt.CumulativeGasUsed
	//r.Bloom = *bloom
	r.Logs = logs
	r.TxHash = pReceipt.TxHash
	r.ContractAddress = pReceipt.ContractAddress
	r.GasUsed = pReceipt.GasUsed
	r.BlockHash = pReceipt.BlockHash
	r.BlockNumber = pReceipt.BlockNumber
	r.TransactionIndex = uint(pReceipt.TransactionIndex)

	return nil
}
