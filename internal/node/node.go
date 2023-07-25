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

package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/amazechain/amc/contracts/deposit"
	"github.com/amazechain/amc/internal/debug"
	"github.com/amazechain/amc/internal/p2p"
	amcsync "github.com/amazechain/amc/internal/sync"
	initialsync "github.com/amazechain/amc/internal/sync/initial-sync"
	"github.com/amazechain/amc/internal/tracers"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common/cmp"
	"google.golang.org/protobuf/proto"
	"runtime"
	"strings"

	consensus_pb "github.com/amazechain/amc/api/protocol/consensus_proto"
	"github.com/amazechain/amc/internal"
	"github.com/amazechain/amc/internal/api"

	"github.com/amazechain/amc/modules"
	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	log2 "github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"

	"github.com/amazechain/amc/internal/metrics/influxdb"
	"github.com/amazechain/amc/log"
	"github.com/rcrowley/go-metrics"

	"os"
	"path/filepath"
	"strconv"

	"sync"
	"time"

	"github.com/amazechain/amc/accounts"
	"github.com/amazechain/amc/accounts/keystore"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"

	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/consensus/apoa"
	"github.com/amazechain/amc/internal/consensus/apos"
	"github.com/amazechain/amc/internal/download"
	"github.com/amazechain/amc/internal/miner"
	"github.com/amazechain/amc/internal/network"
	"github.com/amazechain/amc/internal/pubsub"
	"github.com/amazechain/amc/internal/txspool"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/params"
	"github.com/amazechain/amc/utils"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type Node struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *conf.Config

	//engine       consensus.IEngine
	miner           *miner.Miner
	pubsubServer    common.IPubSub
	genesisBlock    block.IBlock
	service         common.INetwork
	peers           map[peer.ID]common.Peer
	blocks          common.IBlockChain
	engine          consensus.Engine
	db              kv.RwDB
	txspool         txs_pool.ITxsPool
	txsFetcher      *txspool.TxsFetcher
	nodeKey         crypto.PrivKey
	depositContract *deposit.Deposit
	//nodeKey      *ecdsa.PrivateKey

	//downloader
	downloader common.IDownloader

	shutDown chan struct{}

	peerLock sync.RWMutex
	//feed     *event.Event

	api     *api.API
	rpcAPIs []jsonrpc.API

	http          *httpServer
	ipc           *ipcServer
	ws            *httpServer
	inprocHandler *jsonrpc.Server

	//n.config.GenesisBlockCfg.Config.Engine.Etherbase
	etherbase types.Address
	lock      sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)

	accman     *accounts.Manager
	keyDir     string // key store directory
	keyDirTemp bool   // If true, key directory will be removed by Stop

	p2p  p2p.P2P
	sync *amcsync.Service
	is   *initialsync.Service
}

