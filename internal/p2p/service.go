// Package p2p defines the network protocol implementation for Ethereum consensus
// used by beacon nodes, including peer discovery using discv5, gossip-sub
// using libp2p, and handing peer lifecycles + handshakes.
package p2p

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/p2p/encoder"
	"github.com/amazechain/amc/internal/p2p/enode"
	"github.com/amazechain/amc/internal/p2p/enr"
	leakybucket "github.com/amazechain/amc/internal/p2p/leaky-bucket"
	"github.com/amazechain/amc/internal/p2p/peers"
	"github.com/amazechain/amc/internal/p2p/peers/scorers"
	"github.com/amazechain/amc/utils"
	"google.golang.org/protobuf/proto"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

//todo
//var _ runtime.Service = (*Service)(nil)

// In the event that we are at our peer limit, we
// stop looking for new peers and instead poll
// for the current peer limit status for the time period
// defined below.
var pollingPeriod = 6 * time.Second

// Refresh rate of ENR set at twice per block.
var refreshRate = 4 * time.Second

// maxBadResponses is the maximum number of bad responses from a peer before we stop talking to it.
const maxBadResponses = 5

// pubsubQueueSize is the size that we assign to our validation queue and outbound message queue for
// gossipsub.
const pubsubQueueSize = 600

// maxDialTimeout is the timeout for a single peer dial.
const maxDialTimeout = 10 * time.Second

// todo
const ttfbTimeout = 5 * time.Second  // TtfbTimeout is the maximum time to wait for first byte of request response (time-to-first-byte).
const respTimeout = 10 * time.Second // RespTimeout is the maximum time for complete response transfer.

// Service for managing peer to peer (p2p) networking.
type Service struct {
	started               bool
	isPreGenesis          bool
	pingMethod            func(ctx context.Context, id peer.ID) error
	cancel                context.CancelFunc
	cfg                   *conf.P2PConfig
	peers                 *peers.Status
	addrFilter            *multiaddr.Filters
	ipLimiter             *leakybucket.Collector
	privKey               *ecdsa.PrivateKey
	pubsub                *pubsub.PubSub
	joinedTopics          map[string]*pubsub.Topic
	joinedTopicsLock      sync.Mutex
	dv5Listener           Listener
	startupErr            error
	ctx                   context.Context
	host                  host.Host
	genesisHash           types.Hash
	genesisValidatorsRoot []byte
	activeValidatorCount  uint64
	ping                  *sync_pb.Ping
}

// NewService initializes a new p2p service compatible with shared.Service interface. No
// connections are made until the Start function is called during the service registry startup.
func NewService(ctx context.Context, genesisHash types.Hash, cfg *conf.P2PConfig) (*Service, error) {
	var err error
	ctx, cancel := context.WithCancel(ctx)
	_ = cancel // govet fix for lost cancel. Cancel is handled in service.Stop().

	s := &Service{
		ctx:          ctx,
		cancel:       cancel,
		cfg:          cfg,
		ping:         &sync_pb.Ping{SeqNumber: 0},
		isPreGenesis: true,
		joinedTopics: make(map[string]*pubsub.Topic, len(gossipTopicMappings)),
		genesisHash:  genesisHash,
	}

	dv5Nodes := parseBootStrapAddrs(s.cfg.BootstrapNodeAddr)
	//
	cfg.Discv5BootStrapAddr = dv5Nodes

	ipAddr := utils.IPAddr()
	s.privKey, err = privKey(s.cfg)
	if err != nil {
		log.Error("Failed to generate p2p private key", "err", err)
		return nil, err
	}
	s.addrFilter, err = configureFilter(s.cfg)
	if err != nil {
		log.Error("Failed to create address filter", "err", err)
		return nil, err
	}
	//todo
	s.ipLimiter = leakybucket.NewCollector(ipLimit, ipBurst, 30*time.Second, true /* deleteEmptyBuckets */)

	opts := s.buildOptions(ipAddr, s.privKey)
	h, err := libp2p.New(opts...)
	if err != nil {
		log.Error("Failed to create p2p host", "err", err)
		return nil, err
	}

	s.host = h
	// Gossipsub registration is done before we add in any new peers
	// due to libp2p's gossipsub implementation not taking into
	// account previously added peers when creating the gossipsub
	// object.
	psOpts := s.pubsubOptions()
	// Set the pubsub global parameters that we require.
	setPubSubParameters()

	gs, err := pubsub.NewGossipSub(s.ctx, s.host, psOpts...)
	if err != nil {
		log.Error("Failed to start pubsub", "err", err)
		return nil, err
	}
	s.pubsub = gs

	s.peers = peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit: int(s.cfg.MaxPeers),
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold:     maxBadResponses,
				DecayInterval: time.Hour,
			},
		},
	})

	// Initialize Data maps.
	//types.InitializeDataMaps()

	return s, nil
}

