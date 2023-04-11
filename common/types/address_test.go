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
	"crypto/rand"
	"encoding/json"
	"github.com/holiman/uint256"
	"reflect"

	// "github.com/amazechain/amc/utils" cycle import
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
)

func TestAddress_ed25519(t *testing.T) {
	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := PublicToAddress(pub)
	t.Log(addr.String())
}

func TestFromECDSAPub(t *testing.T) {
	_, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := PublicToAddress(pub)
	t.Log(addr.String())
}

// func TestAddress_DecodeString(t *testing.T) {
// 	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	addr := PublicToAddress(pub)
// 	t.Log(addr.String())
// 	t.Log(addr.Bytes())

// 	h := addr.HexBytes()
// 	var c Address
// 	if c.DecodeHexBytes(h) {
// 		t.Log(h)
// 		t.Logf("c: %s", c.String())
// 	}

// 	var a, b Address
// 	if a.DecodeString(addr.String()) {
// 		t.Logf("a: %s", a.String())
// 	}
// 	if b.DecodeBytes(addr.Bytes()) {
// 		t.Logf("b: %s", b.String())
// 	}

// 	t.Log("done!")
// }

func TestPrivateToAddress(t *testing.T) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := PublicToAddress(pub)
	t.Log(addr.String())
	addr2 := PrivateToAddress(priv)
	t.Log(addr2.String())
}

func TestAddress_null(t *testing.T) {
	a := Address{}
	if a.IsNull() {
		t.Log("null address")
	} else {
		t.Log(a)
	}

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := PublicToAddress(pub)

	if addr.IsNull() {
		t.Log("null address")
	} else {
		t.Log(strings.ToUpper(addr.String()))
	}
}

func TestAddress(t *testing.T) {
	for i := 0; i < 100; i++ {
		_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		addr := PublicToAddress(pub)

		if addr.IsNull() {
			t.Log("null address")
		} else {
			t.Log(strings.ToUpper(addr.String()))
		}
	}
}

func TestSign(t *testing.T) {
	data := []byte("hello")
	//priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	priv, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	s, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("sign len (%d)", len(s))

	ok, err := pub.Verify(data, s)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("sign len %d", len(s))
	if !ok {
		t.Log("verify sign failed")
	} else {
		t.Log("verify success")
	}
}

// func TestAddressToInt(t *testing.T) {
// 	priv, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	sp, err := utils.PrivateToString(priv)

// 	addr := PrivateToAddress(priv)
// 	t.Logf("address: %s", addr)
// 	t.Logf("private: %s", sp)
// }

func TestAddressMarshal(t *testing.T) {
	aa := make(map[Address]*uint256.Int)
	var a1, a2 Address
	a1.DecodeString("AMC4541Fc1CCB4e042a3BaDFE46904F9D22d127B682")
	a2.DecodeString("AMC9BA336835422BAeFc537d75642959d2a866500a3")
	aa[a1] = uint256.NewInt(1)
	aa[a2] = uint256.NewInt(2)

	b, err := json.Marshal(aa)
	if nil != err {
		t.Fatal(err)
	}

	bb := make(map[Address]*uint256.Int)

	if err := json.Unmarshal(b, &bb); nil != err {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(aa, bb) {
		t.Error("failed")
	}
}