func NewNode(ctx context.Context, cfg *conf.Config) (*Node, error) {
	//1. init db
	var name = kv.ChainDB.String()
	c, cancel := context.WithCancel(ctx)

	var (
		genesisBlock block.IBlock
		privateKey   crypto.PrivKey
		err          error
		pubsubServer common.IPubSub
		node         Node
		downloader   common.IDownloader
		peers        = map[peer.ID]common.Peer{}
		engine       consensus.Engine
		//err        error
		//chainConfig *params.ChainConfig
	)

	//chainConfig = params.AmazeChainConfig

	if len(cfg.NodeCfg.NodePrivate) <= 0 {
		privateKey, _, err = crypto.GenerateECDSAKeyPair(rand.Reader)
		if err != nil {
			return nil, err
		}
	} else {
		privateKey, err = utils.StringToPrivate(cfg.NodeCfg.NodePrivate)
		if err != nil {
			return nil, err
		}
	}
	log.Info("new node", "address", types.PrivateToAddress(privateKey))

	//
	chainKv, err := OpenDatabase(cfg, nil, name)
	if nil != err {
		return nil, err
	}

	if err := chainKv.Update(ctx, func(tx kv.RwTx) error {
		var genesisErr error
		genesisBlock, genesisErr = WriteGenesisBlock(tx, cfg.GenesisBlockCfg)
		if nil != genesisErr {
			return genesisErr
		}
		if cfg.GenesisBlockCfg.Miners != nil {
			miners := consensus_pb.PBSigners{}
			for _, miner := range cfg.GenesisBlockCfg.Miners {
				addr, err := types.HexToString(miner)
				if err != nil {
					return err
				}
				miners.Signer = append(miners.Signer, &consensus_pb.PBSigner{
					Public:  miner,
					Address: utils.ConvertAddressToH160(addr),
				})
			}
			data, err := proto.Marshal(&miners)
			if err != nil {
				return err
			}

			if err := rawdb.StoreSigners(tx, data); err != nil {
				return err
			}
		}
		return nil

	}); err != nil {
		panic(err)
	}

	s, err := network.NewService(ctx, &cfg.NetworkCfg, peers, node.ProtocolHandshake, node.ProtocolHandshakeInfo)
	if err != nil {
		panic("new service failed")
	}

	//todo deal chainid？
	pubsubServer, err = pubsub.NewPubSub(ctx, s, 1)
	if err != nil {
		return nil, err
	}

	p2p, err := p2p.NewService(c, genesisBlock.Hash(), cfg.P2PCfg)
	if err != nil {
		return nil, err
	}

	switch cfg.GenesisBlockCfg.Config.Engine.EngineName {
	case "APoaEngine":
		engine = apoa.New(cfg.GenesisBlockCfg.Config.Engine, chainKv)
	case "APosEngine":
		engine = apos.New(cfg.GenesisBlockCfg.Config.Engine, chainKv, cfg.GenesisBlockCfg.Config)
	default:
		return nil, fmt.Errorf("invalid engine name %s", cfg.GenesisBlockCfg.Config.Engine.EngineName)
	}

	bc, _ := internal.NewBlockChain(ctx, genesisBlock, engine, downloader, chainKv, p2p, cfg.GenesisBlockCfg.Config, cfg.GenesisBlockCfg.Config.Engine)
	pool, _ := txspool.NewTxsPool(ctx, bc)

	is := initialsync.NewService(c, &initialsync.Config{
		Chain: bc,
		P2P:   p2p,
	})

	syncServer := amcsync.NewService(
		ctx,
		amcsync.WithP2P(p2p),
		amcsync.WithChainService(bc),
		amcsync.WithInitialSync(is),
	)

	//todo
	var txs []*transaction.Transaction
	pending := pool.Pending(false)
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	var bloom *types.Bloom
	if len(txs) > 0 {
		bloom, _ = types.NewBloom(uint64(len(txs)))
		for _, tx := range txs {
			hash := tx.Hash()
			bloom.Add(hash.Bytes())
		}
	}

	txsFetcher := txspool.NewTxsFetcher(ctx, pool.GetTx, pool.AddRemotes, pool.Pending, s, peers, bloom)

	//bc.SetEngine(engine)

	downloader = download.NewDownloader(ctx, bc, s, pubsubServer, peers)

	_ = s.SetHandler(message.MsgDownloader, downloader.ConnHandler)
	_ = s.SetHandler(message.MsgTransaction, txsFetcher.ConnHandler)

	miner := miner.NewMiner(ctx, cfg, bc, engine, pool, nil)

	keyDir, isEphem, err := getKeyStoreDir(&cfg.NodeCfg)
	if err != nil {
		return nil, err
	}
	// Creates an empty AccountManager with no backends. Callers (e.g. cmd/amc)
	// are required to add the backends later on.
	accman := accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: cfg.NodeCfg.InsecureUnlockAllowed})

	node = Node{
		ctx:             c,
		cancel:          cancel,
		config:          cfg,
		miner:           miner,
		genesisBlock:    genesisBlock,
		service:         s,
		nodeKey:         privateKey,
		blocks:          bc,
		db:              chainKv,
		shutDown:        make(chan struct{}),
		pubsubServer:    pubsubServer,
		peers:           peers,
		downloader:      downloader,
		txspool:         pool,
		txsFetcher:      txsFetcher,
		engine:          engine,
		depositContract: deposit.NewDeposit(ctx, cfg.GenesisBlockCfg.Config.Engine, bc, chainKv),

		inprocHandler: jsonrpc.NewServer(),
		http:          newHTTPServer(),
		ws:            newHTTPServer(),
		ipc:           newIPCServer(&cfg.NodeCfg),
		etherbase:     types.HexToAddress(cfg.GenesisBlockCfg.Config.Engine.Etherbase),

		accman:     accman,
		keyDir:     keyDir,
		keyDirTemp: isEphem,

		p2p:  p2p,
		sync: syncServer,
		is:   is,
	}

	// Apply flags.
	//SetNodeConfig(ctx, &cfg)
	// Node doesn't by default populate account manager backends
	if err = setAccountManagerBackends(&node, &cfg.NodeCfg); err != nil {
		log.Errorf("Failed to set account manager backends: %v", err)
	}

	gpoParams := cfg.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = cfg.Miner.GasPrice
	}

	//
	log.Info("")
	log.Info(strings.Repeat("-", 153))
	for _, line := range strings.Split(cfg.GenesisBlockCfg.Config.Description(), "\n") {
		log.Info(line)
	}
	log.Info(strings.Repeat("-", 153))
	log.Info("")

	node.api = api.NewAPI(pubsubServer, s, peers, bc, chainKv, engine, pool, downloader, node.AccountManager(), cfg.GenesisBlockCfg.Config)
	node.api.SetGpo(api.NewOracle(bc, miner, cfg.GenesisBlockCfg.Config, gpoParams))
	return &node, nil
}

