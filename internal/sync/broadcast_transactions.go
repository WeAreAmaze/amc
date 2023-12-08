package sync

import (
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/log"
)

func (s *Service) broadcastTransactions()  {
	defer close(s.txsCh)
	for {
		select {
		case event := <-s.txsCh:
			if err := s.cfg.p2p.Broadcast(s.ctx,  transaction.TransactionsToProtoMessage(event.Txs)); nil != err {
				log.Warn("broadcast transactions failed", "error", err)
			}
		case <-s.txsSub.Err():
			return
		case <-s.ctx.Done():
			return
		}
	}
}