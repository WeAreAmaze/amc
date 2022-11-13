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
	"github.com/amazechain/amc/internal/metrics/influxdb"
	"github.com/amazechain/amc/log"
	"github.com/rcrowley/go-metrics"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amazechain/amc/accounts"
	"github.com/amazechain/amc/accounts/keystore"
	consensus_pb "github.com/amazechain/amc/api/protocol/consensus_proto"
	"github.com/amazechain/amc/api/protocol/types_pb"
	"github.com/amazechain/amc/common"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/message"
	"github.com/amazechain/amc/common/transaction"
	"github.com/amazechain/amc/common/txs_pool"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/internal/amcdb"
	"github.com/amazechain/amc/internal/api"
	"github.com/amazechain/amc/internal/blockchain"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/internal/consensus/apoa"
	"github.com/amazechain/amc/internal/download"
	"github.com/amazechain/amc/internal/kv"
	"github.com/amazechain/amc/internal/kv/mdbx"
	"github.com/amazechain/amc/internal/miner"
	"github.com/amazechain/amc/internal/network"
	"github.com/amazechain/amc/internal/pubsub"
	"github.com/amazechain/amc/internal/txspool"
	event "github.com/amazechain/amc/modules/event/v2"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"github.com/amazechain/amc/params"
	"github.com/amazechain/amc/utils"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
)

type Node struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *conf.Config

	//engine       consensus.IEngine
	miner        *miner.Miner
	pubsubServer common.IPubSub
	genesisBlock block.IBlock
	service      common.INetwork
	peers        map[peer.ID]common.Peer
	blocks       common.IBlockChain
	engine       consensus.Engine
	db           db.IDatabase
	txspool      txs_pool.ITxsPool
	txsFetcher   *txspool.TxsFetcher
	nodeKey      crypto.PrivKey
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

	//n.config.GenesisBlockCfg.Engine.Etherbase
	etherbase types.Address
	lock      sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)

	accman     *accounts.Manager
	keyDir     string // key store directory
	keyDirTemp bool   // If true, key directory will be removed by Stop
}

func NewNode(ctx context.Context, cfg *conf.Config) (*Node, error) {
	//1. init db
	var (
		genesisBlock block.IBlock
		privateKey   crypto.PrivKey
		err          error
	)

	if len(cfg.NodeCfg.NodePrivate) <= 0 {
		privateKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
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

	chainDB, err := amcdb.OpenDB(ctx, &cfg.NodeCfg, &cfg.DatabaseCfg)
	if err != nil {
		log.Error("failed to open db", "db", cfg.DatabaseCfg, "err", err)

		return nil, err
	}

	// todo
	changeDB, err := mdbx.NewMDBX().Path(fmt.Sprintf("%s/changeSet", cfg.NodeCfg.DataDir)).Label(kv.ChainDB).DBVerbosity(kv.DBVerbosityLvl(2)).Open()
	if err != nil {
		log.Error("failed to open kv db %v, err: %v", cfg.DatabaseCfg, err)

		return nil, err
	}

	if err = changeDB.Update(context.Background(), func(tx kv.RwTx) (err error) {
		return params.SetAmcVersion(tx, params.VersionKeyCreated)
	}); err != nil {
		return nil, err
	}

	genesisBlock, err = rawdb.GetGenesis(chainDB)
	if err != nil {
		log.Info("genesis block")

		genesisBlock, err = blockchain.NewGenesisBlockFromConfig(&cfg.GenesisBlockCfg, cfg.GenesisBlockCfg.Engine.EngineName, chainDB, changeDB)
		if err != nil {
			return nil, err
		}

		if err = rawdb.StoreGenesis(chainDB, genesisBlock); err != nil {
			return nil, err
		}
		if cfg.GenesisBlockCfg.Miners != nil {

			miners := consensus_pb.PBSigners{}
			for _, miner := range cfg.GenesisBlockCfg.Miners {
				addr, err := types.HexToString(miner)
				if err != nil {
					return nil, err
				}
				miners.Signer = append(miners.Signer, &consensus_pb.PBSigner{
					Public:  miner,
					Address: addr,
				})
			}
			data, err := proto.Marshal(&miners)
			if err != nil {
				return nil, err
			}

			if err := rawdb.StoreSigners(chainDB, data); err != nil {
				return nil, err
			}
		}
	}

	var (
		pubsubServer common.IPubSub
		node         Node
		downloader   common.IDownloader
		peers        = map[peer.ID]common.Peer{}
		engine       consensus.Engine
		//err        error
	)

	s, err := network.NewService(ctx, &cfg.NetworkCfg, peers, node.ProtocolHandshake, node.ProtocolHandshakeInfo)
	if err != nil {
		panic("new service failed")
	}

	//todo deal chainid？
	pubsubServer, err = pubsub.NewPubSub(ctx, s, 1)
	if err != nil {
		return nil, err
	}

	bc, _ := blockchain.NewBlockChain(ctx, genesisBlock, engine, downloader, chainDB, changeDB, pubsubServer)
	pool, _ := txspool.NewTxsPool(ctx, bc)

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
			hash, _ := tx.Hash()
			bloom.Add(hash.Bytes())
		}
	}

	txsFetcher := txspool.NewTxsFetcher(ctx, pool.GetTx, pool.AddRemotes, pool.Pending, s, peers, bloom)

	switch cfg.GenesisBlockCfg.Engine.EngineName {
	case "APoaEngine":
		engine = apoa.New(&cfg.GenesisBlockCfg.Engine, chainDB)
	default:
		return nil, fmt.Errorf("invalid engine name %s", cfg.GenesisBlockCfg.Engine.EngineName)
	}

	bc.SetEngine(engine)

	c, cancel := context.WithCancel(ctx)

	downloader = download.NewDownloader(ctx, bc, pubsubServer, peers)

	_ = s.SetHandler(message.MsgDownloader, downloader.ConnHandler)
	_ = s.SetHandler(message.MsgTransaction, txsFetcher.ConnHandler)

	miner := miner.NewMiner(ctx, &cfg.GenesisBlockCfg.Engine, bc, engine, pool, nil)

	keyDir, isEphem, err := getKeyStoreDir(&cfg.NodeCfg)
	if err != nil {
		return nil, err
	}
	// Creates an empty AccountManager with no backends. Callers (e.g. cmd/amc)
	// are required to add the backends later on.
	accman := accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: cfg.NodeCfg.InsecureUnlockAllowed})

	node = Node{
		ctx:          c,
		cancel:       cancel,
		config:       cfg,
		miner:        miner,
		genesisBlock: genesisBlock,
		service:      s,
		nodeKey:      privateKey,
		blocks:       bc,
		db:           chainDB,
		shutDown:     make(chan struct{}),
		pubsubServer: pubsubServer,
		peers:        peers,
		downloader:   downloader,
		txspool:      pool,
		txsFetcher:   txsFetcher,
		engine:       engine,

		inprocHandler: jsonrpc.NewServer(),
		http:          newHTTPServer(),
		ws:            newHTTPServer(),
		ipc:           newIPCServer(&cfg.NodeCfg),
		etherbase:     types.HexToAddress(cfg.GenesisBlockCfg.Engine.Etherbase),

		accman:     accman,
		keyDir:     keyDir,
		keyDirTemp: isEphem,
	}

	// Apply flags.
	//SetNodeConfig(ctx, &cfg)
	// Node doesn't by default populate account manager backends
	if err = setAccountManagerBackends(&node, &cfg.NodeCfg); err != nil {
		log.Errorf("Failed to set account manager backends: %v", err)
	}

	node.api = api.NewAPI(pubsubServer, s, peers, bc, chainDB, engine, pool, downloader, node.AccountManager())

	return &node, nil
}

