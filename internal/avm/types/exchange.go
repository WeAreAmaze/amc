package types

import (
	"bytes"
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/crypto"
	"github.com/amazechain/amc/internal/avm/params"
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	"golang.org/x/crypto/sha3"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

// hasherPool holds LegacyKeccak256 hashers for rlpHash.
var hasherPool = sync.Pool{
	New: func() interface{} { return sha3.NewLegacyKeccak256() },
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func ToAmcAddress(addr common.Address) types.Address {
	nullAddress := common.Address{}
	if bytes.Equal(addr[:], nullAddress[:]) {
		return types.Address{0}
	}
	var a types.Address
	copy(a[:], addr[:])
	return a
}

func ToAmcAccessList(accessList AccessList) transaction.AccessList {
	var txAccessList transaction.AccessList
	for _, accessTuple := range accessList {
		txAccessTuple := new(transaction.AccessTuple)
		txAccessTuple.Address = ToAmcAddress(accessTuple.Address)
		for _, hash := range accessTuple.StorageKeys {
			txAccessTuple.StorageKeys = append(txAccessTuple.StorageKeys, ToAmcHash(hash))
		}
		txAccessList = append(txAccessList, *txAccessTuple)
	}
	return txAccessList
}

func FromAmcAddress(address types.Address) *common.Address {
	if address.IsNull() {
		return nil
	}
	var a common.Address
	copy(a[:], address[:])
	return &a
}

func ToAmcHash(hash common.Hash) types.Hash {
	var h types.Hash
	copy(h[:], hash[:])
	return h
}

func FromAmcHash(hash types.Hash) common.Hash {
	var h common.Hash
	copy(h[:], hash[:])
	return h
}

func ToAmcLog(log *Log) *block.Log {
	if log == nil {
		return nil
	}

	var topics []types.Hash
	for _, topic := range log.Topics {
		topics = append(topics, ToAmcHash(topic))
	}

	return &block.Log{
		Address:     ToAmcAddress(log.Address),
		Topics:      topics,
		Data:        log.Data,
		BlockNumber: types.NewInt64(log.BlockNumber),
		TxHash:      ToAmcHash(log.TxHash),
		TxIndex:     log.TxIndex,
		BlockHash:   ToAmcHash(log.BlockHash),
		Index:       log.Index,
		Removed:     log.Removed,
	}
}

func FromAmcLog(log *block.Log) *Log {
	if log == nil {
		return nil
	}

	var topics []common.Hash
	for _, topic := range log.Topics {
		topics = append(topics, FromAmcHash(topic))
	}

	return &Log{
		Address:     *FromAmcAddress(log.Address),
		Topics:      topics,
		Data:        log.Data,
		BlockNumber: log.BlockNumber.Uint64(),
		TxHash:      FromAmcHash(log.TxHash),
		TxIndex:     log.TxIndex,
		BlockHash:   FromAmcHash(log.BlockHash),
		Index:       log.Index,
		Removed:     log.Removed,
	}
}

func ToAmcLogs(logs []*Log) []*block.Log {
	var amcLogs []*block.Log
	for _, log := range logs {
		amcLogs = append(amcLogs, ToAmcLog(log))
	}
	return amcLogs
}

func FromAmcLogs(amcLogs []*block.Log) []*Log {
	var logs []*Log
	for _, log := range amcLogs {
		logs = append(logs, FromAmcLog(log))
	}
	return logs
}

type Transaction struct {
	//Nonce    uint64          // nonce of sender account
	//GasPrice *big.Int        // wei per gas
	//Gas      uint64          // gas limit
	//To       *common.Address `rlp:"nil"` // nil means contract creation
	//Value    *big.Int        // wei amount
	//Data     []byte          // contract invocation input data
	//V, R, S  *big.Int        // signature values

	inner TxData    // Consensus contents of a transaction
	time  time.Time // Time first seen locally (spam avoidance)

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// NewTx creates a new transaction.
func NewTx(inner TxData) *Transaction {
	tx := new(Transaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28 && v != 1 && v != 0
	}
	// anything not 27 or 28 is considered protected
	return true
}

// Protected says whether the transaction is replay-protected.
func (tx *Transaction) Protected() bool {
	switch tx := tx.inner.(type) {
	case *LegacyTx:
		return tx.V != nil && isProtectedV(tx.V)
	default:
		return true
	}
}

// Type returns the transaction type.
func (tx *Transaction) Type() uint8 {
	return tx.inner.txType()
}

// ChainId returns the EIP155 chain ID of the transaction. The return value will always be
// non-nil. For legacy transactions which are not replay-protected, the return value is
// zero.
func (tx *Transaction) ChainId() *big.Int {
	return tx.inner.chainID()
}

// Data returns the input data of the transaction.
func (tx *Transaction) Data() []byte { return tx.inner.data() }

// AccessList returns the access list of the transaction.
func (tx *Transaction) AccessList() AccessList { return tx.inner.accessList() }

// Gas returns the gas limit of the transaction.
func (tx *Transaction) Gas() uint64 { return tx.inner.gas() }

// GasPrice returns the gas price of the transaction.
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.inner.gasPrice()) }

// GasTipCap returns the gasTipCap per gas of the transaction.
func (tx *Transaction) GasTipCap() *big.Int { return new(big.Int).Set(tx.inner.gasTipCap()) }

// GasFeeCap returns the fee cap per gas of the transaction.
func (tx *Transaction) GasFeeCap() *big.Int { return new(big.Int).Set(tx.inner.gasFeeCap()) }

// Value returns the ether amount of the transaction.
func (tx *Transaction) Value() *big.Int { return new(big.Int).Set(tx.inner.value()) }

// Nonce returns the sender account nonce of the transaction.
func (tx *Transaction) Nonce() uint64 { return tx.inner.nonce() }

// To returns the recipient address of the transaction.
// For contract-creation transactions, To returns nil.
func (tx *Transaction) To() *common.Address {
	return copyAddressPtr(tx.inner.to())
}

func (tx *Transaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.inner.rawSignatureValues()
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previously cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.inner)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := tx.inner.copy()
	cpy.setSignatureValues(signer.ChainID(), v, r, s)
	return &Transaction{inner: cpy, time: tx.time}, nil
}

