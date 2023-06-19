package p2p

import (
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/pkg/errors"
)

// ForkDigest returns the current fork digest of
// the node according to the local clock.
func (s *Service) currentForkDigest() ([4]byte, error) {
	if !s.isInitialized() {
		return [4]byte{}, errors.New("state is not initialized")
	}
	return [4]byte{}, nil
}

// Compares fork ENRs between an incoming peer's record and our node's
// local record values for current and next fork version/epoch.
func (s *Service) compareForkENR(record *enr.Record) error {

	return nil
}

// Adds a fork entry as an ENR record under the Ethereum consensus EnrKey for
// the local node. The fork entry is an ssz-encoded enrForkID type
// which takes into account the current fork version from the current
// epoch to create a fork digest, the next fork version,
// and the next fork epoch.
func addForkEntry(
	node *enode.LocalNode,
	genesisTime time.Time,
	genesisValidatorsRoot []byte,
) (*enode.LocalNode, error) {

	return node, nil
}
