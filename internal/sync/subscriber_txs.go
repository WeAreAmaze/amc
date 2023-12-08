package sync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/proto"
)

func (s *Service) transactionSubscriber(ctx context.Context, msg proto.Message) error {
	txs, err := transaction.TransactionsFromProtoMessage(msg)
	if nil != err {
		return err
	}

	errs := s.txpool.AddRemotes(txs)
	for _, e := range errs {
		if nil != e {
			log.Warn("add subscribe transaction failed", "error", e)
		}
	}
	return nil
}


func (s *Service) validateTransactionsPubSub(ctx context.Context, pid peer.ID, msg *pubsub.Message) (pubsub.ValidationResult, error) {
	//receivedTime := time.Now()
	// Validation runs on publish (not just subscriptions), so we should approve any message from
	// ourselves.
	if pid == s.cfg.p2p.PeerID() {
		return pubsub.ValidationAccept, nil
	}

	ctx, span := trace.StartSpan(ctx, "sync.validateBlockPubSub")
	defer span.End()

	m, err := s.decodePubsubMessage(msg)
	if err != nil {
		//tracing.AnnotateError(span, err)
		return pubsub.ValidationReject, errors.Wrap(err, "Could not decode message")
	}

	s.validateBlockLock.Lock()
	defer s.validateBlockLock.Unlock()

	txs, ok := m.(*types_pb.Transactions)
	if !ok {
		return pubsub.ValidationReject, errors.New("msg is not types_pb.Transactions")
	}

	transactions, err := transaction.TransactionsFromProtoMessage(txs)
	if nil != err {
		return pubsub.ValidationReject,err
	}

	// filter dup transaction
	var newTxs transaction.Transactions
	for _, tx := range transactions {
		if !s.txpool.Has(tx.Hash()) {
			newTxs = append(newTxs, tx)
		}
	}

	if len(newTxs) == 0 {
		return pubsub.ValidationIgnore, nil
	}

	msg.ValidatorData = transaction.TransactionsToProtoMessage(newTxs)

	return pubsub.ValidationAccept, nil
}