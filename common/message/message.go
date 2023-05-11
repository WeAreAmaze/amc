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

package message

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	MsgConnect = MessageType(iota + 1)
	MsgPingReq
	MsgPingResp
	MsgSystem
	MsgAppHandshake
	MsgApplication
	MsgDownloader
	MsgNewBlock
	MsgDisconnect
	MsgTransaction
	MsgTypeFirstInvalid
)

type MessageType uint8

func (mt MessageType) IsValid() bool {
	return mt >= MsgConnect && mt < MsgTypeFirstInvalid
}

// String implements the stringer interface.
func (mt MessageType) String() string {
	switch mt {
	case MsgConnect:
		return "Connect"
	case MsgPingReq:
		return "PingRequest"
	case MsgPingResp:
		return "PingResponse"
	case MsgAppHandshake:
		return "AppHandshake"
	case MsgApplication:
		return "MsgApplication"
	case MsgDownloader:
		return "MsgDownloader"
	case MsgTransaction:
		return "MsgTransaction"
	default:
		return "unknown"
	}
}

type IMessage interface {
	Type() MessageType
	Peer() peer.ID
	Broadcast() bool
	Encode() ([]byte, error)
	Decode(MessageType, []byte) error
}
