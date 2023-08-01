package p2p

const (
	// GossipProtocolAndDigest represents the protocol and fork digest prefix in a gossip topic.
	GossipProtocolAndDigest = "/amc/%x/"

	// Message Types

	// GossipBlockMessage is the name for the block message type.
	GossipBlockMessage = "block"
	// GossipExitMessage is the name for the voluntary exit message type.
	GossipExitMessage = "voluntary_exit"
	// GossipTransactionMessage is the name for the transaction message type.
	GossipTransactionMessage = "transaction"

	// Topic Formats

	// BlockTopicFormat is the topic format for the block subnet.
	BlockTopicFormat = GossipProtocolAndDigest + GossipBlockMessage
	// ExitBlockTopicFormat is the topic format for the voluntary exit.
	ExitBlockTopicFormat = GossipProtocolAndDigest + GossipExitMessage

	// TransactionTopicFormat is the topic format for the block subnet.
	TransactionTopicFormat = GossipProtocolAndDigest + GossipTransactionMessage
	//ExitTransactionTopicFormat is the topic format for the voluntary exit.
	//ExitTransactionTopicFormat = GossipProtocolAndDigest + GossipExitMessage
)
