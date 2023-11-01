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
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/contracts/deposit"
	"github.com/amazechain/amc/contracts/deposit/AMT"
	nftdeposit "github.com/amazechain/amc/contracts/deposit/NFT"
	"github.com/amazechain/amc/internal/debug"
	"github.com/amazechain/amc/internal/metrics/prometheus"
	"github.com/amazechain/amc/internal/p2p"
	amcsync "github.com/amazechain/amc/internal/sync"
	initialsync "github.com/amazechain/amc/internal/sync/initial-sync"
	"github.com/amazechain/amc/internal/tracers"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common/cmp"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"hash/crc32"
	"net"
	"path"
	"runtime"
	"strings"

	"github.com/amazechain/amc/internal"
	"github.com/amazechain/amc/internal/api"

	"github.com/amazechain/amc/modules"
	"github.com/c2h5oh/datasize"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	log2 "github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"

	"github.com/amazechain/amc/log"
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

const datadirJWTKey = "jwtsecret" // Path within the datadir to the node's jwt secret

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
	httpAuth      *httpServer //
	wsAuth        *httpServer //
	inprocHandler *jsonrpc.Server

	//n.config.GenesisCfg.Engine.Etherbase
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

	c, cancel := context.WithCancel(ctx)

	var (
		genesisBlock  block.IBlock
		privateKey    crypto.PrivKey
		pubsubServer  common.IPubSub
		node          Node
		downloader    common.IDownloader
		peers         = map[peer.ID]common.Peer{}
		engine        consensus.Engine
		genesisHash   types.Hash
		genesisConfig *conf.Genesis
		chainConfig   *params.ChainConfig
		chainKv       kv.RwDB
		err           error
	)

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

	//
	chainKv, err = OpenDatabase(cfg, nil, kv.ChainDB.String())
	if nil != err {
		return nil, err
	}

	if err := chainKv.View(ctx, func(tx kv.Tx) error {
		//
		genesisHash, err = rawdb.ReadCanonicalHash(tx, 0)
		//
		if genesisHash == (types.Hash{}) && err != nil {
			//return fmt.Errorf("GenesisHash is missing err:%w", err)
			return internal.ErrGenesisNoConfig
		}
		if genesisHash == (types.Hash{}) && err == nil {
			//needs WriteGenesisBlock
			return nil
		}
		//
		chainConfig, err = rawdb.ReadChainConfig(tx, genesisHash)
		if err != nil {
			return err
		}
		//
		if genesisBlock, err = rawdb.ReadBlockByHash(tx, genesisHash); genesisBlock == nil {
			return fmt.Errorf("genesisBlock is missing err:%w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if genesisHash == (types.Hash{}) {
		genesisHash = *params.GenesisHashByChainName(cfg.NodeCfg.Chain)
		genesisConfig = internal.GenesisByChainName(cfg.NodeCfg.Chain)
		chainConfig = params.ChainConfigByChainName(cfg.NodeCfg.Chain)
		if err := chainKv.Update(ctx, func(tx kv.RwTx) error {
			var genesisErr error
			genesisBlock, genesisErr = WriteGenesisBlock(tx, genesisConfig)
			if nil != genesisErr {
				return genesisErr
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	cfg.ChainCfg = chainConfig

	s, err := network.NewService(ctx, &cfg.NetworkCfg, peers, node.ProtocolHandshake, node.ProtocolHandshakeInfo)
	if err != nil {
		panic("new service failed")
	}

	//todo deal chainid？
	pubsubServer, err = pubsub.NewPubSub(ctx, s, 1)
	if err != nil {
		return nil, err
	}

	p2p, err := p2p.NewService(c, genesisBlock.Hash(), cfg.P2PCfg, cfg.NodeCfg)
	if err != nil {
		return nil, err
	}

	switch cfg.ChainCfg.Consensus {
	case params.CliqueConsensus:
		engine = apoa.New(cfg.ChainCfg.Clique, chainKv)
	case params.AposConsensu:
		engine = apos.New(cfg.ChainCfg.Apos, chainKv, cfg.ChainCfg)
	default:
		return nil, fmt.Errorf("invalid engine name %s", cfg.ChainCfg.Consensus)
	}

	bc, _ := internal.NewBlockChain(ctx, genesisBlock, engine, downloader, chainKv, p2p, cfg.ChainCfg)
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

	_ = s.SetHandler(message.MsgTransaction, txsFetcher.ConnHandler)

	miner := miner.NewMiner(ctx, cfg, bc, engine, pool, nil)

	keyDir, isEphem, err := getKeyStoreDir(&cfg.NodeCfg)
	if err != nil {
		return nil, err
	}
	// Creates an empty AccountManager with no backends. Callers (e.g. cmd/amc)
	// are required to add the backends later on.
	accman := accounts.NewManager(&accounts.Config{InsecureUnlockAllowed: cfg.NodeCfg.InsecureUnlockAllowed})

	log.Info("new node", "GenesisHash", genesisBlock.Hash(), "CurrentBlockNr", bc.CurrentBlock().Number64().Uint64())

	node = Node{
		ctx:          c,
		cancel:       cancel,
		config:       cfg,
		miner:        miner,
		genesisBlock: genesisBlock,
		service:      s,
		nodeKey:      privateKey,
		blocks:       bc,
		db:           chainKv,
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
		wsAuth:        newHTTPServer(),
		httpAuth:      newHTTPServer(),
		ipc:           newIPCServer(&cfg.NodeCfg),
		etherbase:     types.HexToAddress(cfg.Miner.Etherbase),

		accman:     accman,
		keyDir:     keyDir,
		keyDirTemp: isEphem,

		p2p:  p2p,
		sync: syncServer,
		is:   is,
	}

	if cfg.ChainCfg.Apos != nil {
		depositContracts := make(map[types.Address]deposit.DepositContract, 0)
		if depositContractAddress := cfg.ChainCfg.Apos.DepositContract; depositContractAddress != "" {
			var addr types.Address
			if !addr.DecodeString(depositContractAddress) {
				panic(fmt.Sprintf("cannot decode DepositContract address: %s", depositContractAddress))
			}
			depositContracts[addr] = new(amtdeposit.Contract)
		}
		if depositNFTContractAddress := cfg.ChainCfg.Apos.DepositNFTContract; depositNFTContractAddress != "" {
			var addr types.Address
			if !addr.DecodeString(depositNFTContractAddress) {
				panic(fmt.Sprintf("cannot decode DepositNFTContract address: %s", depositNFTContractAddress))
			}
			depositContracts[addr] = new(nftdeposit.Contract)
		}
		node.depositContract = deposit.NewDeposit(ctx, bc, chainKv, depositContracts)
	}

	pool.SetDeposit(node.depositContract)

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
	for _, line := range strings.Split(cfg.ChainCfg.Description(), "\n") {
		log.Info(line)
	}
	log.Info(strings.Repeat("-", 153))
	log.Info("")

	node.api = api.NewAPI(pubsubServer, s, peers, bc, chainKv, engine, pool, downloader, node.AccountManager(), cfg.ChainCfg)
	node.api.SetGpo(api.NewOracle(bc, miner, cfg.ChainCfg, gpoParams))
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

	n.rpcAPIs = append(n.rpcAPIs, n.engine.APIs(n.blocks)...)
	n.rpcAPIs = append(n.rpcAPIs, n.api.Apis()...)
	n.rpcAPIs = append(n.rpcAPIs, tracers.APIs(n.api)...)
	n.rpcAPIs = append(n.rpcAPIs, debug.APIs()...)

	if err := n.startRPC(); err != nil {
		log.Error("failed start jsonrpc service", zap.Error(err))
		return err
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

	if n.depositContract != nil {
		n.depositContract.Start()
	}

	go n.is.Start()

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
	defer txsSub.Unsubscribe()

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
		case <-n.ctx.Done():
			close(txsCh)
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
	defer sub.Cancel()

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

// getAPIs return two sets of APIs, both the ones that do not require
// authentication, and the complete set
func (n *Node) getAPIs() (unauthenticated, all []jsonrpc.API) {
	for _, api := range n.rpcAPIs {
		if !api.Authenticated {
			unauthenticated = append(unauthenticated, api)
		}
	}
	return unauthenticated, n.rpcAPIs
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

// obtainJWTSecret loads the jwt-secret, either from the provided config,
// or from the default location. If neither of those are present, it generates
// a new secret and stores to the default location.
func (n *Node) obtainJWTSecret(cliParam string) ([]byte, error) {
	fileName := cliParam
	if len(fileName) == 0 {
		// no path provided, use default
		fileName = path.Join(n.config.NodeCfg.DataDir, datadirJWTKey)
	}
	// try reading from file
	if data, err := os.ReadFile(fileName); err == nil {
		jwtSecret, err := hexutil.Decode(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to decode hex (%s) string", strings.TrimSpace(string(data))))
		}
		if len(jwtSecret) == 32 {
			log.Info("Loaded JWT secret file", "path", fileName, "crc32", fmt.Sprintf("%#x", crc32.ChecksumIEEE(jwtSecret)))
			return jwtSecret, nil
		}
		log.Error("Invalid JWT secret", "path", fileName, "length", len(jwtSecret))
		return nil, errors.New("invalid JWT secret")
	}
	// Need to generate one
	jwtSecret := make([]byte, 32)
	rand.Read(jwtSecret)

	if err := os.WriteFile(fileName, []byte(hexutil.Encode(jwtSecret)), 0600); err != nil {
		return nil, err
	}
	log.Info("Generated JWT secret", "path", fileName)
	return jwtSecret, nil
}

func (n *Node) startRPC() error {

	openAPIs, allAPIs := n.getAPIs()

	if err := n.startInProc(); err != nil {
		return err
	}

	if n.ipc.endpoint != "" {
		//if err := n.ipc.start(n.rpcAPIs); err != nil {
		//	return err
		//}
	}
	if n.config.NodeCfg.HTTP {
		//todo []string{"eth", "web3", "debug", "net", "apoa", "txpool", "apos"}
		config := httpConfig{
			CorsAllowedOrigins: utils.SplitAndTrim(n.config.NodeCfg.HTTPCors),
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
			Origins:   utils.SplitAndTrim(n.config.NodeCfg.WSOrigins),
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

	// Configure authenticated API
	if len(openAPIs) != len(allAPIs) && n.config.NodeCfg.AuthRPC {
		jwtSecret, err := n.obtainJWTSecret(n.config.NodeCfg.JWTSecret)
		if err != nil {
			return err
		}
		config := httpConfig{
			CorsAllowedOrigins: utils.SplitAndTrim(n.config.NodeCfg.HTTPCors),
			Vhosts:             []string{"*"},
			Modules:            []string{"admin", "apos"},
			prefix:             "",
			jwtSecret:          jwtSecret,
		}

		if err := n.httpAuth.setListenAddr(n.config.NodeCfg.AuthAddr, n.config.NodeCfg.AuthPort); err != nil {
			return err
		}
		if err := n.httpAuth.enableRPC(n.rpcAPIs, config); err != nil {
			return err
		}
		if err := n.httpAuth.start(); err != nil {
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
	defer sub.Cancel()

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
	//select {
	//case <-n.ctx.Done():
	//	return
	//default:
	//	n.cancel()
	//	close(n.shutDown)
	//	n.db.Close()
	//}
	n.cancel()
	n.miner.Close()
	n.stopRPC()
	n.db.Close()
	close(n.shutDown)
}

func (n *Node) Wait() {
	<-n.shutDown
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
	if config.Enable {
		if config.HTTP != "" {
			address := net.JoinHostPort(config.HTTP, fmt.Sprintf("%d", config.Port))
			log.Info("Enabling stand-alone metrics HTTP endpoint", "address", address)
			prometheus.Setup(address, log.Root())
		} else if config.Port != 0 {
			log.Warn(fmt.Sprintf("--%s specified without --%s, metrics server will not start.", "metrics.port", "metrics.addr"))
		}
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

func WriteGenesisBlock(db kv.RwTx, genesis *conf.Genesis) (*block.Block, error) {
	if genesis == nil {
		return nil, internal.ErrGenesisNoConfig
	}

	g := &internal.GenesisBlock{
		"",
		genesis,
		//config,
	}
	log.Info("Writing genesis block")
	block, _, err := g.Write(db)
	if nil != err {
		return nil, err
	}
	if err := rawdb.WriteChainConfig(db, block.Hash(), genesis.Config); err != nil {
		log.Error("cannot get chain config from db", "err", err)
		return nil, err
	}
	return block, nil

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
