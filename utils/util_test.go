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
	"crypto/rand"
	"encoding/hex"
	"github.com/amazechain/amc/common/types"
	"github.com/libp2p/go-libp2p-core/crypto"
	"testing"
)

func TestHash256toS(t *testing.T) {
	data := []byte("123aaaaa")
	s := Hash256toS(data)
	t.Log(s)
	s1 := Hash256toS(data)
	t.Log(s1)
	s2 := Hash256toS(data)
	t.Log(s2)
	s3 := Hash256toS(data)
	t.Log(s3)

}

func TestKeccak256(t *testing.T) {
	data := []byte("123aaaaa")
	s := Keccak256(data)
	t.Log(hex.EncodeToString(s))
}

func TestPrivate(t *testing.T) {
	privstr := "CAMSeTB3AgEBBCCpK8butBo2fq28tb1zdfeIZBZRcXcbcxmAJcd+h+nrjKAKBggqhkjOPQMBB6FEA0IABFYoS6MEcv5opOrOmC6dNsu9tW/isw6e5Qymx7p5mO3CvgR1WiKzbuT3FJ5N9Kn6kUNY3mzmeo04Rj/wihxVEh8="
	priv, err := StringToPrivate(privstr)
	if err != nil {
		t.Fatal(err)
	}

	public := priv.GetPublic()

	s, err := PublicToString(public)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("public: %s", s)

	a := types.PrivateToAddress(priv)
	t.Logf("address: %s", a.String())
}

func TestStringToPublic(t *testing.T) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	//priv, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	s, err := PublicToString(pub)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("public: %s", s)

	pub2, err := StringToPublic(s)
	if err != nil {
		t.Fatal(err)
	}

	if pub2.Equals(pub) {
		t.Log("success")
	} else {
		t.Logf("failed")
	}

	ps, err := PrivateToString(priv)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("private :%s", ps)
	t.Log(len(ps))
}
