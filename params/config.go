// Copyright 2023 The AmazeChain Authors
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

package params

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/common/paths"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/params/networkname"
	"golang.org/x/crypto/sha3"
	"math/big"
	"path"
	"sort"
	"strconv"
)

//go:embed chainspecs
var chainspecs embed.FS

func readChainSpec(filename string) *conf.Genesis {
	f, err := chainspecs.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("Could not open chainspec for %s: %v", filename, err))
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	spec := &conf.Genesis{}
	err = decoder.Decode(&spec)
	if err != nil {
		panic(fmt.Sprintf("Could not parse chainspec for %s: %v", filename, err))
	}
	return spec
}

type ConsensusType string

const (
	AuRaConsensus   ConsensusType = "aura"
	EtHashConsensus ConsensusType = "ethash"
	CliqueConsensus ConsensusType = "clique"
	ParliaConsensus ConsensusType = "parlia"
	BorConsensus    ConsensusType = "bor"
	ApoaConsensu    ConsensusType = "apos"
	Faker           ConsensusType = "faker" // faker consensus
)

// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = types.HexToHash("0x138734b7044254e5ecbabf8056f5c2b73cd0847aaa5acac7345507cbeab387b8")
	TestnetGenesisHash = types.HexToHash("0x5c0555d9ec963f58c63112862294e7e4836b12802304c23f2ec480a8f55cc5bb")
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = readChainSpec("chainspecs/mainnet.json")

	// TestnetChainConfig contains the chain parameters to run a node on the Test network.
	TestnetChainConfig = readChainSpec("chainspecs/testnet.json")

	CliqueSnapshot = NewSnapshotConfig(10, 1024, 16384, true, "")

	TestChainConfig = &ChainConfig{
		ChainID:               big.NewInt(1),
		Consensus:             EtHashConsensus,
		HomesteadBlock:        big.NewInt(0),
		DAOForkBlock:          nil,
		DAOForkSupport:        false,
		TangerineWhistleBlock: big.NewInt(0),
		TangerineWhistleHash:  types.Hash{},
		SpuriousDragonBlock:   big.NewInt(0),
		ByzantiumBlock:        big.NewInt(0),
		ConstantinopleBlock:   big.NewInt(0),
		PetersburgBlock:       big.NewInt(0),
		IstanbulBlock:         big.NewInt(0),
		MuirGlacierBlock:      big.NewInt(0),
		BerlinBlock:           big.NewInt(0),
		LondonBlock:           nil,
		ArrowGlacierBlock:     nil,
		Ethash:                new(EthashConfig),
		Clique:                nil,
	}

	TestRules = TestChainConfig.Rules(0)

	AmazeChainConfig = &ChainConfig{
		ChainID:               big.NewInt(100100100),
		HomesteadBlock:        big.NewInt(0),
		DAOForkBlock:          nil,
		DAOForkSupport:        false,
		TangerineWhistleBlock: big.NewInt(0),
		TangerineWhistleHash:  types.Hash{},
		SpuriousDragonBlock:   big.NewInt(0),
		ByzantiumBlock:        big.NewInt(0),
		ConstantinopleBlock:   big.NewInt(0),
		PetersburgBlock:       big.NewInt(0),
		IstanbulBlock:         big.NewInt(0),
		MuirGlacierBlock:      big.NewInt(0),
		BerlinBlock:           big.NewInt(0),
		LondonBlock:           big.NewInt(0),
		ArrowGlacierBlock:     big.NewInt(0),
		GrayGlacierBlock:      big.NewInt(0),
		BeijingBlock:          big.NewInt(10000),
	}

	// AmazeChainTrustedCheckpoint contains the light client trusted checkpoint for the main network.
	AmazeChainTrustedCheckpoint = &TrustedCheckpoint{
		SectionIndex: 413,
		SectionHead:  types.HexToHash("0x8aa8e64ceadcdc5f23bc41d2acb7295a261a5cf680bb00a34f0e01af08200083"),
		CHTRoot:      types.HexToHash("0x008af584d385a2610706c5a439d39f15ddd4b691c5d42603f65ae576f703f477"),
		BloomRoot:    types.HexToHash("0x5a081af71a588f4d90bced242545b08904ad4fb92f7effff2ceb6e50e6dec157"),
	}
	// AmazeChainCheckpointOracle contains a set of configs for the main network oracle.
	AmazeChainCheckpointOracle = &CheckpointOracleConfig{
		Address: types.HexToAddress("0x9a9070028361F7AAbeB3f2F2Dc07F82C4a98A02a"),
		Signers: []types.Address{
			types.HexToAddress("0x1b2C260efc720BE89101890E4Db589b44E950527"), // Peter
			types.HexToAddress("0x78d1aD571A1A09D60D9BBf25894b44e4C8859595"), // Martin
			types.HexToAddress("0x286834935f4A8Cfb4FF4C77D5770C2775aE2b0E7"), // Zsolt
			types.HexToAddress("0xb86e2B0Ab5A4B1373e40c51A7C712c70Ba2f9f8E"), // Gary
			types.HexToAddress("0x0DF8fa387C602AE62559cC4aFa4972A7045d6707"), // Guillaume
		},
		Threshold: 2,
	}
)

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	ChainName string
	ChainID   *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection

	Consensus ConsensusType `json:"consensus,omitempty"` // aura, ethash or clique

	HomesteadBlock *big.Int `json:"homesteadBlock,omitempty"` // Homestead switch block (nil = no fork, 0 = already homestead)

	DAOForkBlock   *big.Int `json:"daoForkBlock,omitempty"`   // TheDAO hard-fork switch block (nil = no fork)
	DAOForkSupport bool     `json:"daoForkSupport,omitempty"` // Whether the nodes supports or opposes the DAO hard-fork

	// Tangerine Whistle (EIP150) implements the Gas price changes (https://github.com/ethereum/EIPs/issues/150)
	TangerineWhistleBlock *big.Int   `json:"eip150Block,omitempty"` // EIP150 HF block (nil = no fork)
	TangerineWhistleHash  types.Hash `json:"eip150Hash,omitempty"`  // EIP150 HF hash (needed for header only clients as only gas pricing changed)

	SpuriousDragonBlock *big.Int `json:"eip155Block,omitempty"` // Spurious Dragon HF block

	ByzantiumBlock      *big.Int `json:"byzantiumBlock,omitempty"`      // Byzantium switch block (nil = no fork, 0 = already on byzantium)
	ConstantinopleBlock *big.Int `json:"constantinopleBlock,omitempty"` // Constantinople switch block (nil = no fork, 0 = already activated)
	PetersburgBlock     *big.Int `json:"petersburgBlock,omitempty"`     // Petersburg switch block (nil = same as Constantinople)
	IstanbulBlock       *big.Int `json:"istanbulBlock,omitempty"`       // Istanbul switch block (nil = no fork, 0 = already on istanbul)
	MuirGlacierBlock    *big.Int `json:"muirGlacierBlock,omitempty"`    // EIP-2384 (bomb delay) switch block (nil = no fork, 0 = already activated)
	BerlinBlock         *big.Int `json:"berlinBlock,omitempty"`         // Berlin switch block (nil = no fork, 0 = already on berlin)
	LondonBlock         *big.Int `json:"londonBlock,omitempty"`         // London switch block (nil = no fork, 0 = already on london)
	ArrowGlacierBlock   *big.Int `json:"arrowGlacierBlock,omitempty"`   // EIP-4345 (bomb delay) switch block (nil = no fork, 0 = already activated)
	GrayGlacierBlock    *big.Int `json:"grayGlacierBlock,omitempty"`    // EIP-5133 (bomb delay) switch block (nil = no fork, 0 = already activated)

	// EIP-3675: Upgrade consensus to Proof-of-Stake
	TerminalTotalDifficulty       *big.Int `json:"terminalTotalDifficulty,omitempty"`       // The merge happens when terminal total difficulty is reached
	TerminalTotalDifficultyPassed bool     `json:"terminalTotalDifficultyPassed,omitempty"` // Disable PoW sync for networks that have already passed through the Merge
	MergeNetsplitBlock            *big.Int `json:"mergeNetsplitBlock,omitempty"`            // Virtual fork after The Merge to use as a network splitter; see FORK_NEXT_VALUE in EIP-3675

	ShanghaiBlock    *big.Int `json:"shanghaiBlock,omitempty"` // Shanghai switch block (nil = no fork, 0 = already activated)
	CancunBlock      *big.Int `json:"cancunTime,omitempty"`
	ShardingForkTime *big.Int `json:"shardingForkTime,omitempty"`
	PragueTime       *big.Int `json:"pragueTime,omitempty"`

	// Parlia fork blocks
	//RamanujanBlock  *big.Int    `json:"ramanujanBlock,omitempty" toml:",omitempty"`  // ramanujanBlock switch block (nil = no fork, 0 = already activated)
	//NielsBlock      *big.Int    `json:"nielsBlock,omitempty" toml:",omitempty"`      // nielsBlock switch block (nil = no fork, 0 = already activated)
	//MirrorSyncBlock *big.Int    `json:"mirrorSyncBlock,omitempty" toml:",omitempty"` // mirrorSyncBlock switch block (nil = no fork, 0 = already activated)
	//BrunoBlock      *big.Int    `json:"brunoBlock,omitempty" toml:",omitempty"`      // brunoBlock switch block (nil = no fork, 0 = already activated)
	//EulerBlock      *big.Int    `json:"eulerBlock,omitempty" toml:",omitempty"`      // eulerBlock switch block (nil = no fork, 0 = already activated)
	//GibbsBlock      *big.Int    `json:"gibbsBlock,omitempty" toml:",omitempty"`      // gibbsBlock switch block (nil = no fork, 0 = already activated)
	NanoBlock    *big.Int `json:"nanoBlock,omitempty" toml:",omitempty"`    // nanoBlock switch block (nil = no fork, 0 = already activated)
	MoranBlock   *big.Int `json:"moranBlock,omitempty" toml:",omitempty"`   // moranBlock switch block (nil = no fork, 0 = already activated)
	BeijingBlock *big.Int `json:"beijingBlock,omitempty" toml:",omitempty"` // beijingBlock switch block (nil = no fork, 0 = already activated)
	//Apos         *AposConfig `json:"apos,omitempty"`

	// Gnosis Chain fork blocks
	//PosdaoBlock *big.Int `json:"posdaoBlock,omitempty"`
	//
	Eip1559FeeCollector           *types.Address `json:"eip1559FeeCollector,omitempty"`           // (Optional) Address where burnt EIP-1559 fees go to
	Eip1559FeeCollectorTransition *big.Int       `json:"eip1559FeeCollectorTransition,omitempty"` // (Optional) Block from which burnt EIP-1559 fees go to the Eip1559FeeCollector

	// Various consensus engines
	Ethash *EthashConfig `json:"ethash,omitempty"`
	Clique *CliqueConfig `json:"clique,omitempty"`
	Aura   *AuRaConfig   `json:"aura,omitempty"`
	Parlia *ParliaConfig `json:"parlia,omitempty" toml:",omitempty"`
	Bor    *BorConfig    `json:"bor,omitempty"`
	Apos   *APosConfig   `json:"apos,omitempty"`
}

