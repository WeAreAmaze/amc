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

package utils

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/sha3"
	"os"
)

func Hash256toS(data []byte) string {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	hash := h.Sum(nil)
	return hex.EncodeToString(hash)
}

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()

	for _, b := range data {
		d.Write(b)
	}

	return d.Sum(nil)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h types.Hash) {
	d := sha3.NewLegacyKeccak256()

	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:])
	return h
}

func StringToPrivate(s string) (crypto.PrivKey, error) {
	bKey, err := crypto.ConfigDecodeKey(s)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(bKey)
}

func PrivateToString(key crypto.PrivKey) (string, error) {
	priv, err := crypto.MarshalPrivateKey(key)
	if err != nil {
		return "", err
	}

	return crypto.ConfigEncodeKey(priv), nil
}

func PublicToString(key crypto.PubKey) (string, error) {
	pub, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return "", err
	}
	return crypto.ConfigEncodeKey(pub), nil
}

func StringToPublic(s string) (crypto.PubKey, error) {
	b, err := crypto.ConfigDecodeKey(s)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPublicKey(b)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}

	return true
}
func MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}

	return nil
}

func HexPrefix(a, b []byte) ([]byte, int) {
	var i, length = 0, len(a)
	if len(b) < length {
		length = len(b)
	}

	for ; i < length; i++ {
		if a[i] != b[i] {
			break
		}
	}

	return a[:i], i
}

type appKey struct{}

type AppInfo interface {
	ID() string
	Name() string
	Version() string
	Amcdata() map[string]string
	Endpoint() []string
}

// NewContext returns a new Context that carries value.
func NewContext(ctx context.Context, s AppInfo) context.Context {
	return context.WithValue(ctx, appKey{}, s)
}

// FromContext returns the Transport value stored in ctx, if any.
func FromContext(ctx context.Context) (s AppInfo, ok bool) {
	s, ok = ctx.Value(appKey{}).(AppInfo)
	return
}

func Copy(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func ConvertH256ToUint256Int(h256 *types_pb.H256) *uint256.Int {
	// Note: uint256.Int is an array of 4 uint64 in little-endian order, i.e. most significant word is [3]
	var i uint256.Int
	i[3] = h256.Hi.Hi
	i[2] = h256.Hi.Lo
	i[1] = h256.Lo.Hi
	i[0] = h256.Lo.Lo
	return &i
}

func ConvertUint256IntToH256(i *uint256.Int) *types_pb.H256 {
	// Note: uint256.Int is an array of 4 uint64 in little-endian order, i.e. most significant word is [3]
	return &types_pb.H256{
		Lo: &types_pb.H128{Lo: i[0], Hi: i[1]},
		Hi: &types_pb.H128{Lo: i[2], Hi: i[3]},
	}
}

func ConvertH256ToHash(h256 *types_pb.H256) [32]byte {
	var hash [32]byte
	binary.BigEndian.PutUint64(hash[0:], h256.Hi.Hi)
	binary.BigEndian.PutUint64(hash[8:], h256.Hi.Lo)
	binary.BigEndian.PutUint64(hash[16:], h256.Lo.Hi)
	binary.BigEndian.PutUint64(hash[24:], h256.Lo.Lo)
	return hash
}

func ConvertH512ToHash(h512 *types_pb.H512) [64]byte {
	var b [64]byte
	binary.BigEndian.PutUint64(b[0:], h512.Hi.Hi.Hi)
	binary.BigEndian.PutUint64(b[8:], h512.Hi.Hi.Lo)
	binary.BigEndian.PutUint64(b[16:], h512.Hi.Lo.Hi)
	binary.BigEndian.PutUint64(b[24:], h512.Hi.Lo.Lo)
	binary.BigEndian.PutUint64(b[32:], h512.Lo.Hi.Hi)
	binary.BigEndian.PutUint64(b[40:], h512.Lo.Hi.Lo)
	binary.BigEndian.PutUint64(b[48:], h512.Lo.Lo.Hi)
	binary.BigEndian.PutUint64(b[56:], h512.Lo.Lo.Lo)
	return b
}

func ConvertHashesToH256(hashes []types.Hash) []*types_pb.H256 {
	res := make([]*types_pb.H256, len(hashes))
	for i := range hashes {
		res[i] = ConvertHashToH256(hashes[i])
	}
	return res
}

func Uint256sToH256(u256s []uint256.Int) []*types_pb.H256 {
	p := make([]*types_pb.H256, len(u256s))
	for i, u := range u256s {
		p[i] = ConvertUint256IntToH256(&u)
	}
	return p
}

func H256sToHashes(h256s []*types_pb.H256) []types.Hash {
	p := make([]types.Hash, len(h256s))
	for i, h := range h256s {
		p[i] = ConvertH256ToHash(h)
	}
	return p
}

func ConvertHashToH256(hash [32]byte) *types_pb.H256 {
	return &types_pb.H256{
		Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(hash[24:]), Hi: binary.BigEndian.Uint64(hash[16:])},
		Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(hash[8:]), Hi: binary.BigEndian.Uint64(hash[0:])},
	}
}

