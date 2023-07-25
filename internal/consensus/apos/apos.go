// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

// Package APos implements the proof-of-authority consensus engine.

package apos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/aggsign"
	"github.com/amazechain/amc/common/crypto/bls/blst"
	"github.com/amazechain/amc/internal/consensus/misc"
	"github.com/holiman/uint256"

	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/modules/rawdb"

	"github.com/amazechain/amc/accounts"
	amcCommon "github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/rlp"
	mvm_types "github.com/amazechain/amc/internal/avm/types"

	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/ledgerwatch/erigon-lib/kv"

	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

const (
	checkpointInterval = 2048 // Number of blocks after which to save the vote snapshot to the database
	inmemorySnapshots  = 128  // Number of recent vote snapshots to keep in memory
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory

	wiggleTime       = 500 * time.Millisecond // Random delay (per signer) to allow concurrent signers
	mergeSignMinTime = 4                      // min time for merge sign
)

// APos proof-of-authority protocol constants.
var (
	epochLength = uint64(30000) // Default number of blocks after which to checkpoint and reset the pending votes

	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for signer seal

	nonceAuthVote = hexutil.MustDecode("0xffffffffffffffff") // Magic nonce number to vote on adding a new signer
	nonceDropVote = hexutil.MustDecode("0x0000000000000000") // Magic nonce number to vote on removing a signer.

	//uncleHash = mvm_types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.

	diffInTurn = uint256.NewInt(2) // Block difficulty for in-turn signatures
	diffNoTurn = uint256.NewInt(1) // Block difficulty for out-of-turn signatures
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errInvalidCheckpointBeneficiary is returned if a checkpoint/epoch transition
	// block has a beneficiary set to non-zeroes.
	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")

	// errInvalidVote is returned if a nonce value is something else that the two
	// allowed constants of 0x00..0 or 0xff..f.
	errInvalidVote = errors.New("vote nonce not 0x00..0 or 0xff..f")

	// errInvalidCheckpointVote is returned if a checkpoint/epoch transition block
	// has a vote nonce set to non-zeroes.
	errInvalidCheckpointVote = errors.New("vote nonce in checkpoint block non-zero")

	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

	// errExtraSigners is returned if non-checkpoint block contain signer data in
	// their extra-data fields.
	errExtraSigners = errors.New("non-checkpoint block contains extra signer list")

	// errInvalidCheckpointSigners is returned if a checkpoint block contains an
	// invalid list of signers (i.e. non divisible by 20 bytes).
	errInvalidCheckpointSigners = errors.New("invalid signer list on checkpoint block")

	// errMismatchingCheckpointSigners is returned if a checkpoint block contains a
	// list of signers different than the one the local node calculated.
	errMismatchingCheckpointSigners = errors.New("mismatching signer list on checkpoint block")

	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// errInvalidDifficulty is returned if the difficulty of a block neither 1 or 2.
	errInvalidDifficulty = errors.New("invalid difficulty")

	// errWrongDifficulty is returned if the difficulty of a block doesn't match the
	// turn of the signer.
	errWrongDifficulty = errors.New("wrong difficulty")

	// errInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	errInvalidTimestamp = errors.New("invalid timestamp")

	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidVotingChain = errors.New("invalid voting chain")

	// errUnauthorizedSigner is returned if a header is signed by a non-authorized entity.
	errUnauthorizedSigner = errors.New("unauthorized signer")

	// errRecentlySigned is returned if a header is signed by an authorized entity
	// that already signed a header recently, thus is temporarily not allowed to.
	errRecentlySigned = errors.New("recently signed")
)

// SignerFn hashes and signs the data to be signed by a backing account.
// todo types.address to  account
type SignerFn func(signer accounts.Account, mimeType string, message []byte) ([]byte, error)

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(iHeader block.IHeader, sigcache *lru.ARCCache) (types.Address, error) {
	header := iHeader.(*block.Header)
	// If the signature's already cached, return that
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(types.Address), nil
	}
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return types.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	//Recover the public key and the Ethereum(AMC) address
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
	if err != nil {
		return types.Address{}, err
	}
	var signer types.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, signer)
	return signer, nil
}

