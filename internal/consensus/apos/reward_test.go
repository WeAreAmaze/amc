package apos

import (
	"github.com/holiman/uint256"
	"math/big"
	"testing"

	"github.com/amazechain/amc/params"
)

func Test_Int256ModTests(t *testing.T) {
	a := uint256.NewInt(100)
	b := uint256.NewInt(20)
	if c := uint256.NewInt(0).Mod(a, b); c.Cmp(uint256.NewInt(0)) == 0 {
		t.Log("ok")
	} else {
		t.Log("err")
	}
}
func TestReward_GetRewards(t *testing.T) {
	// var chainKv kv.RwDB
	// _ = chainKv
	// var err error
	// logger := log2.New()

	// dbPath := "./mdbx.db"

	// var openFunc = func(exclusive bool) (kv.RwDB, error) {
	// 	//if config.Http.DBReadConcurrency > 0 {
	// 	//	roTxLimit = int64(config.Http.DBReadConcurrency)
	// 	//}
	// 	roTxsLimiter := semaphore.NewWeighted(int64(cmp.Max(32, runtime.GOMAXPROCS(-1)*8))) // 1 less than max to allow unlocking to happen
	// 	opts := mdbx.NewMDBX(logger).
	// 		WriteMergeThreshold(4 * 8192).
	// 		Path(dbPath).Label(kv.ChainDB).
	// 		DBVerbosity(kv.DBVerbosityLvl(2)).RoTxsLimiter(roTxsLimiter)
	// 	if exclusive {
	// 		opts = opts.Exclusive()
	// 	}

	// 	modules.AmcInit()
	// 	kv.ChaindataTablesCfg = modules.AmcTableCfg

	// 	opts = opts.MapSize(8 * datasize.TB)
	// 	return opts.Open()
	// }
	// chainKv, err = openFunc(false)
	// if err != nil {
	// 	t.Error(err)
	// }

	// n, err := node.NewNode(context.Background(), &DefaultConfig)
	// if err != nil {
	// 	t.Error(err)
	// }

	// _ = n
	// newReward(chainKv, nil, nil)
}

// var DefaultConfig = conf.Config{
// 	NodeCfg: conf.NodeConfig{
// 		NodePrivate: "",
// 		HTTP:        true,
// 		HTTPHost:    "127.0.0.1",
// 		HTTPPort:    "8545",
// 		IPCPath:     "amc.ipc",
// 		Miner:       false,
// 	},
// 	NetworkCfg: conf.NetWorkConfig{
// 		Bootstrapped: true,
// 	},
// 	LoggerCfg: conf.LoggerConfig{
// 		LogFile:    "./logger.log",
// 		Level:      "debug",
// 		MaxSize:    10,
// 		MaxBackups: 10,
// 		MaxAge:     30,
// 		Compress:   true,
// 	},
// 	PprofCfg: conf.PprofConfig{
// 		MaxCpu:     0,
// 		Port:       6060,
// 		TraceMutex: true,
// 		TraceBlock: true,
// 		Pprof:      false,
// 	},
// 	DatabaseCfg: conf.DatabaseConfig{
// 		DBType:     "lmdb",
// 		DBPath:     "chaindata",
// 		DBName:     "amc",
// 		SubDB:      []string{"chain"},
// 		Debug:      false,
// 		IsMem:      false,
// 		MaxDB:      100,
// 		MaxReaders: 1000,
// 	},
// 	MetricsCfg: conf.MetricsConfig{
// 		InfluxDBEndpoint:     "",
// 		InfluxDBToken:        "",
// 		InfluxDBBucket:       "",
// 		InfluxDBOrganization: "",
// 	},

// 	GenesisBlockCfg: conf.GenesisBlockConfig{
// 		ChainID: 0,
// 		Engine: conf.ConsensusConfig{
// 			EngineName: "APoaEngine", //APosEngine,APoaEngine
// 			BlsKey:     "bls",
// 			Period:     8,
// 			GasCeil:    30000000,
// 			APoa: conf.APoaConfig{
// 				Epoch:              30000,
// 				CheckpointInterval: 0,
// 				InmemorySnapshots:  0,
// 				InmemorySignatures: 0,
// 				InMemory:           false,
// 			},
// 			APos: conf.APosConfig{
// 				Epoch:              30000,
// 				CheckpointInterval: 0,
// 				InmemorySnapshots:  0,
// 				InmemorySignatures: 0,
// 				InMemory:           false,

// 				RewardEpoch: 2000,
// 				RewardLimit: 100,
// 			},
// 		},
// 		Miners: []string{"AMCA2142AB3F25EAA9985F22C3F5B1FF9FA378DAC21"},
// 		Number: 0,
// 		// Timestamp: toLocation(),
// 		// Alloc:     readPrealloc("allocs/amc.json"),
// 	},
// }

func TestReward_epoch2number(t *testing.T) {
	r := &Reward{
		chainConfig: &params.ChainConfig{
			BeijingBlock: &big.Int{},
		},
	}
	r.rewardEpoch = uint256.NewInt(10800)
	if got := r.epoch2number(uint256.NewInt(3)); got.Cmp(uint256.NewInt(32400)) != 0 {
		t.Error("got ", got)
	}
}
