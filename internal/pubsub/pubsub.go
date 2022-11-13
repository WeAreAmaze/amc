// Copyright 2022 The AmazeChain Authors
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

package pubsub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/log"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var (
	errorInvalidTopic    = errors.New("invalid topic")
	errorNotRunning      = errors.New("amc pubsub not run")
	errorPubSubIsRunning = errors.New("amc pubsub is running")
)

type AmcPubSub struct {
	topicLock sync.Mutex
	topicsMap map[string]*pubsub.Topic

	p2pserver common.INetwork

	pubsub  *pubsub.PubSub
	running int32

	host host.Host

	ctx context.Context

	chainID uint64
}

func NewPubSub(ctx context.Context, p2pserver common.INetwork, chainid uint64) (common.IPubSub, error) {
	amc := AmcPubSub{
		ctx:       ctx,
		host:      p2pserver.Host(),
		p2pserver: p2pserver,
		running:   0,
		topicsMap: make(map[string]*pubsub.Topic),
		chainID:   chainid,
	}

	return &amc, nil
}

func (m *AmcPubSub) Start() error {
	if m.isRunning() {
		return errorPubSubIsRunning
	}

	atomic.StoreInt32(&m.running, 1)

	var options []pubsub.Option

	options = append(options, pubsub.WithRawTracer(newRawTracer()))
	// todo for test
	if false {
		tracer, err := pubsub.NewJSONTracer("./trace.json")
		if err != nil {
			return err
		}
		options = append(options, pubsub.WithEventTracer(tracer))
	}

	gossip, err := pubsub.NewGossipSub(m.ctx, m.host, options...)
	if err != nil {
		atomic.StoreInt32(&m.running, 0)
		return err
	}

	gossip.GetTopics()

	m.pubsub = gossip

	return nil
}

func (m *AmcPubSub) JoinTopic(topic string) (*pubsub.Topic, error) {
	if !m.isRunning() {
		return nil, errorNotRunning
	}
	m.topicLock.Lock()
	defer m.topicLock.Unlock()
	if t, ok := m.topicsMap[topic]; ok {
		return t, nil
	}

	if _, ok := message.TopicMappings[topic]; ok {
		topicHandle, err := m.pubsub.Join(topic)
		if err != nil {
			return nil, err
		}
		m.topicsMap[topic] = topicHandle
		return topicHandle, nil
	}

	return nil, errorInvalidTopic
}

func (m *AmcPubSub) isRunning() bool {
	if atomic.LoadInt32(&m.running) <= 0 {
		return false
	}
	return true
}

func (m *AmcPubSub) Publish(topic string, msg proto.Message) error {
	if !m.isRunning() {
		return errorNotRunning
	}
	m.topicLock.Lock()
	defer m.topicLock.Unlock()
	if t, ok := m.topicsMap[topic]; ok {
		data, err := proto.Marshal(msg)
		if err != nil {
			log.Errorf("failed to publish topic(%s), data: %s, err: %v", topic, msg.String(), err)
			return err
		}

		return t.Publish(m.ctx, data)
	}

	return errorInvalidTopic
}

func (m *AmcPubSub) Subscription(topic string) (*pubsub.Subscription, error) {
	if !m.isRunning() {
		return nil, errorNotRunning
	}
	m.topicLock.Lock()
	defer m.topicLock.Unlock()
	if t, ok := m.topicsMap[topic]; ok {
		return t.Subscribe()
	}

	return nil, errorInvalidTopic
}

func (m *AmcPubSub) GetTopics() []string {
	var topics []string
	for k, _ := range m.topicsMap {
		topics = append(topics, k)
	}

	return topics
}
