package types

import (
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/common/math"
	"math/big"
)

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	gasFeeCap  *big.Int
	gasTipCap  *big.Int
	data       []byte
	accessList AccessList
	isFake     bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice, gasFeeCap, gasTipCap *big.Int, data []byte, accessList AccessList, isFake bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		gasFeeCap:  gasFeeCap,
		gasTipCap:  gasTipCap,
		data:       data,
		accessList: accessList,
		isFake:     isFake,
	}
}

// // AsMessage returns the transaction as a core.Message.
func AsMessage(tx *transaction.Transaction, baseFee *big.Int, isFake bool) Message {
	f, _ := tx.From()

	from := &common.Address{}
	if !f.IsNull() {
		from = FromAmcAddress(&f)
	}

	msg := Message{
		nonce:     tx.Nonce(),
		gasLimit:  tx.Gas(),
		gasPrice:  tx.GasPrice().ToBig(),
		gasFeeCap: tx.GasFeeCap().ToBig(),
		gasTipCap: tx.GasTipCap().ToBig(),
		to:        FromAmcAddress(tx.To()),
		amount:    tx.Value().ToBig(),
		data:      tx.Data(),
		isFake:    isFake,
		from:      *from,
	}

	if baseFee != nil {
		msg.gasPrice = math.BigMin(msg.gasPrice.Add(msg.gasTipCap, baseFee), msg.gasFeeCap)
	}
	var accessList AccessList
	if tx.AccessList() != nil {
		for _, v := range tx.AccessList() {
			var at AccessTuple
			at.Address = *FromAmcAddress(&v.Address)
			for _, h := range v.StorageKeys {
				at.StorageKeys = append(at.StorageKeys, FromAmcHash(h))
			}
			accessList = append(accessList, at)
		}
	} else {
		accessList = nil
	}

	msg.accessList = accessList

	//log.Infof("as message : %+V", msg)
	return msg
}

func (m Message) From() common.Address   { return m.from }
func (m Message) To() *common.Address    { return m.to }
func (m Message) GasPrice() *big.Int     { return m.gasPrice }
func (m Message) GasFeeCap() *big.Int    { return m.gasFeeCap }
func (m Message) GasTipCap() *big.Int    { return m.gasTipCap }
func (m Message) Value() *big.Int        { return m.amount }
func (m Message) Gas() uint64            { return m.gasLimit }
func (m Message) Nonce() uint64          { return m.nonce }
func (m Message) Data() []byte           { return m.data }
func (m Message) AccessList() AccessList { return m.accessList }
func (m Message) IsFake() bool           { return m.isFake }
