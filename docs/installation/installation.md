# Installation

AMC runs on Linux, Windows, and macOS.

There are three primary methods to obtain amc:

* [Pre-built binaries](./binaries.md)
* [Docker images](./docker.md)
* [Building from source.](./source.md)

## Hardware Requirements

The hardware requirements for running amc depend on the node configuration and may change over time as the network grows or new features are implemented.

The most important requirement is by far the disk, whereas CPU and RAM requirements are relatively flexible.



|           | Archive Node                            |  |
|-----------|-----------------------------------------|--|
| Disk      | At least 300GB (TLC NVMe recommended)   |  |
| Memory    | 8GB+                                    |  |
| CPU       | Higher clock speed over core count      |  |
| Bandwidth | Stable 24Mbps+                          |  |

#### QLC and TLC

It is crucial to understand the difference between QLC and TLC NVMe drives when considering the disk requirement.

QLC (Quad-Level Cell) NVMe drives utilize four bits of data per cell, allowing for higher storage density and lower manufacturing costs. However, this increased density comes at the expense of performance. QLC drives have slower read and write speeds compared to TLC drives. They also have a lower endurance, meaning they may have a shorter lifespan and be less suitable for heavy workloads or constant data rewriting.

TLC (Triple-Level Cell) NVMe drives, on the other hand, use three bits of data per cell. While they have a slightly lower storage density compared to QLC drives, TLC drives offer faster performance. They typically have higher read and write speeds, making them more suitable for demanding tasks such as data-intensive applications, gaming, and multimedia editing. TLC drives also tend to have a higher endurance, making them more durable and longer-lasting.

Prior to purchasing an NVMe drive, it is advisable to research and determine whether the disk will be based on QLC or TLC technology. An overview of recommended and not-so-recommended NVMe boards can be found at [here]( https://gist.github.com/yorickdowne/f3a3e79a573bf35767cd002cc977b038).

### Disk

There are multiple types of disks to sync amc, with varying size requirements, depending on the syncing mode.
NVMe drives are recommended for the best performance, with SSDs being a cheaper alternative. HDDs are the cheapest option, but they will take the longest to sync, and are not recommended.

> **Note**
>
> It is highly recommended to choose a TLC drive when using NVMe, and not a QLC drive. See [the note](#qlc-and-tlc) above. A list of recommended drives can be found [here]( https://gist.github.com/yorickdowne/f3a3e79a573bf35767cd002cc977b038).

### CPU

Most of the time during syncing is spent executing transactions, which is a single-threaded operation because transactions may depend on the state of previous transactions.

Therefore, the number of cores is less important, but generally higher clock speeds are better. More cores are better for parallelizable stages (like sender recovery or body downloading), but these stages are not the primary bottleneck for syncing.

### Memory

It is recommended to use at least 8GB of RAM.

Unless you are under heavy RPC load, most of amc's components tend to consume a low amount of memory, so this should matter less than the other requirements.

Higher memory is generally better as it allows for better caching, resulting in less stress on the disk.

### Bandwidth

A stable and reliable internet connection is crucial for both syncing a node from genesis and for keeping up with the chain's tip.

Once you're synced to the tip, you will need a reliable connection, especially if you're operating a validator. A 24Mbps connection is recommended, but you can probably get away with less. Ensure your ISP does not cap your bandwidth.

## What hardware can I get?

If you are buying your own NVMe SSD, please consult this actively maintained hardware comparison. We do not recommend purchasing DRAM-less or QLC devices as these are noticeably slower.

All our benchmarks have been produced on Latitude.sh, a bare metal provider. We use c3.large.x86 boxes, and also recommend trying the s2.small.x86 box for pruned/full nodes. So far, our experience has been smooth, with some users reporting that the NVMe there outperforms AWS NVMe by 3x or more. We are excited for more amc nodes on Latitude.sh, so for a limited time, you can use a discount to get $250 off. Run a node now!