func (s *Service) GetPing() *sync_pb.Ping {
	return proto.Clone(s.ping).(*sync_pb.Ping)
}

func (s *Service) IncSeqNumber() {
	s.ping.SeqNumber++
}

func (s *Service) GetConfig() *conf.P2PConfig {
	return s.cfg
}

// Start the p2p service.
func (s *Service) Start() {
	if s.started {
		log.Error("Attempted to start p2p service when it was already started")
		return
	}

	s.isPreGenesis = false

	var relayNodes []string
	if s.cfg.RelayNodeAddr != "" {
		relayNodes = append(relayNodes, s.cfg.RelayNodeAddr)
		if err := dialRelayNode(s.ctx, s.host, s.cfg.RelayNodeAddr); err != nil {
			log.Error("Could not dial relay node", "err", err)
		}
	}

	if !s.cfg.NoDiscovery {
		ipAddr := utils.IPAddr()
		listener, err := s.startDiscoveryV5(
			ipAddr,
			s.privKey,
		)
		if err != nil {
			log.Crit("Failed to start discovery", "err", err)
			s.startupErr = err
			return
		}
		err = s.connectToBootnodes()
		if err != nil {
			log.Error("Could not add bootnode to the exclusion list", "err", err)
			s.startupErr = err
			return
		}
		s.dv5Listener = listener
		go s.listenForNewNodes()
	}

	s.started = true

	if len(s.cfg.StaticPeers) > 0 {
		addrs, err := PeersFromStringAddrs(s.cfg.StaticPeers)
		if err != nil {
			log.Error("Could not connect to static peer", "err", err)
		}
		s.connectWithAllPeers(addrs)
	}
	// Initialize metadata according to the
	// current epoch.
	s.RefreshENR()

	// Periodic functions.
	if len(relayNodes) > 0 {
		utils.RunEvery(s.ctx, ttfbTimeout, func() {
			ensurePeerConnections(s.ctx, s.host, relayNodes...)
		})
	}

	utils.RunEvery(s.ctx, 30*time.Minute, s.Peers().Prune)
	utils.RunEvery(s.ctx, 10*time.Second, s.updateMetrics)
	utils.RunEvery(s.ctx, refreshRate, s.RefreshENR)
	utils.RunEvery(s.ctx, 1*time.Minute, func() {
		//utils.RunEvery(s.ctx, 5*time.Second, func() {
		log.Trace("Peer summary", "inbound", len(s.peers.InboundConnected()), "outbound", len(s.peers.OutboundConnected()), "activePeers", len(s.peers.Active()), "disconnectedPeers", len(s.peers.Disconnected()))
		s.peers.Prune()
		for _, p := range s.peers.Active() {
			//addr, _ := s.peers.Address(p)
			//IP, _ := s.peers.IP(p)
			//ENR, _ := s.peers.ENR(p)
			dialArgs, _ := s.peers.DialArgs(p)
			direction, _ := s.peers.Direction(p)
			connState, _ := s.peers.ConnState(p)
			chainState, err := s.peers.ChainState(p)
			if nil != err {
				log.Error("chainState", "error", err)
			}
			// hexutil.Encode([]byte(p))

			log.Debug("Peer details", "perrId", p, "dialArgs", dialArgs, "Direction", direction, "connState", connState, "currentHeight", utils.ConvertH256ToUint256Int(chainState.CurrentHeight).Uint64())
			pids, _ := s.host.Peerstore().SupportsProtocols(p, s.host.Mux().Protocols()...)
			for _, id := range pids {
				log.Trace("Protocol details:", "ProtocolID", id)
			}
		}

		allNodes := s.dv5Listener.AllNodes()
		log.Trace("Nodes stored in the discovery table:")
		for i, n := range allNodes {
			log.Trace(fmt.Sprintf("P2P details %d", i), "ENR", n.String(), "Node ID", n.ID(), "IP", n.IP(), "UDP", n.UDP(), "TCP", n.TCP())
		}
	})

	multiAddrs := s.host.Network().ListenAddresses()
	logIPAddr(s.host.ID(), multiAddrs...)

	p2pHostAddress := s.cfg.HostAddress
	p2pTCPPort := s.cfg.TCPPort

	if p2pHostAddress != "" {
		logExternalIPAddr(s.host.ID(), p2pHostAddress, p2pTCPPort)
		verifyConnectivity(p2pHostAddress, p2pTCPPort, "tcp")
	}

	p2pHostDNS := s.cfg.HostDNS
	if p2pHostDNS != "" {
		logExternalDNSAddr(s.host.ID(), p2pHostDNS, p2pTCPPort)
	}
	//todo
	//go s.forkWatcher()
}

// Stop the p2p service and terminate all peer connections.
func (s *Service) Stop() error {
	defer s.cancel()
	s.started = false
	if s.dv5Listener != nil {
		s.dv5Listener.Close()
	}
	return nil
}