// EthashConfig is the consensus engine configs for proof-of-work based sealing.
type EthashConfig struct{}

// String implements the stringer interface, returning the consensus engine details.
func (c *EthashConfig) String() string {
	return "ethash"
}

// CliqueConfig is the consensus engine configs for proof-of-authority based sealing.
type CliqueConfig struct {
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *CliqueConfig) String() string {
	return "clique"
}

type APosConfig struct {
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint

	RewardEpoch uint64   `json:"rewardEpoch"`
	RewardLimit *big.Int `json:"rewardLimit"`

	DepositContract    string `json:"depositContract"`    // Deposit contract
	DepositNFTContract string `json:"depositNFTContract"` // Deposit NFT contract
}

// String implements the stringer interface, returning the consensus engine details.
func (b *APosConfig) String() string {
	return fmt.Sprintf("{DepositContract: %v, NFTDepositContract:%v, Period: %v, Epoch: %v, RewardEpoch: %v, RewardLimit: %v}",
		b.DepositContract,
		b.DepositNFTContract,
		b.Period,
		b.Epoch,
		b.RewardEpoch,
		b.RewardLimit,
	)
}

// AuRaConfig is the consensus engine configs for proof-of-authority based sealing.
type AuRaConfig struct {
	DBPath    string
	InMemory  bool
	Etherbase common.Address // same as miner etherbase
}

