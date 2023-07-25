// Copyright 2023 The AmazeChain Authors
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

package modules

import (
	"sort"
	"strings"

	"github.com/ledgerwatch/erigon-lib/kv"
)

//const PlainState = "PlainState"

// StateInfo
const (
	// DatabaseInfo is used to store information about data layout.
	DatabaseInfo = "DbInfo"
	ChainConfig  = "ChainConfig"
)

// PlainState
const (
	//key - contract code hash
	//value - contract code
	Code    = "Code"    // contract code hash -> contract code
	Account = "Account" // address(un hashed) -> account encoded
	Storage = "Storage" // address (un hashed) + incarnation + storage key (un hashed) -> storage value(types.Hash)
	Reward  = "Reward"  // ...
	Deposit = "Deposit" // Deposit info

	//key - addressHash+incarnation
	//value - code hash
	ContractCode = "HashedCodeHash"

	PlainContractCode = "PlainCodeHash" // address+incarnation -> code hash

	// IncarnationMap "incarnation" - uint16 number - how much times given account was SelfDestruct'ed
	IncarnationMap = "IncarnationMap" // address -> incarnation of account when it was last deleted
)

// HistoryState
const (
	AccountChangeSet = "AccountChangeSet" // blockNum_u64 ->  address + account(encoded)
	AccountsHistory  = "AccountHistory"   // address + shard_id_u64 -> roaring bitmap  - list of block where it changed

	StorageChangeSet = "StorageChangeSet" // blockNum_u64 + address + incarnation_u64 ->  plain_storage_key + value
	StorageHistory   = "StorageHistory"   // address + storage_key + shard_id_u64 -> roaring bitmap - list of block where it changed
)

// Block
const (
	Headers         = "Header"                 // block_num_u64 + hash -> header
	HeaderNumber    = "HeaderNumber"           // header_hash -> num_u64
	HeaderTD        = "HeadersTotalDifficulty" // block_num_u64 + hash -> td
	HeaderCanonical = "CanonicalHeader"        // block_num_u64 -> header hash

	// headBlockKey tracks the latest know full block's hash.
	HeadBlockKey = "LastBlock"

	HeadHeaderKey = "LastHeader"

	BlockBody       = "BlockBody"               // block_num_u64 + hash -> block body
	BlockTx         = "BlockTransaction"        // tbl_sequence_u64 -> (tx)
	NonCanonicalTxs = "NonCanonicalTransaction" // tbl_sequence_u64 -> rlp(tx)
	MaxTxNum        = "MaxTxNum"                // block_number_u64 -> max_tx_num_in_block_u64
	TxLookup        = "BlockTransactionLookup"  // hash -> transaction/receipt lookup metadata

	BlockVerify  = "BlockVerify"
	BlockRewards = "BlockRewards"

	// Transaction senders - stored separately from the block bodies
	Senders = "TxSender" // block_num_u64 + blockHash -> sendersList (no serialization format, every 20 bytes is new sender)

	Receipts = "Receipt"        // block_num_u64 -> canonical block receipts (non-canonical are not stored)
	Log      = "TransactionLog" // block_num_u64 + txId -> logs of transaction

	// Stores bitmap indices - in which block numbers saw logs of given 'address' or 'topic'
	// [addr or topic] + [2 bytes inverted shard number] -> bitmap(blockN)
	// indices are sharded - because some bitmaps are >1Mb and when new incoming blocks process it
	//	 updates ~300 of bitmaps - by append small amount new values. It cause much big writes (MDBX does copy-on-write).
	//
	// if last existing shard size merge it with delta
	// if serialized size of delta > ShardLimit - break down to multiple shards
	// shard number - it's biggest value in bitmap
	LogTopicIndex   = "LogTopicIndex"
	LogAddressIndex = "LogAddressIndex"

	// CallTraceSet is the name of the table that contain the mapping of block number to the set (sorted) of all accounts
	// touched by call traces. It is DupSort-ed table
	// 8-byte BE block number -> account address -> two bits (one for "from", another for "to")
	CallTraceSet = "CallTraceSet"
	// Indices for call traces - have the same format as LogTopicIndex and LogAddressIndex
	// Store bitmap indices - in which block number we saw calls from (CallFromIndex) or to (CallToIndex) some addresses
	CallFromIndex = "CallFromIndex"
	CallToIndex   = "CallToIndex"

	Sequence = "Sequence" // tbl_name -> seq_u64

	Stake = "Stake" // stakes   amc_stake -> bytes

)

const (
	SignersDB   = "signersDB"
	PoaSnapshot = "poaSnapshot"
)

var AmcTables = []string{
	Code,
	Account,
	Storage,
	PlainContractCode,
	IncarnationMap,

	DatabaseInfo,
	ChainConfig,

	AccountsHistory,
	AccountChangeSet,
	StorageHistory,
	StorageChangeSet,

	Headers,
	HeaderTD,
	HeaderCanonical,
	HeaderNumber,

	HeadBlockKey,
	HeadHeaderKey,

	BlockBody,
	BlockTx,

	TxLookup,
	Senders,
	Receipts,
	Log,

	SignersDB,
	PoaSnapshot,
	Sequence,

	Reward,
	Deposit,
	BlockVerify,
	BlockRewards,
}

var AmcTableCfg = kv.TableCfg{
	AccountChangeSet: {Flags: kv.DupSort},
	StorageChangeSet: {Flags: kv.DupSort},
	Storage: {
		Flags:                     kv.DupSort,
		AutoDupSortKeysConversion: true,
		DupFromLen:                54,
		DupToLen:                  34,
	},
	//
	Deposit: {Flags: kv.DupSort},
}

func AmcInit() {
	sort.SliceStable(AmcTables, func(i, j int) bool {
		return strings.Compare(AmcTables[i], AmcTables[j]) < 0
	})
	for _, name := range AmcTables {
		_, ok := AmcTableCfg[name]
		if !ok {
			AmcTableCfg[name] = kv.TableCfgItem{}
		}
	}
}
