package p2p

import (
	"github.com/holiman/uint256"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// gossipTopicMappings represent the protocol ID to protobuf message type map for easy
// lookup.
var gossipTopicMappings = map[string]proto.Message{}

// GossipTopicMappings is a function to return the assigned data type
// versioned by epoch.
func GossipTopicMappings(topic string, number *uint256.Int) proto.Message {

	return gossipTopicMappings[topic]
}

// AllTopics returns all topics stored in our
// gossip mapping.
func AllTopics() []string {
	var topics []string
	for k := range gossipTopicMappings {
		topics = append(topics, k)
	}
	return topics
}

// GossipTypeMapping is the inverse of GossipTopicMappings so that an arbitrary protobuf message
// can be mapped to a protocol ID string.
var GossipTypeMapping = make(map[reflect.Type]string, len(gossipTopicMappings))

func init() {
	for k, v := range gossipTopicMappings {
		GossipTypeMapping[reflect.TypeOf(v)] = k
	}
	// Specially handle Altair objects.
	//GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockAltair{})] = BlockSubnetTopicFormat
	// Specially handle Bellatrix objects.
	//GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockBellatrix{})] = BlockSubnetTopicFormat
	// Specially handle Capella objects
	//GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockCapella{})] = BlockSubnetTopicFormat
}
