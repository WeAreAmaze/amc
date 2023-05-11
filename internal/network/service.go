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
	"crypto/rand"
	"github.com/amazechain/amc/api/protocol/msg_proto"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/log"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/utils"
	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/multiformats/go-multiaddr"
	"github.com/rcrowley/go-metrics"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	sTopic = "newBlock"
)

var (
	egressTrafficMeter  = metrics.GetOrRegisterMeter("p2p/egress", nil)
	ingressTrafficMeter = metrics.GetOrRegisterMeter("p2p/ingress", nil)
)

type metricsLog struct{}

func (l metricsLog) Printf(format string, v ...interface{}) {
	log.Debugf(format, v...)
}

type Service struct {
	networkConfig *conf.NetWorkConfig
	dht           *kaddht.IpfsDHT
	ctx           context.Context
	cancel        context.CancelFunc

	host host.Host

	nodes common.PeerMap
	boots []multiaddr.Multiaddr

	lock sync.RWMutex

	removeCh chan peer.ID
	addCh    chan peer.AddrInfo

	bc common.IBlockChain

	handlers map[message.MessageType]common.ConnHandler

	joinedTopics     map[string]*pubsub.Topic
	joinedTopicsLock sync.Mutex

	peerCallback common.ProtocolHandshakeFn
	peerInfo     common.ProtocolHandshakeInfo

	amcPubSub common.IPubSub
}

func NewService(ctx context.Context, config *conf.NetWorkConfig, peers common.PeerMap, callback common.ProtocolHandshakeFn, info common.ProtocolHandshakeInfo) (common.INetwork, error) {
	c, cancel := context.WithCancel(ctx)

	s := Service{
		networkConfig: config,
		ctx:           c,
		cancel:        cancel,
		nodes:         peers,
		removeCh:      make(chan peer.ID, 10),
		addCh:         make(chan peer.AddrInfo, 10),
		peerCallback:  callback,
		peerInfo:      info,
		handlers:      make(map[message.MessageType]common.ConnHandler),
	}

	var peerKey crypto.PrivKey
	var err error
	if len(s.networkConfig.LocalPeerKey) <= 0 {
		peerKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			log.Error("create peer key failed", err)
			return nil, err
		}
	} else {
		peerKey, err = utils.StringToPrivate(s.networkConfig.LocalPeerKey)
		if err != nil {
			log.Errorf("Failed parse string to private key %v", err)
			return nil, err
		}
	}

	h, err := libp2p.New(libp2p.Identity(peerKey), libp2p.ListenAddrStrings(s.networkConfig.ListenersAddress...), libp2p.Security(libp2ptls.ID, libp2ptls.New))
	if err != nil {
		log.Error("create p2p host failed", "err", err)
		return nil, err
	}

	h.SetStreamHandler(MSGProtocol, s.handleStream)
	s.host = h

	return &s, nil
}

func (s *Service) Bootstrapped() bool {
	//return s.networkConfig.Bootstrapped
	return len(s.networkConfig.BootstrapPeers) == 0
}

func (s *Service) Host() host.Host {
	return s.host
}

func (s *Service) Start() error {

	peersInfo := make([]peer.AddrInfo, 0)
	for _, bootstrapPeer := range s.networkConfig.BootstrapPeers {
		peerAddr, err := multiaddr.NewMultiaddr(bootstrapPeer)
		if err != nil {
			log.Warnf("failed parse string to muaddr %v, err %v", bootstrapPeer, err)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			log.Warnf("failed get peer info from muaddr %v, err  %v", peerAddr.String(), err)
			continue
		}

		if peerInfo.ID == s.host.ID() {
			continue
		}
		peersInfo = append(peersInfo, *peerInfo)
		s.boots = append(s.boots, peerAddr)
	}

	kadDHT, err := NewKadDht(s.ctx, s, s.networkConfig.Bootstrapped, peersInfo...)
	if err != nil {
		log.Errorf("failed to new kadDht %v", err)
		return err
	}

	if err := kadDHT.Start(); err != nil {
		return err
	}

	if len(peersInfo) > 0 {
		log.Debug("start connect bootstrap peers")
		s.connectBootsStraps(peersInfo)
	}

	hash, blockNr, _ := s.peerInfo()
	log.Info("local peer", "PeerId", s.host.ID(), "PeerAddress", s.host.Addrs(), "BlockNr", blockNr.Uint64(), "genesisHash", hash)

	go s.nodeManager(s.addCh)

	return nil
}

func (s *Service) PeerCount() int {
	return len(s.nodes)
}

// nodeManager node insert
func (s *Service) nodeManager(peerCh chan peer.AddrInfo) {

	stateTimer := time.NewTicker(60 * time.Second)
	defer stateTimer.Stop()
	defer close(peerCh)

	for {
		select {
		case <-s.ctx.Done():
			return
		case p, ok := <-peerCh:
			if ok {
				if !s.checkNode(p.ID) {
					log.Debug("discover new peer", "PeerID", p.ID, "PeerAddress", p.Addrs)
					if node, err := NewNode(s.ctx, s.host, s, p, s.handlers); err == nil {
						hash, number, err := s.peerInfo()
						if err != nil {
							_ = node.Close()
							continue
						}
						var h msg_proto.ProtocolHandshakeMessage
						if err := node.ProtocolHandshake(&h, AppProtocol, hash, number, true); err != nil {
							log.Warn("cannot Handshake", "PeerID", p.ID, "PeerAddress", p.Addrs, "ProtocolID", AppProtocol, "err", err)
							_ = node.Close()
						} else {
							if cPeer, ok := s.peerCallback(node, utils.ConvertH256ToHash(h.GenesisHash), utils.ConvertH256ToUint256Int(h.CurrentHeight)); ok {
								node.Start()
								s.addNode(cPeer)
								log.Info("connected peer", "peerInfo", p.String(), "blockNumber", utils.ConvertH256ToUint256Int(h.CurrentHeight).Uint64())
								event.GlobalEvent.Send(&common.PeerJoinEvent{Peer: cPeer.ID()})
							} else {
								log.Error("Peer Handshake failed", "PeerID", p.ID, "PeerAddress", p.Addrs)
								_ = node.Close()
							}
						}
					}
				}
			}
		case id, ok := <-s.removeCh:
			if ok {
				log.Infof("delete node id:%s", id.String())
				s.deleteNode(id)
				event.GlobalEvent.Send(&common.PeerDropEvent{Peer: id})
				if !s.checkBootsStrap(id.String()) {
					s.host.Peerstore().RemovePeer(id)
				}
			}
		case <-stateTimer.C:
			s.state()
		}
	}
}

