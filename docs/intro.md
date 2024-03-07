# AMC DOCS
Documentation provided for amc users and developers.

[![Telegram Chat][tg-badge]][tg-url]

AMC (Amaze Chain) is a blockchain full node implementation characterized by being user-friendly, highly modular, and fast and efficient.


## What is this about?

AMC is a node implementation compatible with all node protocols that support Amaze Chain.

It was originally built and promoted by Amaze, licensed under Apache and MIT licenses.

As a complete Amaze Chain node, amc allows users to connect to the Amaze Chain network and interact with the Amaze Chain blockchain.

This includes sending and receiving transactions, querying logs and traces, as well as accessing and interacting with smart contracts.

Creating a successful Amaze Chain node requires creating a high-quality implementation that is both secure and efficient, and easy to use on consumer hardware. It also requires building a strong community of contributors to help support and improve the software.

## What are the goals of amc?

**1. Modularity**

Every component of amc is built as a library: well-tested, heavily documented, and benchmarked. We envision developers importing the node's packages, mixing and matching, and innovating on top of them.

Examples of such usage include, but are not limited to, launching standalone P2P networks, talking directly to a node's database, or "unbundling" the node into the components you need.

To achieve this, we are licensing amc under the Apache/MIT permissive license.

**2. Performance**

AMC aims to be fast, so we used golang and parallel virtual machine sync node architecture.

We also used tested and optimized Amaze Chain libraries.

**3. Free for anyone to use any way they want**

AMC is free open-source software, built by the community for the community.

By licensing the software under the Apache/MIT license, we want developers to use it without being bound by business licenses, or having to think about the implications of GPL-like licenses.

**4. Client Diversity**

The Amaze Chain protocol becomes more antifragile when no node implementation dominates. This ensures that if there's a software bug, the network does not confirm a wrong block. By building a new client, we hope to contribute to Amaze Chain's antifragility.

**5. Used by a wide demographic**

We aim to solve for node operators who care about fast historical queries, but also for hobbyists who cannot operate on large hardware.

We also want to support teams and individuals who want both sync from genesis and via "fast sync".

We envision that amc will be flexible enough for the trade-offs each team faces.

## Who is this for?

amc is a new Amaze Chain full node allowing users to sync and interact with the entire blockchain, including its historical state if in archive mode.
- Full node: It can be used as a full node, storing and processing the entire blockchain, validating blocks and transactions, and participating in the consensus process.
- Archive node: It can also be used as an archive node, storing the entire history of the blockchain, which is useful for applications that need access to historical data. As a data er/analyst, or as a data indexer, you'll want to use Archive mode. For all other use cases where historical access is not needed, you can use Full mode. 

As a data engineer/analyst, or as a data indexer, you'll want to use Archive mode. For all other use cases where historical access is not needed, you can use Full mode.

## Is this secure?

AMC implements the specification of Amaze Chain as defined in the repository. To ensure the node is built securely, we run the following tests:

1. Virtual machine state tests are run on every Pull Request
1. We regularly re-sync multiple nodes from scratch.
1. We operate multiple nodes at the tip of Amaze Chain mainnet and various testnets.
1. We extensively unit test, fuzz test, and document all our code, while also restricting PRs with aggressive lint rules.
1. We also plan to audit / fuzz the virtual machine & parts of the codebase. Please reach out if you're interested in collaborating on securing this codebase.

We intend to also audit / fuzz the EVM & parts of the codebase. Please reach out if you're interested in collaborating on securing this codebase.

## Sections

Here are some useful sections to jump to:

- Install amc by following the [guide](./installation/installation.md).
- Sync your node on any [official network](./run/run-a-node.md).
- View [statistics and metrics](./run/observability.md) about your node.
- Query the [JSON-RPC](./jsonrpc/intro.md) using Foundry's `cast` or `curl`.
- Set up your [development environment and contribute](./developers/contribute.md)!

> 📖 **About this book**
>
> The book is continuously rendered [here](https://github.com/WeAreAmaze/amc/docs)!
> You can contribute to this book on [GitHub][gh-book].

[tg-badge]: https://img.shields.io/endpoint?color=neon&logo=telegram&label=chat&url=https%3A%2F%2Ftg.sumanjay.workers.dev%2Fparadigm%5Freth
[tg-url]: https://t.me/amazechain
[gh-book]: https://github.com/WeAreAmaze/amc/docs
