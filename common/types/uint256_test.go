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
	"github.com/holiman/uint256"
	"testing"
)

func TestM(t *testing.T) {
	u := uint256.NewInt(1234)
	b, err := u.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(u.Bytes(), u.Uint64())

	u2 := uint256.NewInt(0)
	err = u2.UnmarshalText(b)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(u2.Bytes(), u2.Uint64())
}

func TestSetBytes(t *testing.T) {
	u := uint256.NewInt(1234)
	t.Log(u.Bytes())
	t.Log(u.Uint64())
	t.Log(u.String())
	u2 := uint256.NewInt(0)
	u2.SetBytes(u.Bytes())
	t.Log(u2.Bytes())
	t.Log(u2.Uint64())
}

func TestE(t *testing.T) {
	a := NewInt64(1)
	t.Log(a.String())

	b := NewInt64(20)
	t.Log(b.String())

	if a == b {
		t.Log("a == b")
	} else {
		t.Log("a != b")
	}
}

func TestBytews(t *testing.T) {
	a := NewInt64(0)
	ba1, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	ba2, err := a.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ba1:%v", ba1)
	t.Logf("ba2:%v", ba2)
	t.Logf("ba3:%v", a.Bytes())
}