// String implements the stringer interface, returning the consensus engine details.
func (c *AuRaConfig) String() string {
	return "aura"
}

type ParliaConfig struct {
	DBPath   string
	InMemory bool
	Period   uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch    uint64 `json:"epoch"`  // Epoch length to update validatorSet
}

// String implements the stringer interface, returning the consensus engine details.
func (b *ParliaConfig) String() string {
	return "parlia"
}

//type AposConfig struct {
//	DepositContract string `json:"depositContract"` // Deposit contract
//	Period          uint64 `json:"period"`          // Number of seconds between blocks to enforce
//	Epoch           uint64 `json:"epoch"`           // Epoch length to reset votes and checkpoint
//
//	RewardEpoch uint64 `json:"rewardEpoch"`
//	RewardLimit uint64 `json:"rewardLimit"`
//}

// BorConfig is the consensus engine configs for Matic bor based sealing.
type BorConfig struct {
	Period                map[string]uint64 `json:"period"`                // Number of seconds between blocks to enforce
	ProducerDelay         uint64            `json:"producerDelay"`         // Number of seconds delay between two producer interval
	Sprint                uint64            `json:"sprint"`                // Epoch length to proposer
	BackupMultiplier      map[string]uint64 `json:"backupMultiplier"`      // Backup multiplier to determine the wiggle time
	ValidatorContract     string            `json:"validatorContract"`     // Validator set contract
	StateReceiverContract string            `json:"stateReceiverContract"` // State receiver contract

	OverrideStateSyncRecords map[string]int         `json:"overrideStateSyncRecords"` // override state records count
	BlockAlloc               map[string]interface{} `json:"blockAlloc"`
	JaipurBlock              uint64                 `json:"jaipurBlock"` // Jaipur switch block (nil = no fork, 0 = already on jaipur)
}

// String implements the stringer interface, returning the consensus engine details.
func (b *BorConfig) String() string {
	return "bor"
}

func (c *BorConfig) CalculateBackupMultiplier(number uint64) uint64 {
	return c.calculateBorConfigHelper(c.BackupMultiplier, number)
}

func (c *BorConfig) CalculatePeriod(number uint64) uint64 {
	return c.calculateBorConfigHelper(c.Period, number)
}

func (c *BorConfig) IsJaipur(number uint64) bool {
	return number >= c.JaipurBlock
}

