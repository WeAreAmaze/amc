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
// 2133
package network

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/log"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	kademliaDHT "github.com/libp2p/go-libp2p-kad-dht"
)

type KadDHT struct {
	*kademliaDHT.IpfsDHT

	service *Service

	routingDiscovery *discovery.RoutingDiscovery

	ctx    context.Context
	cancel context.CancelFunc
}

func NewKadDht(ctx context.Context, s *Service, isServer bool, bootstrappers ...peer.AddrInfo) (*KadDHT, error) {

	c, cancel := context.WithCancel(ctx)
	var mode kademliaDHT.ModeOpt
	if isServer {
		mode = kademliaDHT.ModeServer
	} else {
		mode = kademliaDHT.ModeClient
	}

	dht, err := kademliaDHT.New(c, s.host, kademliaDHT.Mode(mode), kademliaDHT.BootstrapPeers(bootstrappers...))

	if err != nil {
		log.Error("create dht failed", err)
		cancel()
		return nil, err
	}

	kDHT := &KadDHT{
		IpfsDHT:          dht,
		routingDiscovery: discovery.NewRoutingDiscovery(dht),
		ctx:              c,
		cancel:           cancel,
		service:          s,
	}

	return kDHT, nil
}

func (k *KadDHT) Start() error {
	err := k.Bootstrap(k.ctx)
	if err != nil {
		log.Error("setup bootstrap failed", err)
		return err
	}

	//todo
	//if err := k.discoverLocal(); err != nil {
	//	log.Error("setup mdns discover failed", err)
	//	return err
	//}

	discovery.Advertise(k.ctx, k.routingDiscovery, DiscoverProtocol)
	go k.loopDiscoverRemote()

	return nil
}

func (k *KadDHT) loopDiscoverRemote() {

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			peerChan, err := k.routingDiscovery.FindPeers(k.ctx, DiscoverProtocol)
			if err != nil {
				log.Warn("find peers failed", err)
			} else {
				for p := range peerChan {
					if p.ID != k.service.host.ID() {
						log.Tracef("find peer %s", fmt.Sprintf("info: %s", p.String()))
						k.service.HandlePeerFound(p)
					}
				}
			}

			ticker.Reset(60 * time.Second)
		}
	}
}

func (k *KadDHT) findPeers() []peer.AddrInfo {
	peers, err := discovery.FindPeers(k.ctx, k.routingDiscovery, DiscoverProtocol)
	if err != nil {
		return nil
	} else {
		return peers
	}
}

func (k *KadDHT) discoverLocal() error {

	//m := mdns.NewMdnsService(k.service.host, "", k.service.HandlePeerFound)
	//if err := m.Start(); err != nil {
	//	log.Error("mdns start failed", err)
	//	return err
	//}
	//
	return nil
}
