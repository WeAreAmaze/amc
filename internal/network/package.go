// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package network

import (
	"bytes"
	"github.com/amazechain/amc/common/message"
	"github.com/libp2p/go-libp2p/core/peer"
	"io"
)

const (
	// Maximum payload size in bytes (256MiB - 1B).
	MaxPayloadSize = (1 << (4 * 7)) - 1
)

type P2PMessage struct {
	MsgType message.MessageType
	Payload []byte
	id      peer.ID
}

func (m *P2PMessage) Type() message.MessageType {
	return m.MsgType
}

func (m *P2PMessage) Encode() ([]byte, error) {
	return m.Payload, nil
}

func (m *P2PMessage) Decode(t message.MessageType, payload []byte) error {
	m.MsgType = t
	m.Payload = payload
	return nil
}

func (m *P2PMessage) Peer() peer.ID {
	return m.id
}

func (m *P2PMessage) Broadcast() bool {
	return false
}

type Message struct {
	msgType message.MessageType
	payload []byte
}

type Header struct {
	DupFlag, Retain bool
}

func (hdr *Header) Encode(w io.Writer, msgType message.MessageType, remainingLength int32) error {
	buf := new(bytes.Buffer)
	err := hdr.encodeInto(buf, msgType, remainingLength)
	if err != nil {
		return err
	}
	_, err = w.Write(buf.Bytes())
	return err
}

func (hdr *Header) encodeInto(buf *bytes.Buffer, msgType message.MessageType, remainingLength int32) error {
	if !msgType.IsValid() {
		return badMsgTypeError
	}

	val := byte(msgType) << 4
	val |= (boolToByte(hdr.DupFlag) << 3)
	val |= boolToByte(hdr.Retain)
	buf.WriteByte(val)
	encodeLength(remainingLength, buf)
	return nil
}

func (hdr *Header) Decode(r io.Reader) (msgType message.MessageType, remainingLength int32, err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	var buf [1]byte

	if _, err = io.ReadFull(r, buf[:]); err != nil {
		return
	}

	byte1 := buf[0]
	msgType = message.MessageType(byte1 & 0xF0 >> 4)

	*hdr = Header{
		DupFlag: byte1&0x08 > 0,
		Retain:  byte1&0x01 > 0,
	}

	remainingLength = decodeLength(r)

	return
}

func boolToByte(val bool) byte {
	if val {
		return byte(1)
	}
	return byte(0)
}

func decodeLength(r io.Reader) int32 {
	var v int32
	var buf [1]byte
	var shift uint
	for i := 0; i < 4; i++ {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			raiseError(err)
		}

		b := buf[0]
		v |= int32(b&0x7f) << shift

		if b&0x80 == 0 {
			return v
		}
		shift += 7
	}

	raiseError(badLengthEncodingError)
	panic("unreachable")
}

func encodeLength(length int32, buf *bytes.Buffer) {
	if length == 0 {
		buf.WriteByte(0)
		return
	}
	for length > 0 {
		digit := length & 0x7f
		length = length >> 7
		if length > 0 {
			digit = digit | 0x80
		}
		buf.WriteByte(byte(digit))
	}
}
