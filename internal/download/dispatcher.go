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

package download

import "time"
import "github.com/libp2p/go-libp2p-core/peer"

type Request struct {
	peer *peer.ID
	id   uint64

	sink   chan *Response // Channel to deliver the response on
	cancel chan struct{}  // Channel to cancel requests ahead of time

	code uint64      // Message code of the request packet
	want uint64      // Message code of the response packet
	data interface{} // Data content of the request packet

	Peer string    // Demultiplexer if cross-peer requests are batched together
	Sent time.Time // Timestamp when the request was sent
}

type request struct {
	req  *Request
	fail chan error
}

type Response struct {
	id   uint64    // Request ID to match up this reply to
	recv time.Time // Timestamp when the request was received
	code uint64    // Response packet type to cross validate with request

	Req  *request      // Original request to cross-reference with
	Res  interface{}   // Remote response for the request query
	Meta interface{}   // Metadata generated locally on the receiver thread
	Time time.Duration // Time it took for the request to be served
	Done chan error    // Channel to signal message handling to the reader
}

type response struct {
	res  *Response
	fail chan error
}
