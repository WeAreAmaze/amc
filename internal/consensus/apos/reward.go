package apos

import (
	"context"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/math"
	"github.com/holiman/uint256"
	"sort"
	"strings"

	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/contracts/deposit"
	"github.com/amazechain/amc/params"

	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/rawdb"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type RewardResponse struct {
	Address types.Address        `json:"address" yaml:"address"`
	Data    RewardResponseValues `json:"data" yaml:"data"`
	Total   *uint256.Int         `json:"total" yaml:"total"`
}
type RewardResponseValue struct {
	Value       uint256.Int    `json:"value" yaml:"value"`
	Timestamp   hexutil.Uint64 `json:"timestamp" yaml:"timestamp"`
	BlockNumber string         `json:"blockNumber" yaml:"blockNumber"`
}
type RewardResponseValues []*RewardResponseValue

func (r RewardResponseValues) Len() int {
	return len(r)
}

func (r RewardResponseValues) Less(i, j int) bool {
	return r[i].Timestamp < r[j].Timestamp
}

func (r RewardResponseValues) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type RewardEpochMap rawdb.RewardEntry

type Reward struct {
	config      *conf.ConsensusConfig
	chainConfig *params.ChainConfig

	ctx         context.Context
	rewardLimit *uint256.Int
	rewardEpoch *uint256.Int
}

type AccountReward struct {
	Account string
	Number  *uint256.Int
	Value   *uint256.Int
}
type AccountRewards []*AccountReward

func (r AccountRewards) Len() int {
	return len(r)
}

func (r AccountRewards) Less(i, j int) bool {
	return strings.Compare(r[i].Account, r[j].Account) > 0
}

func (r AccountRewards) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func newReward(config *conf.ConsensusConfig, chainConfig *params.ChainConfig) *Reward {
	rewardLimitBig, _ := uint256.FromBig(config.APos.RewardLimit)
	return &Reward{
		ctx: context.TODO(), config: config, chainConfig: chainConfig,
		rewardLimit: rewardLimitBig,
		rewardEpoch: uint256.NewInt(config.APos.RewardEpoch),
	}
}

func (r *Reward) GetRewards(tx kv.Getter, addr types.Address, from *uint256.Int, to *uint256.Int) (*RewardResponse, error) {
	resp := new(RewardResponse)
	resp.Address = addr
	resp.Data = make([]*RewardResponseValue, 0)

	if from.Uint64() > to.Uint64() {
		return nil, errors.New("from > to number")
	}
	endEpoch := r.number2epoch(to)
	startEpoch := r.number2epoch(from)

	resp.Total = uint256.NewInt(0)

	for i := endEpoch; i.Cmp(startEpoch) > 0; i = i.SubUint64(i, 1) {

		reward, err := rawdb.GetEpochReward(tx, i)
		if err != nil {
			return nil, err
		}

		if entry, ok := reward[addr.String()]; ok && entry != nil && entry.Cmp(uint256.NewInt(0)) == 1 {
			blockNumber := r.epoch2number(i)
			hash, err := rawdb.ReadCanonicalHash(tx, blockNumber.Uint64())
			if nil != err {
				log.Error("cannot open chain db", "err", err)
				return nil, err
			}
			if hash == (types.Hash{}) {
				log.Debug("readcanonicalhash got empty", "number", blockNumber)
				continue
			}
			header := rawdb.ReadHeader(tx, hash, blockNumber.Uint64())
			if header == nil {
				return nil, errors.New("buildreward header type assert error")
			}
			resp.Data = append(resp.Data, &RewardResponseValue{
				Value:       *entry,
				Timestamp:   hexutil.Uint64(header.Time),
				BlockNumber: blockNumber.String(),
			})
			resp.Total = resp.Total.Add(resp.Total, entry)
		}
	}
	sort.Sort(resp.Data)

	return resp, nil
}

func (r *Reward) GetBlockRewards(tx kv.Getter, header block.IHeader) (map[types.Address]uint256.Int, error) {

	reward, err := rawdb.GetEpochReward(tx, r.number2epoch(header.Number64()))

	if err != nil {
		return nil, err
	}
	resp := make(map[types.Address]uint256.Int, len(reward))
	for addr, value := range reward {
		resp[types.HexToAddress(addr)] = *value
	}

	return resp, nil
}

func (r *Reward) SetRewards(tx kv.RwTx, number *uint256.Int, setRewards bool) (AccountRewards, error) {
	currentRewardMap, err := r.buildRewards(tx, number, setRewards)
	if err != nil {
		return nil, err
	}
	resp := make(AccountRewards, 0, len(currentRewardMap))
	for k, v := range currentRewardMap {
		resp = append(resp, &AccountReward{
			Account: k,
			Value:   v,
		})
	}
	sort.Sort(resp)
	return resp, nil
}

