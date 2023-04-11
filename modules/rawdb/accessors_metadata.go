package rawdb

import (
	"encoding/json"
	"fmt"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules"

	"github.com/amazechain/amc/params"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db kv.Getter, hash types.Hash) (*params.ChainConfig, error) {
	data, err := db.GetOne(modules.ChainConfig, modules.ConfigKey(hash))
	if err != nil {
		return nil, fmt.Errorf("fetch ChainConfig from db ,error: %v", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("ChainConfig are empty")
	}
	var config params.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid chain config JSON err: %v", err)
	}
	return &config, nil
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db kv.RwTx, hash types.Hash, cfg *params.ChainConfig) error {
	if cfg == nil {
		return fmt.Errorf("invalid cfg")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Error("Failed to JSON encode chain config", "err", err)
		return err
	}
	if err := db.Put(modules.ChainConfig, modules.ConfigKey(hash), data); err != nil {
		log.Error("Failed to store chain config", "err", err)
		return err
	}
	return nil
}
