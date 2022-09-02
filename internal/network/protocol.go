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
	"context"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

const (
	DefaultP2PListenAddress = "/ip4/0.0.0.0/tcp/21324"
	MSGProtocol             = protocol.ID("/amc/msg/1.0.0")
	DiscoverProtocol        = "/amc/discover/1.0.0"
	AppProtocol             = "/amc/app/1.0.0"
	P2ProtocolVersion       = "0.0.1"
)

type discoveryNotifee struct {
	h      host.Host
	ctx    context.Context
	peerCh chan peerInfo
}

func (m *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	select {
	case <-m.ctx.Done():
		return
	default:
		if pi.ID == m.h.ID() {
			log.Warnf("is self peer remote=%s == self=%s", pi.ID.ShortString(), m.h.ID().ShortString())
			return
		}

		log.Debugf("Found %s", pi.ID.String())
		m.peerCh <- peerInfo{
			peer:          pi,
			Connectedness: m.h.Network().Connectedness(pi.ID),
		}
	}
}

type peerInfo struct {
	peer peer.AddrInfo
	network.Connectedness
}

type Handshake func(genesisHash types.Hash, currentHeight uint64) error
