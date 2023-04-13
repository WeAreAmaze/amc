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
// 213333333
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

// The structure of Kademlia Distributed Hash Table includes IpfsDHT that is an implementation of Kademlia with S/Kademlia modifications.
// It is used to implement the base Routing module.
type KadDHT struct {
	*kademliaDHT.IpfsDHT

	service *Service

	routingDiscovery *discovery.RoutingDiscovery // calculate the distance of nodes.

	ctx    context.Context
	cancel context.CancelFunc
}

// The function allows the user to specify whether the node is a server or a client, and what peers to bootstrap from.
// It also sets up a routing discovery service for finding other peers in the network.
// The function returns a pointer to the KadDHT object and an error value if any.
func NewKadDht(ctx context.Context, s *Service, isServer bool, bootstrappers ...peer.AddrInfo) (*KadDHT, error) {

	c, cancel := context.WithCancel(ctx)
	var mode kademliaDHT.ModeOpt //  the kademliaDHT of Server or Client
	if isServer {
		mode = kademliaDHT.ModeServer
	} else {
		mode = kademliaDHT.ModeClient
	}
	// create new Distributed Hash Table.
	//creates a new KadDHT struct with the fields initialized from the parameters and the created dht and routingDiscovery.
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

	return kDHT, nil //It returns a pointer to a KadDHT struct and an error value.
}

func (k *KadDHT) Start() error {
	err := k.Bootstrap(k.ctx) //  Bootstrap tells the DHT to get into a bootstrapped state satisfying the IpfsRouter interface and build the connection with other nodes.
	if err != nil {
		log.Error("setup bootstrap failed", err)
		return err
	}

	//todo
	//if err := k.discoverLocal(); err != nil {
	//	log.Error("setup mdns discover failed", err)  //use mdns to discover other nodes of the local net.
	//	return err
	//}

	discovery.Advertise(k.ctx, k.routingDiscovery, DiscoverProtocol) //it advertises the KadDHT instance using its routingDiscovery field and the DiscoverProtocol constant.
	go k.loopDiscoverRemote()                                        //starts a goroutine to loop over the discovered remote peers and handle them.

	return nil
}

// takes a pointer to a KadDHT struct as a receiver. The function creates a timer that executes every 10 seconds.
// This function uses the time package to create a timer, the log package to log messages, and the discovery package to discover peers.
func (k *KadDHT) loopDiscoverRemote() {

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-k.ctx.Done(): // the k.ctx.Done() channel has a value, it means the context has been canceled and the function returns.
			return
		case <-ticker.C:
			peerChan, err := k.routingDiscovery.FindPeers(k.ctx, DiscoverProtocol)
			if err != nil {
				log.Warn("find peers failed", err)
			} else { //iterates over the peer channel, filters out its own ID, prints out the peer information, and calls the k.service.HandlePeerFound method to handle the peer.
				for p := range peerChan {
					if p.ID != k.service.host.ID() {
						log.Tracef("find peer %s", fmt.Sprintf("info: %s", p.String()))
						k.service.HandlePeerFound(p)
					}
				}
			}

			ticker.Reset(60 * time.Second) //resets the timer to 60 seconds
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
