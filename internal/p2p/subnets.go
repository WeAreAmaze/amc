package p2p

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

// The value used with the subnet, inorder
// to create an appropriate key to retrieve
// the relevant lock. This is used to differentiate
// sync subnets from attestation subnets. This is deliberately
// chosen as more than 64(attestation subnet count).
const syncLockerVal = 100

const minimumPeersInSubnetSearch = 10
const minimumPeersPerSubnet = 2

// FindPeersWithSubnet performs a network search for peers
// subscribed to a particular subnet. Then we try to connect
// with those peers. This method will block until the required amount of
// peers are found, the method only exits in the event of context timeouts.
func (s *Service) FindPeersWithSubnet(ctx context.Context, topic string,
	index uint64, threshold int) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "p2p.FindPeersWithSubnet")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("index", int64(index))) // lint:ignore uintcast -- It's safe to do this for tracing.

	if s.dv5Listener == nil {
		// return if discovery isn't set
		return false, nil
	}

	topic += s.Encoding().ProtocolSuffix()
	iterator := s.dv5Listener.RandomNodes()
	switch {
	//case strings.Contains(topic, GossipAttestationMessage):
	//	iterator = filterNodes(ctx, iterator, s.filterPeerForAttSubnet(index))
	//case strings.Contains(topic, GossipSyncCommitteeMessage):
	//	iterator = filterNodes(ctx, iterator, s.filterPeerForSyncSubnet(index))
	default:
		return false, errors.New("no subnet exists for provided topic")
	}

	currNum := len(s.pubsub.ListPeers(topic))
	wg := new(sync.WaitGroup)
	for {
		if err := ctx.Err(); err != nil {
			return false, errors.Errorf("unable to find requisite number of peers for topic %s - "+
				"only %d out of %d peers were able to be found", topic, currNum, threshold)
		}
		if currNum >= threshold {
			break
		}
		nodes := enode.ReadNodes(iterator, minimumPeersInSubnetSearch)
		for _, node := range nodes {
			info, _, err := convertToAddrInfo(node)
			if err != nil {
				continue
			}
			wg.Add(1)
			go func() {
				if err := s.connectWithPeer(ctx, *info); err != nil {
					log.Trace(fmt.Sprintf("Could not connect with peer %s", info.String()), "err", err)
				}
				wg.Done()
			}()
		}
		// Wait for all dials to be completed.
		wg.Wait()
		currNum = len(s.pubsub.ListPeers(topic))
	}
	return true, nil
}

//
//// returns a method with filters peers specifically for a particular attestation subnet.
//func (s *Service) filterPeerForAttSubnet(index uint64) func(node *enode.Node) bool {
//	return func(node *enode.Node) bool {
//		if !s.filterPeer(node) {
//			return false
//		}
//		subnets, err := attSubnets(node.Record())
//		if err != nil {
//			return false
//		}
//		indExists := false
//		for _, comIdx := range subnets {
//			if comIdx == index {
//				indExists = true
//				break
//			}
//		}
//		return indExists
//	}
//}
//
//// returns a method with filters peers specifically for a particular sync subnet.
//func (s *Service) filterPeerForSyncSubnet(index uint64) func(node *enode.Node) bool {
//	return func(node *enode.Node) bool {
//		if !s.filterPeer(node) {
//			return false
//		}
//		subnets, err := syncSubnets(node.Record())
//		if err != nil {
//			return false
//		}
//		indExists := false
//		for _, comIdx := range subnets {
//			if comIdx == index {
//				indExists = true
//				break
//			}
//		}
//		return indExists
//	}
//}

// lower threshold to broadcast object compared to searching
// for a subnet. So that even in the event of poor peer
// connectivity, we can still broadcast an attestation.
func (s *Service) hasPeerWithSubnet(topic string) bool {
	// In the event peer threshold is lower, we will choose the lower
	// threshold.
	minPeers := math.Min(1, minimumPeersPerSubnet)
	return len(s.pubsub.ListPeers(topic+s.Encoding().ProtocolSuffix())) >= int(minPeers) // lint:ignore uintcast -- Min peers can be safely cast to int.
}

// The subnet locker is a map which keeps track of all
// mutexes stored per subnet. This locker is re-used
// between both the attestation and sync subnets. In
// order to differentiate between attestation and sync
// subnets. Sync subnets are stored by (subnet+syncLockerVal). This
// is to prevent conflicts while allowing both subnets
// to use a single locker.
func (s *Service) subnetLocker(i uint64) *sync.RWMutex {
	s.subnetsLockLock.Lock()
	defer s.subnetsLockLock.Unlock()
	l, ok := s.subnetsLock[i]
	if !ok {
		l = &sync.RWMutex{}
		s.subnetsLock[i] = l
	}
	return l
}

// Determines the number of bytes that are used
// to represent the provided number of bits.
func byteCount(bitCount int) int {
	numOfBytes := bitCount / 8
	if bitCount%8 != 0 {
		numOfBytes++
	}
	return numOfBytes
}
