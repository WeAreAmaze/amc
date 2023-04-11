package state

import (
	"bytes"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type HistoryStateReader struct {
	accHistoryC, storageHistoryC kv.Cursor
	accChangesC, storageChangesC kv.CursorDupSort
	blockNr                      uint64
	tx                           kv.Tx
	db                           kv.Getter
}

func NewStateHistoryReader(tx kv.Tx, db kv.Getter, blockNr uint64) *HistoryStateReader {
	c1, _ := tx.Cursor(modules.AccountsHistory)
	c2, _ := tx.Cursor(modules.StorageHistory)
	c3, _ := tx.CursorDupSort(modules.AccountChangeSet)
	c4, _ := tx.CursorDupSort(modules.StorageChangeSet)
	return &HistoryStateReader{
		tx:          tx,
		blockNr:     blockNr,
		db:          db,
		accHistoryC: c1, storageHistoryC: c2, accChangesC: c3, storageChangesC: c4,
	}
}

func (dbr *HistoryStateReader) Rollback() {
	dbr.tx.Rollback()
}

func (dbr *HistoryStateReader) SetBlockNumber(blockNr uint64) {
	dbr.blockNr = blockNr
}

func (dbr *HistoryStateReader) GetOne(bucket string, key []byte) ([]byte, error) {
	if len(bucket) == 0 {
		return nil, nil
	}
	return dbr.db.GetOne(bucket, key[:])
}

func (r *HistoryStateReader) ReadAccountData(address types.Address) (*account.StateAccount, error) {
	acc := new(account.StateAccount)
	// defer fmt.Printf("ReadAccount address: %s, account: %+v \n", address, acc)
	v, err := r.db.GetOne(modules.Account, address[:])
	if err == nil && len(v) > 0 {
		//var acc account.StateAccount
		if err := acc.DecodeForStorage(v); err != nil {
			return nil, err
		}
		return acc, nil
	}
	enc, err := GetAsOf(r.tx, r.accHistoryC, r.accChangesC, false /* storage */, address[:], r.blockNr)
	if err != nil || enc == nil || len(enc) == 0 {
		return nil, nil
	}
	//var acc account.StateAccount
	if err := acc.DecodeForStorage(enc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (r *HistoryStateReader) ReadAccountStorage(address types.Address, incarnation uint16, key *types.Hash) ([]byte, error) {
	compositeKey := modules.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())
	v, err := r.db.GetOne(modules.Storage, compositeKey)
	if err == nil && len(v) > 0 {
		return v, nil
	}
	return GetAsOf(r.tx, r.storageHistoryC, r.storageChangesC, true /* storage */, compositeKey, r.blockNr)
}

func (r *HistoryStateReader) ReadAccountCode(address types.Address, incarnation uint16, codeHash types.Hash) ([]byte, error) {
	if bytes.Equal(codeHash[:], crypto.Keccak256(nil)) {
		return nil, nil
	}
	var val []byte
	v, err := r.tx.GetOne(modules.Code, codeHash[:])
	if err != nil || len(v) == 0 {
		panic(err)
		return nil, err
	}
	val = types.CopyBytes(v)
	return val, nil
}

func (r *HistoryStateReader) ReadAccountCodeSize(address types.Address, incarnation uint16, codeHash types.Hash) (int, error) {
	code, err := r.ReadAccountCode(address, incarnation, codeHash)
	if err != nil {
		return 0, err
	}
	return len(code), nil
}

func (r *HistoryStateReader) ReadAccountIncarnation(address types.Address) (uint16, error) {
	return 0, nil
}
