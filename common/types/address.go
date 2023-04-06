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
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/amazechain/amc/common/hexutil"
	"github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	AddressLength = 20
	// IncarnationLength length of uint64 for contract incarnations
	IncarnationLength = 2
	// Address32Length is the expected length of the Starknet address (in bytes)
	Address32Length = 32
)

var (
	prefixAddress = "AMC"
	nullAddress   = Address{0}
)

type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) Address { return BytesToAddress(FromHex1(s)) }

func PublicToAddress(key crypto.PubKey) Address {
	bPub, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return Address{0}
	}

	h := sha3.New256()
	h.Write(bPub)
	hash := h.Sum(nil)
	var addr Address
	copy(addr[:], hash[:AddressLength])
	return addr
}

func PrivateToAddress(key crypto.PrivKey) Address {
	return PublicToAddress(key.GetPublic())
}

func HexToString(hexs string) (Address, error) {
	a := Address{0}
	if !strings.HasPrefix(strings.ToUpper(hexs), prefixAddress) {
		return a, fmt.Errorf("invalid prefix address")
	}

	b, err := hex.DecodeString(hexs[len(prefixAddress):])
	if err != nil {
		return a, err
	}

	copy(a[:], b)

	return a, nil
}

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexAddress(s string) bool {
	if has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressLength && isHex(s)
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Hash converts an address to a hash by left-padding it with zeros.
func (a Address) Hash() Hash { return BytesToHash(a[:]) }

// Hex returns an EIP55-compliant hex string representation of the address.
func (a Address) Hex() string {
	return string(a.checksumHex())
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Hex()
}

// Addresses is a slice of common.Address, implementing sort.Interface
type Addresses []Address

func (addrs Addresses) Len() int {
	return len(addrs)
}
func (addrs Addresses) Less(i, j int) bool {
	return bytes.Compare(addrs[i][:], addrs[j][:]) == -1
}
func (addrs Addresses) Swap(i, j int) {
	addrs[i], addrs[j] = addrs[j], addrs[i]
}

func (a *Address) checksumHex() []byte {
	buf := a.hex()

	// compute checksum
	sha := sha3.NewLegacyKeccak256()
	//nolint:errcheck
	sha.Write(buf[2:])
	hash := sha.Sum(nil)
	for i := 2; i < len(buf); i++ {
		hashByte := hash[(i-2)/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if buf[i] > '9' && hashByte > 7 {
			buf[i] -= 32
		}
	}
	return buf
}

func (a Address) hex() []byte {
	var buf [len(a)*2 + 2]byte
	copy(buf[:2], "0x")
	hex.Encode(buf[2:], a[:])
	return buf[:]
}

func (a *Address) DecodeBytes(b []byte) bool {
	if len(b) != AddressLength {
		return false
	}

	copy(a[:], b)
	return true
}

func (a *Address) DecodeString(s string) bool {
	if !strings.HasPrefix(strings.ToUpper(s), prefixAddress) {
		return false
	}

	b, err := hex.DecodeString(s[len(prefixAddress):])
	if err != nil {
		a = &Address{0}
		return false
	}

	copy(a[:], b)
	return true
}

func (a Address) Equal(other Address) bool {
	return bytes.Equal(a[:], other[:])
}

func (a *Address) IsNull() bool {
	return bytes.Equal(a[:], nullAddress[:])
}

func (a Address) Marshal() ([]byte, error) {
	return a.Bytes(), nil
}

func (a *Address) MarshalTo(data []byte) (n int, err error) {
	copy(data, a[:])
	return len(data), nil
}

func (a *Address) Unmarshal(data []byte) error {
	if len(data) != AddressLength {
		return fmt.Errorf("invalid bytes len: %d, hex: %s", len(data), a.Hex())
	}

	copy(a[:], data)
	return nil
}

// SetBytes sets the address to the value of b.
// If b is larger than len(a), b will be cropped from the left.
func (a *Address) SetBytes(b []byte) *Address {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

// Scan implements Scanner for database/sql.
func (a *Address) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Address", src)
	}
	if len(srcB) != AddressLength {
		return fmt.Errorf("can't scan []byte of len %d into Address, want %d", len(srcB), AddressLength)
	}
	copy(a[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (a Address) Value() (driver.Value, error) {
	return a[:], nil
}

func (a Address) Size() int {
	return AddressLength
}

// isHex validates whether each byte is valid hexadecimal string.
func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}

// isHexCharacter returns bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}
