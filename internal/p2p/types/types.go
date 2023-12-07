// Package types contains all the respective p2p types that are required for sync
// but cannot be represented as a protobuf schema. This package also contains those
// types associated fast ssz methods.
package p2ptypes

import (
	"github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
)

const rootLength = 32

const maxErrorLength = 256

// todo
const maxRequestBlocks = 1024

// SSZBytes is a bytes slice that satisfies the fast-ssz interface.
type SSZBytes []byte

// HashTreeRoot hashes the uint64 object following the SSZ standard.
func (b *SSZBytes) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(b)
}

// HashTreeRootWith hashes the uint64 object with the given hasher.
func (b *SSZBytes) HashTreeRootWith(hh *ssz.Hasher) error {
	indx := hh.Index()
	hh.PutBytes(*b)
	hh.Merkleize(indx)
	return nil
}

// BlockByRootsReq specifies the block by roots request type.
type BlockByRootsReq [][rootLength]byte

// MarshalSSZTo marshals the block by roots request with the provided byte slice.
func (r *BlockByRootsReq) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshalledObj, err := r.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	return append(dst, marshalledObj...), nil
}

// MarshalSSZ Marshals the block by roots request type into the serialized object.
func (r *BlockByRootsReq) MarshalSSZ() ([]byte, error) {
	if len(*r) > maxRequestBlocks {
		return nil, errors.Errorf("block by roots request exceeds max size: %d > %d", len(*r), maxRequestBlocks)
	}
	buf := make([]byte, 0, r.SizeSSZ())
	for _, r := range *r {
		buf = append(buf, r[:]...)
	}
	return buf, nil
}

// SizeSSZ returns the size of the serialized representation.
func (r *BlockByRootsReq) SizeSSZ() int {
	return len(*r) * rootLength
}

// UnmarshalSSZ unmarshals the provided bytes buffer into the
// block by roots request object.
func (r *BlockByRootsReq) UnmarshalSSZ(buf []byte) error {
	bufLen := len(buf)
	maxLength := maxRequestBlocks * rootLength
	if bufLen > maxLength {
		return errors.Errorf("expected buffer with length of upto %d but received length %d", maxLength, bufLen)
	}
	if bufLen%rootLength != 0 {
		return ssz.ErrIncorrectByteSize
	}
	numOfRoots := bufLen / rootLength
	roots := make([][rootLength]byte, 0, numOfRoots)
	for i := 0; i < numOfRoots; i++ {
		var rt [rootLength]byte
		copy(rt[:], buf[i*rootLength:(i+1)*rootLength])
		roots = append(roots, rt)
	}
	*r = roots
	return nil
}

// ErrorMessage describes the error message type.
type ErrorMessage []byte

// MarshalSSZTo marshals the error message with the provided byte slice.
func (m *ErrorMessage) MarshalSSZTo(dst []byte) ([]byte, error) {
	marshalledObj, err := m.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	return append(dst, marshalledObj...), nil
}

// MarshalSSZ Marshals the error message into the serialized object.
func (m *ErrorMessage) MarshalSSZ() ([]byte, error) {
	if len(*m) > maxErrorLength {
		return nil, errors.Errorf("error message exceeds max size: %d > %d", len(*m), maxErrorLength)
	}
	buf := make([]byte, m.SizeSSZ())
	copy(buf, *m)
	return buf, nil
}

// SizeSSZ returns the size of the serialized representation.
func (m *ErrorMessage) SizeSSZ() int {
	return len(*m)
}

// UnmarshalSSZ unmarshals the provided bytes buffer into the
// error message object.
func (m *ErrorMessage) UnmarshalSSZ(buf []byte) error {
	bufLen := len(buf)
	maxLength := maxErrorLength
	if bufLen > maxLength {
		return errors.Errorf("expected buffer with length of upto %d but received length %d", maxLength, bufLen)
	}
	errMsg := make([]byte, bufLen)
	copy(errMsg, buf)
	*m = errMsg
	return nil
}