func (c *BorConfig) calculateBorConfigHelper(field map[string]uint64, number uint64) uint64 {
	keys := make([]string, 0, len(field))
	for k := range field {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i := 0; i < len(keys)-1; i++ {
		valUint, _ := strconv.ParseUint(keys[i], 10, 64)
		valUintNext, _ := strconv.ParseUint(keys[i+1], 10, 64)
		if number > valUint && number < valUintNext {
			return field[keys[i]]
		}
	}
	return field[keys[len(keys)-1]]
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v, Homestead: %v, DAO: %v, DAO Support: %v, Tangerine Whistle: %v, Spurious Dragon: %v, Byzantium: %v, Constantinople: %v, Petersburg: %v, Istanbul: %v, Muir Glacier: %v, Berlin: %v, London: %v, Arrow Glacier: %v, Gray Glacier: %v, Terminal Total Difficulty: %v, Merge Netsplit: %v, Shanghai: %v, Cancun: %v}",
		c.ChainID,
		c.HomesteadBlock,
		c.DAOForkBlock,
		c.DAOForkSupport,
		c.TangerineWhistleBlock,
		c.SpuriousDragonBlock,
		c.ByzantiumBlock,
		c.ConstantinopleBlock,
		c.PetersburgBlock,
		c.IstanbulBlock,
		c.MuirGlacierBlock,
		c.BerlinBlock,
		c.LondonBlock,
		c.ArrowGlacierBlock,
		c.GrayGlacierBlock,
		c.TerminalTotalDifficulty,
		c.MergeNetsplitBlock,
		c.ShanghaiBlock,
		c.CancunBlock,
	)
}

func (c *ChainConfig) IsHeaderWithSeal() bool {
	return c.Consensus == AuRaConsensus
}

type ConsensusSnapshotConfig struct {
	CheckpointInterval uint64 // Number of blocks after which to save the vote snapshot to the database
	InmemorySnapshots  int    // Number of recent vote snapshots to keep in memory
	InmemorySignatures int    // Number of recent block signatures to keep in memory
	DBPath             string
	InMemory           bool
}

const cliquePath = "clique"

func NewSnapshotConfig(checkpointInterval uint64, inmemorySnapshots int, inmemorySignatures int, inmemory bool, dbPath string) *ConsensusSnapshotConfig {
	if len(dbPath) == 0 {
		dbPath = paths.DefaultDataDir()
	}

	return &ConsensusSnapshotConfig{
		checkpointInterval,
		inmemorySnapshots,
		inmemorySignatures,
		path.Join(dbPath, cliquePath),
		inmemory,
	}
}

// NetworkNames are user friendly names to use in the chain spec banner.
var NetworkNames = map[string]string{
	"100100100": "testnet",
	"94":        "mainnet",
	//"131":       "testnet",
}

// Description returns a human-readable description of ChainConfig.
func (c *ChainConfig) Description() string {
	var banner string

	// Create some basinc network config output
	network := NetworkNames[c.ChainID.String()]
	if network == "" {
		network = "unknown"
	}
	banner += fmt.Sprintf("Chain ID:  %v (%s)\n", c.ChainID, network)

	// Create a list of forks with a short description of them. Forks that only
	// makes sense for mainnet should be optional at printing to avoid bloating
	// the output for testnets and private networks.
	//banner += fmt.Sprintf(" - Homestead:                   #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/homestead.md)\n", c.HomesteadBlock)
	//if c.DAOForkBlock != nil {
	//	banner += fmt.Sprintf(" - DAO Fork:                    #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/dao-fork.md)\n", c.DAOForkBlock)
	//}
	//banner += fmt.Sprintf(" - Tangerine Whistle (EIP 150): #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/tangerine-whistle.md)\n", c.TangerineWhistleBlock)
	//banner += fmt.Sprintf(" - Spurious Dragon/1 (EIP 155): #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/spurious-dragon.md)\n", c.SpuriousDragonBlock)
	//banner += fmt.Sprintf(" - Spurious Dragon/2 (EIP 158): #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/spurious-dragon.md)\n", c.SpuriousDragonBlock)
	//banner += fmt.Sprintf(" - Byzantium:                   #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/byzantium.md)\n", c.ByzantiumBlock)
	//banner += fmt.Sprintf(" - Constantinople:              #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/constantinople.md)\n", c.ConstantinopleBlock)
	//banner += fmt.Sprintf(" - Petersburg:                  #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/petersburg.md)\n", c.PetersburgBlock)
	//banner += fmt.Sprintf(" - Istanbul:                    #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/istanbul.md)\n", c.IstanbulBlock)
	//if c.MuirGlacierBlock != nil {
	//	banner += fmt.Sprintf(" - Muir Glacier:                #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/muir-glacier.md)\n", c.MuirGlacierBlock)
	//}
	//banner += fmt.Sprintf(" - Berlin:                      #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/berlin.md)\n", c.BerlinBlock)
	//banner += fmt.Sprintf(" - London:                      #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/london.md)\n", c.LondonBlock)
	//if c.ArrowGlacierBlock != nil {
	//	banner += fmt.Sprintf(" - Arrow Glacier:               #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/arrow-glacier.md)\n", c.ArrowGlacierBlock)
	//}
	//if c.GrayGlacierBlock != nil {
	//	banner += fmt.Sprintf(" - Gray Glacier:                #%-8v (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/gray-glacier.md)\n", c.GrayGlacierBlock)
	//}
	banner += "\n"

	// Add a special section for the merge as it's non-obvious
	//if c.TerminalTotalDifficulty == nil {
	//	banner += "The Merge is not yet available for this network!\n"
	//	banner += " - Hard-fork specification: https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/paris.md\n"
	//} else {
	//	banner += "Merge configured:\n"
	//	banner += " - Hard-fork specification:    https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/paris.md\n"
	//	banner += fmt.Sprintf(" - Network known to be merged: %v\n", c.TerminalTotalDifficultyPassed)
	//	banner += fmt.Sprintf(" - Total terminal difficulty:  %v\n", c.TerminalTotalDifficulty)
	//	if c.MergeNetsplitBlock != nil {
	//		banner += fmt.Sprintf(" - Merge netsplit block:       #%-8v\n", c.MergeNetsplitBlock)
	//	}
	//}
	banner += "\n"

	return banner
}