func (s *Service) checkBootsStrap(id string) bool {
	for _, peerAddr := range s.boots {
		peerInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		if peerInfo.ID.String() == id {
			return true
		}
	}
	return false
}

// todo
func (s *Service) connectBootsStraps(bootsStraps []peer.AddrInfo) {

	var wg sync.WaitGroup
	for _, peerInfo := range bootsStraps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.host.Connect(s.ctx, peerInfo); err != nil {
				log.Warn("Connection bootnode failed", "err", err)
			} else {
				log.Info("Connection established with bootnode ", "PeerID", peerInfo.ID, "PeerAddress", peerInfo.Addrs)
			}
		}()
	}
	wg.Wait()
}

func (s *Service) Wait() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT)

	select {
	case <-stop:
		s.host.Close()
		os.Exit(0)
	}
}

func (s *Service) state() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for ID, p := range s.nodes {
		log.Debug("Peer state:", "PeerID", ID, "PeerAddress", "peerCurrentNumber", p.CurrentHeight.Uint64(), p.IPeer.(*Node).Addrs(), "connectTime", common.PrettyDuration(time.Since(p.AddTimer)))
		for protocolID, stream := range p.IPeer.(*Node).streams {
			log.Debug("	stream state:", "protocolID", protocolID, "StreamID", stream.ID(), "connectTime", common.PrettyDuration(time.Since(stream.Stat().Opened)), "connectDirection", stream.Stat().Direction)
		}
	}
}

func (s *Service) addNode(node common.Peer) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.nodes[node.ID()]; !ok {
		s.nodes[node.ID()] = node
	}
}

func (s *Service) deleteNode(id peer.ID) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.nodes[id]; ok {
		delete(s.nodes, id)
	}
}

func (s *Service) checkNode(id peer.ID) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if _, ok := s.nodes[id]; ok {
		return ok
	}

	return false
}

func (s *Service) handleStream(stream network.Stream) {

	if !s.checkNode(stream.Conn().RemotePeer()) {
		log.Info("receive peer stream connect", "streamID", stream.ID(), "protocolID", stream.Protocol(), "PeerID", stream.Conn().RemotePeer())
		p := s.host.Peerstore().PeerInfo(stream.Conn().RemotePeer())

		if node, err := NewNode(s.ctx, s.host, s, p, s.handlers, WithStream(stream)); err != nil {
			stream.Close()
			log.Errorf("failed to new node %v, err %v", p.String(), err)
			return
		} else {
			hash, number, err := s.peerInfo()
			if err != nil {
				log.Errorf("failed to get peer info, err:%v", err)
				return
			}
			var h msg_proto.ProtocolHandshakeMessage
			if err := node.AcceptHandshake(&h, AppProtocol, hash, number); err == nil {
				if cp, ok := s.peerCallback(node, hash, number); ok {
					node.Start()
					s.addNode(cp)
				} else {
					log.Debugf("AcceptHandshake")
				}
			} else {
				log.Debugf("failed accept handshake, err: %v", err)
			}

		}
	}

	log.Debugf("already add node %s", stream.Conn().ID())
}

func (s *Service) HandlePeerFound(p peer.AddrInfo) {
	select {
	case <-s.ctx.Done():
		return
	default:
		if p.ID == s.host.ID() {
			log.Warnf("is self peer remote=%s == self=%s", p.ID.ShortString(), s.host.ID().ShortString())
			return
		}
		s.addCh <- p
	}
}

func (s *Service) SendMsgToPeer(id string, data []byte) error {
	msg := P2PMessage{
		MsgType: message.MsgApplication,
		Payload: data,
	}

	s.lock.RLock()
	defer s.lock.RUnlock()
	if n, ok := s.nodes[peer.ID(id)]; ok {
		return n.Write(&msg)
	}

	return notFoundPeer
}

func (s *Service) ID() string {
	if s.host != nil {
		return s.host.ID().String()
	}
	return ""
}

func (s *Service) WriterMessage(messageType message.MessageType, payload []byte, peer peer.ID) error {
	msg := P2PMessage{
		MsgType: messageType,
		Payload: payload,
	}

	s.lock.RLock()
	defer s.lock.RUnlock()
	if n, ok := s.nodes[peer]; ok {
		return n.Write(&msg)
	}

	return notFoundPeer
}

func (s *Service) SetHandler(mt message.MessageType, handler common.ConnHandler) error {
	if _, ok := s.handlers[mt]; ok {
		return nil
	} else {
		s.handlers[mt] = handler
	}
	return nil
}

func (s *Service) ClosePeer(id peer.ID) error {
	return nil
}
