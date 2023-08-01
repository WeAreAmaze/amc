package encoder

import (
	"io"

	"github.com/pkg/errors"
)

const maxVarintLength = 10

var errExcessMaxLength = errors.Errorf("provided header exceeds the max varint length of %d bytes", maxVarintLength)

// readVarint at the beginning of a byte slice. This varint may be used to indicate
// the length of the remaining bytes in the reader.
func readVarint(r io.Reader) (uint64, error) {
	b := make([]byte, 0, maxVarintLength)
	for i := 0; i < maxVarintLength; i++ {
		b1 := make([]byte, 1)
		n, err := r.Read(b1)
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, errors.New("did not read a byte from stream")
		}
		b = append(b, b1[0])

		// If most significant bit is not set, we have reached the end of the Varint.
		if b1[0]&0x80 == 0 {
			break
		}

		// If the varint is larger than 10 bytes, it is invalid as it would
		// exceed the size of MaxUint64.
		if i+1 >= maxVarintLength {
			return 0, errExcessMaxLength
		}
	}

	vi, n := DecodeVarint(b)
	if n != len(b) {
		return 0, errors.New("varint did not decode entire byte slice")
	}
	return vi, nil
}

// DecodeVarint reads a varint-encoded integer from the slice.
// It returns the integer and the number of bytes consumed, or
// zero if there is not enough.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func DecodeVarint(buf []byte) (x uint64, n int) {
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(buf) {
			return 0, 0
		}
		b := uint64(buf[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n
		}
	}

	// The number is too large to represent in a 64-bit value.
	return 0, 0
}

const maxVarintBytes = 10 // maximum length of a varint

// EncodeVarint returns the varint encoding of x.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
// Not used by the package itself, but helpful to clients
// wishing to use the same encoding.
func EncodeVarint(x uint64) []byte {
	var buf [maxVarintBytes]byte
	var n int
	for n = 0; x > 127; n++ {
		buf[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	buf[n] = uint8(x)
	n++
	return buf[0:n]
}
