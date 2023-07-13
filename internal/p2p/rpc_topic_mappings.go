package p2p

import (
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"reflect"

	"github.com/pkg/errors"
)

// SchemaVersionV1 specifies the schema version for our rpc protocol ID.
const SchemaVersionV1 = "/1"

// Specifies the protocol prefix for all our Req/Resp topics.
const protocolPrefix = "/rpc"

// StatusMessageName specifies the name for the status message topic.
const StatusMessageName = "/status"

// GoodbyeMessageName specifies the name for the goodbye message topic.
const GoodbyeMessageName = "/goodbye"

// PingMessageName Specifies the name for the ping message topic.
const PingMessageName = "/ping"

// BodiesByRangeMessageName specifies the name for the Bodies by range message topic.
const BodiesByRangeMessageName = "/bodies_by_range"

// HeadersByRangeMessageName specifies the name for the Headers by range message topic.
const HeadersByRangeMessageName = "/headers_by_range"

const (
	// V1 RPC Topics
	// RPCStatusTopicV1 defines the v1 topic for the status rpc method.
	RPCStatusTopicV1 = protocolPrefix + StatusMessageName + SchemaVersionV1
	// RPCGoodByeTopicV1 defines the v1 topic for the goodbye rpc method.
	RPCGoodByeTopicV1 = protocolPrefix + GoodbyeMessageName + SchemaVersionV1
	// RPCPingTopicV1 defines the v1 topic for the ping rpc method.
	RPCPingTopicV1 = protocolPrefix + PingMessageName + SchemaVersionV1

	// RPCBodiesDataTopicV1 defines the v1 topic for the Bodies rpc method.
	RPCBodiesDataTopicV1 = protocolPrefix + BodiesByRangeMessageName + SchemaVersionV1

	// RPCHeadersDataTopicV1 defines the v1 topic for the Headers rpc method.
	RPCHeadersDataTopicV1 = protocolPrefix + HeadersByRangeMessageName + SchemaVersionV1
)

// RPC errors for topic parsing.
const (
	invalidRPCMessageType = "provided message type doesn't have a registered mapping"
)

// RPCTopicMappings map the base message type to the rpc request.
var RPCTopicMappings = map[string]interface{}{
	// RPC Status Message
	RPCStatusTopicV1:     new(sync_pb.Status),
	RPCPingTopicV1:       new(sync_pb.Ping),
	RPCBodiesDataTopicV1: new(types_pb.Block),
}

// Maps all registered protocol prefixes.
var protocolMapping = map[string]bool{
	protocolPrefix: true,
}

// Maps all the protocol message names for the different rpc
// topics.
var messageMapping = map[string]bool{
	StatusMessageName:         true,
	GoodbyeMessageName:        true,
	PingMessageName:           true,
	BodiesByRangeMessageName:  true,
	HeadersByRangeMessageName: true,
}

var versionMapping = map[string]bool{
	SchemaVersionV1: true,
}

// VerifyTopicMapping verifies that the topic and its accompanying
// message type is correct.
func VerifyTopicMapping(topic string, msg interface{}) error {
	msgType, ok := RPCTopicMappings[topic]
	if !ok {
		return errors.New("rpc topic is not registered currently")
	}
	receivedType := reflect.TypeOf(msg)
	registeredType := reflect.TypeOf(msgType)
	typeMatches := registeredType.AssignableTo(receivedType)

	if !typeMatches {
		return errors.Errorf("accompanying message type is incorrect for topic: wanted %v  but got %v",
			registeredType.String(), receivedType.String())
	}
	return nil
}

// TopicDeconstructor splits the provided topic to its logical sub-sections.
// It is assumed all input topics will follow the specific schema:
// /protocol-prefix/message-name/schema-version/...
// For the purposes of deconstruction, only the first 3 components are
// relevant.
func TopicDeconstructor(topic string) (string, string, string, error) {
	origTopic := topic
	protPrefix := ""
	message := ""
	version := ""

	// Iterate through all the relevant mappings to find the relevant prefixes,messages
	// and version for this topic.
	for k := range protocolMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			protPrefix = k
			topic = topic[keyLen:]
		}
	}

	if protPrefix == "" {
		return "", "", "", errors.Errorf("unable to find a valid protocol prefix for %s", origTopic)
	}

	for k := range messageMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			message = k
			topic = topic[keyLen:]
		}
	}

	if message == "" {
		return "", "", "", errors.Errorf("unable to find a valid message for %s", origTopic)
	}

	for k := range versionMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			version = k
			topic = topic[keyLen:]
		}
	}

	if version == "" {
		return "", "", "", errors.Errorf("unable to find a valid schema version for %s", origTopic)
	}

	return protPrefix, message, version, nil
}

// RPCTopic is a type used to denote and represent a req/resp topic.
type RPCTopic string

// ProtocolPrefix returns the protocol prefix of the rpc topic.
func (r RPCTopic) ProtocolPrefix() string {
	prefix, _, _, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return prefix
}

// MessageType returns the message type of the rpc topic.
func (r RPCTopic) MessageType() string {
	_, message, _, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return message
}

// Version returns the schema version of the rpc topic.
func (r RPCTopic) Version() string {
	_, _, version, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return version
}

// TopicFromMessage constructs the rpc topic from the provided message
// type and epoch.
func TopicFromMessage(msg string) (string, error) {
	if !messageMapping[msg] {
		return "", errors.Errorf("%s: %s", invalidRPCMessageType, msg)
	}
	version := SchemaVersionV1

	return protocolPrefix + msg + version, nil
}