func ConvertHashToH512(hash [64]byte) *types_pb.H512 {
	return ConvertBytesToH512(hash[:])
}

func ConvertH160toAddress(h160 *types_pb.H160) [20]byte {
	var addr [20]byte
	binary.BigEndian.PutUint64(addr[0:], h160.Hi.Hi)
	binary.BigEndian.PutUint64(addr[8:], h160.Hi.Lo)
	binary.BigEndian.PutUint32(addr[16:], h160.Lo)
	return addr
}

func ConvertH160ToPAddress(h160 *types_pb.H160) *types.Address {
	p := new(types.Address)
	addr := ConvertH160toAddress(h160)
	p.SetBytes(addr[:])
	return p
}

func ConvertAddressToH160(addr [20]byte) *types_pb.H160 {
	return &types_pb.H160{
		Lo: binary.BigEndian.Uint32(addr[16:]),
		Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(addr[8:]), Hi: binary.BigEndian.Uint64(addr[0:])},
	}
}

func ConvertAddrsToH160(ps []types.Address) []*types_pb.H160 {
	b := make([]*types_pb.H160, len(ps))
	for i, p := range ps {
		b[i] = ConvertAddressToH160(p)
	}
	return b
}

func H160sToAddress(ps []*types_pb.H160) []types.Address {
	b := make([]types.Address, len(ps))
	for i, p := range ps {
		b[i] = ConvertH160toAddress(p)
	}
	return b
}

func ConvertH512ToBytes(h512 *types_pb.H512) []byte {
	b := ConvertH512ToHash(h512)
	return b[:]
}

func ConvertBytesToH512(b []byte) *types_pb.H512 {
	if len(b) < 64 {
		var b1 [64]byte
		copy(b1[:], b)
		b = b1[:]
	}
	return &types_pb.H512{
		Lo: &types_pb.H256{
			Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(b[56:]), Hi: binary.BigEndian.Uint64(b[48:])},
			Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(b[40:]), Hi: binary.BigEndian.Uint64(b[32:])},
		},
		Hi: &types_pb.H256{
			Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(b[24:]), Hi: binary.BigEndian.Uint64(b[16:])},
			Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(b[8:]), Hi: binary.BigEndian.Uint64(b[0:])},
		},
	}
}

func ConvertH384ToPublicKey(h384 *types_pb.H384) [48]byte {
	var pub [48]byte
	binary.BigEndian.PutUint64(pub[0:], h384.Hi.Hi.Hi)
	binary.BigEndian.PutUint64(pub[8:], h384.Hi.Hi.Lo)
	binary.BigEndian.PutUint64(pub[16:], h384.Hi.Lo.Hi)
	binary.BigEndian.PutUint64(pub[24:], h384.Hi.Lo.Lo)
	binary.BigEndian.PutUint64(pub[32:], h384.Lo.Hi)
	binary.BigEndian.PutUint64(pub[40:], h384.Lo.Lo)
	return pub
}

