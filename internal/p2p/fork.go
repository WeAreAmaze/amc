package p2p

import (
	"bytes"
	"fmt"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/p2p/enode"
	"github.com/amazechain/amc/internal/p2p/enr"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
)

const amtENRKey = "amcEnr"

// ForkDigest returns the current fork digest of
// the node according to the local clock.
func (s *Service) currentForkDigest() ([4]byte, error) {
	return utils.CreateForkDigest(new(uint256.Int), s.genesisHash)
}

// Compares fork ENRs between an incoming peer's record and our node's
// local record values for current and next fork version/epoch.
func (s *Service) compareForkENR(record *enr.Record) error {
	remoteForkDigest := make([]byte, 4)
	entry := enr.WithEntry(amtENRKey, &remoteForkDigest)
	err := record.Load(entry)
	if err != nil {
		return err
	}
	enrString, err := SerializeENR(record)
	if err != nil {
		return err
	}

	currentForkDigest, err := s.currentForkDigest()
	if err != nil {
		err := errors.Wrap(err, "could not retrieve fork digest")
		//tracing.AnnotateError(span, err)
		return err
	}

	if !bytes.Equal(remoteForkDigest, currentForkDigest[:]) {
		return fmt.Errorf(
			"fork digest of peer with ENR %s: %v, does not match local value: %v",
			enrString,
			currentForkDigest,
			remoteForkDigest,
		)
	}
	return nil
}

// Adds a fork entry as an ENR record under the Ethereum consensus EnrKey for
// the local node. The fork entry is an ssz-encoded enrForkID type
// which takes into account the current fork version from the current
// epoch to create a fork digest, the next fork version,
// and the next fork epoch.
func addForkEntry(node *enode.LocalNode, genesisHash types.Hash) (*enode.LocalNode, error) {
	//todo
	enc, err := utils.CreateForkDigest(new(uint256.Int), genesisHash)
	if err != nil {
		return nil, err
	}
	forkEntry := enr.WithEntry(amtENRKey, enc[:])
	node.Set(forkEntry)
	return node, nil
}
