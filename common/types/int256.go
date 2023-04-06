package types

import (
	"github.com/holiman/uint256"
	"math/big"
)

func Int256Min(a, b *uint256.Int) *uint256.Int {
	if a.Cmp(b) > 0 {
		return b
	}
	return a
}

// BigToHash sets byte representation of b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BigToHash(b *big.Int) Hash { return BytesToHash(b.Bytes()) }
