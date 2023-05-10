# AmazeChain 
AmazeChain(AMC) is an implementation of public blockchain (execution client), on the efficiency frontier, written in Go.AmazeChain(AMC) uses modern cryptography methods and other techniques to increase submission efficiency and security.Join our world to develop.

**Disclaimer: this software is currently a tech preview. We will do our best to keep it stable and make no breaking changes, but we don't guarantee anything. Things can and will break.**

## System Requirements

* For an Full node :  >=200GB  storage space.

SSD or NVMe. Do not recommend HDD.

RAM: >=16GB, 64-bit architecture, [Golang version >= 1.19](https://golang.org/doc/install)


## Build from source code
For building the latest alpha release (this will be suitable for most users just wanting to run a node):

### Most Linux systems and macOS

AMC is written in Go, so building from source code requires the most recent version of Go to be installed.
Instructions for installing Go are available at the [Go installation page](https://golang.org/doc/install) and necessary bundles can be downloaded from the [Go download page](https://golang.org/dl/).
And the repository should be cloned to a local repository. Then, the command make amc configures everything for a temporary build and cleans up afterwards. This method of building only works on UNIX-like operating systems
```sh
git clone https://github.com/amazechain/amc.git
cd amc
git checkout alpha
make amc
./build/bin/amc
```
### Windows

Windows users may run AMC in 3 possible ways:

* Build executable binaries natively for Windows using [Chocolatey package manager](https://chocolatey.org/)
* Use Docker :  see [docker-compose.yml](./docker-compose.yml)
* Use WSL (Windows Subsystem for Linux) **strictly on version 2**. Under this option you can build amc just as you would on a regular Linux distribution. You can point your data also to any of the mounted Windows partitions (eg. `/mnt/c/[...]`, `/mnt/d/[...]` etc) but in such case be advised performance is impacted: this is due to the fact those mount points use `DrvFS` which is a [network file system](#blocks-execution-is-slow-on-cloud-network-drives) and, additionally, MDBX locks the db for exclusive access which implies only one process at a time can access data.  This has consequences on the running of `rpcdaemon` which has to be configured as [Remote DB](#for-remote-db) even if it is executed on the very same computer. If instead your data is hosted on the native Linux filesystem non limitations apply. **Please also note the default WSL2 environment has its own IP address which does not match the one of the network interface of Windows host: take this into account when configuring NAT for port 30303 on your router.**


### Docker container
Docker allows for building and running AMC via containers. This alleviates the need for installing build dependencies onto the host OS.
see [docker-compose.yml](./docker-compose.yml) [dockerfile](./Dockerfile).
For convenience we provide the following commands:
```sh
make images # build docker images than contain executable AMC binaries
make up # alias for docker-compose up -d && docker-compose logs -f 
make down # alias for docker-compose down && clean docker data
make start #  alias for docker-compose start && docker-compose logs -f 
make stop # alias for docker-compose stop
```

## Executables

The AmazeChain project comes with one wrappers/executables found in the `cmd`
directory.

|    Command    | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| :-----------: | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
|  **`AmazeChain`**   | Our main AmazeChain CLI client.  It can be used by other processes as a gateway into the AmazeChain network via JSON RPC endpoints exposed on top of HTTP transports. `AmazeChain --help`  for command line options.          |


## AMC ports

| Port  | Protocol  |               Purpose               |  Expose |
|:-----:|:---------:|:-----------------------------------:|:-------:|
| 61016 | TCP & UDP | amc/msg/ && amc/discover && amc/app |  Public |
| 20012 |    TCP    |            Json rpc/HTTP            |  Public |
| 20013 |    TCP    |         Json rpc/Websocket          |  Public |
| 4000  |    TCP    |         BlockChain Explorer         |  Public |

## License
The AmazeChain library is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html).
