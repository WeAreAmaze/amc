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

package transaction

import (
	"github.com/amazechain/amc/common/types"
	"math/big"
)

type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *Transaction) (types.Address, error)

	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(tx *Transaction, sig []byte) (r, s, v types.Int256, err error)
	ChainID() *big.Int

	// Hash returns 'signature hash', i.e. the transaction hash that is signed by the
	// private key. This hash does not uniquely identify the transaction.
	Hash(tx *Transaction) types.Hash

	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}