// APos is the proof-of-authority consensus engine proposed to support the
// Ethereum testnet following the Ropsten attacks.
type APos struct {
	config      *params.ConsensusConfig // Consensus engine configuration parameters
	chainConfig *params.ChainConfig
	db          kv.RwDB // Database to store and retrieve snapshot checkpoints

	recents    *lru.ARCCache // Snapshots for recent block to speed up reorgs
	signatures *lru.ARCCache // Signatures of recent blocks to speed up mining

	proposals map[types.Address]bool // Current list of proposals we are pushing

	signer types.Address // Ethereum address of the signing key
	signFn SignerFn      // Signer function to authorize hashes with
	lock   sync.RWMutex  // Protects the signer and proposals fields

	// The fields below are for testing only
	fakeDiff bool // Skip difficulty verifications

	bc amcCommon.IBlockChain
}

// New creates a APos proof-of-authority consensus engine with the initial
// signers set to the ones provided by the user.
func New(config *params.ConsensusConfig, db kv.RwDB, chainConfig *params.ChainConfig) consensus.Engine {
	// Set any missing consensus parameters to their defaults
	conf := *config
	if conf.APos.Epoch == 0 {
		conf.APos.Epoch = epochLength
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	return &APos{
		config:      &conf,
		chainConfig: chainConfig,
		db:          db,
		recents:     recents,
		signatures:  signatures,
		proposals:   make(map[types.Address]bool),
	}
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (c *APos) Author(header block.IHeader) (types.Address, error) {
	return ecrecover(header, c.signatures)
}

func (c *APos) SetBlockChain(bc amcCommon.IBlockChain) {
	c.bc = bc
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *APos) VerifyHeader(chain consensus.ChainHeaderReader, header block.IHeader, seal bool) error {
	return c.verifyHeader(chain, header, nil)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers. The
// method returns a quit channel to abort the operations and a results channel to
// retrieve the async verifications (the order is that of the input slice).
func (c *APos) VerifyHeaders(chain consensus.ChainHeaderReader, headers []block.IHeader, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := c.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (c *APos) verifyHeader(chain consensus.ChainHeaderReader, iHeader block.IHeader, parents []block.IHeader) error {
	header := iHeader.(*block.Header)
	if header.Number.IsZero() {
		return errUnknownBlock
	}
	number := header.Number.Uint64()

	// Don't waste time checking blocks from the future
	if header.Time > uint64(time.Now().Unix()) {
		return errors.New("block in the future")
	}
	// Checkpoint blocks need to enforce zero beneficiary
	checkpoint := (number % c.config.APos.Epoch) == 0
	if checkpoint && header.Coinbase != (types.Address{}) {
		return errInvalidCheckpointBeneficiary
	}
	// Nonces must be 0x00..0 or 0xff..f, zeroes enforced on checkpoints
	if !bytes.Equal(header.Nonce[:], nonceAuthVote) && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		return errInvalidVote
	}
	if checkpoint && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		return errInvalidCheckpointVote
	}
	// Check that the extra-data contains both the vanity and signature
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
	// Ensure that the extra-data contains a signer list on checkpoint, but none otherwise
	signersBytes := len(header.Extra) - extraVanity - extraSeal
	if !checkpoint && signersBytes != 0 {
		return errExtraSigners
	}
	if checkpoint && signersBytes%types.AddressLength != 0 {
		return errInvalidCheckpointSigners
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	//if header.MixDigest != (types.Hash{}) {
	//	return errInvalidMixDigest
	//}
	// Ensure that the block doesn't contain any uncles which are meaningless in PoA
	//if header.UncleHash != uncleHash {
	//	return errInvalidUncleHash
	//}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if number > 0 {
		if header.Difficulty.IsZero() || (header.Difficulty.Cmp(diffInTurn) != 0 && header.Difficulty.Cmp(diffNoTurn) != 0) {
			return errInvalidDifficulty
		}
	}
	// Verify that the gas limit is <= 2^63-1
	if header.GasLimit > params.MaxGasLimit {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
	}
	// If all checks passed, validate any special fields for hard forks
	//todo
	//if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
	//	return err
	//}
	// All basic checks passed, verify cascading fields
	return c.verifyCascadingFields(chain, header, parents)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (c *APos) verifyCascadingFields(chain consensus.ChainHeaderReader, iHeader block.IHeader, parents []block.IHeader) error {
	header := iHeader.(*block.Header)
	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	// Ensure that the block's timestamp isn't too close to its parent
	var parent block.IHeader
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, uint256.NewInt(number-1))
	}
	if parent.(*block.Header) == nil || parent.Number64().Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return errUnknownBlock
	}
	// todo
	rawParent := parent.(*block.Header)
	if rawParent.Time+c.config.Period > header.Time {
		return errInvalidTimestamp
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	if !chain.Config().IsLondon(header.Number.Uint64()) {
		// Verify BaseFee not present before EIP-1559 fork.
		if header.BaseFee != nil {
			return fmt.Errorf("invalid baseFee before fork: have %d, want <nil>", header.BaseFee)
		}
		if err := misc.VerifyGaslimit(parent.(*block.Header).GasLimit, header.GasLimit); err != nil {
			return err
		}
	} else if err := misc.VerifyEip1559Header(chain.Config(), parent.(*block.Header), header); err != nil {
		// Verify the header's EIP-1559 attributes.
		return err
	}
	// Retrieve the snapshot needed to verify this header and cache it
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}
	// If the block is a checkpoint block, verify the signer list
	if number%c.config.APos.Epoch == 0 {
		signers := make([]byte, len(snap.Signers)*types.AddressLength)
		for i, signer := range snap.signers() {
			copy(signers[i*types.AddressLength:], signer[:])
		}
		extraSuffix := len(header.Extra) - extraSeal
		if !bytes.Equal(header.Extra[extraVanity:extraSuffix], signers) {
			return errMismatchingCheckpointSigners
		}
	}
	// All basic checks passed, verify the seal and return
	return c.verifySeal(snap, header, parents)
}