// IsHomestead returns whether num is either equal to the homestead block or greater.
func (c *ChainConfig) IsHomestead(num uint64) bool {
	return isForked(c.HomesteadBlock, num)
}

// IsDAOFork returns whether num is either equal to the DAO fork block or greater.
func (c *ChainConfig) IsDAOFork(num uint64) bool {
	return isForked(c.DAOForkBlock, num)
}

// IsTangerineWhistle returns whether num is either equal to the Tangerine Whistle (EIP150) fork block or greater.
func (c *ChainConfig) IsTangerineWhistle(num uint64) bool {
	return isForked(c.TangerineWhistleBlock, num)
}

// IsSpuriousDragon returns whether num is either equal to the Spurious Dragon fork block or greater.
func (c *ChainConfig) IsSpuriousDragon(num uint64) bool {
	return isForked(c.SpuriousDragonBlock, num)
}

// IsByzantium returns whether num is either equal to the Byzantium fork block or greater.
func (c *ChainConfig) IsByzantium(num uint64) bool {
	return isForked(c.ByzantiumBlock, num)
}

// IsConstantinople returns whether num is either equal to the Constantinople fork block or greater.
func (c *ChainConfig) IsConstantinople(num uint64) bool {
	return isForked(c.ConstantinopleBlock, num)
}

func (c *ChainConfig) IsMoran(num uint64) bool {
	return isForked(c.MoranBlock, num)
}

//	func (c *ChainConfig) IsOnMoran(num *big.Int) bool {
//		return configNumEqual(c.MoranBlock, num)
//	}
//
// IsNano returns whether num is either equal to the euler fork block or greater.
func (c *ChainConfig) IsNano(num uint64) bool {
	return isForked(c.NanoBlock, num)
}

//
//func (c *ChainConfig) IsOnNano(num *big.Int) bool {
//	return configNumEqual(c.NanoBlock, num)
//}

// IsMuirGlacier returns whether num is either equal to the Muir Glacier (EIP-2384) fork block or greater.
func (c *ChainConfig) IsMuirGlacier(num uint64) bool {
	return isForked(c.MuirGlacierBlock, num)
}

// IsPetersburg returns whether num is either
// - equal to or greater than the PetersburgBlock fork block,
// - OR is nil, and Constantinople is active
func (c *ChainConfig) IsPetersburg(num uint64) bool {
	return isForked(c.PetersburgBlock, num) || c.PetersburgBlock == nil && isForked(c.ConstantinopleBlock, num)
}

// IsIstanbul returns whether num is either equal to the Istanbul fork block or greater.
func (c *ChainConfig) IsIstanbul(num uint64) bool {
	return isForked(c.IstanbulBlock, num)
}

// IsBerlin returns whether num is either equal to the Berlin fork block or greater.
func (c *ChainConfig) IsBerlin(num uint64) bool {
	return isForked(c.BerlinBlock, num)
}

// IsLondon returns whether num is either equal to the London fork block or greater.
func (c *ChainConfig) IsLondon(num uint64) bool {
	return isForked(c.LondonBlock, num)
}

// IsArrowGlacier returns whether num is either equal to the Arrow Glacier (EIP-4345) fork block or greater.
func (c *ChainConfig) IsArrowGlacier(num uint64) bool {
	return isForked(c.ArrowGlacierBlock, num)
}

// IsGrayGlacier returns whether num is either equal to the Gray Glacier (EIP-5133) fork block or greater.
func (c *ChainConfig) IsGrayGlacier(num uint64) bool {
	return isForked(c.GrayGlacierBlock, num)
}

// IsShanghai returns whether num is either equal to the Shanghai fork block or greater.
func (c *ChainConfig) IsShanghai(num uint64) bool {
	return isForked(c.ShanghaiBlock, num)
}

// IsCancun returns whether num is either equal to the Cancun fork block or greater.
func (c *ChainConfig) IsCancun(num uint64) bool {
	return isForked(c.CancunBlock, num)
}

// IsPrague returns whether time is either equal to the Prague fork time or greater.
func (c *ChainConfig) IsPrague(time uint64) bool {
	return isForked(c.PragueTime, time)
}

// IsBeijing returns whether num is either equal to the IsBeijing fork block or greater.
func (c *ChainConfig) IsBeijing(num uint64) bool {
	return isForked(c.BeijingBlock, num)
}

func (c *ChainConfig) IsEip1559FeeCollector(num uint64) bool {
	return c.Eip1559FeeCollector != nil && isForked(c.Eip1559FeeCollectorTransition, num)
}

//func (c *ChainConfig) IsMoran(num uint64) bool {
//	return isForked(c.MoranBlock, num)
//}
//
//func (c *ChainConfig) IsOnMoran(num *big.Int) bool {
//	return numEqual(c.MoranBlock, num)
//}
//
//// IsNano returns whether num is either equal to the euler fork block or greater.
//func (c *ChainConfig) IsNano(num uint64) bool {
//	return isForked(c.NanoBlock, num)
//}