// Status of the p2p service. Will return an error if the service is considered unhealthy to
// indicate that this node should not serve traffic until the issue has been resolved.
func (s *Service) Status() error {
	if s.isPreGenesis {
		return nil
	}
	if !s.started {
		return errors.New("not running")
	}
	if s.startupErr != nil {
		return s.startupErr
	}
	return nil
}

// Started returns true if the p2p service has successfully started.
func (s *Service) Started() bool {
	return s.started
}

// Encoding returns the configured networking encoding.
func (_ *Service) Encoding() encoder.NetworkEncoding {
	return &encoder.SszNetworkEncoder{}
}

// PubSub returns the p2p pubsub framework.
func (s *Service) PubSub() *pubsub.PubSub {
	return s.pubsub
}

// Host returns the currently running libp2p
// host of the service.
func (s *Service) Host() host.Host {
	return s.host
}

// SetStreamHandler sets the protocol handler on the p2p host multiplexer.
// This method is a pass through to libp2pcore.Host.SetStreamHandler.
func (s *Service) SetStreamHandler(topic string, handler network.StreamHandler) {
	s.host.SetStreamHandler(protocol.ID(topic), handler)
}

// PeerID returns the Peer ID of the local peer.
func (s *Service) PeerID() peer.ID {
	return s.host.ID()
}

// Disconnect from a peer.
func (s *Service) Disconnect(pid peer.ID) error {
	return s.host.Network().ClosePeer(pid)
}

// Connect to a specific peer.
func (s *Service) Connect(pi peer.AddrInfo) error {
	return s.host.Connect(s.ctx, pi)
}

// Peers returns the peer status interface.
func (s *Service) Peers() *peers.Status {
	return s.peers
}

// ENR returns the local node's current ENR.
func (s *Service) ENR() *enr.Record {
	if s.dv5Listener == nil {
		return nil
	}
	return s.dv5Listener.Self().Record()
}

// DiscoveryAddresses represents our enr addresses as multiaddresses.
func (s *Service) DiscoveryAddresses() ([]multiaddr.Multiaddr, error) {
	if s.dv5Listener == nil {
		return nil, nil
	}
	return convertToUdpMultiAddr(s.dv5Listener.Self())
}

// AddPingMethod adds the metadata ping rpc method to the p2p service, so that it can
// be used to refresh ENR.
func (s *Service) AddPingMethod(reqFunc func(ctx context.Context, id peer.ID) error) {
	s.pingMethod = reqFunc
}

func (s *Service) pingPeers() {
	if s.pingMethod == nil {
		return
	}
	for _, pid := range s.peers.Connected() {
		go func(id peer.ID) {
			if err := s.pingMethod(s.ctx, id); err != nil {
				log.Debug("Failed to ping peer", "peer", id)
			}
		}(pid)
	}
}

func (s *Service) connectWithAllPeers(multiAddrs []multiaddr.Multiaddr) {
	addrInfos, err := peer.AddrInfosFromP2pAddrs(multiAddrs...)
	if err != nil {
		log.Error("Could not convert to peer address info's from multiaddresses", "err", err)
		return
	}
	for _, info := range addrInfos {
		// make each dial non-blocking
		go func(info peer.AddrInfo) {
			if err := s.connectWithPeer(s.ctx, info); err != nil {
				log.Trace(fmt.Sprintf("Could not connect with peer %s", info.String()), "err", err)
			}
		}(info)
	}
}

func (s *Service) connectWithPeer(ctx context.Context, info peer.AddrInfo) error {
	ctx, span := trace.StartSpan(ctx, "p2p.connectWithPeer")
	defer span.End()

	if info.ID == s.host.ID() {
		log.Warn("bootNode ID == localNode ID")
		return nil
	}
	if s.Peers().IsBad(info.ID) {
		return errors.New("refused to connect to bad peer")
	}
	ctx, cancel := context.WithTimeout(ctx, maxDialTimeout)
	defer cancel()

	log.Debug("start connect", "peer info", info)
	if err := s.host.Connect(ctx, info); err != nil {
		s.Peers().Scorers().BadResponsesScorer().Increment(info.ID)
		return err
	}
	return nil
}

func (s *Service) connectToBootnodes() error {
	nodes := make([]*enode.Node, 0, len(s.cfg.Discv5BootStrapAddr))
	for _, addr := range s.cfg.Discv5BootStrapAddr {
		bootNode, err := enode.Parse(enode.ValidSchemes, addr)
		if err != nil {
			return err
		}
		// do not dial bootnodes with their tcp ports not set
		if err := bootNode.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
			if !enr.IsNotFound(err) {
				log.Error("Could not retrieve tcp port", "err", err)
			}
			continue
		}
		nodes = append(nodes, bootNode)
	}
	multiAddresses := convertToMultiAddr(nodes)
	s.connectWithAllPeers(multiAddresses)
	return nil
}
