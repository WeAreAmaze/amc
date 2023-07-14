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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"io"
	"sync"
	"time"

	"github.com/amazechain/amc/api/protocol/msg_proto"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/types"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var (
	pingPongTimeOut  = time.Duration(1) * time.Second
	handshakeTimeOut = time.Duration(2) * time.Second
)

type NodeConfig struct {
	Stream     network.Stream
	Protocol   protocol.ID
	CacheCount int
}

type NodeOption func(*NodeConfig)

func WithStream(stream network.Stream) NodeOption {
	return func(config *NodeConfig) {
		config.Stream = stream
	}
}

func WithNodeProtocol(protocol protocol.ID) NodeOption {
	return func(config *NodeConfig) {
		config.Protocol = protocol
	}
}

func WithNodeCacheCount(count int) NodeOption {
	return func(config *NodeConfig) {
		config.CacheCount = count
	}
}

type Node struct {
	host.Host

	peer    *libpeer.AddrInfo
	streams map[string]network.Stream // send and read msg stream protocolID -> stream

	sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	msgCh chan message.IMessage

	service *Service

	config      *NodeConfig
	msgCallback map[message.MessageType]common.ConnHandler
	msgLock     sync.RWMutex

	isOK bool
}

func NewNode(ctx context.Context, h host.Host, s *Service, peer libpeer.AddrInfo, callback map[message.MessageType]common.ConnHandler, opts ...NodeOption) (*Node, error) {
	c, cancel := context.WithCancel(ctx)
	node := &Node{
		Host:        h,
		peer:        &peer,
		streams:     make(map[string]network.Stream),
		ctx:         c,
		cancel:      cancel,
		isOK:        false,
		service:     s,
		msgCallback: callback,
	}

	config := NodeConfig{
		Protocol:   MSGProtocol,
		CacheCount: 100,
	}
	for _, opt := range opts {
		opt(&config)
	}

	var stream network.Stream
	var err error

	node.msgCh = make(chan message.IMessage, config.CacheCount)

	if config.Stream != nil {
		stream = config.Stream
	} else {
		stream, err = node.NewStream(node.ctx, peer.ID, config.Protocol)
		if err != nil {
			log.Error("failed to created stream to peer", "PeerID", peer.ID, "PeerAddress", peer.Addrs, "err", err)
			return nil, err
		}
	}

	node.Lock()
	defer node.Unlock()
	node.streams[string(config.Protocol)] = stream
	node.config = &config

	return node, nil
}

func (n *Node) Start() {
	n.isOK = true
	for _, s := range n.streams {
		go n.readData(s)
		go n.writeData(s)
	}
}