func (n *Node) Start() error {
	if err := n.service.Start(); err != nil {
		log.Errorf("failed setup p2p service, err: %v", err)
		return err
	}

	if err := n.pubsubServer.Start(); err != nil {
		log.Errorf("failed setup amc pubsub service, err: %v", err)
		return err
	}

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
		var poa *apoa.Apoa
		if c, ok := n.engine.(*apoa.Apoa); ok {
			poa = c
		}
		if poa != nil {
			wallet, err := n.accman.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			poa.Authorize(eb, wallet.SignData)
		}

		n.miner.SetCoinbase(eb)
		n.miner.Start()
	}

	if err := n.downloader.Start(); err != nil {
		log.Errorf("failed setup downloader service, err: %v", err)
		return err
	}

	if n.config.NodeCfg.HTTP {

		n.rpcAPIs = append(n.rpcAPIs, n.engine.APIs(n.blocks)...)
		n.rpcAPIs = append(n.rpcAPIs, n.api.Apis()...)
		if err := n.startRPC(); err != nil {
			log.Error("failed start jsonrpc service", zap.Error(err))
			return err
		}
	}

	n.SetupMetrics(n.config.MetricsCfg)

	if err := n.txsFetcher.Start(); err != nil {
		log.Error("failed start txsFetcher service", zap.Error(err))
		return err
	}

	go n.txsBroadcastLoop()
	go n.txsMessageFetcherLoop()

	log.Debug("node setup success!")

	return nil
}

func (n *Node) ProtocolHandshake(peer common.IPeer, genesisHash types.Hash, currentHeight types.Int256) (common.Peer, bool) {
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

func (n *Node) ProtocolHandshakeInfo() (types.Hash, types.Int256, error) {
	current := n.blocks.CurrentBlock()

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
		case <-n.shutDown:
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

		//todo
		config := httpConfig{
			CorsAllowedOrigins: []string{},
			Vhosts:             []string{"*"},
			Modules:            []string{"eth", "web3", "debug", "net", "apoa", "txpool"},
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
			Modules:   []string{"eth", "web3", "debug", "net", "apoa", "txpool"},
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

			var blockMsg types_pb.PBlock
			if err := proto.Unmarshal(msg.Data, &blockMsg); err == nil {
				var block block.Block
				if err := block.FromProtoMessage(&blockMsg); err == nil {
					log.Infof("receive pubsub new block msg number:%v", block.Number64().String())
					currentBlock := n.blocks.CurrentBlock()
					if block.Number64().Equal(currentBlock.Number64().Add(types.NewInt64(1))) || block.Number64().Equal(currentBlock.Number64()) {

					} else {

					}
					//if block.Number64().Compare(n.blocks.CurrentBlock().Number64().Add(types.NewInt64(1))) == 1 && block.Number64().Compare(d.highestNumber) == 1 {
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
		_ = n.db.Close()
	}
}

// AccountManager retrieves the account manager used by the protocol stack.
func (n *Node) AccountManager() *accounts.Manager {
	return n.accman
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
