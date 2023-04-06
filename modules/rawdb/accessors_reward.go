package rawdb

import (
	"encoding/json"
	"fmt"
	"github.com/holiman/uint256"

	"github.com/amazechain/amc/modules"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type RewardEntry map[string]*uint256.Int

func NewRewardEntry() RewardEntry {
	return make(map[string]*uint256.Int, 0)
}

// PutEpochReward
func PutEpochReward(db kv.Putter, key string, val RewardEntry) error {
	valBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return db.Put(modules.Reward, []byte(key), valBytes)
}

// PutAccountReward
func PutAccountReward(db kv.Putter, key string, val uint256.Int) error {
	return db.Put(modules.Reward, []byte(key), val.Bytes())
}

// GetAccountReward
func GetAccountReward(db kv.Getter, key string) (*uint256.Int, error) {
	val, err := db.GetOne(modules.Reward, []byte(key))
	if err != nil {
		return uint256.NewInt(0), err
	}
	return uint256.NewInt(0).SetBytes(val), nil
}

func GetEpochReward(db kv.Getter, epoch *uint256.Int) (RewardEntry, error) {
	key := fmt.Sprintf("epoch:%s", epoch.String())
	valBytes, err := db.GetOne(modules.Reward, []byte(key))
	if err != nil {
		return nil, err
	}
	if len(valBytes) == 0 {
		return nil, nil
	}
	re := NewRewardEntry()
	if err := json.Unmarshal(valBytes, &re); err != nil {
		return nil, err
	}
	return re, nil
}
