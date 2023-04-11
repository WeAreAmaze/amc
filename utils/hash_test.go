package utils

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"testing"
)

func TestNilHash(t *testing.T) {
	sha := HasherPool.Get().(crypto.KeccakState)
	defer HasherPool.Put(sha)
	sha.Reset()

	t.Logf("nil hash: %s", types.BytesToHash(sha.Sum(nil)).String())

	t.Logf("EmptyUncleHash : %s", RlpHash([]*block.Header(nil)).String())

}