// CheckCompatible checks whether scheduled fork transitions have been imported
// with a mismatching chain configuration.
func (c *ChainConfig) CheckCompatible(newcfg *ChainConfig, height uint64) *ConfigCompatError {
	bhead := height

	// Iterate checkCompatible to find the lowest conflict.
	var lasterr *ConfigCompatError
	for {
		err := c.checkCompatible(newcfg, bhead)
		if err == nil || (lasterr != nil && err.RewindTo == lasterr.RewindTo) {
			break
		}
		lasterr = err
		bhead = err.RewindTo
	}
	return lasterr
}

// CheckConfigForkOrder checks that we don't "skip" any forks, geth isn't pluggable enough
// to guarantee that forks can be implemented in a different order than on official networks
func (c *ChainConfig) CheckConfigForkOrder() error {
	if c != nil && c.ChainID != nil && c.ChainID.Uint64() == 77 {
		return nil
	}
	type fork struct {
		name     string
		block    *big.Int
		optional bool // if true, the fork may be nil and next fork is still allowed
	}
	var lastFork fork
	for _, cur := range []fork{
		{name: "homesteadBlock", block: c.HomesteadBlock},
		{name: "daoForkBlock", block: c.DAOForkBlock, optional: true},
		{name: "eip150Block", block: c.TangerineWhistleBlock},
		{name: "eip155Block", block: c.SpuriousDragonBlock},
		{name: "byzantiumBlock", block: c.ByzantiumBlock},
		{name: "constantinopleBlock", block: c.ConstantinopleBlock},
		{name: "petersburgBlock", block: c.PetersburgBlock},
		{name: "istanbulBlock", block: c.IstanbulBlock},
		{name: "muirGlacierBlock", block: c.MuirGlacierBlock, optional: true},
		//{name: "eulerBlock", block: c.EulerBlock, optional: true},
		//{name: "gibbsBlock", block: c.GibbsBlock, optional: true},
		{name: "berlinBlock", block: c.BerlinBlock},
		{name: "londonBlock", block: c.LondonBlock},
		{name: "arrowGlacierBlock", block: c.ArrowGlacierBlock, optional: true},
		{name: "grayGlacierBlock", block: c.GrayGlacierBlock, optional: true},
		{name: "mergeNetsplitBlock", block: c.MergeNetsplitBlock, optional: true},
		{name: "shanghaiBlock", block: c.ShanghaiBlock},
		{name: "cancunBlock", block: c.CancunBlock},
	} {
		if lastFork.name != "" {
			// Next one must be higher number
			if lastFork.block == nil && cur.block != nil {
				return fmt.Errorf("unsupported fork ordering: %v not enabled, but %v enabled at %v",
					lastFork.name, cur.name, cur.block)
			}
			if lastFork.block != nil && cur.block != nil {
				if lastFork.block.Cmp(cur.block) > 0 {
					return fmt.Errorf("unsupported fork ordering: %v enabled at %v, but %v enabled at %v",
						lastFork.name, lastFork.block, cur.name, cur.block)
				}
			}
			// If it was optional and not set, then ignore it
		}
		if !cur.optional || cur.block != nil {
			lastFork = cur
		}
	}
	return nil
}

