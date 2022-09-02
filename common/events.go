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
	block2 "github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/transaction"
	"github.com/libp2p/go-libp2p-core/peer"
)

// NewLocalTxsEvent local txs
type NewLocalTxsEvent struct{ Txs []*transaction.Transaction }

// PeerJoinEvent Peer join
type PeerJoinEvent struct{ Peer peer.ID }

// PeerDropEvent Peer drop
type PeerDropEvent struct{ Peer peer.ID }

// DownloaderStartEvent start download
type DownloaderStartEvent struct{}

// DownloaderFinishEvent finish download
type DownloaderFinishEvent struct{}

type ChainHighestBlock struct {
	Block    block2.Block
	Inserted bool
}
