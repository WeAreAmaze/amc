package txspool

import (
	"context"
	"github.com/amazechain/amc/common/account"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/modules"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type ReadState interface {
	GetNonce(types.Address) uint64
	GetBalance(types.Address) *uint256.Int
	State(types.Address) (*account.StateAccount, error)
}

type StateCli struct {
	db  kv.RoDB
	ctx context.Context
}

func StateClient(ctx context.Context, db kv.RoDB) ReadState {
	return &StateCli{
		db:  db,
		ctx: ctx,
	}
}

func (c *StateCli) GetNonce(addr types.Address) uint64 {
	var nonce uint64
	_ = c.db.View(c.ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(modules.Account, addr.Bytes())
		if err != nil {
			return err
		}
		if len(v) == 0 {
			return nil
		}
		sc := new(account.StateAccount)
		if err := sc.DecodeForStorage(v); nil != err {
			return err
		}
		nonce = uint64(sc.Nonce)
		return nil
	})
	return nonce
}

func (c *StateCli) GetBalance(addr types.Address) *uint256.Int {
	balance := uint256.NewInt(0)
	_ = c.db.View(c.ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(modules.Account, addr.Bytes())
		if err != nil {
			return err
		}
		if len(v) == 0 {
			return nil
		}
		sc := new(account.StateAccount)
		if err := sc.DecodeForStorage(v); nil != err {
			return err
		}
		balance = &sc.Balance
		return nil
	})
	return balance
}
func (c *StateCli) State(addr types.Address) (*account.StateAccount, error) {
	s := new(account.StateAccount)
	err := c.db.View(c.ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(modules.Account, addr.Bytes())
		if err != nil {
			return err
		}
		if len(v) == 0 {
			return nil
		}

		if err := s.DecodeForStorage(v); nil != err {
			return err
		}
		return nil
	})
	return s, err
}