func (n *Node) Start() error {
	//if err := n.service.Start(); err != nil {
	//	log.Errorf("failed setup p2p service, err: %v", err)
	//	return err
	//}
	//
	//if err := n.pubsubServer.Start(); err != nil {
	//	log.Errorf("failed setup amc pubsub service, err: %v", err)
	//	return err
	//}

	if err := n.blocks.Start(); err != nil {
		log.Errorf("failed setup blocks service, err: %v", err)
		return err
	}

	if n.config.NodeCfg.Miner {

		// Configure the local mining address
		eb, err := n.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}

		if poa, ok := n.engine.(*apoa.Apoa); ok {
			wallet, err := n.accman.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			poa.Authorize(eb, wallet.SignData)
		} else if pos, ok := n.engine.(*apos.APos); ok {
			wallet, err := n.accman.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			pos.Authorize(eb, wallet.SignData)
		}

		n.miner.SetCoinbase(eb)
		n.miner.Start()
	}

	if pos, ok := n.engine.(*apos.APos); ok {
		pos.SetBlockChain(n.blocks)
	}

	if err := n.downloader.Start(); err != nil {
		log.Errorf("failed setup downloader service, err: %v", err)
		return err
	}

	if n.config.NodeCfg.HTTP {

		n.rpcAPIs = append(n.rpcAPIs, n.engine.APIs(n.blocks)...)
		n.rpcAPIs = append(n.rpcAPIs, n.api.Apis()...)
		n.rpcAPIs = append(n.rpcAPIs, tracers.APIs(n.api)...)
		n.rpcAPIs = append(n.rpcAPIs, debug.APIs()...)
		if err := n.startRPC(); err != nil {
			log.Error("failed start jsonrpc service", zap.Error(err))
			return err
		}
	}

	//n.p2p.AddConnectionHandler()
	n.p2p.Start()
	n.sync.Start()

	n.SetupMetrics(n.config.MetricsCfg)

	if err := n.txsFetcher.Start(); err != nil {
		log.Error("failed start txsFetcher service", zap.Error(err))
		return err
	}

	//go n.txsBroadcastLoop()
	//go n.txsMessageFetcherLoop()

	n.depositContract.Start()

	n.is.Start()

	//rwTx, _ := n.db.BeginRw(n.ctx)
	//defer rwTx.Rollback()
	//rawdb.PutDeposit(rwTx, types.HexToAddress("0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9").Bytes(), []byte("1111"))
	//data, _ := rawdb.GetDeposit(rwTx, types.HexToAddress("0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9").Bytes())
	//log.Info(string(data))

	//_ = n.db.View(context.Background(), func(tx kv.Tx) error {
	//	info := deposit.GetDepositInfo(tx, types.HexToAddress("0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9"))
	//	if info != nil {
	//		log.Info("load deposit info", "pubkey", info.PublicKey.Marshal(), "amount", info.DepositAmount.String())
	//	}
	//	return nil
	//
	//})

	log.Debug("node setup success!")

	return nil
}

