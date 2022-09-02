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
	"encoding/hex"
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
