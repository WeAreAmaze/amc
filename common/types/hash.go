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

package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/internal/avm/common/hexutil"
	"golang.org/x/crypto/sha3"
)

const (
	HashLength = 32
)

type Hash [HashLength]byte

//type HashMap map[Hash]Hash

//func (h HashMap) Marshal() ([]byte, error) {
//	return h.Bytes(), nil
//}
//
//func (h *HashMap) MarshalTo(data []byte) (n int, err error) {
//	copy(data, h.Bytes())
//	return len(h.Bytes()), err
//}
//
//func (h *HashMap) Unmarshal(data []byte) error {
//	return h.SetBytes(data)
//}
//
//func (h HashMap) MarshalJSON() ([]byte, error) {
//	if len(h.Bytes()) <= 0 {
//		return nil, fmt.Errorf("hash is nil")
//	}
//
//	return json.Marshal(h.Bytes())
//}
//
//func (h *HashMap) Size() int {
//	return len(h.Bytes())
//}
//
//func (h *HashMap) UnmarshalJSON(data []byte) error {
//	v := new([]byte)
//	err := json.Unmarshal(data, v)
//	if err != nil {
//		return err
//	}
//
//	return h.Unmarshal(*v)
//}

func BytesToHash(b []byte) Hash {
	h3 := sha3.New256()
	h3.Write(b)
	r := h3.Sum(nil)
	var h Hash
	copy(h[:], r[:HashLength])
	return h
}

func StringToHash(s string) Hash {
	var h Hash
	b, err := hex.DecodeString(s)
	if err == nil {
		//copy(h[:], b[:HashLength])
		return BytesToHash(b)
	}

	return h
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
	//return hexutil.Encode(h[:])
}

func (h Hash) HexBytes() []byte {
	s := h.String()
	return []byte(s)
}

func (h *Hash) SetBytes(b []byte) error {
	if len(b) != HashLength {
		return fmt.Errorf("invalid bytes len %d", len(b))
	}

	copy(h[:], b[:HashLength])
	return nil
}

func (h *Hash) SetString(s string) error {
	if len(s) != HashLength*2 {
		return fmt.Errorf("Invalid string len %v， len(%d)", string(s), len(s))
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	return h.SetBytes(b)
}

func (h Hash) Marshal() ([]byte, error) {
	return h.Bytes(), nil
}

func (h *Hash) MarshalTo(data []byte) (n int, err error) {
	copy(data, h.Bytes())
	return len(h.Bytes()), err
}

func (h *Hash) Unmarshal(data []byte) error {
	return h.SetBytes(data)
}

func (h *Hash) Size() int {
	return len(h.Bytes())
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	v := new([]byte)
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}

	return h.Unmarshal(*v)
}

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x..%x", h[:3], h[29:])
}

func (h Hash) Equal(other Hash) bool {
	return bytes.Equal(h.Bytes(), other.Bytes())
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// HashDifference returns a new set which is the difference between a and b.
func HashDifference(a, b []Hash) []Hash {
	keep := make([]Hash, 0, len(a))

	remove := make(map[Hash]struct{})
	for _, hash := range b {
		remove[hash] = struct{}{}
	}

	for _, hash := range a {
		if _, ok := remove[hash]; !ok {
			keep = append(keep, hash)
		}
	}

	return keep
}
