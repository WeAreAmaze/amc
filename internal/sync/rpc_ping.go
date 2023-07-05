package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/internal/p2p"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"
	"strings"
	"time"

	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/peer"
)

// pingHandler reads the incoming ping rpc message from the peer.
func (s *Service) pingHandler(_ context.Context, msg interface{}, stream libp2pcore.Stream) error {
	SetRPCStreamDeadlines(stream)

	m, ok := msg.(*sync_pb.Ping)
	if !ok {
		return fmt.Errorf("wrong message type for ping, got %T, wanted *uint64", msg)
	}
	if err := s.rateLimiter.validateRequest(stream, 1); err != nil {
		return err
	}
	s.rateLimiter.add(stream, 1)
	valid, err := s.validateSequenceNum(m.GetSeqNumber(), stream.Conn().RemotePeer())
	if err != nil {
		// Descore peer for giving us a bad sequence number.
		if errors.Is(err, p2ptypes.ErrInvalidSequenceNum) {
			s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
			s.writeErrorResponseToStream(responseCodeInvalidRequest, p2ptypes.ErrInvalidSequenceNum.Error(), stream)
		}
		return err
	}
	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}
	sq := &sync_pb.Ping{SeqNumber: s.cfg.p2p.MetadataSeq()}
	if _, err := s.cfg.p2p.Encoding().EncodeWithMaxLength(stream, sq); err != nil {
		return err
	}

	closeStream(stream)

	if valid {
		// If the sequence number was valid we're done.
		return nil
	}

	// The sequence number was not valid.  Start our own ping back to the peer.
	go func() {
		// New context so the calling function doesn't cancel on us.
		ctx, cancel := context.WithTimeout(context.Background(), ttfbTimeout)
		defer cancel()
		md, err := s.sendMetaDataRequest(ctx, stream.Conn().RemotePeer())
		if err != nil {
			// We cannot compare errors directly as the stream muxer error
			// type isn't compatible with the error we have, so a direct
			// equality checks fails.
			if !strings.Contains(err.Error(), p2ptypes.ErrIODeadline.Error()) {
				log.Debug("Could not send metadata request", "peer", stream.Conn().RemotePeer(), "err", err)
			}
			return
		}
		// update metadata if there is no error
		s.cfg.p2p.Peers().SetMetadata(stream.Conn().RemotePeer(), md)
	}()

	return nil
}

func (s *Service) sendPingRequest(ctx context.Context, id peer.ID) error {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	pingReq := &sync_pb.Ping{SeqNumber: s.cfg.p2p.MetadataSeq()}
	topic, err := p2p.TopicFromMessage(p2p.PingMessageName)
	if err != nil {
		return err
	}
	stream, err := s.cfg.p2p.Send(ctx, pingReq, topic, id)
	if err != nil {
		return err
	}
	currentTime := time.Now()
	defer closeStream(stream)

	code, errMsg, err := ReadStatusCode(stream, s.cfg.p2p.Encoding())
	if err != nil {
		return err
	}
	// Records the latency of the ping request for that peer.
	s.cfg.p2p.Host().Peerstore().RecordLatency(id, time.Now().Sub(currentTime))

	if code != 0 {
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return errors.New(errMsg)
	}
	var pingResponse *sync_pb.Ping
	if err := s.cfg.p2p.Encoding().DecodeWithMaxLength(stream, pingResponse); err != nil {
		return err
	}
	valid, err := s.validateSequenceNum(pingResponse.GetSeqNumber(), stream.Conn().RemotePeer())
	if err != nil {
		// Descore peer for giving us a bad sequence number.
		if errors.Is(err, p2ptypes.ErrInvalidSequenceNum) {
			s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		}
		return err
	}
	if valid {
		return nil
	}
	md, err := s.sendMetaDataRequest(ctx, stream.Conn().RemotePeer())
	if err != nil {
		// do not increment bad responses, as its
		// already done in the request method.
		return err
	}
	s.cfg.p2p.Peers().SetMetadata(stream.Conn().RemotePeer(), md)
	return nil
}

// validates the peer's sequence number.
func (s *Service) validateSequenceNum(seq uint64, id peer.ID) (bool, error) {
	md, err := s.cfg.p2p.Peers().Metadata(id)
	if err != nil {
		return false, err
	}
	if md == nil {
		return false, nil
	}
	// Return error on invalid sequence number.
	if md.GetSeqNumber() > uint64(seq) {
		return false, p2ptypes.ErrInvalidSequenceNum
	}
	return md.GetSeqNumber() == uint64(seq), nil
}