// UnmarshalBinary todo compatible with other txs
func (tx *Transaction) UnmarshalBinary(b []byte) error {
	if len(b) > 0 && b[0] > 0x7f {
		// It's a legacy transaction.
		var data LegacyTx
		err := rlp.DecodeBytes(b, &data)
		if err != nil {
			return err
		}
		tx.setDecoded(&data, len(b))
		return nil
	}
	// It's an EIP2718 typed transaction envelope.
	inner, err := tx.decodeTyped(b)
	if err != nil {
		return err
	}
	tx.setDecoded(inner, len(b))
	return nil
}

// decodeTyped decodes a typed transaction from the canonical format.
func (tx *Transaction) decodeTyped(b []byte) (TxData, error) {
	if len(b) <= 1 {
		return nil, fmt.Errorf("typed transaction too short")
	}
	switch b[0] {
	case AccessListTxType:
		var inner AccessListTx
		err := rlp.DecodeBytes(b[1:], &inner)
		return &inner, err
	case DynamicFeeTxType:
		var inner DynamicFeeTx
		err := rlp.DecodeBytes(b[1:], &inner)
		return &inner, err
	default:
		return nil, fmt.Errorf("transaction type not valid in this context")
	}
}

// setDecoded sets the inner transaction and size after decoding.
func (tx *Transaction) setDecoded(inner TxData, size int) {
	tx.inner = inner
	tx.time = time.Now()
	if size > 0 {
		tx.size.Store(common.StorageSize(size))
	}
}

func (tx *Transaction) ToAmcTransaction(chainConfig *params.ChainConfig, blockNumber *big.Int) (*transaction.Transaction, error) {

	switch tx.Type() {
	case LegacyTxType:
		log.Debugf("tx type is LegacyTxType")
	case AccessListTxType:
		log.Debugf("tx type is AccessListTxType")
	case DynamicFeeTxType:
		log.Debugf("tx type is DynamicFeeTxType")
	}

	gasPrice, ok := types.FromBig(tx.GasPrice())
	if !ok {
		return nil, fmt.Errorf("cannot convert big int to int256")
	}

	to := types.Address{0}
	if tx.To() != nil {
		to = ToAmcAddress(*tx.To())
	}

	value, ok := types.FromBig(tx.Value())
	if !ok {
		return nil, fmt.Errorf("cannot convert big int to int256")
	}

	log.Debugf("tx %+v", tx.inner)

	signer := MakeSigner(chainConfig, blockNumber)
	from, err := Sender(signer, tx)
	if err != nil {
		return nil, err
	}

	return transaction.NewTransaction(tx.Nonce(), ToAmcAddress(from), to, value, tx.Gas(), gasPrice, tx.Data()), err
}

func FromAmcHeader(iHeader block.IHeader, engine consensus.Engine) *Header {
	header := iHeader.(*block.Header)
	author, _ := engine.Author(iHeader)

	var baseFee *big.Int
	if !header.BaseFee.IsZero() {
		baseFee = header.BaseFee.ToBig()
	}
	return &Header{
		ParentHash:  FromAmcHash(header.ParentHash),
		UncleHash:   common.Hash{},
		Coinbase:    *FromAmcAddress(author),
		Root:        FromAmcHash(header.Root),
		TxHash:      FromAmcHash(header.TxHash),
		ReceiptHash: FromAmcHash(header.ReceiptHash),
		Difficulty:  header.Difficulty.ToBig(),
		Number:      header.Number.ToBig(),
		GasLimit:    header.GasLimit,
		GasUsed:     header.GasUsed,
		Time:        header.Time,
		Extra:       header.Extra,
		MixDigest:   FromAmcHash(header.MixDigest),
		Nonce:       EncodeNonce(header.Nonce.Uint64()),
		BaseFee:     baseFee,
	}
}

func rlpHash(x interface{}) (h common.Hash) {
	sha := hasherPool.Get().(crypto.KeccakState)
	defer hasherPool.Put(sha)
	sha.Reset()
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}

// prefixedRlpHash writes the prefix into the hasher before rlp-encoding x.
// It's used for typed transactions.
func prefixedRlpHash(prefix byte, x interface{}) (h common.Hash) {
	sha := hasherPool.Get().(crypto.KeccakState)
	defer hasherPool.Put(sha)
	sha.Reset()
	sha.Write([]byte{prefix})
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}
