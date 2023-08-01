package sync

import (
	"github.com/amazechain/amc/internal/p2p"
	"strings"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
	"google.golang.org/protobuf/proto"
)

var errNilPubsubMessage = errors.New("nil pubsub message")
var errInvalidTopic = errors.New("invalid topic format")

func (s *Service) decodePubsubMessage(msg *pubsub.Message) (ssz.Unmarshaler, error) {
	if msg == nil || msg.Topic == nil || *msg.Topic == "" {
		return nil, errNilPubsubMessage
	}
	topic := *msg.Topic
	//todo
	_, err := p2p.ExtractGossipDigest(topic)
	if err != nil {
		return nil, errors.Wrapf(err, "extraction failed for topic: %s", topic)
	}
	topic = strings.TrimSuffix(topic, s.cfg.p2p.Encoding().ProtocolSuffix())
	topic, err = s.replaceForkDigest(topic)
	if err != nil {
		return nil, err
	}

	base := p2p.GossipTopicMappings(topic)
	if base == nil {
		return nil, p2p.ErrMessageNotMapped
	}
	m, ok := proto.Clone(base).(ssz.Unmarshaler)
	if !ok {
		return nil, errors.Errorf("message of %T does not support marshaller interface", base)
	}

	if err := s.cfg.p2p.Encoding().DecodeGossip(msg.Data, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Replaces our fork digest with the formatter.
func (_ *Service) replaceForkDigest(topic string) (string, error) {
	subStrings := strings.Split(topic, "/")
	if len(subStrings) != 4 {
		return "", errInvalidTopic
	}
	subStrings[2] = "%x"
	return strings.Join(subStrings, "/"), nil
}
