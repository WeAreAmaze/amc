package sync

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/amazechain/amc/internal/p2p"
	"github.com/amazechain/amc/internal/p2p/encoder"
	p2ptypes "github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/log"

	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
)

var responseCodeSuccess = byte(0x00)
var responseCodeInvalidRequest = byte(0x01)
var responseCodeServerError = byte(0x02)

func (s *Service) generateErrorResponse(code byte, reason string) ([]byte, error) {
	return createErrorResponse(code, reason, s.cfg.p2p)
}

// ReadStatusCode response from a RPC stream.
func ReadStatusCode(stream network.Stream, encoding encoder.NetworkEncoding) (uint8, string, error) {
	// Set ttfb deadline.
	SetStreamReadDeadline(stream, ttfbTimeout)
	b := make([]byte, 1)
	_, err := stream.Read(b)
	if err != nil {
		return 0, "", err
	}

	if b[0] == responseCodeSuccess {
		// Set response deadline on a successful response code.
		SetStreamReadDeadline(stream, respTimeout)

		return 0, "", nil
	}

	// Set response deadline, when reading error message.
	SetStreamReadDeadline(stream, respTimeout)
	msg := &p2ptypes.ErrorMessage{}
	if err := encoding.DecodeWithMaxLength(stream, msg); err != nil {
		return 0, "", err
	}

	return b[0], string(*msg), nil
}

func writeErrorResponseToStream(responseCode byte, reason string, stream libp2pcore.Stream, encoder p2p.EncodingProvider) {
	resp, err := createErrorResponse(responseCode, reason, encoder)
	if err != nil {
		log.Debug("Could not generate a response error", "err", err)
	} else if _, err := stream.Write(resp); err != nil {
		log.Debug("Could not write to stream", "err", err)
	} else {
		// If sending the error message succeeded, close to send an EOF.
		closeStream(stream)
	}
}

func createErrorResponse(code byte, reason string, encoder p2p.EncodingProvider) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{code})
	errMsg := p2ptypes.ErrorMessage(reason)
	if _, err := encoder.Encoding().EncodeWithMaxLength(buf, &errMsg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// reads data from the stream without applying any timeouts.
func readStatusCodeNoDeadline(stream network.Stream, encoding encoder.NetworkEncoding) (uint8, string, error) {
	b := make([]byte, 1)
	_, err := stream.Read(b)
	if err != nil {
		return 0, "", err
	}

	if b[0] == responseCodeSuccess {
		return 0, "", nil
	}

	msg := &p2ptypes.ErrorMessage{}
	if err := encoding.DecodeWithMaxLength(stream, msg); err != nil {
		return 0, "", err
	}

	return b[0], string(*msg), nil
}

// only returns true for errors that are valid (no resets or expectedEOF errors).
func isValidStreamError(err error) bool {
	// check the error message itself as well as libp2p doesn't currently
	// return the correct error type from Close{Read,Write,}.
	return err != nil && !errors.Is(err, network.ErrReset) && err.Error() != network.ErrReset.Error()
}

func closeStream(stream network.Stream) {
	if err := stream.Close(); isValidStreamError(err) {
		log.Debug(fmt.Sprintf("Could not reset stream with protocol %s", stream.Protocol()), "err", err)
	}
}

func closeStreamAndWait(stream network.Stream) {
	if err := stream.CloseWrite(); err != nil {
		_err := stream.Reset()
		_ = _err
		if isValidStreamError(err) {
			log.Debug(fmt.Sprintf("Could not reset stream with protocol %s", stream.Protocol()), "err", err)
		}
		return
	}
	// Wait for the remote side to respond.
	//
	// 1. On success, we expect to read an EOF (remote side received our
	//    response and closed the stream.
	// 2. On failure (e.g., disconnect), we expect to receive an error.
	// 3. If the remote side misbehaves, we may receive data.
	//
	// However, regardless of what happens, we just close the stream and
	// walk away. We only read to wait for a response, we close regardless.
	_, _err := stream.Read([]byte{0})
	_ = _err
	_err = stream.Close()
	_ = _err
}