func (c *ChainConfig) checkCompatible(newcfg *ChainConfig, head uint64) *ConfigCompatError {
	// Ethereum mainnet forks
	if isForkIncompatible(c.HomesteadBlock, newcfg.HomesteadBlock, head) {
		return newCompatError("Homestead fork block", c.HomesteadBlock, newcfg.HomesteadBlock)
	}
	if isForkIncompatible(c.DAOForkBlock, newcfg.DAOForkBlock, head) {
		return newCompatError("DAO fork block", c.DAOForkBlock, newcfg.DAOForkBlock)
	}
	if c.IsDAOFork(head) && c.DAOForkSupport != newcfg.DAOForkSupport {
		return newCompatError("DAO fork support flag", c.DAOForkBlock, newcfg.DAOForkBlock)
	}
	if isForkIncompatible(c.TangerineWhistleBlock, newcfg.TangerineWhistleBlock, head) {
		return newCompatError("Tangerine Whistle fork block", c.TangerineWhistleBlock, newcfg.TangerineWhistleBlock)
	}
	if isForkIncompatible(c.SpuriousDragonBlock, newcfg.SpuriousDragonBlock, head) {
		return newCompatError("Spurious Dragon fork block", c.SpuriousDragonBlock, newcfg.SpuriousDragonBlock)
	}
	if c.IsSpuriousDragon(head) && !configNumEqual(c.ChainID, newcfg.ChainID) {
		return newCompatError("EIP155 chain ID", c.SpuriousDragonBlock, newcfg.SpuriousDragonBlock)
	}
	if isForkIncompatible(c.ByzantiumBlock, newcfg.ByzantiumBlock, head) {
		return newCompatError("Byzantium fork block", c.ByzantiumBlock, newcfg.ByzantiumBlock)
	}
	if isForkIncompatible(c.ConstantinopleBlock, newcfg.ConstantinopleBlock, head) {
		return newCompatError("Constantinople fork block", c.ConstantinopleBlock, newcfg.ConstantinopleBlock)
	}
	if isForkIncompatible(c.PetersburgBlock, newcfg.PetersburgBlock, head) {
		// the only case where we allow Petersburg to be set in the past is if it is equal to Constantinople
		// mainly to satisfy fork ordering requirements which state that Petersburg fork be set if Constantinople fork is set
		if isForkIncompatible(c.ConstantinopleBlock, newcfg.PetersburgBlock, head) {
			return newCompatError("Petersburg fork block", c.PetersburgBlock, newcfg.PetersburgBlock)
		}
	}
	if isForkIncompatible(c.IstanbulBlock, newcfg.IstanbulBlock, head) {
		return newCompatError("Istanbul fork block", c.IstanbulBlock, newcfg.IstanbulBlock)
	}
	if isForkIncompatible(c.MuirGlacierBlock, newcfg.MuirGlacierBlock, head) {
		return newCompatError("Muir Glacier fork block", c.MuirGlacierBlock, newcfg.MuirGlacierBlock)
	}
	if isForkIncompatible(c.BerlinBlock, newcfg.BerlinBlock, head) {
		return newCompatError("Berlin fork block", c.BerlinBlock, newcfg.BerlinBlock)
	}
	if isForkIncompatible(c.LondonBlock, newcfg.LondonBlock, head) {
		return newCompatError("London fork block", c.LondonBlock, newcfg.LondonBlock)
	}
	if isForkIncompatible(c.ArrowGlacierBlock, newcfg.ArrowGlacierBlock, head) {
		return newCompatError("Arrow Glacier fork block", c.ArrowGlacierBlock, newcfg.ArrowGlacierBlock)
	}
	if isForkIncompatible(c.GrayGlacierBlock, newcfg.GrayGlacierBlock, head) {
		return newCompatError("Gray Glacier fork block", c.GrayGlacierBlock, newcfg.GrayGlacierBlock)
	}
	if isForkIncompatible(c.MergeNetsplitBlock, newcfg.MergeNetsplitBlock, head) {
		return newCompatError("Merge netsplit block", c.MergeNetsplitBlock, newcfg.MergeNetsplitBlock)
	}
	if isForkIncompatible(c.ShanghaiBlock, newcfg.ShanghaiBlock, head) {
		return newCompatError("Shanghai fork block", c.ShanghaiBlock, newcfg.ShanghaiBlock)
	}
	if isForkIncompatible(c.CancunBlock, newcfg.CancunBlock, head) {
		return newCompatError("Cancun fork block", c.CancunBlock, newcfg.CancunBlock)
	}

	// Parlia forks
	//if isForkIncompatible(c.RamanujanBlock, newcfg.RamanujanBlock, head) {
	//	return newCompatError("Ramanujan fork block", c.RamanujanBlock, newcfg.RamanujanBlock)
	//}
	//if isForkIncompatible(c.NielsBlock, newcfg.NielsBlock, head) {
	//	return newCompatError("Niels fork block", c.NielsBlock, newcfg.NielsBlock)
	//}
	//if isForkIncompatible(c.MirrorSyncBlock, newcfg.MirrorSyncBlock, head) {
	//	return newCompatError("MirrorSync fork block", c.MirrorSyncBlock, newcfg.MirrorSyncBlock)
	//}
	//if isForkIncompatible(c.BrunoBlock, newcfg.BrunoBlock, head) {
	//	return newCompatError("Bruno fork block", c.BrunoBlock, newcfg.BrunoBlock)
	//}
	//if isForkIncompatible(c.EulerBlock, newcfg.EulerBlock, head) {
	//	return newCompatError("Euler fork block", c.EulerBlock, newcfg.EulerBlock)
	//}
	//if isForkIncompatible(c.GibbsBlock, newcfg.GibbsBlock, head) {
	//	return newCompatError("Gibbs fork block", c.GibbsBlock, newcfg.GibbsBlock)
	//}
	//if isForkIncompatible(c.NanoBlock, newcfg.NanoBlock, head) {
	//	return newCompatError("Nano fork block", c.NanoBlock, newcfg.NanoBlock)
	//}
	//if isForkIncompatible(c.MoranBlock, newcfg.MoranBlock, head) {
	//	return newCompatError("moran fork block", c.MoranBlock, newcfg.MoranBlock)
	//}
	return nil
}

// isForkIncompatible returns true if a fork scheduled at s1 cannot be rescheduled to
// block s2 because head is already past the fork.
func isForkIncompatible(s1, s2 *big.Int, head uint64) bool {
	return (isForked(s1, head) || isForked(s2, head)) && !configNumEqual(s1, s2)
}

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s *big.Int, head uint64) bool {
	if s == nil {
		return false
	}
	return s.Uint64() <= head
}

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// ConfigCompatError is raised if the locally-stored blockchain is initialised with a
// ChainConfig that would alter the past.
type ConfigCompatError struct {
	What string
	// block numbers of the stored and new configurations
	StoredConfig, NewConfig *big.Int
	// the block number to which the local chain must be rewound to correct the error
	RewindTo uint64
}

func newCompatError(what string, storedblock, newblock *big.Int) *ConfigCompatError {
	var rew *big.Int
	switch {
	case storedblock == nil:
		rew = newblock
	case newblock == nil || storedblock.Cmp(newblock) < 0:
		rew = storedblock
	default:
		rew = newblock
	}
	err := &ConfigCompatError{what, storedblock, newblock, 0}
	if rew != nil && rew.Sign() > 0 {
		err.RewindTo = rew.Uint64() - 1
	}
	return err
}

