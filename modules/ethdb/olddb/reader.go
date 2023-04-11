package olddb

import (
	"bytes"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type StateReader struct {
	accHistoryC, storageHistoryC kv.Cursor
	accChangesC, storageChangesC kv.CursorDupSort
	blockNr                      uint64
	readCodeF                    func(types.Hash) ([]byte, error)
	data                         map[string][]byte
	codes                        map[types.Hash][]byte
	db                           kv.Getter
}

func NewStateReader(data map[string][]byte, codes map[types.Hash][]byte, db kv.Getter, blockNr uint64) *StateReader {
	return &StateReader{
		codes:   codes,
		data:    data,
		blockNr: blockNr,
		db:      db,
	}
}

func (dbr *StateReader) SetBlockNumber(blockNr uint64) {
	dbr.blockNr = blockNr
}

func (dbr *StateReader) SetReadCodeF(readCodeF func(types.Hash) ([]byte, error)) {
	dbr.readCodeF = readCodeF
}

func (dbr *StateReader) GetOne(bucket string, key []byte) ([]byte, error) {
	if len(bucket) == 0 {
		return nil, nil
	}
	b, err := dbr.db.GetOne(bucket, key[:])
	if err == nil && len(b) > 0 {
		return b, err
	}
	v, ok := dbr.data[*(*string)(unsafe.Pointer(&key))]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (r *StateReader) ReadAccountData(address types.Address) (*account.StateAccount, error) {
	v, err := r.db.GetOne(modules.Account, address[:])
	if err == nil && len(v) > 0 {
		var acc account.StateAccount
		if err := acc.DecodeForStorage(v); err != nil {
			return nil, err
		}
		return &acc, nil
	}
	b := address.Bytes()
	v, ok := r.data[*(*string)(unsafe.Pointer(&b))]
	if !ok {
		return nil, nil
	}
	if ok && len(v) > 0 {
		var acc account.StateAccount
		if err := acc.DecodeForStorage(v); err != nil {
			return nil, err
		}
		return &acc, nil
	}
	return nil, nil
}

func (r *StateReader) ReadAccountStorage(address types.Address, incarnation uint16, key *types.Hash) ([]byte, error) {
	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())
	v, err := r.db.GetOne(modules.Storage, compositeKey)
	if err == nil && len(v) > 0 {
		return v, nil
	}
	vv, ok := r.data[*(*string)(unsafe.Pointer(&compositeKey))]
	if !ok {
		return nil, nil
	}
	return vv, nil
}

func (r *StateReader) ReadAccountCode(address types.Address, incarnation uint16, codeHash types.Hash) ([]byte, error) {
	if bytes.Equal(codeHash[:], crypto.Keccak256(nil)) {
		return nil, nil
	}
	if r.readCodeF != nil {
		return r.readCodeF(codeHash)
	}
	return r.codes[codeHash], nil
}

func (r *StateReader) ReadAccountCodeSize(address types.Address, incarnation uint16, codeHash types.Hash) (int, error) {
	code, err := r.ReadAccountCode(address, incarnation, codeHash)
	if err != nil {
		return 0, err
	}
	return len(code), nil
}

func (r *StateReader) ReadAccountIncarnation(address types.Address) (uint16, error) {
	return 0, nil
}
