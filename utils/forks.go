package utils

import (
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
)

// CreateForkDigest creates a fork digest from a genesis time and genesis
// validators root, utilizing the current slot to determine
// the active fork version in the node.
func CreateForkDigest(currentBlockNr *uint256.Int, genesisValidatorsRoot types.Hash) ([4]byte, error) {
	//todo currentBlockNr
	return ToBytes4(genesisValidatorsRoot[:]), nil
}
