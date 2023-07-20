package sync

import (
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
)

// Specifies the fixed size context length.
const forkDigestLength = 4

// writes peer's current context for the expected payload to the stream.
func writeContextToStream(objCtx []byte, stream network.Stream, chain common.IBlockChain) error {
	_, err := stream.Write(objCtx)
	return err
}

// reads any attached context-bytes to the payload.
func readContextFromStream(stream network.Stream) ([]byte, error) {
	// Read context (fork-digest) from stream
	b := make([]byte, forkDigestLength)
	if _, err := stream.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// retrieve expected context depending on rpc topic schema version.
func rpcContext(stream network.Stream, chain common.IBlockChain) ([]byte, error) {
	_, _, version, err := p2p.TopicDeconstructor(string(stream.Protocol()))
	if err != nil {
		return nil, err
	}
	switch version {
	case p2p.SchemaVersionV1:
		// Return empty context for a v1 method.
		return []byte{}, nil
	default:
		return nil, errors.New("invalid version of %s registered for topic: %s")
	}
}

// Minimal interface for a stream with a protocol.
type withProtocol interface {
	Protocol() protocol.ID
}

// Validates that the rpc topic matches the provided version.
func validateVersion(version string, stream withProtocol) error {
	_, _, streamVersion, err := p2p.TopicDeconstructor(string(stream.Protocol()))
	if err != nil {
		return err
	}
	if streamVersion != version {
		return errors.Errorf("stream version of %s doesn't match provided version %s", streamVersion, version)
	}
	return nil
}
