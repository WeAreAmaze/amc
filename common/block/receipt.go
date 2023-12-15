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
	"bytes"
	"errors"
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/amazechain/amc/params"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"math/big"

)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

type Receipts []*Receipt

func (rs *Receipts) Marshal() ([]byte, error) {
	pb := rs.ToProtoMessage()
	return proto.Marshal(pb)
}

func (rs *Receipts) Unmarshal(data []byte) error {
	pb := new(types_pb.Receipts)
	if err := proto.Unmarshal(data, pb); nil != err {
		return err
	}

	return rs.FromProtoMessage(pb)
}

// Len returns the number of receipts in this list.
func (rs Receipts) Len() int { return len(rs) }

// EncodeIndex encodes the i'th receipt to w.
func (rs Receipts) EncodeIndex(i int, w *bytes.Buffer) {
	r := rs[i]

	logs := make([]*storedLog, len(r.Logs))

	for k, log := range r.Logs {
		logs[k] = &storedLog{
			Address: log.Address,
			Topics:  log.Topics,
			Data:    log.Data,
		}
	}
	data := &storedReceipt{r.Status, r.CumulativeGasUsed, logs}

	rlp.Encode(w, data)
	//byte, _ := json.Marshal(data)
	//w.Write(byte)
}

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

// DeriveFields fills the receipts with their computed fields based on consensus
// data and contextual infos like containing block and transactions.
func (rs Receipts) DeriveFields(config *params.ChainConfig, hash types.Hash, number uint64, txs []*transaction.Transaction) error {
	signer := transaction.MakeSigner(config, new(big.Int).SetUint64(number))

	logIndex := uint(0)
	if len(txs) != len(rs) {
		return errors.New("transaction and receipt count mismatch")
	}
	for i := 0; i < len(rs); i++ {
		// The transaction type and hash can be retrieved from the transaction itself
		rs[i].Type = txs[i].Type()
		rs[i].TxHash = txs[i].Hash()

		//rs[i].EffectiveGasPrice = txs[i].inner.effectiveGasPrice(new(big.Int), baseFee)

		// block location fields
		rs[i].BlockHash = hash
		rs[i].BlockNumber = new(uint256.Int).SetUint64(number)
		rs[i].TransactionIndex = uint(i)
		// Deriving the signer is expensive, only do if it's actually needed
		from, _ := transaction.Sender(signer, txs[i])

		// The contract address can be derived from the transaction itself
		if txs[i].To() == nil {

			rs[i].ContractAddress = crypto.CreateAddress(from, txs[i].Nonce())
		} else {
			rs[i].ContractAddress = types.Address{}
		}

		// The used gas can be calculated based on previous r
		if i == 0 {
			rs[i].GasUsed = rs[i].CumulativeGasUsed
		} else {
			rs[i].GasUsed = rs[i].CumulativeGasUsed - rs[i-1].CumulativeGasUsed
		}

		// The derived log fields can simply be set from the block and transaction
		for j := 0; j < len(rs[i].Logs); j++ {
			rs[i].Logs[j].BlockNumber = uint256.NewInt(number)
			rs[i].Logs[j].BlockHash = hash
			rs[i].Logs[j].TxHash = rs[i].TxHash
			rs[i].Logs[j].TxIndex = uint(i)
			rs[i].Logs[j].Index = logIndex
			rs[i].Logs[j].Sender = from
			logIndex++
		}
	}
	return nil
}

type Receipt struct {
	// Consensus fields: These fields are defined by the Yellow Paper
	Type              uint8  `json:"type,omitempty"`
	PostState         []byte `json:"root"`
	Status            uint64 `json:"status"`
	CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`
	Bloom             Bloom  `json:"logsBloom"         gencodec:"required"`
	Logs              []*Log `json:"logs"              gencodec:"required"`

	// Implementation fields: These fields are added by geth when processing a transaction.
	// They are stored in the chain database.
	TxHash          types.Hash    `json:"transactionHash" gencodec:"required"`
	ContractAddress types.Address `json:"contractAddress"`
	GasUsed         uint64        `json:"gasUsed" gencodec:"required"`

	// Inclusion information: These fields provide information about the inclusion of the
	// transaction corresponding to this receipt.
	BlockHash        types.Hash   `json:"blockHash,omitempty"`
	BlockNumber      *uint256.Int `json:"blockNumber,omitempty"`
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
		logs = append(logs, log.ToProtoMessage().(*types_pb.Log))
	}
	pb := &types_pb.Receipt{
		Type:              uint32(r.Type),
		PostState:         r.PostState,
		Status:            r.Status,
		CumulativeGasUsed: r.CumulativeGasUsed,
		Logs:              logs,
		TxHash:            utils.ConvertHashToH256(r.TxHash),
		ContractAddress:   utils.ConvertAddressToH160(r.ContractAddress),
		GasUsed:           r.GasUsed,
		BlockHash:         utils.ConvertHashToH256(r.BlockHash),
		BlockNumber:       utils.ConvertUint256IntToH256(r.BlockNumber),
		TransactionIndex:  uint64(r.TransactionIndex),
		Bloom:             utils.ConvertBytesToH2048(r.Bloom[:]),
	}
	return pb
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

		if err := log.FromProtoMessage(logMessage); err != nil {
			return fmt.Errorf("type conversion failure log %s", err)
		}
		logs = append(logs, log)
	}

	r.Type = uint8(pReceipt.Type)
	r.PostState = pReceipt.PostState
	r.Status = pReceipt.Status
	r.CumulativeGasUsed = pReceipt.CumulativeGasUsed
	r.Bloom = utils.ConvertH2048ToBloom(pReceipt.Bloom)
	r.Logs = logs
	r.TxHash = utils.ConvertH256ToHash(pReceipt.TxHash)
	r.ContractAddress = utils.ConvertH160toAddress(pReceipt.ContractAddress)
	r.GasUsed = pReceipt.GasUsed
	r.BlockHash = utils.ConvertH256ToHash(pReceipt.BlockHash)
	r.BlockNumber = utils.ConvertH256ToUint256Int(pReceipt.BlockNumber)
	r.TransactionIndex = uint(pReceipt.TransactionIndex)

	return nil
}

// Copy creates a deep copy of the Receipt.
func (r *Receipt) Copy() *Receipt {
	pb := r.toProtoMessage()
	cr := new(Receipt)
	cr.fromProtoMessage(pb)
	return cr
}

// storedReceipt is the consensus encoding of a receipt.
type storedReceipt struct {
	PostStateOrStatus uint64
	CumulativeGasUsed uint64
	Logs              []*storedLog
}

type storedLog struct {
	Address types.Address
	Topics  []types.Hash
	Data    []byte
}