func ConvertPublicKeyToH384(p [48]byte) *types_pb.H384 {
	return &types_pb.H384{
		Lo: &types_pb.H128{
			Lo: binary.BigEndian.Uint64(p[40:]),
			Hi: binary.BigEndian.Uint64(p[32:]),
		},
		Hi: &types_pb.H256{
			Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[24:]), Hi: binary.BigEndian.Uint64(p[16:])},
			Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[8:]), Hi: binary.BigEndian.Uint64(p[0:])},
		},
	}
}

func ConvertPubsToH384(ps []types.PublicKey) []*types_pb.H384 {
	b := make([]*types_pb.H384, len(ps))
	for i, p := range ps {
		b[i] = ConvertPublicKeyToH384(p)
	}
	return b
}

func H384sToPubs(ps []*types_pb.H384) []types.PublicKey {
	b := make([]types.PublicKey, len(ps))
	for i, p := range ps {
		b[i] = ConvertH384ToPublicKey(p)
	}
	return b
}
func ConvertH768ToSignature(h768 *types_pb.H768) [96]byte {
	var b [96]byte
	binary.BigEndian.PutUint64(b[0:], h768.Hi.Hi.Hi.Hi)
	binary.BigEndian.PutUint64(b[8:], h768.Hi.Hi.Hi.Lo)
	binary.BigEndian.PutUint64(b[16:], h768.Hi.Hi.Lo.Hi)
	binary.BigEndian.PutUint64(b[24:], h768.Hi.Hi.Lo.Lo)
	binary.BigEndian.PutUint64(b[32:], h768.Hi.Lo.Hi)
	binary.BigEndian.PutUint64(b[40:], h768.Hi.Lo.Lo)

	binary.BigEndian.PutUint64(b[48:], h768.Lo.Hi.Hi.Hi)
	binary.BigEndian.PutUint64(b[56:], h768.Lo.Hi.Hi.Lo)
	binary.BigEndian.PutUint64(b[64:], h768.Lo.Hi.Lo.Hi)
	binary.BigEndian.PutUint64(b[72:], h768.Lo.Hi.Lo.Lo)
	binary.BigEndian.PutUint64(b[80:], h768.Lo.Lo.Hi)
	binary.BigEndian.PutUint64(b[88:], h768.Lo.Lo.Lo)
	return b
}

func ConvertSignatureToH768(p [96]byte) *types_pb.H768 {
	return &types_pb.H768{
		Lo: &types_pb.H384{
			Lo: &types_pb.H128{
				Lo: binary.BigEndian.Uint64(p[88:]),
				Hi: binary.BigEndian.Uint64(p[80:]),
			},
			Hi: &types_pb.H256{
				Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[72:]), Hi: binary.BigEndian.Uint64(p[64:])},
				Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[56:]), Hi: binary.BigEndian.Uint64(p[48:])},
			},
		},
		Hi: &types_pb.H384{
			Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[40:]), Hi: binary.BigEndian.Uint64(p[32:])},
			Hi: &types_pb.H256{
				Lo: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[24:]), Hi: binary.BigEndian.Uint64(p[16:])},
				Hi: &types_pb.H128{Lo: binary.BigEndian.Uint64(p[8:]), Hi: binary.BigEndian.Uint64(p[0:])},
			},
		},
	}
}

func ConvertH2048ToBloom(h2048 *types_pb.H2048) [256]byte {
	var bloom [256]byte
	copy(bloom[:], ConvertH512ToBytes(h2048.Hi.Hi))
	copy(bloom[64:], ConvertH512ToBytes(h2048.Hi.Lo))
	copy(bloom[128:], ConvertH512ToBytes(h2048.Lo.Hi))
	copy(bloom[192:], ConvertH512ToBytes(h2048.Lo.Lo))
	return bloom
}

func ConvertBytesToH2048(data []byte) *types_pb.H2048 {
	return &types_pb.H2048{
		Hi: &types_pb.H1024{
			Hi: ConvertBytesToH512(data),
			Lo: ConvertBytesToH512(data[64:]),
		},
		Lo: &types_pb.H1024{
			Hi: ConvertBytesToH512(data[128:]),
			Lo: ConvertBytesToH512(data[192:]),
		},
	}
}
