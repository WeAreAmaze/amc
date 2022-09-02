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

package poa

import (
	"crypto/rand"
	"encoding/json"
	"github.com/amazechain/amc/common/types"
	"github.com/libp2p/go-libp2p-core/crypto"
	"sort"
	"testing"
)

func TestSort(t *testing.T) {
	var (
		strs []string
	)

	t.Log("-----create addr")
	for i := 0; i < 10; i++ {
		_, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		addr := types.PublicToAddress(pub)

		strs = append(strs, addr.String())
	}
	t.Logf("strs: %v", strs)
	t.Log("-----end addr")

	sort.Strings(strs)

	t.Logf("sort: %v", strs)

}

func TestRecent(t *testing.T) {
	r := make(map[types.Int256]types.Address)
	r[types.NewInt64(0)] = types.Address{0x1}
	r[types.NewInt64(1)] = types.Address{0x2}
	r[types.NewInt64(2)] = types.Address{0x3}

	r2 := make(map[string]types.Address)
	for i, j := range r {
		n := i.String()
		r2[n] = j
	}

	b, err := json.Marshal(r2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("bytes:%s", string(b))

	var r3 map[string]types.Address
	if err = json.Unmarshal(b, &r3); err != nil {
		t.Fatal(err)
	}

	t.Logf("r3: %v", r3)
}
