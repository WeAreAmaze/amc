package p2p

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

const (
	// beaconBlockWeight specifies the scoring weight that we apply to
	// our beacon block topic.
	beaconBlockWeight = 0.8
	// aggregateWeight specifies the scoring weight that we apply to
	// our aggregate topic.
	aggregateWeight = 0.5
	// syncContributionWeight specifies the scoring weight that we apply to
	// our sync contribution topic.
	syncContributionWeight = 0.2
	// attestationTotalWeight specifies the scoring weight that we apply to
	// our attestation subnet topic.
	attestationTotalWeight = 1
	// syncCommitteesTotalWeight specifies the scoring weight that we apply to
	// our sync subnet topic.
	syncCommitteesTotalWeight = 0.4
	// attesterSlashingWeight specifies the scoring weight that we apply to
	// our attester slashing topic.
	attesterSlashingWeight = 0.05
	// proposerSlashingWeight specifies the scoring weight that we apply to
	// our proposer slashing topic.
	proposerSlashingWeight = 0.05
	// voluntaryExitWeight specifies the scoring weight that we apply to
	// our voluntary exit topic.
	voluntaryExitWeight = 0.05
	// blsToExecutionChangeWeight specifies the scoring weight that we apply to
	// our bls to execution topic.
	blsToExecutionChangeWeight = 0.05

	// maxInMeshScore describes the max score a peer can attain from being in the mesh.
	maxInMeshScore = 10
	// maxFirstDeliveryScore describes the max score a peer can obtain from first deliveries.
	maxFirstDeliveryScore = 40

	// decayToZero specifies the terminal value that we will use when decaying
	// a value.
	decayToZero = 0.01

	// dampeningFactor reduces the amount by which the various thresholds and caps are created.
	dampeningFactor = 90
)

var (
	// a bool to check if we enable scoring for messages in the mesh sent for near first deliveries.
	meshDeliveryIsScored = false

	// Defines the variables representing the different time periods.
	oneHundredBlocks   = 100 * oneBlockDuration()
	invalidDecayPeriod = 50 * oneBlockDuration()
	twentyBlocks       = 20 * oneBlockDuration()
	tenBlocks          = 10 * oneBlockDuration()
)

func peerScoringParams() (*pubsub.PeerScoreParams, *pubsub.PeerScoreThresholds) {
	thresholds := &pubsub.PeerScoreThresholds{
		GossipThreshold:             -4000,
		PublishThreshold:            -8000,
		GraylistThreshold:           -16000,
		AcceptPXThreshold:           100,
		OpportunisticGraftThreshold: 5,
	}
	scoreParams := &pubsub.PeerScoreParams{
		Topics:        make(map[string]*pubsub.TopicScoreParams),
		TopicScoreCap: 32.72,
		AppSpecificScore: func(p peer.ID) float64 {
			return 0
		},
		AppSpecificWeight:           1,
		IPColocationFactorWeight:    -35.11,
		IPColocationFactorThreshold: 10,
		IPColocationFactorWhitelist: nil,
		BehaviourPenaltyWeight:      -15.92,
		BehaviourPenaltyThreshold:   6,
		BehaviourPenaltyDecay:       scoreDecay(tenBlocks),
		DecayInterval:               oneBlockDuration(),
		DecayToZero:                 decayToZero,
		RetainScore:                 oneHundredBlocks,
	}
	return scoreParams, thresholds
}

func (s *Service) topicScoreParams(topic string) (*pubsub.TopicScoreParams, error) {
	switch {
	case strings.Contains(topic, GossipBlockMessage):
		return defaultBlockTopicParams(), nil
	case strings.Contains(topic, GossipExitMessage):
		return defaultVoluntaryExitTopicParams(), nil
	default:
		return nil, errors.Errorf("unrecognized topic provided for parameter registration: %s", topic)
	}
}

// Based on the lighthouse parameters.
// https://gist.github.com/blacktemplar/5c1862cb3f0e32a1a7fb0b25e79e6e2c

