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
	"errors"
)

var (
	badMsgTypeError        = errors.New("p2p: message type is invalid")
	badLengthEncodingError = errors.New("p2p: remaining length field exceeded maximum of 4 bytes")
	badReturnCodeError     = errors.New("p2p: is invalid")
	dataExceedsPacketError = errors.New("p2p: data exceeds packet length")
	msgTooLongError        = errors.New("p2p: message is too long")
	packageMsgError        = errors.New("p2p: package message error")
	notFoundPeer           = errors.New("p2p: not found peer info")
)

type panicErr struct {
	err error
}

func (p panicErr) Error() string {
	return p.err.Error()
}

func raiseError(err error) {
	panic(panicErr{err})
}

func recoverError(existingErr error, recovered interface{}) error {
	if recovered != nil {
		if pErr, ok := recovered.(panicErr); ok {
			return pErr.err
		} else {
			panic(recovered)
		}
	}
	return existingErr
}
