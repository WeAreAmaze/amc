package utils

import (
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/rlp"
	"golang.org/x/crypto/sha3"
	"sync"
)

var (
	// NilHash sum(nil)
	NilHash = types.HexToHash("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	// EmptyUncleHash rlpHash([]*Header(nil))
	EmptyUncleHash = types.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")
)

// HasherPool holds LegacyKeccak256 hashers for rlpHash.
var HasherPool = sync.Pool{
	New: func() interface{} { return sha3.NewLegacyKeccak256() },
}

func RlpHash(x interface{}) (h types.Hash) {
	sha := HasherPool.Get().(crypto.KeccakState)
	defer HasherPool.Put(sha)
	sha.Reset()
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}

// PrefixedRlpHash writes the prefix into the hasher before rlp-encoding x.
// It's used for typed transactions.
func PrefixedRlpHash(prefix byte, x interface{}) (h types.Hash) {
	sha := HasherPool.Get().(crypto.KeccakState)
	defer HasherPool.Put(sha)
	sha.Reset()
	sha.Write([]byte{prefix})
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}
