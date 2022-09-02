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

package block

import (
	"encoding/hex"
	"encoding/json"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/internal/avm/rlp"
	"github.com/gogo/protobuf/proto"
	"testing"
)

func getBlock(tb testing.TB) *Block {
	//sBlock := "0af5010a200c138c091cfdedd52b8a6f35f664243525de3dba58260788a8e0c1806f3fd0911214f61c62199512e8e85d7744d513b1b74bbe98aea11a200000000000000000000000000000000000000000000000000000000000000000222000000000000000000000000000000000000000000000000000000000000000002a20000000000000000000000000000000000000000000000000000000000000000032033078313a0330783850dfe882950662033078306a40e6bf01023645789ba3cb7e8200eb54289bccbc292b4f4796d2268fee49a7a9a993c539ef50815018014438436b5e5a10c27a4ef2ea4dda9efe5ed5b30ab6fa061200"
	sBlock := "0af5010a2009876b2d89375c505dd4128e2ea6bc588d8e9beb5ec11af94fc2f9d3e9eae5081214f61c62199512e8e85d7744d513b1b74bbe98aea11a20de7f0cbe30d14da2d6950c0643fe164b9e6ea421550263a02eaaaff257c2f6cc22208ac5c253af5118c7a98eecb957a37bf72a1f5556b8dfd1932322de5e10e32f312a2015647dcb388cef8e815a09408301361aed626048764adb507a06dc08ba593ae332033078323a0330783150858283950662033078306a4080e6d9a605d8be11339047b2541d7f59f3a193a0dad1e27360d016fba82c5449310b485172b8b2a705c8a8111c960149bccdd5c0ffea924c3fe6343aafd258021200"
	bBlock, err := hex.DecodeString(sBlock)
	if err != nil {
		tb.Fatal(err)
	}

	var block types_pb.PBlock
	err = proto.Unmarshal(bBlock, &block)
	if err != nil {
		tb.Fatal(err)
	}

	//tb.Logf("block number: %v", block.Header.Number.String())

	var b Block
	err = b.FromProtoMessage(&block)
	if err != nil {
		tb.Fatal(err)
	}

	return &b
}

type exBlock struct {
	H *Header
	B *Body
}

func TestSize(t *testing.T) {
	block := getBlock(t)
	pb := block.ToProtoMessage()

	buf1, err := proto.Marshal(pb)
	if err != nil {
		t.Fatal(err)
	}

	exB := exBlock{
		H: block.header,
		B: block.body,
	}

	buf2, err := rlp.EncodeToBytes(&exB)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("proto size: %d, rlp size: %d", len(buf1), len(buf2))
}

func BenchmarkProtobuf(b *testing.B) {
	block := getBlock(b)
	pb := block.ToProtoMessage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = proto.Marshal(pb)
	}
}

func BenchmarkRlp(b *testing.B) {
	block := getBlock(b)
	exB := exBlock{
		H: block.header,
		B: block.body,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rlp.EncodeToBytes(&exB)
	}
}

func BenchmarkJson(b *testing.B) {
	block := getBlock(b)
	exB := exBlock{
		H: block.header,
		B: block.body,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(exB)
	}
}

func BenchmarkProtobufUnmarshal(b *testing.B) {
	//sBlock := "0af5010a200c138c091cfdedd52b8a6f35f664243525de3dba58260788a8e0c1806f3fd0911214f61c62199512e8e85d7744d513b1b74bbe98aea11a200000000000000000000000000000000000000000000000000000000000000000222000000000000000000000000000000000000000000000000000000000000000002a20000000000000000000000000000000000000000000000000000000000000000032033078313a0330783850dfe882950662033078306a40e6bf01023645789ba3cb7e8200eb54289bccbc292b4f4796d2268fee49a7a9a993c539ef50815018014438436b5e5a10c27a4ef2ea4dda9efe5ed5b30ab6fa061200"
	sBlock := "0af5010a2009876b2d89375c505dd4128e2ea6bc588d8e9beb5ec11af94fc2f9d3e9eae5081214f61c62199512e8e85d7744d513b1b74bbe98aea11a20de7f0cbe30d14da2d6950c0643fe164b9e6ea421550263a02eaaaff257c2f6cc22208ac5c253af5118c7a98eecb957a37bf72a1f5556b8dfd1932322de5e10e32f312a2015647dcb388cef8e815a09408301361aed626048764adb507a06dc08ba593ae332033078323a0330783150858283950662033078306a4080e6d9a605d8be11339047b2541d7f59f3a193a0dad1e27360d016fba82c5449310b485172b8b2a705c8a8111c960149bccdd5c0ffea924c3fe6343aafd258021200"
	bBlock, err := hex.DecodeString(sBlock)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var block types_pb.PBlock
		_ = proto.Unmarshal(bBlock, &block)
	}
}

func BenchmarkRlpDecode(b *testing.B) {
	block := getBlock(b)
	exB := exBlock{
		H: block.header,
		B: block.body,
	}

	buf, err := rlp.EncodeToBytes(&exB)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var exB exBlock
		_ = rlp.DecodeBytes(buf, &exB)
	}
}

func BenchmarkJsonDecode(b *testing.B) {
	block := getBlock(b)
	exB := exBlock{
		H: block.header,
		B: block.body,
	}
	bytes, err := json.Marshal(exB)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var exB exBlock
		_ = json.Unmarshal(bytes, &exB)
	}
}