// ProcessHandshake read peer's genesisHash and currentHeight
func (n *Node) ProcessHandshake(h *msg_proto.ProtocolHandshakeMessage) error {
	stream, ok := n.streams[string(n.config.Protocol)]
	if !ok {
		return fmt.Errorf("invalid protocol %stream", n.config.Protocol)
	}

	errCh := make(chan error)
	defer close(errCh)

	go func() {
		var header Header
		reader := bufio.NewReader(stream)
		msgType, payloadLen, err := header.Decode(reader)
		if err != nil {
			log.Error("failed to decode protocol handshake msg", "ID", n.peer.ID, "data", stream, "err", err)
			errCh <- err
			return
		}

		if msgType != message.MsgAppHandshake {
			log.Errorf(badMsgTypeError.Error())
			errCh <- badMsgTypeError
			return
		}

		payload := make([]byte, payloadLen)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			log.Error("failed read payload", fmt.Errorf("payload len %d", payloadLen), err)
			errCh <- err
			return
		}

		var m msg_proto.MessageData
		if err = proto.Unmarshal(payload, &m); err != nil {
			log.Errorf("failed unmarshal to msg, from peer:%s", stream.Conn().ID())
			errCh <- err
			return
		}
		if err = proto.Unmarshal(m.Payload, h); err != nil {
			log.Errorf("failed unmarshal to protocol handshake msessage err: %v", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-n.ctx.Done():
		return n.ctx.Err()
	case <-time.After(handshakeTimeOut):
		log.Warn("Peer handshake timeout", "PeerID", n.peer.ID, "StreamId", stream.ID(), "ProtocolId", n.config.Protocol)
		_ = stream.Close()
	case err, ok := <-errCh:
		if ok {
			if err != nil {
				return err
			}
			return err
		}
	}

	return fmt.Errorf("unknown cause failure")
}

func (n *Node) AcceptHandshake(h *msg_proto.ProtocolHandshakeMessage, version string, genesisHash types.Hash, currentHeight *uint256.Int) error {
	if err := n.ProcessHandshake(h); err != nil {
		return err
	}

	if err := n.ProtocolHandshake(h, version, genesisHash, currentHeight, false); err != nil {
		return err
	}

	return nil
}

// ProtocolHandshake send current peer's genesisHash and height
func (n *Node) ProtocolHandshake(h *msg_proto.ProtocolHandshakeMessage, version string, genesisHash types.Hash, currentHeight *uint256.Int, process bool) error {
	phm := msg_proto.ProtocolHandshakeMessage{
		Version:       version,
		GenesisHash:   utils.ConvertHashToH256(genesisHash),
		CurrentHeight: utils.ConvertUint256IntToH256(currentHeight),
	}

	b, err := proto.Marshal(&phm)
	if err != nil {
		return err
	}

	msg := P2PMessage{
		MsgType: message.MsgAppHandshake,
		Payload: b,
		id:      "",
	}

	s, ok := n.streams[string(n.config.Protocol)]
	if !ok {
		return fmt.Errorf("invalid protocol %s", n.config.Protocol)
	}

	if err := n.writeMsg(s, &msg); err != nil {
		return err
	}

	if process {
		return n.ProcessHandshake(h)
	}

	return nil
}

func (n *Node) SetHandler(msgType message.MessageType, handler common.ConnHandler) error {
	n.Lock()
	defer n.Unlock()

	if _, ok := n.msgCallback[msgType]; ok {
		return nil
	}

	n.msgCallback[msgType] = handler

	return nil
}

func (n *Node) readData(stream network.Stream) error {
	defer func() {
		log.Debugf("quit")
		if n.isOK != false {
			n.isOK = false
			stream.Close()
			n.cancel()
			n.service.removeCh <- stream.Conn().RemotePeer()
		}
	}()
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-n.ctx.Done():
			return stream.Close()
		default:
			//log.Debug("start receive peer msg")
			var header Header
			msgType, payloadLen, err := header.Decode(reader)
			if err != nil {
				log.Error("failed read msg", "StreamID", stream.ID(), "PeerID", stream.Conn().RemotePeer().String(), "err", err)
				return err
			}
			//
			ingressTrafficMeter.Mark(int64(payloadLen))

			payload := make([]byte, payloadLen)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				log.Error("failed read payload", fmt.Errorf("payload len %d", payloadLen), err)
				return err
			}

			//log.Debugf("read %d data, payload len %d", c, payloadLen)
			var msg msg_proto.MessageData
			if err = proto.Unmarshal(payload, &msg); err != nil {
				log.Errorf("failed unmarshal to msg, from peer:%s", stream.Conn().ID())
				return err
			}

			switch msgType {
			case message.MsgPingReq:
				log.Tracef("receive ping msg %s", string(msg.Payload))
				msg := P2PMessage{
					MsgType: message.MsgPingResp,
					Payload: []byte("Hi boy!"),
				}
				n.Write(&msg)
			case message.MsgPingResp:
				log.Tracef("receive pong msg %s", string(msg.Payload))
			default:
				n.msgLock.RLock()

				if h, ok := n.msgCallback[msgType]; ok {
					log.Debug("receive a p2p msg ", "msgType", msgType, "PeerID", n.ID(), "Content", hexutil.Encode(msg.Payload))
					if err := h(msg.Payload, n.ID()); err != nil {
						n.msgLock.RUnlock()
						log.Errorf("failed dispense data err: %v", err)
						return err
					}
				} else {
					log.Warnf("receive invalid msg, err: %v", badMsgTypeError)
				}
				n.msgLock.RUnlock()
			}
		}
	}
}

func (n *Node) makeMsg(payload []byte) proto.Message {
	key := n.Peerstore().PrivKey(n.Host.ID())
	//log.Errorf("id: %v", n.Host.ID())
	sign, err := key.Sign(payload)
	if err != nil {
		log.Error("failed to sign ping msg", "err", err)
		return nil
	}

	nodePubKey, err := crypto.MarshalPublicKey(n.Peerstore().PubKey(n.Host.ID()))
	if err != nil {
		log.Errorf("failed to get public key for sender from local peer store id=%s", n.ID())
		return nil
	}

	msg := msg_proto.MessageData{
		ClientVersion: "",
		Timestamp:     time.Now().Unix(),
		Id:            "",
		NodeID:        n.ID().String(),
		NodePubKey:    nodePubKey,
		Sign:          sign,
		Payload:       payload,
		Gossip:        false,
	}

	return &msg
}

