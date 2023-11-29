# Ports

This section provides essential information about the ports used by the system, their primary purposes, and recommendations for exposure settings.

## Peering Ports

- **Port:** 61016
- **Protocol:** TCP
- **Purpose:** Peering with other nodes for synchronization of blockchain data. Nodes communicate through this port to maintain network consensus and share updated information.
- **Exposure Recommendation:** This port should be exposed to enable seamless interaction and synchronization with other nodes in the network.


- **Port:** 61015
- **Protocol:** UDP
- **Purpose:** Peering discovery other peering using DiscoveryV5 protocol.

## Metrics Port

- **Port:** 6060
- **Protocol:** TCP
- **Purpose:** This port is designated for serving metrics related to the system's performance and operation. It allows internal monitoring and data collection for analysis.
- **Exposure Recommendation:** By default, this port should not be exposed to the public. It is intended for internal monitoring and analysis purposes.

## HTTP RPC Port

- **Port:** 20012   
- **Protocol:** TCP
- **Purpose:** Port 8545 provides an HTTP-based Remote Procedure Call (RPC) interface. It enables external applications to interact with the blockchain by sending requests over HTTP.
- **Exposure Recommendation:** Similar to the metrics port, exposing this port to the public is not recommended by default due to security considerations.

## WS RPC Port

- **Port:** 20013
- **Protocol:** TCP
- **Purpose:** Port 8546 offers a WebSocket-based Remote Procedure Call (RPC) interface. It allows real-time communication between external applications and the blockchain.
- **Exposure Recommendation:** As with the HTTP RPC port, the WS RPC port should not be exposed by default for security reasons.

