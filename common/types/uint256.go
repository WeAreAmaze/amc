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
	"math/big"
)

type Int256 struct {
	uint256.Int
}

func NewInt64(val uint64) Int256 {
	return Int256{
		Int: *uint256.NewInt(val),
	}
}

func FromBig(b *big.Int) (Int256, bool) {
	if b == nil {
		//todo deal nil?
		i := big.NewInt(0)
		int, _ := uint256.FromBig(i)
		return Int256{Int: *int}, true
	}
	i, overflow := uint256.FromBig(b)
	return Int256{Int: *i}, !overflow
}

func FromHex(hex string) (Int256, error) {
	i, err := uint256.FromHex(hex)
	if err != nil {
		return Int256{}, err
	}

	return Int256{Int: *i}, nil
}

func (u Int256) Hex() string {
	return u.Int.Hex()
}

func Int256Min(x, y Int256) Int256 {
	if x.Compare(y) > 0 {
		return y
	}
	return x
}

func Int256Max(x, y Int256) Int256 {
	if x.Compare(y) < 0 {
		return y
	}
	return x
}

func (u Int256) Marshal() ([]byte, error) {
	//if u.Int == nil {
	//	u.Int = uint256.NewInt(0)
	//}
	return u.MarshalText()
}

func (u *Int256) MarshalTo(data []byte) (n int, err error) {
	//if u.Int == nil {
	//	u.Int = uint256.NewInt(0)
	//}

	b, err := u.MarshalText()
	if err != nil {
		return 0, err
	}

	copy(data, b)
	return len(b), nil
}

func (u *Int256) Unmarshal(data []byte) error {
	if len(data) == 0 {
		u = nil
		return nil
	}

	//if u.Int == nil {
	//	u.Int = uint256.NewInt(0)
	//}
	return u.UnmarshalText(data)
}

func (u *Int256) Size() int {
	bz, _ := u.Marshal()
	return len(bz)
}

func (u Int256) Compare(other Int256) int {
	return u.Cmp(&other.Int)
}

// Equal returns true u == other
func (u Int256) Equal(other Int256) bool {
	return u.Eq(&other.Int)
}

func (u *Int256) Set(other Int256) *Int256 {
	u.Int = other.Int
	return u
}

// Add returns u + other
func (u Int256) Add(other Int256) Int256 {
	z := uint256.NewInt(0)
	z.Add(&u.Int, &other.Int)
	return Int256{
		Int: *z,
	}
}

// Slt returns true u < other
func (u Int256) Slt(other Int256) bool {
	return u.Int.Slt(&other.Int)
}

// Sub returns u - other
func (u Int256) Sub(other Int256) Int256 {
	z := uint256.NewInt(0)
	z.Sub(&u.Int, &other.Int)
	return Int256{Int: *z}
}

func (u Int256) String() string {
	i := &u.Int
	return i.String()
}

//func (u Int256) Sub(x, y Int256) Int256 {
//	i := &u.Int
//	i.Sub(&x.Int, &y.Int)
//	return Int256{
//		Int: *i,
//	}
//}

func (u Int256) Div(x, y Int256) Int256 {
	i := uint256.NewInt(0)
	i.Div(&x.Int, &y.Int)
	return Int256{
		Int: *i,
	}
}

func (u Int256) Mul(x, y Int256) Int256 {
	i := uint256.NewInt(0)
	i.Mul(&x.Int, &y.Int)
	return Int256{
		Int: *i,
	}
}

func (u Int256) Mul2(other Int256) Int256 {
	z := uint256.NewInt(0)
	z.Mul(&u.Int, &other.Int)
	return Int256{Int: *z}
}

// Uint64 return u=>uint64 or u !=>uint64 return 0
func (u Int256) Uint64() uint64 {
	if u.Int.IsUint64() {
		i := &u.Int
		return i.Uint64()
	}

	return 0
}

func (u Int256) ToBig() *big.Int {
	return u.Int.ToBig()
}

// Mod return u%other, if other == 0 return 0
func (u Int256) Mod(other Int256) Int256 {
	z := uint256.NewInt(0)
	z.Mod(&u.Int, &other.Int)

	return Int256{Int: *z}
}

func (u *Int256) IsEmpty() bool {
	//if u.Int == nil {
	//	return true
	//}
	return u.Int.IsZero()
	//return false
}
