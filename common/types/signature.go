package types

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/amazechain/amc/common/hexutil"
	"reflect"
)

const SignatureLength = 96
const PublicKeyLength = 48

var (
	signatureT = reflect.TypeOf(Signature{})
	publicKeyT = reflect.TypeOf(PublicKey{})
)

type Signature [SignatureLength]byte

func (h Signature) Hex() string { return hexutil.Encode(h[:]) }
func (h Signature) String() string {
	return h.Hex()
}

func (h Signature) Size() int {
	return SignatureLength
}

func (h Signature) Bytes() []byte { return h[:] }

func (h *Signature) SetBytes(data []byte) error {
	if len(data) != SignatureLength {
		return fmt.Errorf("invalid bytes len %d", len(data))
	}
	copy(h[:], data[:SignatureLength])
	return nil
}

func (h Signature) Marshal() ([]byte, error) {
	return h.Bytes(), nil
}

func (h *Signature) Unmarshal(data []byte) error {
	return h.SetBytes(data)
}

// Hash supports the %v, %s, %q, %x, %X and %d format verbs.
func (h Signature) Format(s fmt.State, c rune) {
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
		fmt.Fprintf(s, "%%!%c(signature=%x)", c, h)
	}
}

// UnmarshalText parses a hash in hex syntax.
func (h *Signature) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Signature", input, h[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (h *Signature) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(signatureT, input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Signature) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

type PublicKey [PublicKeyLength]byte

func (h PublicKey) Hex() string { return hexutil.Encode(h[:]) }
func (h PublicKey) String() string {
	return h.Hex()
}

func (h PublicKey) Bytes() []byte {
	return h[:]
}

func (h *PublicKey) SetBytes(b []byte) error {
	if len(b) != PublicKeyLength {
		return fmt.Errorf("invalid bytes len %d", len(b))
	}

	copy(h[:], b[:PublicKeyLength])
	return nil
}

func (h PublicKey) Size() int {
	return PublicKeyLength
}

func (h PublicKey) Marshal() ([]byte, error) {
	return h.Bytes(), nil
}

func (h *PublicKey) Unmarshal(data []byte) error {
	return h.SetBytes(data)
}

// MarshalText returns the hex representation of a.
func (a PublicKey) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *PublicKey) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("PublicKey", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *PublicKey) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(publicKeyT, input, a[:])
}

// Format implements fmt.Formatter.
// supports the %v, %s, %q, %x, %X and %d format verbs.
func (h PublicKey) Format(s fmt.State, c rune) {
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
		fmt.Fprintf(s, "%%!%c(publickey=%x)", c, h)
	}
}