func (n *Node) writeData(stream network.Stream) error {
	defer func() {
		if n.isOK != false {
			n.isOK = false
			n.cancel()
			n.service.removeCh <- stream.Conn().RemotePeer()
			stream.Close()
		}
		close(n.msgCh)
	}()

	ticker := time.NewTicker(pingPongTimeOut)
	defer ticker.Stop()

	writeF := func(msg message.IMessage) error {
		var header Header
		//log.Debugf("send msg to peer[%s] type %d", stream.Conn().ID(), msg.Type())
		payload, err := msg.Encode()
		if err != nil {
			log.Errorf("failed to encode msg")
			return err
		}
		if msgData := n.makeMsg(payload); msgData != nil {
			data, err := proto.Marshal(msgData)
			if err != nil {
				log.Errorf("failed marshal message data to byts")
			} else {
				if err := header.Encode(stream, msg.Type(), int32(len(data))); err != nil {
					log.Error("failed to send header", msg.Type(), len(data), err)
					return err
				} else {
					if _, err := stream.Write(data); err != nil {
						log.Errorf("failed to send payload node:%s", stream.Conn().ID())
					} else {
						egressTrafficMeter.Mark(int64(len(data)))
						//log.Debugf("send %d size to node:%s", c, stream.Conn().ID())
					}
				}
			}
		}
		return nil
	}

	for {
		select {
		case <-n.ctx.Done():
			return stream.Close()
		case m, ok := <-n.msgCh:
			if ok {
				if err := writeF(m); err != nil {
					return err
				}
			} else {
				log.Error("chan was closed")
				return fmt.Errorf("chan ws closed")
			}
		case <-ticker.C:
			msg := P2PMessage{
				MsgType: message.MsgPingReq,
				Payload: []byte("Hello girl"),
			}
			if err := writeF(&msg); err != nil {
				return err
			}
			ticker.Reset(pingPongTimeOut)
		}
	}
}

func (n *Node) writeMsg(stream network.Stream, msg message.IMessage) error {
	var header Header
	payload, err := msg.Encode()
	if err != nil {
		log.Errorf("failed to encode msg")
		return err
	}
	if msgData := n.makeMsg(payload); msgData != nil {
		data, err := proto.Marshal(msgData)
		if err != nil {
			log.Errorf("failed marshal message data to byts")
		} else {
			var buf = new(bytes.Buffer)
			if err := header.Encode(buf, msg.Type(), int32(len(data))); err != nil {
				log.Error("failed to send header", msg.Type(), len(data))
				return err
			} else {
				buf.Write(data)
				if _, err := stream.Write(buf.Bytes()); err != nil {
					log.Errorf("failed to send payload node:%s", stream.Conn().ID())
				} else {
					//Trace
					log.Debug("send data to peer", "data", hexutil.Encode(buf.Bytes()), "PeerID", stream.Conn().RemotePeer(), "ProtocolID", stream.Protocol(), "StreamID", stream.Conn().ID(), "StreamDirection", stream.Stat().Direction)
				}
			}
		}
	}
	return nil
}

func (n *Node) Write(msg message.IMessage) error {
	if !n.isOK {
		return fmt.Errorf("node already closed")
	}

	n.msgCh <- msg
	return nil
}

func (n *Node) WriteMsg(messageType message.MessageType, payload []byte) error {
	if !n.isOK {
		return fmt.Errorf("node already closed")
	}
	//n.msgLock.Lock()
	//defer n.msgLock.Unlock()
	msg := P2PMessage{
		MsgType: messageType,
		Payload: payload,
	}

	//n.msgCh <- &msg

	return n.Write(&msg)
}

func (n *Node) Close() error {
	n.Lock()
	defer n.Unlock()
	if n.isOK {
		n.isOK = false
		n.cancel()
	}
	return nil
}

func (n *Node) ID() libpeer.ID {
	return n.peer.ID
}

func (n *Node) ClearHandler(msgType message.MessageType) error {
	n.msgLock.Lock()
	defer n.msgLock.Unlock()
	if _, ok := n.msgCallback[msgType]; ok {
		delete(n.msgCallback, msgType)
	}
	return fmt.Errorf("failed to remove connhanlder, msg type %v", msgType)
}
