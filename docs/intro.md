# AMC DOCS
Documentation provided for AMC users and developers.

[![Telegram Chat][tg-badge]][tg-url]

AMC (Amaze Chain) is a blockchain full node implementation characterized by being user-friendly, highly modular, and fast and efficient.

## What is this about?

AMC is a flexible node implementation designed to work with all node protocols that support the Amaze Chain ecosystem. Initially led by Amaze, it is licensed under both Apache and MIT licenses.

As a comprehensive node for Amaze Chain, AMC enables users to easily connect to the Amaze Chain network and participate in blockchain activities. This includes processing transactions, accessing logs and traces, and interacting with smart contracts.

Building a successful Amaze Chain node relies on creating a robust implementation that emphasizes security, efficiency, and ease of use, ensuring smooth performance even on standard consumer hardware. Additionally, fostering an active community of contributors is vital for continually improving and maintaining the software.

## What are the goals of AMC?

**1. Modularity**

Every component of AMC is built as a library: well-tested, heavily documented, and benchmarked. We envision developers importing the node's packages, mixing and matching, and innovating on top of them.

Examples of such usage include, but are not limited to, launching standalone P2P networks, talking directly to a node's database, or "unbundling" the node into the components you need.

To achieve this, we are licensing AMC under the Apache/MIT permissive license.

**2. Performance**

AMC aims to be fast, so we used Golang and parallel virtual machine sync node architecture.

We also used tested and optimized Amaze Chain libraries.

**3. Free for anyone to use any way they want**

AMC is free open-source software, built by the community for the community.

By licensing the software under the Apache/MIT license, we want developers to use it without being bound by business licenses, or having to think about the implications of GPL-like licenses.

**4. Client Diversity**

The Amaze Chain protocol becomes more antifragile when no node implementation dominates. This ensures that if there's a software bug, the network does not confirm a wrong block. By building a new client, we hope to contribute to Amaze Chain's antifragility.

**5. Used by a wide demographic**

We aim to solve for node operators who care about fast historical queries, but also for hobbyists who cannot operate on large hardware.

We also want to support teams and individuals who want both sync from genesis and via "fast sync".

We envision that AMC will be flexible enough for the trade-offs each team faces.

## Who is this for?

AMC is a new Amaze Chain full node allowing users to sync and interact with the entire blockchain, including its historical state if in archive mode.

- Full node: It can be used as a full node, storing and processing the entire blockchain, validating blocks and transactions, and participating in the consensus process.

- Archive node: It can also be used as an archive node, storing the entire history of the blockchain, which is useful for applications that need access to historical data. As a data engineer/analyst, or as a data indexer, you'll want to use Archive mode. For all other use cases where historical access is not needed, you can use Full mode.

## Is this secure?

AMC implements the specification of Amaze Chain as defined in the repository. To ensure the node is built securely, we run the following tests:

1. Virtual machine state tests are run on every Pull Request.
2. We regularly re-sync multiple nodes from scratch.
3. We operate multiple nodes at the tip of Amaze Chain mainnet and various testnets.
4. We extensively unit test, fuzz test, and document all our code, while also restricting PRs with aggressive lint rules.
5. We also plan to audit/fuzz the virtual machine & parts of the codebase. Please reach out if you're interested in collaborating on securing this codebase.

We intend to also audit/fuzz the EVM & parts of the codebase. Please reach out if you're interested in collaborating on securing this codebase.

## Sections

Here are some useful sections to jump to:

- Install AMC by following the [guide](./installation/installation.md).
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