// snapshot retrieves the authorization snapshot at a given point in time.
func (c *APos) snapshot(chain consensus.ChainHeaderReader, number uint64, hash types.Hash, parents []block.IHeader) (*Snapshot, error) {
	// Search for a snapshot in memory or on disk for checkpoints
	var (
		headers []block.IHeader
		snap    *Snapshot
	)
	for snap == nil {
		// If an in-memory snapshot was found, use that
		if s, ok := c.recents.Get(hash); ok {
			snap = s.(*Snapshot)
			break
		}
		// If an on-disk checkpoint snapshot can be found, use that
		if number%checkpointInterval == 0 {
			if err := c.db.View(context.Background(), func(tx kv.Tx) error {
				var err error
				s, err := loadSnapshot(c.config.APos, c.signatures, tx, hash)
				if err == nil {
					log.Debug("Loaded voting snapshot from disk", "number", number, "hash", hash)
					snap = s
				}
				return err
			}); nil == err {
				break
			}
		}
		// If we're at the genesis, snapshot the initial state. Alternatively if we're
		// at a checkpoint block without a parent (light client CHT), or we have piled
		// up more headers than allowed to be reorged (chain reinit from a freezer),
		// consider the checkpoint trusted and snapshot it.
		h := chain.GetHeaderByNumber(uint256.NewInt(number - 1))
		if number == 0 || (number%c.config.APos.Epoch == 0 && (len(headers) > params.FullImmutabilityThreshold || h == nil)) {
			checkpoint := chain.GetHeaderByNumber(uint256.NewInt(number))
			if checkpoint != nil {
				rawCheckpoint := checkpoint.(*block.Header)
				hash := checkpoint.Hash()

				signers := make([]types.Address, (len(rawCheckpoint.Extra)-extraVanity-extraSeal)/types.AddressLength)
				for i := 0; i < len(signers); i++ {
					copy(signers[i][:], rawCheckpoint.Extra[extraVanity+i*types.AddressLength:])
				}
				snap = newSnapshot(c.config.APos, c.signatures, number, hash, signers)
				if err := c.db.Update(context.Background(), func(tx kv.RwTx) error {
					if err := snap.store(tx); err != nil {
						return err
					}
					return nil
				}); nil != err {
					return nil, err
				}
				log.Info("Stored checkpoint snapshot to disk", "number", number, "hash", hash)
				break
			}
		}
		// No snapshot for this header, gather the header and move backward
		var header block.IHeader
		if len(parents) > 0 {
			// If we have explicit parents, pick from there (enforced)
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number64().Uint64() != number {
				return nil, errUnknownBlock
			}
			parents = parents[:len(parents)-1]
		} else {
			// No explicit parents (or no more left), reach out to the database
			header = chain.GetHeader(hash, uint256.NewInt(number))
			if header == nil {
				return nil, errUnknownBlock
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.(*block.Header).ParentHash
	}
	// Previous snapshot found, apply any pending headers on top of it
	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	snap, err := snap.apply(headers)
	if err != nil {
		return nil, err
	}
	c.recents.Add(snap.Hash, snap)

	// If we've generated a new checkpoint snapshot, save to disk
	if snap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = c.db.Update(context.Background(), func(tx kv.RwTx) error {
			if err := snap.store(tx); err != nil {
				return err
			}
			return nil
		}); nil != err {
			return nil, err
		}

		log.Debug("Stored voting snapshot to disk", "number", snap.Number, "hash", snap.Hash)
	}
	return snap, err
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (c *APos) VerifyUncles(chain consensus.ChainReader, block block.IBlock) error {
	//if len(block.Uncles()) > 0 {
	//	return errors.New("uncles not allowed")
	//}
	return nil
}

// verifySeal checks whether the signature contained in the header satisfies the
// consensus protocol requirements. The method accepts an optional list of parent
// headers that aren't yet part of the local blockchain to generate the snapshots
// from.
func (c *APos) verifySeal(snap *Snapshot, h block.IHeader, parents []block.IHeader) error {
	// Verifying the genesis block is not supported
	header := h.(*block.Header)
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	// Resolve the authorization key and check against signers
	signer, err := ecrecover(header, c.signatures)
	if err != nil {
		return err
	}
	if _, ok := snap.Signers[signer]; !ok {
		log.Infof("err signer: %s, ", signer.String())
		return errUnauthorizedSigner
	}
	for seen, recent := range snap.Recents {
		if recent == signer {
			// Signer is among recents, only fail if the current block doesn't shift it out
			if limit := uint64(len(snap.Signers)/2 + 1); seen > number-limit {
				return errRecentlySigned
			}
		}
	}
	// Ensure that the difficulty corresponds to the turn-ness of the signer
	if !c.fakeDiff {
		inturn := snap.inturn(header.Number.Uint64(), signer)
		if inturn && header.Difficulty.Cmp(diffInTurn) != 0 {
			return errWrongDifficulty
		}
		if !inturn && header.Difficulty.Cmp(diffNoTurn) != 0 {
			return errWrongDifficulty
		}
	}
	return nil
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (c *APos) Prepare(chain consensus.ChainHeaderReader, header block.IHeader) error {
	rawHeader := header.(*block.Header)
	// If the block isn't a checkpoint, cast a random vote (good enough for now)
	rawHeader.Coinbase = types.Address{}
	rawHeader.Nonce = block.BlockNonce{}

	number := rawHeader.Number.Uint64()
	// Assemble the voting snapshot to check which votes make sense
	snap, err := c.snapshot(chain, number-1, rawHeader.ParentHash, nil)
	if err != nil {
		return err
	}
	c.lock.RLock()
	if number%c.config.APos.Epoch != 0 {
		// Gather all the proposals that make sense voting on
		addresses := make([]types.Address, 0, len(c.proposals))
		for address, authorize := range c.proposals {
			if snap.validVote(address, authorize) {
				addresses = append(addresses, address)
			}
		}
		// If there's pending proposals, cast a vote on them
		if len(addresses) > 0 {
			rawHeader.Coinbase = addresses[rand.Intn(len(addresses))]
			if c.proposals[rawHeader.Coinbase] {
				copy(rawHeader.Nonce[:], nonceAuthVote)
			} else {
				copy(rawHeader.Nonce[:], nonceDropVote)
			}
		}
	}

	// Copy signer protected by mutex to avoid race condition
	signer := c.signer
	c.lock.RUnlock()

	// Set the correct difficulty
	rawHeader.Difficulty = calcDifficulty(snap, signer)

	// Ensure the extra data has all its components
	if len(rawHeader.Extra) < extraVanity {
		rawHeader.Extra = append(rawHeader.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(rawHeader.Extra))...)
	}
	rawHeader.Extra = rawHeader.Extra[:extraVanity]

	if number%c.config.APos.Epoch == 0 {
		for _, signer := range snap.signers() {
			rawHeader.Extra = append(rawHeader.Extra, signer[:]...)
		}
	}
	rawHeader.Extra = append(rawHeader.Extra, make([]byte, extraSeal)...)

	// Mix digest is reserved for now, set to empty
	rawHeader.MixDigest = types.Hash{}

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(rawHeader.ParentHash, uint256.NewInt(0).Sub(rawHeader.Number, uint256.NewInt(1)))
	if parent == nil {
		return errors.New("unknown ancestor")
	}
	rawHeader.Time = parent.(*block.Header).Time + c.config.Period
	if rawHeader.Time < uint64(time.Now().Unix())+mergeSignMinTime {
		rawHeader.Time = uint64(time.Now().Unix()) + mergeSignMinTime
	}
	return nil
}

func (c *APos) Rewards(tx kv.RwTx, header block.IHeader, state *state.IntraBlockState, setRewards bool) ([]*block.Reward, error) {
	//calc rewards
	var rewards []*block.Reward

	beijing, _ := uint256.FromBig(c.chainConfig.BeijingBlock)
	if new(uint256.Int).Mod(new(uint256.Int).Sub(header.Number64(), beijing), uint256.NewInt(c.config.APos.RewardEpoch)).
		Cmp(uint256.NewInt(0)) == 0 {
		log.Info("begin setreward", "headnumber", header.Number64().ToBig().String())

		rewardService := newReward(c.config, c.chainConfig)
		accRewards, err := rewardService.SetRewards(tx, header.Number64(), setRewards)
		if err != nil {
			log.Error("setreward error", "err", err)
			return nil, err
		}

		for _, detail := range accRewards {
			if detail.Value.Cmp(uint256.NewInt(0)) > 0 {
				addr := detail.Account
				if !state.Exist(addr) {
					state.CreateAccount(addr, false)
				}

				state.AddBalance(addr, detail.Value)
				rewards = append(rewards, &block.Reward{
					Address: addr,
					Amount:  detail.Value,
				})

				log.Debug("set rewards balance:", "addr", addr, "value", detail.Value)
			}
		}
	}

	return rewards, nil
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given.
func (c *APos) Finalize(chain consensus.ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction, uncles []block.IHeader) ([]*block.Reward, map[types.Address]*uint256.Int, error) {
	// No block rewards in PoA, so the state remains as is and uncles are dropped
	//chain.Config().IsEIP158(header.Number)

	rewards, unpayMap, err := doReward(c.chainConfig, c.config, state, header.(*block.Header), chain)
	if err != nil {
		return nil, nil, err
	}
	rawHeader := header.(*block.Header)
	rawHeader.Root = state.IntermediateRoot()
	// Todo can not verify author
	rawHeader.MixDigest = state.BeforeStateRoot()
	//todo
	//rawHeader.UncleHash = types.CalcUncleHash(nil)
	return rewards, unpayMap, nil
}

// FinalizeAndAssemble implements consensus.Engine, ensuring no uncles are set,
// nor block rewards given, and returns the final block.
func (c *APos) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header block.IHeader, state *state.IntraBlockState, txs []*transaction.Transaction, uncles []block.IHeader, receipts []*block.Receipt) (block.IBlock, []*block.Reward, map[types.Address]*uint256.Int, error) {
	// Finalize block
	rewards, unpay, err := c.Finalize(chain, header, state, txs, uncles)
	if nil != err {
		return nil, nil, nil, err
	}

	// Assemble and return the final block for sealing
	block := block.NewBlockFromReceipt(header, txs, uncles, receipts, rewards)
	return block, rewards, unpay, nil
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
// todo init
func (c *APos) Authorize(signer types.Address, signFn SignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.signer = signer
	c.signFn = signFn
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (c *APos) Seal(chain consensus.ChainHeaderReader, b block.IBlock, results chan<- block.IBlock, stop <-chan struct{}) error {
	header := b.Header().(*block.Header)

	// Sealing the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	// For 0-period chains, refuse to seal empty blocks (no reward but would spin sealing)
	if c.config.Period == 0 && len(b.Transactions()) == 0 {
		return errors.New("sealing paused while waiting for transactions")
	}
	// Don't hold the signer fields for the entire sealing procedure
	c.lock.RLock()
	signer, signFn := c.signer, c.signFn
	c.lock.RUnlock()

	// Bail out if we're unauthorized to sign a block
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if _, authorized := snap.Signers[signer]; !authorized {
		log.Infof("err signer: %s, ", signer.String())
		return errUnauthorizedSigner
	}
	// If we're amongst the recent signers, wait for the next block
	for seen, recent := range snap.Recents {
		if recent == signer {
			// Signer is among recents, only wait if the current block doesn't shift it out
			if limit := uint64(len(snap.Signers)/2 + 1); number < limit || seen > number-limit {
				///limit 1  seen 9999 number 9999- 8919
				return errors.New(fmt.Sprintf("signed recently, must wait for others %d %d %d %s", limit, seen, number, signer.String()))
			}
		}
	}

	// Sweet, the protocol permits us to sign the block, wait for our time
	delay := time.Unix(int64(header.Time), 0).Sub(time.Now()) // nolint: gosimple
	if header.Difficulty.Cmp(diffNoTurn) == 0 {
		wiggle := time.Duration(len(snap.Signers)/2+1) * wiggleTime
		delay += time.Duration(rand.Int63n(int64(wiggle)))

		log.Infof("wiggle %s , time %s, number %d", common.PrettyDuration(wiggle), common.PrettyDuration(delay), header.Number.Uint64())
		log.Trace("Out-of-turn signing requested", "wiggle", common.PrettyDuration(wiggle))
	}

	if c.chainConfig.IsBeijing(header.Number.Uint64()) {
		ctx, cancle := context.WithTimeout(context.Background(), delay)
		defer cancle()
		member := c.CountDepositor()
		aggSign, verifiers, err := SignMerge(ctx, header, member)
		if nil != err {
			return err
		}
		ss := make([]bls.PublicKey, len(verifiers))
		for i, p := range verifiers {
			blsP, err := bls.PublicKeyFromBytes(p.PublicKey[:])
			if nil != err {
				return err
			}
			ss[i] = blsP
		}

		sig, err := bls.SignatureFromBytes(aggSign[:])
		if nil != err {
			return err
		}
		if !sig.FastAggregateVerify(ss, header.Root) {
			return fmt.Errorf("AggSignature verify falied")
		}

		header.Signature = aggSign
		body := b.Body().(*block.Body)
		body.Verifiers = verifiers
		delay = time.Unix(int64(header.Time), 0).Sub(time.Now())
	}

	// Sign all the things!
	sighash, err := signFn(accounts.Account{Address: signer}, accounts.MimetypeClique, APosProto(header))
	if err != nil {
		return err
	}

	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	// Wait until sealing is terminated or delay timeout.
	log.Debug("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		select {
		case results <- b.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header))
		}
	}()

	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have:
// * DIFF_NOTURN(2) if BLOCK_NUMBER % SIGNER_COUNT != SIGNER_INDEX
// * DIFF_INTURN(1) if BLOCK_NUMBER % SIGNER_COUNT == SIGNER_INDEX
func (c *APos) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent block.IHeader) *uint256.Int {
	snap, err := c.snapshot(chain, parent.Number64().Uint64(), parent.Hash(), nil)
	if err != nil {
		return uint256.NewInt(0)
	}
	c.lock.RLock()
	signer := c.signer
	c.lock.RUnlock()
	return calcDifficulty(snap, signer)
}

func calcDifficulty(snap *Snapshot, signer types.Address) *uint256.Int {
	if snap.inturn(snap.Number+1, signer) {
		return diffInTurn
	}
	return diffNoTurn
}

// SealHash returns the hash of a block prior to it being sealed.
func (c *APos) SealHash(header block.IHeader) types.Hash {
	return SealHash(header)
}

// Close implements consensus.Engine. It's a noop for Apoa as there are no background threads.
func (c *APos) Close() error {
	return nil
}

// APIs implements consensus.Engine, returning the user facing RPC API to allow
// controlling the signer voting.
func (c *APos) APIs(chain consensus.ChainReader) []jsonrpc.API {
	return []jsonrpc.API{{
		Namespace: "apos",
		Service:   &API{chain: chain, apos: c},
	}}
}

func (c *APos) Type() params.ConsensusType {
	return params.CliqueConsensus
}

// SealHash returns the hash of a block prior to it being sealed.
func SealHash(header block.IHeader) (hash types.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.(crypto.KeccakState).Read(hash[:])
	return hash
}

func APosProto(header block.IHeader) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header)
	return b.Bytes()
}

