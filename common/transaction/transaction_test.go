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

package transaction

import (
	"crypto/rand"
	"encoding/json"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p/core/crypto"
	"testing"
)

func TestNewLegacyTx(t *testing.T) {
	_, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := types.PublicToAddress(pub)

	tx := NewTransaction(1, addr, &addr, uint256.NewInt(10000), 21000, uint256.NewInt(10000000), []byte("hello"))
	t.Logf("tx: %v", tx)

	buf1, err := json.Marshal(tx)
	t.Log(types.BytesHash(buf1).String())

	switch txt := tx.inner.(type) {
	case *LegacyTx:
		buf, err := json.Marshal(txt)
		if err != nil {
			t.Fatal(err)
		}

		txHash := types.BytesHash(buf)
		t.Log(txHash.String())
	}

	hash := tx.Hash()

	t.Log(hash.String())

}

func TestTDin(t *testing.T) {
	_, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	addr := types.PublicToAddress(pub)

	tx := NewTransaction(1, addr, &addr, uint256.NewInt(10000), 21000, uint256.NewInt(10000000), []byte("hello"))
	t.Logf("tx: %v", tx)

	b, err := tx.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	tx.Unmarshal(b)

	t.Log(b)
}

func TestNewDynamicTx(t *testing.T) {
	//_, pub, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//addr := types.PublicToAddress(pub)

}
