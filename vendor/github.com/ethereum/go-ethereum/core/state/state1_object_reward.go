package state

import (
	"math/big"
)

// GetEpochRewardBalance returns a value in Reward trie
func (self *state1Object) GetEpochRewardBalance(key uint64) *big.Int {
	return self.data.EpochReward[key]
}

// SetEpochRewardBalance updates a value in Epoch Reward.
func (self *state1Object) SetEpochRewardBalance(key uint64, rewardAmount *big.Int) {

	epochReward := self.EpochReward()
	epochReward[key] = rewardAmount
	if rewardAmount == nil || rewardAmount.Sign() == 0 {
		delete(epochReward, key)
	}

	self.SetEpochReward(epochReward)
}

