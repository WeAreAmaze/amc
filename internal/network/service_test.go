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
	"crypto/rand"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"testing"
)

func TestNewService(t *testing.T) {
	//blockchain := configs.NetWorkConfig{}
	//NewService(context.TODO(), &blockchain)

}

func TestPeerKey(t *testing.T) {
	peerKey, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	id, err := peer.IDFromPrivateKey(peerKey)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("id = %s", id.String())

	bKey, err := crypto.MarshalPrivateKey(peerKey)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("bKey %v", bKey)
	sKey := crypto.ConfigEncodeKey(bKey)
	t.Logf("sKey %s", sKey)

	bKey2, err := crypto.ConfigDecodeKey(sKey)
	if err != nil {
		t.Fatal(err)
	}

	peerKey2, err := crypto.UnmarshalPrivateKey(bKey2)
	if err != nil {
		t.Fatal(err)
	}

	id2, err := peer.IDFromPrivateKey(peerKey2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("id2 = %s , id1 = %s", id2.String(), id.String())
}

func TestStartService(t *testing.T) {
	//strKey := "CAMSeTB3AgEBBCBZandzyO1LLLNawa6diRUh/A7FTOkxLlHuaIaQJ2piDqAKBggqhkjOPQMBB6FEA0IABEhI/zTNfeaDp31XUoGwUACXZ1HswgWtJGxYoqq9CIxfEvMl3HnMn2ZVcSz2z590k/CpfnrsTLik/8dUiXBfh2U="
	//bKey, err := crypto.ConfigDecodeKey(strKey)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//peerKey, err := crypto.UnmarshalPrivateKey(bKey)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//blockchain := configs.NetWorkConfig{
	//	ListenersAddress: []string{"/ip4/0.0.0.0/tcp/0"},
	//	BootstrapPeers:   nil,
	//	LocalPeerKey:     strKey,
	//}
	//
	////s := NewService(context.TODO(), &blockchain)
	//if s == nil {
	//	t.Fatal("error")
	//}
	//
	//if err := s.Start(); err != nil {
	//	t.Fatal(err)
	//}
}
