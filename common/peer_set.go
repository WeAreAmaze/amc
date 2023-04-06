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

package common

import (
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p-core/peer"
	"time"
)

type Peer struct {
	IPeer
	CurrentHeight *uint256.Int
	AddTimer      time.Time
}

type PeerSet []Peer
type PeerMap map[peer.ID]Peer

func (pm PeerMap) ToSlice() PeerSet {
	peerSet := PeerSet{}
	for k, _ := range pm {
		peerSet = append(peerSet, pm[k])
	}

	return peerSet
}

func (ps PeerSet) Len() int {
	return len(ps)
}

func (ps PeerSet) Less(i, j int) bool {
	if ps[i].CurrentHeight.Cmp(ps[j].CurrentHeight) == 1 {
		return true
	}
	return false
}

func (ps PeerSet) Swap(i, j int) {
	ps[i], ps[j] = ps[j], ps[i]
}