func (r *Reward) buildRewards(tx kv.RwTx, number *uint256.Int, setRewards bool) (rawdb.RewardEntry, error) {

	epoch := r.number2epoch(number)
	endNumber := new(uint256.Int).Sub(number, r.rewardEpoch)

	//calculate last batch but this one
	currentNr := number.Clone()
	currentNr.SubUint64(currentNr, 1)
	rewardMap := rawdb.NewRewardEntry()
	depositeMap := map[types.Address]*deposit.Info{}

	for currentNr.Cmp(endNumber) >= 0 {
		hash, err := rawdb.ReadCanonicalHash(tx, currentNr.Uint64())
		if nil != err {
			log.Error("cannot open chain db", "err", err)
			return nil, err
		}
		if hash == (types.Hash{}) {
			return nil, err
		}
		header := rawdb.ReadHeader(tx, hash, currentNr.Uint64())
		if header == nil {
			return nil, errors.New("buildreward header type assert error")
		}

		block := rawdb.ReadBlock(tx, header.Hash(), header.Number.Uint64())
		if block == nil {
			return nil, errors.New("buildreward block type assert error")
		}

		verifiers := block.Body().Verifier()
		for _, verifier := range verifiers {
			addrReward, ok := rewardMap[verifier.Address.Hex()]
			if !ok {
				addrReward = uint256.NewInt(0)
			}

			depositInfo, ok := depositeMap[verifier.Address]
			if !ok {
				depositInfo = deposit.GetDepositInfo(tx, verifier.Address)
				if depositInfo == nil {
					continue
				}
				depositeMap[verifier.Address] = depositInfo
				log.Debug("account deposite infos", "addr", verifier.Address, "perblock", depositInfo.RewardPerBlock, "perepoch", depositInfo.MaxRewardPerEpoch)
			}

			rewardMap[verifier.Address.Hex()] = math.Min256(addrReward.Add(addrReward, depositInfo.RewardPerBlock), depositInfo.MaxRewardPerEpoch)
		}

		currentNr.SubUint64(currentNr, 1)
	}

	for rk, _ := range rewardMap {
		var amount = rewardMap[rk]
		var payAmount, unpayAmount uint256.Int
		addr := types.HexToAddress(rk)

		lastSedi, err := r.getAccountRewardUnpaid(tx, addr)
		if err != nil {
			log.Debug("build reward Big map get account reward error,err=", err)
			return nil, err
		}
		if lastSedi != nil {
			added := uint256.NewInt(0).Add(amount, lastSedi)
			amount = &(*added)
		}

		if amount.Cmp(r.rewardLimit) >= 0 {
			payAmount = *amount
			unpayAmount = *uint256.NewInt(0)
		} else {
			payAmount = *uint256.NewInt(0)
			unpayAmount = *amount
		}

		rewardMap[rk] = &payAmount

		if setRewards {
			log.Info("🔨 set account reward unpaid", "addr", addr, "non-pay amount", unpayAmount.Uint64(), "pay amount", payAmount.Uint64(), "number", currentNr.String())
			if err := r.setAccountRewardUnpaid(tx, addr, unpayAmount); err != nil {
				return nil, err
			}
		}
	}

	log.Debug("buildrewards maps", "rewardMap", rewardMap, "rewardmap len", len(rewardMap), "issetreward", setRewards)

	if setRewards {
		if err := r.setRewardByEpochPaid(tx, epoch, rewardMap); err != nil {
			return nil, err
		}
	}

	return rewardMap, nil
}

func (r *Reward) setRewardByEpochPaid(tx kv.RwTx, epoch *uint256.Int, rewardMap rawdb.RewardEntry) error {
	if tx == nil {
		return errors.New("setrewardepoch tx nil")
	}
	if len(rewardMap) == 0 {
		return nil
	}
	key := fmt.Sprintf("epoch:%s", epoch.String())
	err := rawdb.PutEpochReward(tx, key, rewardMap)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reward) getAccountRewardUnpaid(tx kv.Getter, account types.Address) (*uint256.Int, error) {
	key := fmt.Sprintf("account:%s", account.String())
	value, err := rawdb.GetAccountReward(tx, key)
	if err != nil {
		return uint256.NewInt(0), err
	}
	return value, nil
}

func (r *Reward) setAccountRewardUnpaid(tx kv.Putter, account types.Address, val uint256.Int) error {
	key := fmt.Sprintf("account:%s", account.String())
	err := rawdb.PutAccountReward(tx, key, val)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reward) number2epoch(number *uint256.Int) *uint256.Int {
	// todo
	epoch, _ := new(uint256.Int).DivMod(number, r.rewardEpoch, uint256.NewInt(0))
	return epoch
}

func (r *Reward) epoch2number(epoch *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(epoch, r.rewardEpoch)
}
