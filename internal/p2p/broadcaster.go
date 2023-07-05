package p2p

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/proto"
)

const (
	//todo
	maxBroadcastTime = 8 * time.Second
)

// ErrMessageNotMapped occurs on a Broadcast attempt when a message has not been defined in the
// GossipTypeMapping.
var ErrMessageNotMapped = errors.New("message type is not mapped to a PubSub topic")

// Broadcast a message to the p2p network, the message is assumed to be
// broadcasted to the current fork.
func (s *Service) Broadcast(ctx context.Context, msg proto.Message) error {
	ctx, span := trace.StartSpan(ctx, "p2p.Broadcast")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, maxBroadcastTime)
	defer cancel()

	forkDigest, err := s.currentForkDigest()
	if err != nil {
		err := errors.Wrap(err, "could not retrieve fork digest")
		//tracing.AnnotateError(span, err)
		return err
	}

	topic, ok := GossipTypeMapping[reflect.TypeOf(msg)]
	if !ok {
		//tracing.AnnotateError(span, ErrMessageNotMapped)
		return ErrMessageNotMapped
	}
	castMsg, ok := msg.(ssz.Marshaler)
	if !ok {
		return errors.Errorf("message of %T does not support marshaller interface", msg)
	}
	return s.broadcastObject(ctx, castMsg, fmt.Sprintf(topic, forkDigest))
}

// method to broadcast messages to other peers in our gossip mesh.
func (s *Service) broadcastObject(ctx context.Context, obj ssz.Marshaler, topic string) error {
	ctx, span := trace.StartSpan(ctx, "p2p.broadcastObject")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("topic", topic))

	buf := new(bytes.Buffer)
	if _, err := s.Encoding().EncodeGossip(buf, obj); err != nil {
		err := errors.Wrap(err, "could not encode message")
		//tracing.AnnotateError(span, err)
		return err
	}

	if span.IsRecordingEvents() {
		//todo
		//id := utils.Hash(buf.Bytes())
		//messageLen := int64(buf.Len())
		//// lint:ignore uintcast -- It's safe to do this for tracing.
		//iid := int64(id)
		//span.AddMessageSendEvent(iid, messageLen /*uncompressed*/, messageLen /*compressed*/)
	}
	if err := s.PublishToTopic(ctx, topic+s.Encoding().ProtocolSuffix(), buf.Bytes()); err != nil {
		err := errors.Wrap(err, "could not publish message")
		//tracing.AnnotateError(span, err)
		return err
	}
	return nil
}
