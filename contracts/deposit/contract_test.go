// Copyright 2023 The AmazeChain Authors
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

package deposit

import (
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"testing"
)

func TestBLS(t *testing.T) {
	sig, _ := hexutil.Decode("0xab22c6b63e3595630ffe8ed2903dfeba2a781c2d33dc66f88442982b65c5fcce9a8078f9ae419c95eecd2a5546a06e371196855311a930a4ad404321083f4f058c41e2d6c2e1f2bf11b1cd9b73d65a0a169a81cc1e60b50164aa7b322396be67")
	bp, _ := hexutil.Decode("0xa20699fa55487f79c1400e2be5bb6acf89b0c5880becfa4b0560b9994bd8050616886a8fb71bdf15065dd31dd2858c18")
	msg := new(uint256.Int).Mul(uint256.NewInt(params.AMT), uint256.NewInt(50)) //50AMT
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		t.Fatal("cannot unpack BLS signature", err)
	}

	publicKey, err := bls.PublicKeyFromBytes(bp)

	if err != nil {
		t.Fatal("cannot unpack BLS publicKey", err)
	}

	if !signature.Verify(publicKey, msg.Bytes()) {
		t.Fatal("bls cannot verify signature")
	}

}

func TestUint256(t *testing.T) {
	amt50Hex, _ := hexutil.Decode("0x2B5E3AF16B1880000") // 50 AMT
	amt50Uint256 := new(uint256.Int).Mul(uint256.NewInt(params.AMT), uint256.NewInt(50))
	//
	t.Logf("50 AMT uint256 bytes:%s, hex Bytes: %s", hexutil.Encode(amt50Uint256.Bytes()), hexutil.Encode(amt50Hex))

	amt500Hex, _ := hexutil.Decode("0x1B1AE4D6E2EF500000") // 500 AMT
	amt500Uint256 := new(uint256.Int).Mul(uint256.NewInt(params.AMT), uint256.NewInt(500))
	//
	t.Logf("500 AMT uint256 bytes:%s, hex Bytes: %s", hexutil.Encode(amt500Uint256.Bytes()), hexutil.Encode(amt500Hex))

	amt100Hex, _ := hexutil.Decode("0x56BC75E2D63100000") // 100 AMT
	amt100Uint256 := new(uint256.Int).Mul(uint256.NewInt(params.AMT), uint256.NewInt(100))
	//
	t.Logf("100 AMT uint256 bytes:%s, hex Bytes: %s", hexutil.Encode(amt100Uint256.Bytes()), hexutil.Encode(amt100Hex))
}

//func TestPrivateKey(t *testing.T) {
//	private, _ := hexutil.Decode("0xde4b76c3dca3d8e10aea7644f77b316a68a6476fbd119d441ead5c6131aa42a7")
//
//	p, err := bls.SecretKeyFromBytes(private)
//	if err != nil {
//		t.Fatal("bls cannot import private key", err)
//	}
//
//	pub, err := p.PublicKey().MarshalText()
//
//	if err != nil {
//		t.Fatal("bls public key cannot MarshalText ", err)
//	}
//	t.Logf("pubkey %s", string(pub))
//}
