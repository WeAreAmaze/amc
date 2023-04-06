package types

import (
	"bytes"
	"fmt"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
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

func ToAmcAddress(addr *common.Address) *types.Address {
	if addr == nil {
		return nil
	}
	nullAddress := common.Address{}
	if bytes.Equal(addr[:], nullAddress[:]) {
		return &types.Address{0}
	}
	var a types.Address
	copy(a[:], addr[:])
	return &a
}

func ToAmcAccessList(accessList AccessList) transaction.AccessList {
	var txAccessList transaction.AccessList
	for _, accessTuple := range accessList {
		txAccessTuple := new(transaction.AccessTuple)
		txAccessTuple.Address = *ToAmcAddress(&accessTuple.Address)
		for _, hash := range accessTuple.StorageKeys {
			txAccessTuple.StorageKeys = append(txAccessTuple.StorageKeys, ToAmcHash(hash))
		}
		txAccessList = append(txAccessList, *txAccessTuple)
	}
	return txAccessList
}

func FromAmcAccessList(accessList transaction.AccessList) AccessList {
	var txAccessList AccessList
	for _, accessTuple := range accessList {
		txAccessTuple := new(AccessTuple)
		txAccessTuple.Address = *FromAmcAddress(&accessTuple.Address)
		for _, hash := range accessTuple.StorageKeys {
			txAccessTuple.StorageKeys = append(txAccessTuple.StorageKeys, FromAmcHash(hash))
		}
		txAccessList = append(txAccessList, *txAccessTuple)
	}
	return txAccessList
}

func FromAmcAddress(address *types.Address) *common.Address {
	if address == nil {
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
		Address:     *ToAmcAddress(&log.Address),
		Topics:      topics,
		Data:        log.Data,
		BlockNumber: uint256.NewInt(log.BlockNumber),
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
		Address:     *FromAmcAddress(&log.Address),
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

// Hash returns the transaction hash.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	var h common.Hash
	if tx.Type() == LegacyTxType {
		h = rlpHash(tx.inner)
	} else {
		h = prefixedRlpHash(tx.Type(), tx.inner)
	}
	tx.hash.Store(h)
	return h
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

// UnmarshalBinary
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

	var inner transaction.TxData
	gasPrice, overflow := uint256.FromBig(tx.GasPrice())
	if overflow {
		return nil, fmt.Errorf("cannot convert big int to int256")
	}

	vl, overflow := uint256.FromBig(tx.Value())
	if overflow {
		return nil, fmt.Errorf("cannot convert big int to int256")
	}

	signer := MakeSigner(chainConfig, blockNumber)
	from, err := Sender(signer, tx)
	if err != nil {
		return nil, err
	}

	v, r, s := tx.RawSignatureValues()
	V, is1 := uint256.FromBig(v)
	R, is2 := uint256.FromBig(r)
	S, is3 := uint256.FromBig(s)
	if is1 || is2 || is3 {
		return nil, fmt.Errorf("r,s,v overflow")
	}
	switch tx.Type() {
	case LegacyTxType:
		inner = &transaction.LegacyTx{
			Nonce:    tx.Nonce(),
			Gas:      tx.Gas(),
			Data:     common.CopyBytes(tx.Data()),
			GasPrice: gasPrice,
			Value:    vl,
			To:       ToAmcAddress(tx.To()),
			From:     ToAmcAddress(&from),
			V:        V,
			R:        R,
			S:        S,
		}

		log.Debug("tx type is LegacyTxType")
	case AccessListTxType:
		at := &transaction.AccessListTx{
			Nonce:      tx.Nonce(),
			Gas:        tx.Gas(),
			Data:       common.CopyBytes(tx.Data()),
			To:         ToAmcAddress(tx.To()),
			GasPrice:   gasPrice,
			Value:      vl,
			From:       ToAmcAddress(&from),
			AccessList: ToAmcAccessList(tx.AccessList()),
			V:          V,
			R:          R,
			S:          S,
		}
		at.ChainID, _ = uint256.FromBig(tx.ChainId())
		inner = at
		log.Debug("tx type is AccessListTxType")
	case DynamicFeeTxType:
		dft := &transaction.DynamicFeeTx{
			Nonce:      tx.Nonce(),
			Gas:        tx.Gas(),
			To:         ToAmcAddress(tx.To()),
			Data:       common.CopyBytes(tx.Data()),
			AccessList: ToAmcAccessList(tx.AccessList()),
			Value:      vl,
			From:       ToAmcAddress(&from),
			V:          V,
			R:          R,
			S:          S,
		}
		dft.ChainID, _ = uint256.FromBig(tx.ChainId())
		dft.GasTipCap, _ = uint256.FromBig(tx.GasTipCap())
		dft.GasFeeCap, _ = uint256.FromBig(tx.GasFeeCap())
		inner = dft
		log.Debug("tx type is DynamicFeeTxType")
	}

	amcTx := transaction.NewTx(inner)
	return amcTx, nil
}

func (tx *Transaction) FromAmcTransaction(amcTx *transaction.Transaction) {

	var inner TxData

	gasPrice := amcTx.GasPrice().ToBig()
	vl := amcTx.Value().ToBig()

	switch amcTx.Type() {
	case transaction.LegacyTxType:
		inner = &LegacyTx{
			Nonce:    amcTx.Nonce(),
			Gas:      amcTx.Gas(),
			Data:     common.CopyBytes(amcTx.Data()),
			GasPrice: gasPrice,
			Value:    vl,
			To:       FromAmcAddress(amcTx.To()),
		}

	case transaction.AccessListTxType:
		at := &AccessListTx{
			Nonce:      amcTx.Nonce(),
			Gas:        amcTx.Gas(),
			Data:       common.CopyBytes(amcTx.Data()),
			To:         FromAmcAddress(amcTx.To()),
			GasPrice:   gasPrice,
			Value:      vl,
			AccessList: FromAmcAccessList(amcTx.AccessList()),
		}
		at.ChainID = amcTx.ChainId().ToBig()
		inner = at

	case transaction.DynamicFeeTxType:
		dft := &DynamicFeeTx{
			Nonce:      amcTx.Nonce(),
			Gas:        amcTx.Gas(),
			To:         FromAmcAddress(amcTx.To()),
			Data:       common.CopyBytes(amcTx.Data()),
			AccessList: FromAmcAccessList(amcTx.AccessList()),
			Value:      vl,
		}
		dft.ChainID = amcTx.ChainId().ToBig()
		dft.GasTipCap = amcTx.GasTipCap().ToBig()
		dft.GasFeeCap = amcTx.GasFeeCap().ToBig()
		inner = dft
	}

	v, r, s := amcTx.RawSignatureValues()
	inner.setSignatureValues(amcTx.ChainId().ToBig(), v.ToBig(), r.ToBig(), s.ToBig())

	tx.setDecoded(inner.copy(), 0)
}

func FromAmcHeader(iHeader block.IHeader) *Header {
	header := iHeader.(*block.Header)
	//author, _ := engine.Author(iHeader)

	var baseFee *big.Int
	if header.BaseFee != nil {
		baseFee = header.BaseFee.ToBig()
	}

	bloom := new(Bloom)
	bloom.SetBytes(header.Bloom.Bytes())

	return &Header{
		ParentHash:  FromAmcHash(header.ParentHash),
		UncleHash:   FromAmcHash(EmptyUncleHash),
		Coinbase:    *FromAmcAddress(&header.Coinbase),
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
		Bloom:       *bloom,
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
