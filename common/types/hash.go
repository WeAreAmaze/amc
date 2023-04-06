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

	"github.com/amazechain/amc/common/hexutil"

	"hash"
	"math/big"
	"math/rand"
	"reflect"

	"golang.org/x/crypto/sha3"
)

const (
	HashLength = 32
)

var (
	hashT    = reflect.TypeOf(Hash{})
	addressT = reflect.TypeOf(Address{})
	//addressSt = reflect.TypeOf(Address32{})
)

type Hash [HashLength]byte

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Big converts a hash to a big integer.
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// SetBytes sets the hash to the value of b.
func (h *Hash) SetBytes(b []byte) error {
	if len(b) != HashLength {
		return fmt.Errorf("invalid bytes len %d", len(b))
	}

	copy(h[:], b[:HashLength])
	return nil
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

// Scan implements Scanner for database/sql.
func (h *Hash) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Hash", src)
	}
	if len(srcB) != HashLength {
		return fmt.Errorf("can't scan []byte of len %d into Hash, want %d", len(srcB), HashLength)
	}
	copy(h[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (h Hash) Value() (driver.Value, error) {
	return h[:], nil
}

func BytesHash(b []byte) Hash {
	h3 := sha3.New256()
	h3.Write(b)
	r := h3.Sum(nil)
	var h Hash
	copy(h[:], r[:HashLength])
	return h
}

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
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

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) HexBytes() []byte {
	s := h.String()
	return []byte(s)
}

func (h *Hash) SetString(s string) error {
	if len(s) != HashLength*2 {
		return fmt.Errorf("invalid string len %v， len(%d)", string(s), len(s))
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

// Format implements fmt.Formatter.
// Hash supports the %v, %s, %v, %x, %X and %d format verbs.
func (h Hash) Format(s fmt.State, c rune) {
	hexb := make([]byte, 2+len(h)*2)
	copy(hexb, "0x")
	hex.Encode(hexb[2:], h[:])

	switch c {
	case 'x', 'X':
		if !s.Flag('#') {
			hexb = hexb[2:]
		}
		if c == 'X' {
			hexb = bytes.ToUpper(hexb)
		}
		fallthrough
	case 'v', 's':
		s.Write(hexb)
	case 'q':
		q := []byte{'"'}
		s.Write(q)
		s.Write(hexb)
		s.Write(q)
	case 'd':
		fmt.Fprint(s, ([len(h)]byte)(h))
	default:
		fmt.Fprintf(s, "%%!%c(hash=%x)", c, h)
	}
}

// UnmarshalJSON parses a hash in hex syntax.
func (h *Hash) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(hashT, input, h[:])
}

func (h Hash) Equal(other Hash) bool {
	return bytes.Equal(h.Bytes(), other.Bytes())
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

// keccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

type Hasher struct {
	Sha keccakState
}

var hasherPools = make(chan *Hasher, 128)

func NewHasher() *Hasher {
	var h *Hasher
	select {
	case h = <-hasherPools:
	default:
		h = &Hasher{Sha: sha3.NewLegacyKeccak256().(keccakState)}
	}
	return h
}

func ReturnHasherToPool(h *Hasher) {
	select {
	case hasherPools <- h:
	default:
		fmt.Printf("Allowing Hasher to be garbage collected, pool is full\n")
	}
}

func HashData(data []byte) (Hash, error) {
	h := NewHasher()
	defer ReturnHasherToPool(h)
	h.Sha.Reset()

	_, err := h.Sha.Write(data)
	if err != nil {
		return Hash{}, err
	}

	var buf Hash
	_, err = h.Sha.Read(buf[:])
	if err != nil {
		return Hash{}, err
	}
	return buf, nil
}

// Bytes2Hex returns the hexadecimal encoding of d.
func Bytes2Hex(d []byte) string {
	return hex.EncodeToString(d)
}

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(FromHex2Bytes(s)) }

// FromHex returns the bytes represented by the hexadecimal string s.
// s may be prefixed with "0x".
func FromHex2Bytes(s string) []byte {
	if has0xPrefix(s) {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

// Hashes is a slice of common.Hash, implementing sort.Interface
type Hashes []Hash

func (hashes Hashes) Len() int {
	return len(hashes)
}
func (hashes Hashes) Less(i, j int) bool {
	return bytes.Compare(hashes[i][:], hashes[j][:]) == -1
}
func (hashes Hashes) Swap(i, j int) {
	hashes[i], hashes[j] = hashes[j], hashes[i]
}
