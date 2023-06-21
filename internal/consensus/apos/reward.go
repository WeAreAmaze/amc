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

package apos

import (
	"bytes"
	"context"
	"errors"
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/math"
	"github.com/holiman/uint256"
	"sort"
	"strings"

	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/contracts/deposit"
	"github.com/amazechain/amc/params"

	"github.com/amazechain/amc/common/types"
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

type Reward struct {
	config      *params.ConsensusConfig
	chainConfig *params.ChainConfig

	ctx         context.Context
	rewardLimit *uint256.Int
	rewardEpoch *uint256.Int
}

type AccountReward struct {
	Account types.Address
	Number  *uint256.Int
	Value   *uint256.Int
}
type AccountRewards []*AccountReward

func (r AccountRewards) Len() int {
	return len(r)
}

func (r AccountRewards) Less(i, j int) bool {
	return strings.Compare(r[i].Account.String(), r[j].Account.String()) > 0
}

func (r AccountRewards) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func newReward(config *params.ConsensusConfig, chainConfig *params.ChainConfig) *Reward {
	rewardLimitBig, _ := uint256.FromBig(config.APos.RewardLimit)
	return &Reward{
		ctx: context.TODO(), config: config, chainConfig: chainConfig,
		rewardLimit: rewardLimitBig,
		rewardEpoch: uint256.NewInt(config.APos.RewardEpoch),
	}
}

func (r *Reward) GetRewards(addr types.Address, from *uint256.Int, to *uint256.Int, getBlockByNumber func(*uint256.Int) (block.IBlock, error)) (*RewardResponse, error) {
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

		blockNr := r.epoch2number(i)
		blk, err := getBlockByNumber(blockNr)
		if err != nil {
			return nil, err
		}

		for _, reward := range blk.Body().Reward() {
			if bytes.Compare(reward.Address[:], addr[:]) == 0 {
				resp.Data = append(resp.Data, &RewardResponseValue{
					Value:       *reward.Amount,
					Timestamp:   hexutil.Uint64(blk.Header().(*block.Header).Time),
					BlockNumber: blockNr.String(),
				})
				resp.Total = resp.Total.Add(resp.Total, reward.Amount)
			}
		}
	}
	sort.Sort(resp.Data)

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

func (r *Reward) buildRewards(tx kv.RwTx, number *uint256.Int, setRewards bool) (map[types.Address]*uint256.Int, error) {

	endNumber := new(uint256.Int).Sub(number, r.rewardEpoch)

	//calculate last batch but this one
	currentNr := number.Clone()
	currentNr.SubUint64(currentNr, 1)
	rewardMap := make(map[types.Address]*uint256.Int, 0)
	depositeMap := map[types.Address]*deposit.Info{}

	for currentNr.Cmp(endNumber) >= 0 {
		// Todo use cache instead ?
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
			depositInfo, ok := depositeMap[verifier.Address]
			if !ok {
				depositInfo = deposit.GetDepositInfo(tx, verifier.Address)
				if depositInfo == nil {
					continue
				}
				depositeMap[verifier.Address] = depositInfo
				log.Debug("account deposite infos", "addr", verifier.Address, "perblock", depositInfo.RewardPerBlock, "perepoch", depositInfo.MaxRewardPerEpoch)
			}

			addrReward, ok := rewardMap[verifier.Address]
			if !ok {
				addrReward = uint256.NewInt(0)
			}

			rewardMap[verifier.Address] = math.Min256(addrReward.Add(addrReward, depositInfo.RewardPerBlock), depositInfo.MaxRewardPerEpoch.Clone())
		}

		currentNr.SubUint64(currentNr, 1)
	}

	for addr, amount := range rewardMap {
		var payAmount, unpayAmount *uint256.Int

		lastSedi, err := r.getAccountRewardUnpaid(tx, addr)
		if err != nil {
			log.Debug("build reward Big map get account reward error,err=", err)
			return nil, err
		}
		if lastSedi != nil {
			amount.Add(amount, lastSedi)
		}

		if amount.Cmp(r.rewardLimit) >= 0 {
			payAmount = amount.Clone()
			unpayAmount = uint256.NewInt(0)
		} else {
			payAmount = uint256.NewInt(0)
			unpayAmount = amount.Clone()
		}

		rewardMap[addr] = payAmount

		if setRewards {
			log.Debug("🔨 set account reward unpaid", "addr", addr, "non-pay amount", unpayAmount.Uint64(), "pay amount", payAmount.Uint64(), "number", currentNr.String())
			if err := r.setAccountRewardUnpaid(tx, addr, unpayAmount); err != nil {
				return nil, err
			}
		}
	}

	log.Debug("buildrewards maps", "rewardMap", rewardMap, "rewardmap len", len(rewardMap), "issetreward", setRewards)

	return rewardMap, nil
}

//func (r *Reward) setRewardByEpochPaid(tx kv.RwTx, epoch *uint256.Int, rewardMap map[types.Address]*uint256.Int) error {
//	if tx == nil {
//		return errors.New("setrewardepoch tx nil")
//	}
//	if len(rewardMap) == 0 {
//		return nil
//	}
//	key := fmt.Sprintf("epoch:%s", epoch.String())
//	err := rawdb.PutEpochReward(tx, key, rewardMap)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func (r *Reward) getAccountRewardUnpaid(tx kv.Getter, account types.Address) (*uint256.Int, error) {
	value, err := rawdb.GetAccountReward(tx, account)
	if err != nil {
		return uint256.NewInt(0), err
	}
	return value, nil
}

func (r *Reward) setAccountRewardUnpaid(tx kv.Putter, account types.Address, val *uint256.Int) error {
	return rawdb.PutAccountReward(tx, account, val)
}

func (r *Reward) number2epoch(number *uint256.Int) *uint256.Int {
	beijingBlock, _ := uint256.FromBig(r.chainConfig.BeijingBlock)
	if beijingBlock.Cmp(number) == 1 {
		return uint256.NewInt(0)
	}
	return new(uint256.Int).Div(new(uint256.Int).Sub(number, beijingBlock), r.rewardEpoch)
}

func (r *Reward) epoch2number(epoch *uint256.Int) *uint256.Int {
	beijingBlock, _ := uint256.FromBig(r.chainConfig.BeijingBlock)
	return new(uint256.Int).Add(new(uint256.Int).Mul(epoch, r.rewardEpoch), beijingBlock)
}
