# Run amc in a private testnet

For those who need a private testnet to validate functionality or scale with amc.

## Initializing the Amc Database
To create a blockchain node that uses this genesis block, first use amc init to import and sets the canonical genesis block for the new chain. This requires the path to genesis.json to be passed as an argument.
```
Amc init —data.dir data genesis.json
```
## Setting Up Networking

With the node configured and initialized, the next step is to set up a peer-to-peer network. This requires a bootstrap node. The bootstrap node is a normal node that is designated to be the entry point that other nodes use to join the network. Any node can be chosen to be the bootstrap node.

To configure a bootstrap node, the IP address of the machine the bootstrap node will run on must be known. The bootstrap node needs to know its own IP address so that it can broadcast it to other nodes. On a local machine this can be found using tools such as ifconfig and on cloud instances such as Amazon EC2 the IP address of the virtual machine can be found in the management console. Any firewalls must allow UDP and TCP traffic on port 61015/61016.

The bootstrap node IP is set using the —p2p.host-ip flag (the command below contains an example address - replace it with the correct one).
```
Amc --data.dir ./node --p2p.host-ip “1.243.83.152"
```
This command should print a base64 string such as the following example. Other nodes will use the information contained in the bootstrap node record to connect to the peer-to-peer network.
Running Member Nodes

Before running a member node, it must be initialized with the same genesis file as used for the bootstrap node. With the bootnode operational and externally reachable (telnet <ip> <port> will confirm that it is indeed reachable), more Amc nodes can be started and connected to them via the bootstrap node using the —p2p.bootstrap-node flag. The process is to start Amc on the same machine as the bootnode, with a separate data directory and listening port and the bootnode node record provided as an argument:

For example, using data directory (example: data2) and listening port (example:  61115/61116.
):

```
Amc --data.dir ./node2 —p2p.bootstrap-node <bootstrap-node-record> —p2p.udp-port 61115 —p2p.tcp-port 61116 
```