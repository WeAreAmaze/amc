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

import "testing"

func TestBloom_Contain(t *testing.T) {
	bloom, _ := NewBloom(10)
	hashes := []Hash{{0x01}, {0x02}, {0x03}, {0x04}, {0x05}, {0x06}, {0x07}, {0x08}, {0x09}, {0x0a}}
	for _, hash := range hashes {
		bloom.Add(hash.Bytes())
	}

	searchHash := []Hash{{0x10}, {0x11}, {0x01}}

	for _, hash := range searchHash {
		if bloom.Contain(hash.Bytes()) {
			t.Logf("hash %d is in hashes %d", hash, hashes)
		} else {
			t.Logf("hash %d is not in hashes %d", hash, hashes)
		}
	}

	b, _ := bloom.Marshal()
	t.Logf("bloom Marshal: %+v", b)

}
