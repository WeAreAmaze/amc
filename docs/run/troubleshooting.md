# Troubleshooting

As amc is still in alpha, while running the node you can experience some problems related to different parts of the system: pipeline sync, blockchain tree, p2p, database, etc.

This page tries to answer how to deal with the most popular issues.

## Database

### Database write error

If you encounter an irrecoverable database-related errors, in most of the cases it's related to the RAM/NVMe/SSD you use. For example:
```console
Error: A stage encountered an irrecoverable error.

Caused by:
   0: An internal database error occurred: Database write error code: -30796
   1: Database write error code: -30796
```

or

```console
Error: A stage encountered an irrecoverable error.

Caused by:
   0: An internal database error occurred: Database read error code: -30797
   1: Database read error code: -30797
```

1. Check your memory health: use [memtest86+](https://www.memtest.org/) or [memtester](https://linux.die.net/man/8/memtester). If your memory is faulty, it's better to resync the node on different hardware.
2. Check database integrity:

```bash
git clone https://github.com/amaze/amc
cd amc
make db-tools
db-tools/mdbx_chk $(amc db path)/mdbx.dat | tee mdbx_chk.log
```
If mdbx_chk has detected any errors, please open an issue and post the output from the mdbx_chk.log file.
