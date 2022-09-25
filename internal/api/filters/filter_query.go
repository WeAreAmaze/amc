package filters

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/types"
	mvm_common "github.com/amazechain/amc/internal/avm/common"
	"github.com/amazechain/amc/internal/avm/common/hexutil"
	mvm_types "github.com/amazechain/amc/internal/avm/types"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
	"math/big"
)

// FilterCriteria contains options for contract log filtering.
type FilterCriteria struct {
	BlockHash types.Hash      // used by eth_getLogs, return logs only from block with this hash
	FromBlock *big.Int        // beginning of the queried range, nil means genesis block
	ToBlock   *big.Int        // end of the rtypes nil means latest block
	Addresses []types.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position AND B in second position
	// {{A}, {B}}         matches topic A in first position AND B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position AND (C OR D) in second position
	Topics [][]types.Hash
}

// UnmarshalJSON sets *args fields with given data.
func (args *FilterCriteria) UnmarshalJSON(data []byte) error {
	type input struct {
		BlockHash *mvm_common.Hash     `json:"blockHash"`
		FromBlock *jsonrpc.BlockNumber `json:"fromBlock"`
		ToBlock   *jsonrpc.BlockNumber `json:"toBlock"`
		Addresses interface{}          `json:"address"`
		Topics    []interface{}        `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.BlockHash != nil {
		if raw.FromBlock != nil || raw.ToBlock != nil {
			// BlockHash is mutually exclusive with FromBlock/ToBlock criteria
			return fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock, choose one or the other")
		}
		args.BlockHash = mvm_types.ToAmcHash(*raw.BlockHash)
	} else {
		if raw.FromBlock != nil {
			args.FromBlock = big.NewInt(raw.FromBlock.Int64())
		}

		if raw.ToBlock != nil {
			args.ToBlock = big.NewInt(raw.ToBlock.Int64())
		}
	}

	args.Addresses = []types.Address{}

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %v", i, err)
					}
					args.Addresses = append(args.Addresses, mvm_types.ToAmcAddress(addr))
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", err)
			}
			args.Addresses = []types.Address{mvm_types.ToAmcAddress(addr)}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		args.Topics = make([][]types.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				args.Topics[i] = []types.Hash{mvm_types.ToAmcHash(top)}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						args.Topics[i] = append(args.Topics[i], mvm_types.ToAmcHash(parsed))
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}

func decodeAddress(s string) (mvm_common.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != mvm_common.AddressLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), mvm_common.AddressLength)
	}
	return mvm_common.BytesToAddress(b), err
}

func decodeTopic(s string) (mvm_common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != mvm_common.HashLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), mvm_common.HashLength)
	}
	return mvm_common.BytesToHash(b), err
}
