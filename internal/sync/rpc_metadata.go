package sync

import (
	"context"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/internal/p2p"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"
	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

// metaDataHandler reads the incoming metadata rpc request from the peer.
func (s *Service) metaDataHandler(_ context.Context, _ interface{}, stream libp2pcore.Stream) error {
	SetRPCStreamDeadlines(stream)

	if err := s.rateLimiter.validateRequest(stream, 1); err != nil {
		return err
	}
	s.rateLimiter.add(stream, 1)

	if s.cfg.p2p.Metadata() == nil {
		nilErr := errors.New("nil metadata stored for host")
		resp, err := s.generateErrorResponse(responseCodeServerError, p2ptypes.ErrGeneric.Error())
		if err != nil {
			log.Debug("Could not generate a response error", "err", err)
		} else if _, err := stream.Write(resp); err != nil {
			log.Debug("Could not write to stream", "err", err)
		}
		return nilErr
	}
	_, _, streamVersion, err := p2p.TopicDeconstructor(string(stream.Protocol()))
	if err != nil {
		resp, genErr := s.generateErrorResponse(responseCodeServerError, p2ptypes.ErrGeneric.Error())
		if genErr != nil {
			log.Debug("Could not generate a response error", "err", genErr)
		} else if _, wErr := stream.Write(resp); wErr != nil {
			log.Debug("Could not write to stream", "err", wErr)
		}
		return err
	}

	var currMd *sync_pb.Metadata

	// todo
	switch streamVersion {
	case p2p.SchemaVersionV1:
		currMd = &sync_pb.Metadata{
			SeqNumber: s.cfg.p2p.MetadataSeq(),
		}
	}

	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}
	_, err = s.cfg.p2p.Encoding().EncodeWithMaxLength(stream, currMd)
	if err != nil {
		return err
	}
	closeStream(stream)
	return nil
}

func (s *Service) sendMetaDataRequest(ctx context.Context, id peer.ID) (*sync_pb.Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	topic, err := p2p.TopicFromMessage(p2p.MetadataMessageName)
	if err != nil {
		return nil, err
	}
	stream, err := s.cfg.p2p.Send(ctx, new(interface{}), topic, id)
	if err != nil {
		return nil, err
	}
	defer closeStream(stream)
	code, errMsg, err := ReadStatusCode(stream, s.cfg.p2p.Encoding())
	if err != nil {
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return nil, err
	}
	if code != 0 {
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return nil, errors.New(errMsg)
	}

	var msg *sync_pb.Metadata
	if err := s.cfg.p2p.Encoding().DecodeWithMaxLength(stream, msg); err != nil {
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return nil, err
	}
	return msg, nil
}
