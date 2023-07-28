package sync

import (
	"fmt"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	types "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/amazechain/amc/internal/p2p/encoder"
	"github.com/amazechain/amc/utils"
	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/pkg/errors"
)

// chunkBlockWriter writes the given message as a chunked response to the given network
// stream.
// response_chunk  ::= <result> | <context-bytes> | <encoding-dependent-header> | <encoded-payload>
func (s *Service) chunkBlockWriter(stream libp2pcore.Stream, blk types.IBlock) error {
	SetStreamWriteDeadline(stream, defaultWriteDuration)
	return WriteBlockChunk(stream, s.cfg.chain, s.cfg.p2p.Encoding(), blk)
}

// WriteBlockChunk writes block chunk object to stream.
// response_chunk  ::= <result> | <context-bytes> | <encoding-dependent-header> | <encoded-payload>
func WriteBlockChunk(stream libp2pcore.Stream, chain common.IBlockChain, encoding encoder.NetworkEncoding, blk types.IBlock) error {
	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}

	digest, err := utils.CreateForkDigest(blk.Number64(), chain.GenesisBlock().Hash())
	if err != nil {
		return err
	}

	if err = writeContextToStream(digest[:], stream, chain); err != nil {
		return err
	}
	protoMsg := blk.ToProtoMessage()
	_, err = encoding.EncodeWithMaxLength(stream, protoMsg.(*types_pb.Block))
	return err
}

// ReadChunkedBlock handles each response chunk that is sent by the
// peer and converts it into a beacon block.
func ReadChunkedBlock(stream libp2pcore.Stream, p2p p2p.EncodingProvider, isFirstChunk bool) (*types_pb.Block, error) {
	// Handle deadlines differently for first chunk
	if isFirstChunk {
		return readFirstChunkedBlock(stream, p2p)
	}

	return readResponseChunk(stream, p2p)
}

// readFirstChunkedBlock reads the first chunked block and applies the appropriate deadlines to
// it.
func readFirstChunkedBlock(stream libp2pcore.Stream, p2p p2p.EncodingProvider) (*types_pb.Block, error) {
	code, errMsg, err := ReadStatusCode(stream, p2p.Encoding())
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s", errMsg)
	}
	_, err = readContextFromStream(stream)
	if err != nil {
		return nil, err
	}
	blk := &types_pb.Block{}
	err = p2p.Encoding().DecodeWithMaxLength(stream, blk)
	return blk, err
}

// readResponseChunk reads the response from the stream and decodes it into the
// provided message type.
func readResponseChunk(stream libp2pcore.Stream, p2p p2p.EncodingProvider) (*types_pb.Block, error) {
	SetStreamReadDeadline(stream, respTimeout)
	code, errMsg, err := readStatusCodeNoDeadline(stream, p2p.Encoding())
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(errMsg)
	}
	// No-op for now with the rpc context. todo
	_, err = readContextFromStream(stream)
	if err != nil {
		return nil, err
	}

	blk := &types_pb.Block{}
	err = p2p.Encoding().DecodeWithMaxLength(stream, blk)
	return blk, err
}
