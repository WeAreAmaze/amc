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
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rcrowley/go-metrics"
)

var (
	egressPubsubTrafficMeter  = metrics.GetOrRegisterMeter("p2p/pubsub/egress", nil)
	ingressPubsubTrafficMeter = metrics.GetOrRegisterMeter("p2p/pubsub/ingress", nil)
)

type rawTracer struct {
}

func newRawTracer() *rawTracer {
	return &rawTracer{}
}

func (m rawTracer) AddPeer(p peer.ID, proto protocol.ID)             {}
func (m rawTracer) RemovePeer(p peer.ID)                             {}
func (m rawTracer) Join(topic string)                                {}
func (m rawTracer) Leave(topic string)                               {}
func (m rawTracer) Graft(p peer.ID, topic string)                    {}
func (m rawTracer) Prune(p peer.ID, topic string)                    {}
func (m rawTracer) ValidateMessage(msg *pubsub.Message)              {}
func (m rawTracer) DeliverMessage(msg *pubsub.Message)               {}
func (m rawTracer) RejectMessage(msg *pubsub.Message, reason string) {}
func (m rawTracer) DuplicateMessage(msg *pubsub.Message)             {}
func (m rawTracer) ThrottlePeer(p peer.ID)                           {}
func (m rawTracer) UndeliverableMessage(msg *pubsub.Message)         {}
func (m rawTracer) DropRPC(rpc *pubsub.RPC, p peer.ID)               {}

func (m rawTracer) RecvRPC(rpc *pubsub.RPC) {
	egressPubsubTrafficMeter.Mark(int64(rpc.Size()))
}

func (m rawTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	ingressPubsubTrafficMeter.Mark(int64(rpc.Size()))
}