func defaultBlockTopicParams() *pubsub.TopicScoreParams {
	//todo
	decayEpoch := time.Duration(5)
	meshWeight := -0.717
	if !meshDeliveryIsScored {
		// Set the mesh weight as zero as a temporary measure, so as to prevent
		// the average nodes from being penalised.
		meshWeight = 0
	}
	return &pubsub.TopicScoreParams{
		TopicWeight:                     beaconBlockWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    1,
		FirstMessageDeliveriesDecay:     scoreDecay(twentyBlocks),
		FirstMessageDeliveriesCap:       23,
		MeshMessageDeliveriesWeight:     meshWeight,
		MeshMessageDeliveriesDecay:      scoreDecay(decayEpoch * oneBlockDuration()),
		MeshMessageDeliveriesCap:        float64(decayEpoch),
		MeshMessageDeliveriesThreshold:  float64(decayEpoch) / 10,
		MeshMessageDeliveriesWindow:     2 * time.Second,
		MeshMessageDeliveriesActivation: 4 * oneBlockDuration(),
		MeshFailurePenaltyWeight:        meshWeight,
		MeshFailurePenaltyDecay:         scoreDecay(decayEpoch * oneBlockDuration()),
		InvalidMessageDeliveriesWeight:  -140.4475,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func defaultSyncContributionTopicParams() *pubsub.TopicScoreParams {
	// Determine the expected message rate for the particular gossip topic.
	//todo
	aggPerSlot := 2
	firstMessageCap, err := decayLimit(scoreDecay(1*oneBlockDuration()), float64(aggPerSlot*2/gossipSubD))
	if err != nil {
		log.Warn("skipping initializing topic scoring", "err", err)
		return nil
	}
	firstMessageWeight := maxFirstDeliveryScore / firstMessageCap
	meshThreshold, err := decayThreshold(scoreDecay(1*oneBlockDuration()), float64(aggPerSlot)/dampeningFactor)
	if err != nil {
		log.Warn("skipping initializing topic scoring", "err", err)
		return nil
	}
	meshWeight := -scoreByWeight(syncContributionWeight, meshThreshold)
	meshCap := 4 * meshThreshold
	if !meshDeliveryIsScored {
		// Set the mesh weight as zero as a temporary measure, so as to prevent
		// the average nodes from being penalised.
		meshWeight = 0
	}
	return &pubsub.TopicScoreParams{
		TopicWeight:                     syncContributionWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    firstMessageWeight,
		FirstMessageDeliveriesDecay:     scoreDecay(1 * oneBlockDuration()),
		FirstMessageDeliveriesCap:       firstMessageCap,
		MeshMessageDeliveriesWeight:     meshWeight,
		MeshMessageDeliveriesDecay:      scoreDecay(1 * oneBlockDuration()),
		MeshMessageDeliveriesCap:        meshCap,
		MeshMessageDeliveriesThreshold:  meshThreshold,
		MeshMessageDeliveriesWindow:     2 * time.Second,
		MeshMessageDeliveriesActivation: 1 * oneBlockDuration(),
		MeshFailurePenaltyWeight:        meshWeight,
		MeshFailurePenaltyDecay:         scoreDecay(1 * oneBlockDuration()),
		InvalidMessageDeliveriesWeight:  -maxScore() / syncContributionWeight,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func defaultAttesterSlashingTopicParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     attesterSlashingWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    36,
		FirstMessageDeliveriesDecay:     scoreDecay(oneHundredBlocks),
		FirstMessageDeliveriesCap:       1,
		MeshMessageDeliveriesWeight:     0,
		MeshMessageDeliveriesDecay:      0,
		MeshMessageDeliveriesCap:        0,
		MeshMessageDeliveriesThreshold:  0,
		MeshMessageDeliveriesWindow:     0,
		MeshMessageDeliveriesActivation: 0,
		MeshFailurePenaltyWeight:        0,
		MeshFailurePenaltyDecay:         0,
		InvalidMessageDeliveriesWeight:  -2000,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func defaultProposerSlashingTopicParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     proposerSlashingWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    36,
		FirstMessageDeliveriesDecay:     scoreDecay(oneHundredBlocks),
		FirstMessageDeliveriesCap:       1,
		MeshMessageDeliveriesWeight:     0,
		MeshMessageDeliveriesDecay:      0,
		MeshMessageDeliveriesCap:        0,
		MeshMessageDeliveriesThreshold:  0,
		MeshMessageDeliveriesWindow:     0,
		MeshMessageDeliveriesActivation: 0,
		MeshFailurePenaltyWeight:        0,
		MeshFailurePenaltyDecay:         0,
		InvalidMessageDeliveriesWeight:  -2000,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func defaultVoluntaryExitTopicParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     voluntaryExitWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    2,
		FirstMessageDeliveriesDecay:     scoreDecay(oneHundredBlocks),
		FirstMessageDeliveriesCap:       5,
		MeshMessageDeliveriesWeight:     0,
		MeshMessageDeliveriesDecay:      0,
		MeshMessageDeliveriesCap:        0,
		MeshMessageDeliveriesThreshold:  0,
		MeshMessageDeliveriesWindow:     0,
		MeshMessageDeliveriesActivation: 0,
		MeshFailurePenaltyWeight:        0,
		MeshFailurePenaltyDecay:         0,
		InvalidMessageDeliveriesWeight:  -2000,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func defaultBlsToExecutionChangeTopicParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                     blsToExecutionChangeWeight,
		TimeInMeshWeight:                maxInMeshScore / inMeshCap(),
		TimeInMeshQuantum:               inMeshTime(),
		TimeInMeshCap:                   inMeshCap(),
		FirstMessageDeliveriesWeight:    2,
		FirstMessageDeliveriesDecay:     scoreDecay(oneHundredBlocks),
		FirstMessageDeliveriesCap:       5,
		MeshMessageDeliveriesWeight:     0,
		MeshMessageDeliveriesDecay:      0,
		MeshMessageDeliveriesCap:        0,
		MeshMessageDeliveriesThreshold:  0,
		MeshMessageDeliveriesWindow:     0,
		MeshMessageDeliveriesActivation: 0,
		MeshFailurePenaltyWeight:        0,
		MeshFailurePenaltyDecay:         0,
		InvalidMessageDeliveriesWeight:  -2000,
		InvalidMessageDeliveriesDecay:   scoreDecay(invalidDecayPeriod),
	}
}

func oneBlockDuration() time.Duration {
	//todo
	return 8 * time.Second
}

// determines the decay rate from the provided time period till
// the decayToZero value. Ex: ( 1 -> 0.01)
func scoreDecay(totalDurationDecay time.Duration) float64 {
	numOfTimes := totalDurationDecay / oneBlockDuration()
	return math.Pow(decayToZero, 1/float64(numOfTimes))
}

// is used to determine the threshold from the decay limit with
// a provided growth rate. This applies the decay rate to a
// computed limit.
func decayThreshold(decayRate, rate float64) (float64, error) {
	d, err := decayLimit(decayRate, rate)
	if err != nil {
		return 0, err
	}
	return d * decayRate, nil
}

// decayLimit provides the value till which a decay process will
// limit till provided with an expected growth rate.
func decayLimit(decayRate, rate float64) (float64, error) {
	if 1 <= decayRate {
		return 0, errors.Errorf("got an invalid decayLimit rate: %f", decayRate)
	}
	return rate / (1 - decayRate), nil
}

// provides the relevant score by the provided weight and threshold.
func scoreByWeight(weight, threshold float64) float64 {
	return maxScore() / (weight * threshold * threshold)
}

// maxScore attainable by a peer.
func maxScore() float64 {
	totalWeight := beaconBlockWeight + aggregateWeight + syncContributionWeight +
		attestationTotalWeight + syncCommitteesTotalWeight + attesterSlashingWeight +
		proposerSlashingWeight + voluntaryExitWeight + blsToExecutionChangeWeight
	return (maxInMeshScore + maxFirstDeliveryScore) * totalWeight
}

// denotes the unit time in mesh for scoring tallying.
func inMeshTime() time.Duration {
	return 1 * oneBlockDuration()
}

// the cap for `inMesh` time scoring.
func inMeshCap() float64 {
	return float64((3600 * time.Second) / inMeshTime())
}

func logGossipParameters(topic string, params *pubsub.TopicScoreParams) {
	// Exit early in the event the parameter struct is nil.
	if params == nil {
		return
	}
	rawParams := reflect.ValueOf(params).Elem()
	numOfFields := rawParams.NumField()

	fields := make([]interface{}, numOfFields*2)
	for i := 0; i < numOfFields; i++ {
		fields = append(fields, reflect.TypeOf(params).Elem().Field(i).Name, rawParams.Field(i).Interface())
	}
	log.Debug(fmt.Sprintf("Topic Parameters for %s", topic), fields...)
}