func (err *ConfigCompatError) Error() string {
	return fmt.Sprintf("mismatching %s in database (have %d, want %d, rewindto %d)", err.What, err.StoredConfig, err.NewConfig, err.RewindTo)
}

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainID                                                 *big.Int
	IsHomestead, IsTangerineWhistle, IsSpuriousDragon       bool
	IsByzantium, IsConstantinople, IsPetersburg, IsIstanbul bool
	IsBerlin, IsLondon, IsShanghai, IsCancun, IsPrague      bool
	IsNano, IsMoran                                         bool
	IsEip1559FeeCollector                                   bool
	IsParlia, IsStarknet, IsAura, IsBeijing                 bool
}

// Rules ensures c's ChainID is not nil.
func (c *ChainConfig) Rules(num uint64) *Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return &Rules{
		ChainID:               new(big.Int).Set(chainID),
		IsHomestead:           c.IsHomestead(num),
		IsTangerineWhistle:    c.IsTangerineWhistle(num),
		IsSpuriousDragon:      c.IsSpuriousDragon(num),
		IsByzantium:           c.IsByzantium(num),
		IsConstantinople:      c.IsConstantinople(num),
		IsPetersburg:          c.IsPetersburg(num),
		IsIstanbul:            c.IsIstanbul(num),
		IsBerlin:              c.IsBerlin(num),
		IsLondon:              c.IsLondon(num),
		IsShanghai:            c.IsShanghai(num),
		IsCancun:              c.IsCancun(num),
		IsPrague:              c.IsPrague(num),
		IsNano:                c.IsNano(num),
		IsMoran:               c.IsMoran(num),
		IsEip1559FeeCollector: c.IsEip1559FeeCollector(num),
		IsParlia:              c.Parlia != nil,
		IsAura:                c.Aura != nil,
		IsBeijing:             c.IsBeijing(num),
	}
}

func ChainConfigByChainName(chain string) *ChainConfig {
	switch chain {
	case networkname.MainnetChainName:
		return MainnetChainConfig.Config
	case networkname.TestnetChainName:
		return TestnetChainConfig.Config
	default:
		return nil
	}
}

func GenesisByChainName(chain string) *conf.Genesis {
	switch chain {
	case networkname.MainnetChainName:
		return MainnetChainConfig
	case networkname.TestnetChainName:
		return TestnetChainConfig
	default:
		return nil
	}
}

func GenesisHashByChainName(chain string) *types.Hash {
	switch chain {
	case networkname.MainnetChainName:
		return &MainnetGenesisHash
	case networkname.TestnetChainName:
		return &TestnetGenesisHash
	default:
		return nil
	}
}

func ChainConfigByGenesisHash(genesisHash types.Hash) *ChainConfig {
	switch {
	case genesisHash == MainnetGenesisHash:
		return MainnetChainConfig.Config
	case genesisHash == TestnetGenesisHash:
		return TestnetChainConfig.Config
	default:
		return nil
	}
}

func NetworkIDByChainName(chain string) uint64 {
	switch chain {
	case networkname.MainnetChainName:
		return 97
	case networkname.TestnetChainName:
		return 100100100
	default:
		config := ChainConfigByChainName(chain)
		if config == nil {
			return 0
		}
		return config.ChainID.Uint64()
	}
}

// TrustedCheckpoint represents a set of post-processed trie roots (CHT and
// BloomTrie) associated with the appropriate section index and head hash. It is
// used to start light syncing from this checkpoint and avoid downloading the
// entire header chain while still being able to securely access old headers/logs.
type TrustedCheckpoint struct {
	SectionIndex uint64     `json:"sectionIndex"`
	SectionHead  types.Hash `json:"sectionHead"`
	CHTRoot      types.Hash `json:"chtRoot"`
	BloomRoot    types.Hash `json:"bloomRoot"`
}

// HashEqual returns an indicator comparing the itself hash with given one.
func (c *TrustedCheckpoint) HashEqual(hash types.Hash) bool {
	if c.Empty() {
		return hash == types.Hash{}
	}
	return c.Hash() == hash
}

// Hash returns the hash of checkpoint's four key fields(index, sectionHead, chtRoot and bloomTrieRoot).
func (c *TrustedCheckpoint) Hash() types.Hash {
	var sectionIndex [8]byte
	binary.BigEndian.PutUint64(sectionIndex[:], c.SectionIndex)

	w := sha3.NewLegacyKeccak256()
	w.Write(sectionIndex[:])
	w.Write(c.SectionHead[:])
	w.Write(c.CHTRoot[:])
	w.Write(c.BloomRoot[:])

	var h types.Hash
	w.Sum(h[:0])
	return h
}

// Empty returns an indicator whether the checkpoint is regarded as empty.
func (c *TrustedCheckpoint) Empty() bool {
	return c.SectionHead == (types.Hash{}) || c.CHTRoot == (types.Hash{}) || c.BloomRoot == (types.Hash{})
}

// CheckpointOracleConfig represents a set of checkpoint contract(which acts as an oracle)
// blockchain which used for light client checkpoint syncing.
type CheckpointOracleConfig struct {
	Address   types.Address   `json:"address"`
	Signers   []types.Address `json:"signers"`
	Threshold uint64          `json:"threshold"`
}