func encodeSigHeader(w io.Writer, iHeader block.IHeader) {
	header := mvm_types.FromAmcHeader(iHeader)
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}

func (c *APos) CountDepositor() uint64 {
	var count uint64
	if err := c.db.View(context.Background(), func(tx kv.Tx) error {
		var err error
		count, err = rawdb.DepositNum(tx)
		if nil != err {
			return err
		}
		return nil
	}); nil != err {
		log.Errorf("rawdb.DepositNum() failed, %v", err)
		return 0
	}
	return count
}

func (c *APos) IsServiceTransaction(sender types.Address, syscall consensus.SystemCall) bool {
	return false
}

func SignMerge(ctx context.Context, header *block.Header, depositNum uint64) (types.Signature, []*block.Verify, error) {
	aggrSigns := make([]bls.Signature, 0)
	verifiers := make([]*block.Verify, 0)
	uniq := make(map[types.Address]struct{})

LOOP:
	for {
		select {
		case s := <-aggsign.SigChannel:
			log.Tracef("accept sign, %+v", s)
			if s.Number != header.Number.Uint64() {
				log.Tracef("discard sign: need block number %d, get %d", header.Number.Uint64(), s.Number)
				continue
			}

			if _, ok := uniq[s.Address]; ok {
				continue
			}

			if !s.Check(header.Root) {
				log.Tracef("discard sign: sign check failed! %v", s)
				continue
			}
			sig, err := bls.SignatureFromBytes(s.Sign[:])
			if nil != err {
				return types.Signature{}, nil, err
			}

			aggrSigns = append(aggrSigns, sig)
			verifiers = append(verifiers, &block.Verify{
				Address:   s.Address,
				PublicKey: s.PublicKey,
			})
			uniq[s.Address] = struct{}{}
		case <-ctx.Done():
			break LOOP
		}
	}
	// todo enough sigs check
	// 1
	// uint64(len(aggrSigns)) < depositNum/2
	// uint64(len(aggrSigns)) < 7
	if uint64(len(aggrSigns)) < 3 {
		return types.Signature{}, nil, consensus.ErrNotEnoughSign
	}

	aggS := blst.AggregateSignatures(aggrSigns)
	var aggSign types.Signature
	copy(aggSign[:], aggS.Marshal())
	return aggSign, verifiers, nil
}
