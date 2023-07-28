package apos

import (
	"github.com/amazechain/amc/common/block"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/internal/consensus"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/state"
	"github.com/amazechain/amc/params"
	"github.com/holiman/uint256"
	"sort"
)

func AccumulateRewards(r *Reward, number *uint256.Int, chain consensus.ChainHeaderReader) (map[types.Address]*uint256.Int, map[types.Address]*uint256.Int, error) {

	// total reword for address this epoch
	//var rewardMap map[types.Address]*uint256.Int
	unpayMap := make(map[types.Address]*uint256.Int, 0)
	endNumber := new(uint256.Int).Sub(number, r.rewardEpoch)
	////calculate last batch but this one
	//currentNr := number.Clone()
	//currentNr.SubUint64(currentNr, 1)
	//
	//depositeMap := map[types.Address]struct {
	//	reward    *uint256.Int
	//	maxReward *uint256.Int
	//}{}
	//for currentNr.Cmp(endNumber) >= 0 {
	//	block, err := chain.GetBlockByNumber(currentNr)
	//	if nil != err {
	//		return nil, nil, err
	//	}
	//
	//	verifiers := block.Body().Verifier()
	//	for _, verifier := range verifiers {
	//		_, ok := depositeMap[verifier.Address]
	//		if !ok {
	//			low, max := chain.GetDepositInfo(verifier.Address)
	//			if low == nil || max == nil {
	//				continue
	//			}
	//			depositeMap[verifier.Address] = struct {
	//				reward    *uint256.Int
	//				maxReward *uint256.Int
	//			}{reward: low, maxReward: max}
	//
	//			log.Debug("account deposite infos", "addr", verifier.Address, "perblock", low, "perepoch", max)
	//		}
	//
	//		addrReward, ok := rewardMap[verifier.Address]
	//		if !ok {
	//			addrReward = uint256.NewInt(0)
	//		}
	//
	//		rewardMap[verifier.Address] = math.Min256(addrReward.Add(addrReward, depositeMap[verifier.Address].reward), depositeMap[verifier.Address].maxReward.Clone())
	//	}
	//
	//	currentNr.SubUint64(currentNr, 1)
	//}
	rewardMap, err := chain.RewardsOfEpoch(number, endNumber)
	if nil != err {
		return nil, nil, err
	}

	for addr, amount := range rewardMap {
		var payAmount, unpayAmount *uint256.Int

		lastSedi, err := chain.GetAccountRewardUnpaid(addr)
		if err != nil {
			log.Debug("build reward Big map get account reward error,err=", err)
			return nil, nil, err
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
		unpayMap[addr] = unpayAmount
	}

	//log.Debug("buildrewards maps", "rewardMap", rewardMap, "rewardmap len", len(rewardMap), "issetreward", setRewards)
	//
	//if setRewards {
	//	if err := r.setRewardByEpochPaid(tx, epoch, rewardMap); err != nil {
	//		return nil, err
	//	}
	//}

	return rewardMap, unpayMap, nil
}

func doReward(chainConf *params.ChainConfig, consConf *params.ConsensusConfig, state *state.IntraBlockState, header *block.Header, chain consensus.ChainHeaderReader) ([]*block.Reward, map[types.Address]*uint256.Int, error) {
	beijing, _ := uint256.FromBig(chainConf.BeijingBlock)
	number := header.Number64()
	var rewards block.Rewards
	var upayMap map[types.Address]*uint256.Int

	if chainConf.IsBeijing(number.Uint64()) && new(uint256.Int).Mod(new(uint256.Int).Sub(number, beijing), uint256.NewInt(consConf.APos.RewardEpoch)).
		Cmp(uint256.NewInt(0)) == 0 {
		r := newReward(consConf, chainConf)
		var (
			err    error
			payMap map[types.Address]*uint256.Int
		)
		payMap, upayMap, err = AccumulateRewards(r, number, chain)
		if nil != err {
			return nil, nil, err
		}
		for addr, value := range payMap {
			if value.Cmp(uint256.NewInt(0)) > 0 {
				if !state.Exist(addr) {
					state.CreateAccount(addr, false)
				}

				log.Info("🔨 set account reward", "addr", addr, "amount", value.Uint64(), "blockNr", header.Number.Uint64())
				state.AddBalance(addr, value)
				rewards = append(rewards, &block.Reward{
					Address: addr,
					Amount:  value,
				})
			}
		}
		state.SoftFinalise()
		sort.Sort(rewards)
	}
	return rewards, upayMap, nil
}
