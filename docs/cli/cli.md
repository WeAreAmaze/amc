# CLI Reference

The AMC node is operated via the CLI by running the `amc` command. To stop it, press `ctrl-c`. You may need to wait a bit as Reth tears down existing p2p connections or other cleanup tasks.

However, amc has more commands than that:

```bash
amc --help
```

See below for the full list of commands.

## Commands

```
$ amc --help
NAME:
   amc - AmazeChain system

USAGE:
   amc [global options] command [command options] [arguments...]

VERSION:
   0.01.1-36074172

COMMANDS:
   wallet   Manage AmazeChain presale wallets
   account  Manage accounts
   export   Export AmazeChain data
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --account.allow.insecure.unlock  Allow insecure account unlocking when account-related RPCs are exposed by http (default: false)
   --account.keystore value         Directory for the keystore (default = inside the datadir)
   --account.lightkdf               Reduce key-derivation RAM & CPU usage at some expense of KDF strength (default: false)
   --account.password value         Password file to use for non-interactive password input
   --account.unlock value           Comma separated list of accounts to unlock
   --authrpc                        Enable the AUTH-RPC server (default: false)
   --authrpc.addr value             Listening address for authenticated APIs
   --authrpc.jwtsecret value        Path to a JWT secret to use for authenticated RPC endpoints
   --authrpc.port value             Listening port for authenticated APIs (default: 0)
   --blockchain value               Loading a Configuration File
   --data.dir value                 data save dir (default: "./amc/")
   --engine.etherbase value         consensus etherbase
   --engine.miner                   miner (default: false)
   --engine.type value              consensus engine (default: "APosEngine")
   --help, -h                       show help (default: false)
   --http                           Enable the HTTP json-rpc server (default: false)
   --http.addr value                HTTP server listening interface (default: "127.0.0.1")
   --http.api value                 API's offered over the HTTP-RPC interface
   --http.corsdomain value          Comma separated list of domains from which to accept cross origin requests (browser enforced)
   --http.port value                HTTP server listening port (default: "20012")
   --ipcpath value                  Filename for IPC socket/pipe within the data dir (explicit paths escape it) (default: "amc.ipc")
   --log.compress                   logger file compress (default: false)
   --log.level value                logger output level (value:[debug,info,warn,error,dpanic,panic,fatal]) (default: "debug")
   --log.maxAge value               logger file max age (default: 30)
   --log.maxBackups value           logger file max backups (default: 10)
   --log.maxSize value              logger file max size M (default: 10)
   --log.name value                 logger file name and path (default: "amc.log")
   --metrics                        Enable metrics collection and reporting (default: false)
   --metrics.addr value             Enable stand-alone metrics HTTP server listening interface. (default: "127.0.0.1")
   --metrics.port value             Metrics HTTP server listening port.
Please note that --metrics.addr must be set to start the server. (default: 6060)
   --node.key value                                           node private
   --p2p.allowlist value                                      The CIDR subnet for allowing only certain peer connections. Using "public" would allow only public subnets. Example: 192.168.0.0/16 would permit connections to peers on your local network only. The default is to accept all connections.
   --p2p.bootstrap value [ --p2p.bootstrap value ]            bootstrap node info
   --p2p.bootstrap-node value [ --p2p.bootstrap-node value ]  The address of bootstrap node. Beacon node will connect for peer discovery via DHT.  Multiple nodes can be passed by using the flag multiple times but not comma-separated. You can also pass YAML files containing multiple nodes.
   --p2p.denylist value [ --p2p.denylist value ]              The CIDR subnets for denying certainty peer connections. Using "private" would deny all private subnets. Example: 192.168.0.0/16 would deny connections from peers on your local network only. The default is to accept all connections.
   --p2p.host-dns value                                       The DNS address advertised by libp2p. This may be used to advertise an external DNS.
   --p2p.host-ip value                                        The IP address advertised by libp2p. This may be used to advertise an external IP.
   --p2p.key value                                            private key of p2p node
   --p2p.limit.block-batch value                              The amount of blocks the local peer is bounded to request and respond to in a batch. (default: 64)
   --p2p.limit.block-burst-factor value                       The factor by which block batch limit may increase on burst. (default: 2)
   --p2p.limit.block-limiter-period value                     Period to calculate expected limit for a single peer. (default: 5)
   --p2p.listen value [ --p2p.listen value ]                  p2p listen address
   --p2p.local-ip value                                       The local ip address to listen for incoming data.
   --p2p.max-peers value                                      The max number of p2p peers to maintain. (default: 5)
   --p2p.metadata value                                       The file containing the metadata to communicate with other peers.
   --p2p.min-sync-peers value                                 The required number of valid peers to connect with before syncing. (default: 1)
   --p2p.no-discovery                                         Enable only local network p2p and do not connect to cloud bootstrap nodes. (default: false)
   --p2p.peer value [ --p2p.peer value ]                      Connect with this peer. This flag may be used multiple times.
   --p2p.priv-key value                                       The file containing the private key to use in communications with other peers.
   --p2p.relay-node value                                     The address of relay node. The beacon node will connect to the relay node and advertise their address via the relay node to other peers
   --p2p.static-id                                            Enables the peer id of the node to be fixed by saving the generated network key to the default key path. (default: true)
   --p2p.tcp-port value                                       The port used by libp2p. (default: 61016)
   --p2p.udp-port value                                       The port used by discv5. (default: 61015)
   --pprof                                                    Enable the pprof HTTP server (default: false)
   --pprof.block                                              Turn on block profiling (default: false)
   --pprof.maxcpu value                                       setup number of cpu (default: 0)
   --pprof.mutex                                              Turn on mutex profiling (default: false)
   --pprof.port value                                         pprof HTTP server listening port (default: 0)
   --version, -v                                              print the version (default: false)
   --ws                                                       Enable the WS-RPC server (default: false)
   --ws.addr value                                            WS-RPC server listening interface
   --ws.api value                                             API's offered over the WS-RPC interface
   --ws.origins value                                         Origins from which to accept websockets requests
   --ws.port value                                            WS-RPC server listening port (default: "20013")
```