func (n *Node) ProtocolHandshake(peer common.IPeer, genesisHash types.Hash, currentHeight *uint256.Int) (common.Peer, bool) {
	if n.blocks.GenesisBlock().Hash().String() != genesisHash.String() {
		return common.Peer{}, false
	}

	if _, ok := n.peers[peer.ID()]; !ok {
		return common.Peer{
			IPeer:         peer,
			CurrentHeight: currentHeight,
			AddTimer:      time.Now(),
		}, true
	}

	return common.Peer{}, false
}

func (n *Node) ProtocolHandshakeInfo() (types.Hash, *uint256.Int, error) {
	current := n.blocks.CurrentBlock()
	log.Infof("local peer info: height %d, genesis hash %v", current.Number64().Uint64(), n.blocks.GenesisBlock().Hash())
	return n.blocks.GenesisBlock().Hash(), current.Number64(), nil
}

func (n *Node) Network() common.INetwork {
	if n.service != nil {
		return n.service
	}

	return nil
}

// txBroadcastLoop announces new transactions to all.
func (n *Node) txsBroadcastLoop() {
	// local txs
	txsCh := make(chan common.NewLocalTxsEvent)
	txsSub := event.GlobalEvent.Subscribe(txsCh)

	for {
		select {
		case event := <-txsCh:
			for _, tx := range event.Txs {
				//log.Infof("start Broadcast local txs")
				n.pubsubServer.Publish(message.GossipTransactionMessage, tx.ToProtoMessage())
			}
		case err := <-txsSub.Err():
			log.Error("NewLocalTxsEvent chan has a error:%v", err)
			return
		case <-n.shutDown:
			return
		}
	}
}

// txBroadcastLoop announces new transactions to all.
func (n *Node) txsMessageFetcherLoop() {

	topic, err := n.pubsubServer.JoinTopic(message.GossipTransactionMessage)

	if err != nil {
		log.Error("cannot join in ")
	}
	sub, _ := topic.Subscribe()

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			msg, _ := sub.Next(n.ctx)
			var protoMsg types_pb.Transaction
			if err := proto.Unmarshal(msg.Data, &protoMsg); err == nil {
				tx, err := transaction.FromProtoMessage(&protoMsg)
				if err == nil {
					errs := n.txspool.AddRemotes([]*transaction.Transaction{tx})
					if errs[0] != nil {
						//log.Errorf("add Remotes err: %v", errs[0])
					}
				} else {
					log.Errorf("cannot transfer proto msg to transaction.Transaction err: %v", err)
				}
			} else {
				log.Errorf("cannot Unmarshal new_transaction msg err: %v", err)
			}
		}
	}
}

