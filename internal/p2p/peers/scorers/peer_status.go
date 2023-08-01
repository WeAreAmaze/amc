package scorers

import (
	"errors"
	"github.com/amazechain/amc/api/protocol/sync_pb"
	"github.com/amazechain/amc/internal/p2p/peers/peerdata"
	"github.com/amazechain/amc/internal/p2p/types"
	"github.com/amazechain/amc/utils"
	"github.com/holiman/uint256"
	"math"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

var _ Scorer = (*PeerStatusScorer)(nil)

// PeerStatusScorer represents scorer that evaluates peers based on their statuses.
// Peer statuses are updated by regularly polling peers (see sync/rpc_status.go).
type PeerStatusScorer struct {
	config *PeerStatusScorerConfig
	store  *peerdata.Store
	//todo
	ourCurrentHeight    *uint256.Int
	highestPeerHeadSlot *uint256.Int
}

// PeerStatusScorerConfig holds configuration parameters for peer status scoring service.
type PeerStatusScorerConfig struct{}

// newPeerStatusScorer creates new peer status scoring service.
func newPeerStatusScorer(store *peerdata.Store, config *PeerStatusScorerConfig) *PeerStatusScorer {
	if config == nil {
		config = &PeerStatusScorerConfig{}
	}
	return &PeerStatusScorer{
		config: config,
		store:  store,
	}
}

// Score returns calculated peer score.
func (s *PeerStatusScorer) Score(pid peer.ID) float64 {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.score(pid)
}

// score is a lock-free version of Score.
func (s *PeerStatusScorer) score(pid peer.ID) float64 {
	if s.isBadPeer(pid) {
		return BadPeerScore
	}
	score := float64(0)
	peerData, ok := s.store.PeerData(pid)
	if !ok || peerData.ChainState == nil {
		return score
	}
	// todo
	if s.ourCurrentHeight != nil && peerData.CurrentHeight().Cmp(s.ourCurrentHeight) == -1 {
		return score
	}
	// Calculate score as a ratio to the known maximum head slot.
	// The closer the current peer's head slot to the maximum, the higher is the calculated score.
	if s.highestPeerHeadSlot.Uint64() > 0 {
		score = float64(peerData.CurrentHeight().Uint64()) / float64(s.highestPeerHeadSlot.Uint64())
		return math.Round(score*ScoreRoundingFactor) / ScoreRoundingFactor
	}
	return score
}

// IsBadPeer states if the peer is to be considered bad.
func (s *PeerStatusScorer) IsBadPeer(pid peer.ID) bool {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.isBadPeer(pid)
}

// isBadPeer is lock-free version of IsBadPeer.
func (s *PeerStatusScorer) isBadPeer(pid peer.ID) bool {
	peerData, ok := s.store.PeerData(pid)
	if !ok {
		return false
	}
	// Mark peer as bad, if the latest error is one of the terminal ones.
	terminalErrs := []error{
		p2ptypes.ErrWrongForkDigestVersion,
		p2ptypes.ErrInvalidFinalizedRoot,
		p2ptypes.ErrInvalidRequest,
	}
	for _, err := range terminalErrs {
		if errors.Is(peerData.ChainStateValidationError, err) {
			return true
		}
	}
	return false
}

// BadPeers returns the peers that are considered bad.
func (s *PeerStatusScorer) BadPeers() []peer.ID {
	s.store.RLock()
	defer s.store.RUnlock()

	badPeers := make([]peer.ID, 0)
	for pid := range s.store.Peers() {
		if s.isBadPeer(pid) {
			badPeers = append(badPeers, pid)
		}
	}
	return badPeers
}

// SetPeerStatus sets chain state data for a given peer.
func (s *PeerStatusScorer) SetPeerStatus(pid peer.ID, chainState *sync_pb.Status, validationError error) {
	s.store.Lock()
	defer s.store.Unlock()

	peerData := s.store.PeerDataGetOrCreate(pid)
	peerData.ChainState = chainState
	peerData.ChainStateLastUpdated = time.Now()
	peerData.ChainStateValidationError = validationError

	// Update maximum known head slot (scores will be calculated with respect to that maximum value).
	//todo
	currentHeight := utils.ConvertH256ToUint256Int(chainState.CurrentHeight)
	if chainState != nil && s.highestPeerHeadSlot == nil || currentHeight.Cmp(s.highestPeerHeadSlot) == 1 {
		s.highestPeerHeadSlot = currentHeight
	}
}

// PeerStatus gets the chain state of the given remote peer.
// This can return nil if there is no known chain state for the peer.
// This will error if the peer does not exist.
func (s *PeerStatusScorer) PeerStatus(pid peer.ID) (*sync_pb.Status, error) {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.peerStatus(pid)
}

// peerStatus lock-free version of PeerStatus.
func (s *PeerStatusScorer) peerStatus(pid peer.ID) (*sync_pb.Status, error) {
	if peerData, ok := s.store.PeerData(pid); ok {
		if peerData.ChainState == nil {
			return nil, peerdata.ErrNoPeerStatus
		}
		return peerData.ChainState, nil
	}
	return nil, peerdata.ErrPeerUnknown
}

// SetCurrentHeight updates known head slot. todo
func (s *PeerStatusScorer) SetCurrentHeight(currentHeight *uint256.Int) {
	s.store.Lock()
	defer s.store.Unlock()
	s.ourCurrentHeight = currentHeight
}
