package sync

import (
	"context"
	"github.com/amazechain/amc/internal/p2p"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"
	"reflect"
	"runtime/debug"
	"time"

	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	ssz "github.com/prysmaticlabs/fastssz"
	"go.opencensus.io/trace"
)

// Time to first byte timeout. The maximum time to wait for first byte of
// request response (time-to-first-byte). The client is expected to give up if
// they don't receive the first byte within 5 seconds.
//var ttfbTimeout = ttfbTimeout

// respTimeout is the maximum time for complete response transfer.
//var respTimeout = params.BeaconNetworkConfig().RespTimeout

// rpcHandler is responsible for handling and responding to any incoming message.
// This method may return an error to internal monitoring, but the error will
// not be relayed to the peer.
type rpcHandler func(context.Context, interface{}, libp2pcore.Stream) error

// registerRPCHandlers for p2p RPC.
func (s *Service) registerRPCHandlers() {
	s.registerRPC(
		p2p.RPCStatusTopicV1,
		s.statusRPCHandler,
	)
	s.registerRPC(
		p2p.RPCGoodByeTopicV1,
		s.goodbyeRPCHandler,
	)
	s.registerRPC(
		p2p.RPCPingTopicV1,
		s.pingHandler,
	)
	s.registerRPC(
		p2p.RPCBodiesDataTopicV1,
		s.bodiesByRangeRPCHandler,
	)
}

// Remove all Stream handlers
func (s *Service) unregisterHandlers() {
	fullBodiesRangeTopic := p2p.RPCBodiesDataTopicV1 + s.cfg.p2p.Encoding().ProtocolSuffix()
	fullStatusTopic := p2p.RPCStatusTopicV1 + s.cfg.p2p.Encoding().ProtocolSuffix()
	fullGoodByeTopic := p2p.RPCGoodByeTopicV1 + s.cfg.p2p.Encoding().ProtocolSuffix()
	fullPingTopic := p2p.RPCPingTopicV1 + s.cfg.p2p.Encoding().ProtocolSuffix()

	s.cfg.p2p.Host().RemoveStreamHandler(protocol.ID(fullBodiesRangeTopic))
	s.cfg.p2p.Host().RemoveStreamHandler(protocol.ID(fullStatusTopic))
	s.cfg.p2p.Host().RemoveStreamHandler(protocol.ID(fullGoodByeTopic))
	s.cfg.p2p.Host().RemoveStreamHandler(protocol.ID(fullPingTopic))
}

// registerRPC for a given topic with an expected protobuf message type.
func (s *Service) registerRPC(baseTopic string, handle rpcHandler) {
	topic := baseTopic + s.cfg.p2p.Encoding().ProtocolSuffix()
	s.cfg.p2p.SetStreamHandler(topic, func(stream network.Stream) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic occurred", "topic", topic, "err", r)
				log.Errorf("%s", debug.Stack())
			}
		}()
		ctx, cancel := context.WithTimeout(s.ctx, ttfbTimeout)
		defer cancel()

		// Resetting after closing is a no-op so defer a reset in case something goes wrong.
		// It's up to the handler to Close the stream (send an EOF) if
		// it successfully writes a response. We don't blindly call
		// Close here because we may have only written a partial
		// response.
		defer func() {
			_err := stream.Reset()
			_ = _err
		}()

		ctx, span := trace.StartSpan(ctx, "sync.rpc")
		defer span.End()
		span.AddAttributes(trace.StringAttribute("topic", topic))
		span.AddAttributes(trace.StringAttribute("peer", stream.Conn().RemotePeer().Pretty()))
		//log := log.WithField("peer", stream.Conn().RemotePeer().Pretty()).WithField("topic", string(stream.Protocol()))

		// Check before hand that peer is valid.
		if s.cfg.p2p.Peers().IsBad(stream.Conn().RemotePeer()) {
			if err := s.sendGoodByeAndDisconnect(ctx, p2ptypes.GoodbyeCodeBanned, stream.Conn().RemotePeer()); err != nil {
				log.Debug("Could not disconnect from peer", "peer", stream.Conn().RemotePeer().Pretty(), "topic", stream.Protocol(), "err", err)
			}
			return
		}
		// Validate request according to peer limits.
		if err := s.rateLimiter.validateRawRpcRequest(stream); err != nil {
			log.Debug("Could not validate rpc request from peer", "peer", stream.Conn().RemotePeer().Pretty(), "topic", stream.Protocol(), "err", err)
			return
		}
		s.rateLimiter.addRawStream(stream)

		if err := stream.SetReadDeadline(time.Now().Add(ttfbTimeout)); err != nil {
			log.Debug("Could not set stream read deadline", "peer", stream.Conn().RemotePeer().Pretty(), "topic", stream.Protocol(), "err", err)
			return
		}

		base, ok := p2p.RPCTopicMappings[baseTopic]
		if !ok {
			log.Errorf("Could not retrieve base message for topic %s", baseTopic)
			return
		}
		t := reflect.TypeOf(base)
		// Copy Base
		base = reflect.New(t)

		// Increment message received counter.
		messageReceivedCounter.WithLabelValues(topic).Inc()

		// Given we have an input argument that can be pointer or the actual object, this gives us
		// a way to check for its reflect.Kind and based on the result, we can decode
		// accordingly.
		if t.Kind() == reflect.Ptr {
			msg, ok := reflect.New(t.Elem()).Interface().(ssz.Unmarshaler)
			if !ok {
				log.Errorf("message of %T does not support marshaller interface", msg)
				return
			}
			if err := s.cfg.p2p.Encoding().DecodeWithMaxLength(stream, msg); err != nil {
				log.Debug("Could not decode stream message", "topic", topic, "err", err)
				//tracing.AnnotateError(span, err)
				s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
				return
			}
			if err := handle(ctx, msg, stream); err != nil {
				messageFailedProcessingCounter.WithLabelValues(topic).Inc()
				if err != p2ptypes.ErrWrongForkDigestVersion {
					log.Debug("Could not handle p2p RPC", "err", err)
				}
				//tracing.AnnotateError(span, err)
			}
		} else {
			nTyp := reflect.New(t)
			msg, ok := nTyp.Interface().(ssz.Unmarshaler)
			if !ok {
				log.Errorf("message of %T does not support marshaller interface", msg)
				return
			}
			if err := s.cfg.p2p.Encoding().DecodeWithMaxLength(stream, msg); err != nil {
				log.Debug("Could not decode stream message", "topic", topic, "err", err)
				//tracing.AnnotateError(span, err)
				s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
				return
			}
			if err := handle(ctx, nTyp.Elem().Interface(), stream); err != nil {
				messageFailedProcessingCounter.WithLabelValues(topic).Inc()
				if err != p2ptypes.ErrWrongForkDigestVersion {
					log.Debug("Could not handle p2p RPC", "topic", topic, "err", err)
				}
				//tracing.AnnotateError(span, err)
			}
		}
	})
}
