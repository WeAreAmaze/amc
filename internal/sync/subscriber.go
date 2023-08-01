package sync

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/amazechain/amc/internal/p2p/peers"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"runtime/debug"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/proto"
)

const pubsubMessageTimeout = 30 * time.Second

// wrappedVal represents a gossip validator which also returns an error along with the result.
type wrappedVal func(context.Context, peer.ID, *pubsub.Message) (pubsub.ValidationResult, error)

// subHandler represents handler for a given subscription.
type subHandler func(context.Context, proto.Message) error

// noopValidator is a no-op that only decodes the message, but does not check its contents.
func (s *Service) noopValidator(_ context.Context, _ peer.ID, msg *pubsub.Message) (pubsub.ValidationResult, error) {
	m, err := s.decodePubsubMessage(msg)
	if err != nil {
		log.Debug("Could not decode message", "err", err)
		return pubsub.ValidationReject, nil
	}
	msg.ValidatorData = m
	return pubsub.ValidationAccept, nil
}

// Register PubSub subscribers
func (s *Service) registerSubscribers(digest [4]byte) {
	s.subscribe(
		p2p.BlockTopicFormat,
		s.validateBlockPubSub,
		s.blockSubscriber,
		digest,
	)
	//todo txs?
	//s.subscribe(
	//	p2p.TransactionTopicFormat,
	//	digest,
	//)
}

// subscribe to a given topic with a given validator and subscription handler.
// The base protobuf message is used to initialize new messages for decoding.
func (s *Service) subscribe(topic string, validator wrappedVal, handle subHandler, digest [4]byte) *pubsub.Subscription {
	//todo
	//genRoot := s.chain.GenesisBlock().Hash()
	//_, e, err := utils.RetrieveForkDataFromDigest(digest, genRoot[:])
	//if err != nil {
	//	// Impossible condition as it would mean digest does not exist.
	//	panic(err)
	//}
	base := p2p.GossipTopicMappings(topic)
	if base == nil {
		// Impossible condition as it would mean topic does not exist.
		panic(fmt.Sprintf("%s is not mapped to any message in GossipTopicMappings", topic))
	}
	return s.subscribeWithBase(s.addDigestToTopic(topic, digest), validator, handle)
}

func (s *Service) subscribeWithBase(topic string, validator wrappedVal, handle subHandler) *pubsub.Subscription {
	topic += s.cfg.p2p.Encoding().ProtocolSuffix()
	//log := log.WithField("topic", topic)

	// Do not resubscribe already seen subscriptions.
	ok := s.subHandler.topicExists(topic)
	if ok {
		log.Debug(fmt.Sprintf("Provided topic already has an active subscription running: %s", topic), "topic", topic)
		return nil
	}

	if err := s.cfg.p2p.PubSub().RegisterTopicValidator(s.wrapAndReportValidation(topic, validator)); err != nil {
		log.Error("Could not register validator for topic", "topic", topic)
		return nil
	}

	sub, err := s.cfg.p2p.SubscribeToTopic(topic)
	if err != nil {
		// Any error subscribing to a PubSub topic would be the result of a misconfiguration of
		// libp2p PubSub library or a subscription request to a topic that fails to match the topic
		// subscription filter.
		log.Error("Could not subscribe topic", "topic", topic, "err", err)
		return nil
	}
	s.subHandler.addTopic(sub.Topic(), sub)

	// Pipeline decodes the incoming subscription data, runs the validation, and handles the
	// message.
	pipeline := func(msg *pubsub.Message) {
		ctx, cancel := context.WithTimeout(s.ctx, pubsubMessageTimeout)
		defer cancel()
		ctx, span := trace.StartSpan(ctx, "sync.pubsub")
		defer span.End()

		defer func() {
			if r := recover(); r != nil {
				//tracing.AnnotateError(span, fmt.Errorf("panic occurred: %v", r))
				log.Error("Panic occurred", "err", r, "topic", topic)
				debug.PrintStack()
			}
		}()

		span.AddAttributes(trace.StringAttribute("topic", topic))

		if msg.ValidatorData == nil {
			log.Error("Received nil message on pubsub")
			messageFailedProcessingCounter.WithLabelValues(topic).Inc()
			return
		}

		if err := handle(ctx, msg.ValidatorData.(proto.Message)); err != nil {
			//tracing.AnnotateError(span, err)
			log.Error("Could not handle p2p pubsub", "err", err, "topic", topic)
			messageFailedProcessingCounter.WithLabelValues(topic).Inc()
			return
		}
	}

	// The main message loop for receiving incoming messages from this subscription.
	messageLoop := func() {
		for {
			msg, err := sub.Next(s.ctx)
			if err != nil {
				// This should only happen when the context is cancelled or subscription is cancelled.
				if err != pubsub.ErrSubscriptionCancelled { // Only log a warning on unexpected errors.
					log.Warn("Subscription next failed", "err", err)
				}
				// Cancel subscription in the event of an error, as we are
				// now exiting topic event loop.
				sub.Cancel()
				return
			}

			if msg.ReceivedFrom == s.cfg.p2p.PeerID() {
				continue
			}

			go pipeline(msg)
		}
	}

	go messageLoop()
	log.Info("Subscribed to topic", "topic", topic)
	return sub
}