func (n *Node) startInProc() error {
	for _, api := range n.rpcAPIs {
		if err := n.inprocHandler.RegisterName(api.Namespace, api.Service); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) stopInProc() {
	n.inprocHandler.Stop()
}

func (n *Node) startRPC() error {
	if err := n.startInProc(); err != nil {
		return err
	}
	if n.ipc.endpoint != "" {
		if err := n.ipc.start(n.rpcAPIs); err != nil {
			return err
		}
	}
	if n.config.NodeCfg.HTTPHost != "" {

		//todo []string{"eth", "web3", "debug", "net", "apoa", "txpool", "apos"}
		config := httpConfig{
			CorsAllowedOrigins: []string{},
			Vhosts:             []string{"*"},
			Modules:            utils.SplitAndTrim(n.config.NodeCfg.HTTPApi),
			prefix:             "",
		}
		port, _ := strconv.Atoi(n.config.NodeCfg.HTTPPort)
		if err := n.http.setListenAddr(n.config.NodeCfg.HTTPHost, port); err != nil {
			return err
		}
		if err := n.http.enableRPC(n.rpcAPIs, config); err != nil {
			return err
		}
		if err := n.http.start(); err != nil {
			return err
		}
	}

	// Configure WebSocket.
	if n.config.NodeCfg.WS {
		port, _ := strconv.Atoi(n.config.NodeCfg.WSPort)
		if err := n.ws.setListenAddr(n.config.NodeCfg.WSHost, port); err != nil {
			return err
		}
		//todo
		config := wsConfig{
			Modules:   utils.SplitAndTrim(n.config.NodeCfg.WSApi),
			Origins:   []string{"*"},
			prefix:    "",
			jwtSecret: []byte{},
		}
		if err := n.ws.enableWS(n.rpcAPIs, config); err != nil {
			return err
		}
		if err := n.ws.start(); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) stopRPC() {
	n.http.stop()
	n.ws.stop()
	n.ipc.stop()
	n.stopInProc()
}

func (n *Node) newBlockSubLoop() {
	defer n.cancel()

	topic, err := n.pubsubServer.JoinTopic(message.GossipBlockMessage)
	if err != nil {
		return
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return
	}

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			msg, err := sub.Next(n.ctx)
			if err != nil {
				return
			}

			var blockMsg types_pb.Block
			if err := proto.Unmarshal(msg.Data, &blockMsg); err == nil {
				var block block.Block
				if err := block.FromProtoMessage(&blockMsg); err == nil {
					log.Infof("receive pubsub new block msg number:%v", block.Number64().String())
					currentBlock := n.blocks.CurrentBlock()
					if block.Number64().Cmp(uint256.NewInt(0).Add(currentBlock.Number64(), uint256.NewInt(1))) == 0 || block.Number64().Cmp(currentBlock.Number64()) == 0 {

					} else {

					}
					//if block.Number64().Compare(n.blocks.CurrentBlock().Number64().Add(uint256.NewInt(1))) == 1 && block.Number64().Compare(d.highestNumber) == 1 {
					//	d.highestNumber = block.Number64()
					//}
				}
			}

		}
	}
}

func (n *Node) Close() {
	select {
	case <-n.ctx.Done():
		return
	default:
		n.cancel()
		close(n.shutDown)
		n.db.Close()
	}
}

// AccountManager retrieves the account manager used by the protocol stack.
func (n *Node) AccountManager() *accounts.Manager {
	return n.accman
}

// AccountManager retrieves the account manager used by the protocol stack.
func (n *Node) BlockChain() common.IBlockChain {
	return n.blocks
}

func (n *Node) Database() kv.RwDB {
	return n.db
}

// getKeyStoreDir retrieves the key directory and will create
// and ephemeral one if necessary.
func getKeyStoreDir(conf *conf.NodeConfig) (string, bool, error) {
	keydir, err := conf.KeyDirConfig()
	if err != nil {
		return "", false, err
	}
	isEphemeral := false
	if keydir == "" {
		// There is no datadir.
		keydir, err = os.MkdirTemp("", "go-ethereum-keystore")
		isEphemeral = true
	}

	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(keydir, 0700); err != nil {
		return "", false, err
	}

	return keydir, isEphemeral, nil
}

func setAccountManagerBackends(stack *Node, conf *conf.NodeConfig) error {
	am := stack.AccountManager()
	keydir := stack.KeyStoreDir()
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	if conf.UseLightweightKDF {
		scryptN = keystore.LightScryptN
		scryptP = keystore.LightScryptP
	}

	// For now, we're using EITHER external signer OR local signers.
	// If/when we implement some form of lockfile for USB and keystore wallets,
	// we can have both, but it's very confusing for the user to see the same
	// accounts in both externally and locally, plus very racey.
	am.AddBackend(keystore.NewKeyStore(keydir, scryptN, scryptP))

	return nil
}

// KeyStoreDir retrieves the key directory
func (n *Node) KeyStoreDir() string {
	return n.keyDir
}

func (n *Node) SetupMetrics(config conf.MetricsConfig) {

	if config.EnableInfluxDB {
		var (
			endpoint     = config.InfluxDBEndpoint
			bucket       = config.InfluxDBBucket
			token        = config.InfluxDBToken
			organization = config.InfluxDBOrganization
			tagsMap      = SplitTagsFlag(config.InfluxDBTags)
		)

		go influxdb.InfluxDBV2WithTags(metrics.DefaultRegistry, 10*time.Second, endpoint, token, bucket, organization, "amc.", tagsMap)
	}
}

