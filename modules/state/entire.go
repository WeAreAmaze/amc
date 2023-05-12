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

package state

import (
	"bytes"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/rlp"
	types2 "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules"
	"io"
	"unsafe"
)

type GetOneFun func(table string, key []byte) ([]byte, error)

type EntireCode struct {
	CoinBase types.Address   `json:"coinBase"`
	Entire   Entire          `json:"entire"`
	Codes    []*HashCode     `json:"codes"`
	Headers  []*block.Header `json:"headers"`
	Rewards  []*block.Reward `json:"rewards"`
}

type HashCode struct {
	Hash types.Hash `json:"hash"`
	Code []byte     `json:"code"`
}

type HashCodes []*HashCode

func (s HashCodes) Len() int           { return len(s) }
func (s HashCodes) Less(i, j int) bool { return bytes.Compare(s[i].Hash[:], s[j].Hash[:]) < 0 }
func (s HashCodes) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type Entire struct {
	Header       *block.Header    `json:"header"`
	Uncles       []*types2.Header `json:"uncles"`
	Transactions [][]byte         `json:"transactions"`
	Snap         *Snapshot        `json:"snap"`
	Proof        types.Hash       `json:"proof"`
	Senders      []types.Address  `json:"senders"`
}

func (e Entire) Clone() Entire {
	var c Entire
	//c.Header = &types2.Header{
	//	ParentHash:  e.Header.ParentHash,
	//	UncleHash:   e.Header.UncleHash,
	//	Coinbase:    e.Header.Coinbase,
	//	Root:        e.Header.Root,
	//	TxHash:      e.Header.TxHash,
	//	ReceiptHash: e.Header.ReceiptHash,
	//	Bloom:       e.Header.Bloom,
	//	Difficulty:  e.Header.Difficulty,
	//	Number:      e.Header.Number,
	//	GasLimit:    e.Header.GasLimit,
	//	GasUsed:     e.Header.GasUsed,
	//	Time:        e.Header.Time,
	//	Extra:       e.Header.Extra,
	//	MixDigest:   e.Header.MixDigest,
	//	Nonce:       e.Header.Nonce,
	//	BaseFee:     e.Header.BaseFee,
	//}
	copyHeader := *e.Header
	c.Header = &copyHeader
	c.Uncles = e.Uncles
	c.Transactions = e.Transactions
	c.Proof = e.Proof
	c.Senders = e.Senders
	c.Snap = &Snapshot{
		Items:     e.Snap.Items,
		OutHash:   e.Snap.OutHash,
		accounts:  e.Snap.accounts,
		storage:   e.Snap.storage,
		written:   e.Snap.written,
		getOneFun: e.Snap.getOneFun,
	}
	return c
}

type Items []*Item

func (s Items) Len() int           { return len(s) }
func (s Items) Less(i, j int) bool { return bytes.Compare(s[i].Key, s[j].Key) < 0 }
func (s Items) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type Item struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type Snapshot struct {
	Items     Items          `json:"items"`
	OutHash   types.Hash     `json:"outHash"`
	accounts  map[string]int `json:"accounts"`
	storage   map[string]int `json:"storage"`
	written   bool           `json:"written"`
	getOneFun GetOneFun      `json:"getOneFun"`
}

func NewWritableSnapshot() *Snapshot {
	return &Snapshot{Items: make([]*Item, 0), written: true, OutHash: emptyCodeHashH, accounts: make(map[string]int), storage: make(map[string]int)}
}

func NewReadableSnapshot() *Snapshot {
	return &Snapshot{Items: make([]*Item, 0), written: false, OutHash: emptyCodeHashH, accounts: make(map[string]int), storage: make(map[string]int)}
}

func (s *Snapshot) SetGetFun(f GetOneFun) {
	s.getOneFun = f
}

func (s *Snapshot) ReadAccountStorage(address types.Address, incarnation uint16, key *types.Hash) ([]byte, error) {
	if s.written {
		return nil, nil
	}
	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())
	index, ok := s.storage[*(*string)(unsafe.Pointer(&compositeKey))]
	if !ok {
		if s.getOneFun != nil {
			return s.getOneFun(modules.Storage, compositeKey)
		}
		return nil, nil
	}
	if s.Items[index] == nil {
		return nil, nil
	}
	return s.Items[index].Value, nil
}

func (s *Snapshot) CanWrite() bool {
	return s.written
}

func ReadSnapshotData(data []byte) (*Snapshot, error) {
	snap := NewReadableSnapshot()
	if len(data) == 0 {
		return snap, nil
	}
	if err := rlp.DecodeBytes(data, &snap); err != nil {
		return nil, err
	}
	for k, v := range snap.Items {
		if len(v.Key) == types.AddressLength {
			snap.accounts[*(*string)(unsafe.Pointer(&v.Key))] = k
		} else {
			snap.storage[*(*string)(unsafe.Pointer(&v.Key))] = k
		}
	}
	return snap, nil
}

func (s *Snapshot) SetHash(hash types.Hash) {
	if s.written {
		return
	}
	s.OutHash = hash
}

func (s *Snapshot) ReadAccountData(address types.Address) (*account.StateAccount, error) {
	if s.written {
		return nil, nil
	}
	addr := address[:]
	index, ok := s.accounts[*(*string)(unsafe.Pointer(&addr))]
	if !ok {
		if s.getOneFun != nil {
			v, err := s.getOneFun(modules.Account, address[:])
			if err != nil {
				return nil, err
			}
			if v == nil {
				return nil, nil
			}
			var acc account.StateAccount
			if err := acc.DecodeForStorage(v); err != nil {
				return nil, err
			}
			return &acc, nil
		}
		return nil, nil
	}
	if s.Items[index] == nil {
		return nil, nil
	}
	var acc account.StateAccount
	if err := acc.DecodeForStorage(s.Items[index].Value); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (s *Snapshot) AddAccount(address types.Address, account *account.StateAccount) {
	if !s.written || account == nil {
		return
	}
	value := make([]byte, account.EncodingLengthForStorage())
	account.EncodeForStorage(value)
	s.Items = append(s.Items, &Item{Key: address[:], Value: value})
	ss := address[:]
	//fmt.Println("address", address.Hex())
	s.accounts[*(*string)(unsafe.Pointer(&ss))] = len(s.Items)
}

func (s *Snapshot) AddStorage(address types.Address, key *types.Hash, incarnation uint16, value []byte) {
	if !s.written || len(value) == 0 {
		return
	}
	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())
	s.Items = append(s.Items, &Item{Key: compositeKey, Value: value})
	//fmt.Println("address", address.Hex(), key.Hex())
	s.storage[*(*string)(unsafe.Pointer(&compositeKey))] = len(s.storage)
}

func EncodeBeforeState(w io.Writer, list Items, codeHash HashCodes) {
	enc := make([]interface{}, 0, len(list)*2+len(codeHash)*2)
	for _, i := range list {
		enc = append(enc, i.Key, i.Value)
	}

	for _, h := range codeHash {
		enc = append(enc, h.Hash, h.Code)
	}
	if err := rlp.Encode(w, enc); nil != err {
		panic("before state encode failed:" + err.Error())
	}
}
