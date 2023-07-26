package sync

import (
	"fmt"
	"github.com/amazechain/amc/internal/p2p"
	leakybucket "github.com/amazechain/amc/internal/p2p/leaky-bucket"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"
	"github.com/trailofbits/go-mutexasserts"
	"reflect"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
)

const defaultBurstLimit = 5

const leakyBucketPeriod = 1 * time.Second

// Dummy topic to validate all incoming rpc requests.
const rpcLimiterTopic = "rpc-limiter-topic"

type limiter struct {
	limiterMap map[string]*leakybucket.Collector
	p2p        p2p.P2P
	sync.RWMutex
}

// Instantiates a multi-rpc protocol rate limiter, providing
// separate collectors for each topic.
func newRateLimiter(p2pProvider p2p.P2P) *limiter {
	// add encoding suffix
	addEncoding := func(topic string) string {
		return topic + p2pProvider.Encoding().ProtocolSuffix()
	}

	// Initialize block limits.
	allowedBlocksPerSecond := float64(p2pProvider.GetConfig().P2PLimit.BlockBatchLimit)
	allowedBlocksBurst := int64(p2pProvider.GetConfig().P2PLimit.BlockBatchLimitBurstFactor * p2pProvider.GetConfig().P2PLimit.BlockBatchLimit)

	blockLimiterPeriod := time.Duration(p2pProvider.GetConfig().P2PLimit.BlockBatchLimiterPeriod) * time.Second

	// Set topic map for all rpc topics.
	topicMap := make(map[string]*leakybucket.Collector, len(p2p.RPCTopicMappings))
	// Goodbye Message
	topicMap[addEncoding(p2p.RPCGoodByeTopicV1)] = leakybucket.NewCollector(1, 1, leakyBucketPeriod, false /* deleteEmptyBuckets */)
	// Ping Message
	topicMap[addEncoding(p2p.RPCPingTopicV1)] = leakybucket.NewCollector(1, defaultBurstLimit, leakyBucketPeriod, false /* deleteEmptyBuckets */)
	// Status Message
	topicMap[addEncoding(p2p.RPCStatusTopicV1)] = leakybucket.NewCollector(1, defaultBurstLimit, leakyBucketPeriod, false /* deleteEmptyBuckets */)

	// Bodies Message
	topicMap[addEncoding(p2p.RPCBodiesDataTopicV1)] = leakybucket.NewCollector(allowedBlocksPerSecond, allowedBlocksBurst, blockLimiterPeriod, false /* deleteEmptyBuckets */)

	// Headers Message
	topicMap[addEncoding(p2p.RPCHeadersDataTopicV1)] = leakybucket.NewCollector(allowedBlocksPerSecond, allowedBlocksBurst, blockLimiterPeriod, false /* deleteEmptyBuckets */)

	// General topic for all rpc requests.
	topicMap[rpcLimiterTopic] = leakybucket.NewCollector(5, defaultBurstLimit*2, leakyBucketPeriod, false /* deleteEmptyBuckets */)

	return &limiter{limiterMap: topicMap, p2p: p2pProvider}
}

// Returns the current topic collector for the provided topic.
func (l *limiter) topicCollector(topic string) (*leakybucket.Collector, error) {
	l.RLock()
	defer l.RUnlock()
	return l.retrieveCollector(topic)
}

// validates a request with the accompanying cost.
func (l *limiter) validateRequest(stream network.Stream, amt uint64) error {
	l.RLock()
	defer l.RUnlock()

	topic := string(stream.Protocol())

	collector, err := l.retrieveCollector(topic)
	if err != nil {
		return err
	}
	key := stream.Conn().RemotePeer().String()
	remaining := collector.Remaining(key)
	// Treat each request as a minimum of 1.
	if amt == 0 {
		amt = 1
	}
	if amt > uint64(remaining) {
		log.Warn("validate Request failure",
			"key", key,
			"topic", topic,
			"count", amt,
			"remaining", remaining,
		)

		l.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		writeErrorResponseToStream(responseCodeInvalidRequest, p2ptypes.ErrRateLimited.Error(), stream, l.p2p)
		return p2ptypes.ErrRateLimited
	}
	return nil
}

// This is used to validate all incoming rpc streams from external peers.
func (l *limiter) validateRawRpcRequest(stream network.Stream) error {
	l.RLock()
	defer l.RUnlock()

	topic := rpcLimiterTopic

	collector, err := l.retrieveCollector(topic)
	if err != nil {
		return err
	}
	key := stream.Conn().RemotePeer().String()
	remaining := collector.Remaining(key)
	// Treat each request as a minimum of 1.
	amt := int64(1)
	if amt > remaining {
		l.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		writeErrorResponseToStream(responseCodeInvalidRequest, p2ptypes.ErrRateLimited.Error(), stream, l.p2p)
		return p2ptypes.ErrRateLimited
	}
	return nil
}

// adds the cost to our leaky bucket for the topic.
func (l *limiter) add(stream network.Stream, amt int64) {
	l.Lock()
	defer l.Unlock()

	topic := string(stream.Protocol())

	collector, err := l.retrieveCollector(topic)
	if err != nil {
		log.Error(fmt.Sprintf("collector with topic '%s' does not exist", topic), "rate limiter", topic)
		return
	}
	key := stream.Conn().RemotePeer().String()
	collector.Add(key, amt)
}

// adds the cost to our leaky bucket for the peer.
func (l *limiter) addRawStream(stream network.Stream) {
	l.Lock()
	defer l.Unlock()

	topic := rpcLimiterTopic

	collector, err := l.retrieveCollector(topic)
	if err != nil {
		log.Error(fmt.Sprintf("collector with topic '%s' does not exist", topic), "rate limiter", topic)
		return
	}
	key := stream.Conn().RemotePeer().String()
	collector.Add(key, 1)
}

// frees all the collectors and removes them.
func (l *limiter) free() {
	l.Lock()
	defer l.Unlock()

	tempMap := map[uintptr]bool{}
	for t, collector := range l.limiterMap {
		// Check if collector has already been cleared off
		// as all collectors are not distinct from each other.
		ptr := reflect.ValueOf(collector).Pointer()
		if tempMap[ptr] {
			// Remove from map
			delete(l.limiterMap, t)
			continue
		}
		collector.Free()
		// Remove from map
		delete(l.limiterMap, t)
		tempMap[ptr] = true
	}
}

// not to be used outside the rate limiter file as it is unsafe for concurrent usage
// and is protected by a lock on all of its usages here.
func (l *limiter) retrieveCollector(topic string) (*leakybucket.Collector, error) {
	if !mutexasserts.RWMutexLocked(&l.RWMutex) && !mutexasserts.RWMutexRLocked(&l.RWMutex) {
		return nil, errors.New("limiter.retrieveCollector: caller must hold read/write lock")
	}
	collector, ok := l.limiterMap[topic]
	if !ok {
		return nil, errors.Errorf("collector does not exist for topic %s", topic)
	}
	return collector, nil
}