func (s *Node) Etherbase() (eb types.Address, err error) {
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()

	if etherbase != (types.Address{}) {
		return etherbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase := accounts[0].Address

			s.lock.Lock()
			s.etherbase = etherbase
			s.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return types.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

func OpenDatabase(cfg *conf.Config, logger log2.Logger, name string) (kv.RwDB, error) {
	var chainKv kv.RwDB
	if cfg.NodeCfg.DataDir == "" {
		chainKv = memdb.New("")
	}
	var err error

	dbPath := filepath.Join(cfg.NodeCfg.DataDir, name)

	var openFunc func(exclusive bool) (kv.RwDB, error)
	log.Info("Opening Database", "label", name, "path", dbPath)
	openFunc = func(exclusive bool) (kv.RwDB, error) {
		//if config.Http.DBReadConcurrency > 0 {
		//	roTxLimit = int64(config.Http.DBReadConcurrency)
		//}
		roTxsLimiter := semaphore.NewWeighted(int64(cmp.Max(32, runtime.GOMAXPROCS(-1)*8))) // 1 less than max to allow unlocking to happen
		opts := mdbx.NewMDBX(logger).
			WriteMergeThreshold(4 * 8192).
			Path(dbPath).Label(kv.ChainDB).
			DBVerbosity(kv.DBVerbosityLvl(2)).RoTxsLimiter(roTxsLimiter)
		if exclusive {
			opts = opts.Exclusive()
		}

		modules.AmcInit()
		kv.ChaindataTablesCfg = modules.AmcTableCfg

		opts = opts.MapSize(8 * datasize.TB)
		return opts.Open()
	}
	chainKv, err = openFunc(false)
	if err != nil {
		return nil, err
	}

	if err = chainKv.Update(context.Background(), func(tx kv.RwTx) (err error) {
		return params.SetAmcVersion(tx, params.VersionKeyCreated)
	}); err != nil {
		return nil, err
	}
	return chainKv, nil
}

func WriteGenesisBlock(db kv.RwTx, genesis *conf.GenesisBlockConfig) (*block.Block, error) {
	if genesis == nil {
		return nil, internal.ErrGenesisNoConfig
	}
	storedHash, storedErr := rawdb.ReadCanonicalHash(db, 0)
	if storedErr != nil {
		return nil, storedErr
	}

	g := &internal.GenesisBlock{
		"",
		genesis,
		//config,
	}
	if storedHash == (types.Hash{}) {
		log.Info("Writing default main-net genesis block")
		block, _, err := g.Write(db)
		if nil != err {
			return nil, err
		}
		return block, nil
	} else {
		if _, err := rawdb.ReadChainConfig(db, storedHash); err != nil {
			//todo just for now
			//return nil, err
			log.Error("cannot get chain config from db", "err", err)
			rawdb.WriteChainConfig(db, storedHash, genesis.Config)
		} else {

			//todo
			//genesis.Config = config
		}

	}
	// Check whether the genesis block is already written.
	if genesis != nil {
		block, _, err1 := g.ToBlock()
		if err1 != nil {
			return nil, err1
		}
		hash := block.Hash()
		if hash != storedHash {
			return block, fmt.Errorf("database contains incompatible genesis (have %x, new %x)\n", storedHash, hash)
		}
	}
	storedBlock, err := rawdb.ReadBlockByHash(db, storedHash)
	if err != nil {
		return nil, err
	}
	return storedBlock, nil
}

func SplitTagsFlag(tagsFlag string) map[string]string {
	tags := strings.Split(tagsFlag, ",")
	tagsMap := map[string]string{}

	for _, t := range tags {
		if t != "" {
			kv := strings.Split(t, "=")

			if len(kv) == 2 {
				tagsMap[kv[0]] = kv[1]
			}
		}
	}

	return tagsMap
}

func (n *Node) Miner() common.IMiner {
	return n.miner
}

func (n *Node) Engine() consensus.Engine {
	return n.engine
}

func (n *Node) ChainDb() kv.RwDB {
	return n.db
}