// Wrap the pubsub validator with a metric monitoring function. This function increments the
// appropriate counter if the particular message fails to validate.
func (s *Service) wrapAndReportValidation(topic string, v wrappedVal) (string, pubsub.ValidatorEx) {
	return topic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) (res pubsub.ValidationResult) {
		defer s.handlePanic(ctx, msg)
		res = pubsub.ValidationIgnore // Default: ignore any message that panics.
		ctx, cancel := context.WithTimeout(ctx, pubsubMessageTimeout)
		defer cancel()
		messageReceivedCounter.WithLabelValues(topic).Inc()
		if msg.Topic == nil {
			messageFailedValidationCounter.WithLabelValues(topic).Inc()
			return pubsub.ValidationReject
		}
		retDigest, err := p2p.ExtractGossipDigest(topic)
		if err != nil {
			log.Error(fmt.Sprintf("Invalid topic format of pubsub topic: %v", err), "topic", topic)
			return pubsub.ValidationIgnore
		}
		currDigest, err := s.currentForkDigest()
		if err != nil {
			log.Error(fmt.Sprintf("Unable to retrieve fork data: %v", err), "topic", topic)
			return pubsub.ValidationIgnore
		}
		if currDigest != retDigest {
			log.Debug(fmt.Sprintf("Received message from outdated fork digest %#x", retDigest), "topic", topic)
			return pubsub.ValidationIgnore
		}
		b, err := v(ctx, pid, msg)

		var fields = make([]interface{}, 0)
		fields = append(fields, "topic", topic)
		fields = append(fields, "peer id", pid.String())
		fields = append(fields, "multiaddress", multiAddr(pid, s.cfg.p2p.Peers()))
		fields = append(fields, "agent", agentString(pid, s.cfg.p2p.Host()))
		fields = append(fields, "gossip score", s.cfg.p2p.Peers().Scorers().GossipScorer().Score(pid))

		if b == pubsub.ValidationReject {
			if enableFullSSZDataLogging {
				fields = append(fields, "message", hexutil.Encode(msg.Data))
			}
			fields = append(fields, "err", err)
			log.Debug("Gossip message was rejected", fields...)
			messageFailedValidationCounter.WithLabelValues(topic).Inc()
		}
		if b == pubsub.ValidationIgnore {
			if err != nil {
				log.Debug("Gossip message was ignored", fields...)
			}
			messageIgnoredValidationCounter.WithLabelValues(topic).Inc()
		}
		return b
	}
}

// revalidate that our currently connected subnets are valid.
func (s *Service) reValidateSubscriptions(subscriptions map[uint64]*pubsub.Subscription,
	wantedSubs []uint64, topicFormat string, digest [4]byte) {
	for k, v := range subscriptions {
		var wanted bool
		for _, idx := range wantedSubs {
			if k == idx {
				wanted = true
				break
			}
		}
		if !wanted && v != nil {
			v.Cancel()
			fullTopic := fmt.Sprintf(topicFormat, digest, k) + s.cfg.p2p.Encoding().ProtocolSuffix()
			s.unSubscribeFromTopic(fullTopic)
			delete(subscriptions, k)
		}
	}
}

func (s *Service) unSubscribeFromTopic(topic string) {
	log.Debug("Unsubscribing from topic", "topic", topic)
	if err := s.cfg.p2p.PubSub().UnregisterTopicValidator(topic); err != nil {
		log.Error("Could not unregister topic validator", "err", err)
	}
	sub := s.subHandler.subForTopic(topic)
	if sub != nil {
		sub.Cancel()
	}
	s.subHandler.removeTopic(topic)
	if err := s.cfg.p2p.LeaveTopic(topic); err != nil {
		log.Error("Unable to leave topic", "err", err)
	}
}

// Add fork digest to topic.
func (_ *Service) addDigestToTopic(topic string, digest [4]byte) string {
	if !strings.Contains(topic, "%x") {
		log.Crit("Topic does not have appropriate formatter for digest")
	}
	return fmt.Sprintf(topic, digest)
}

// Add the digest and index to subnet topic.
func (_ *Service) addDigestAndIndexToTopic(topic string, digest [4]byte, idx uint64) string {
	if !strings.Contains(topic, "%x") {
		log.Crit("Topic does not have appropriate formatter for digest")
	}
	return fmt.Sprintf(topic, digest, idx)
}

func (s *Service) currentForkDigest() ([4]byte, error) {
	genRoot := s.cfg.chain.GenesisBlock().Header().Hash()
	return utils.CreateForkDigest(s.cfg.chain.CurrentBlock().Number64(), genRoot)
}

// Checks if the provided digest matches up with the current supposed digest.
func isDigestValid(digest [4]byte, blockNr *uint256.Int, genValRoot types.Hash) (bool, error) {
	retDigest, err := utils.CreateForkDigest(blockNr, genValRoot)
	if err != nil {
		return false, err
	}
	//isNextEpoch, err := utils.CreateForkDigest(new(uint256.Int).AddUint64(blockNr, 1), genValRoot)
	//if err != nil {
	//	return false, err
	//}

	return retDigest == digest, nil
}

func agentString(pid peer.ID, hst host.Host) string {
	agString := ""
	ok := false
	rawVersion, storeErr := hst.Peerstore().Get(pid, "AgentVersion")
	agString, ok = rawVersion.(string)
	if storeErr != nil || !ok {
		agString = ""
	}
	return agString
}

func multiAddr(pid peer.ID, stat *peers.Status) string {
	addrs, err := stat.Address(pid)
	if err != nil || addrs == nil {
		return ""
	}
	return addrs.String()
}
