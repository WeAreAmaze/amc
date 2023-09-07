package apos

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"testing"
)

func TestSealHash(t *testing.T) {
	have := SealHash(&block.Header{
		Difficulty: new(uint256.Int),
		Number:     new(uint256.Int),
		Extra:      make([]byte, 32+65),
		BaseFee:    new(uint256.Int),
	})
	want := types.HexToHash("0x0412d14012bd17c38d55ec87fed2c28bc325e402fa3404a5e2041c2d790e945c")
	if have != want {
		t.Errorf("have %x, want %x", have, want)
	}
}
