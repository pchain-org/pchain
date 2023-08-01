package state

import (
	"math/big"
	"github.com/ethereum/go-ethereum/log"
)

// GetEpochRewardBalance returns a value in Reward trie
func (self *state1Object) GetEpochRewardBalance(key uint64) *big.Int {
	return self.data.EpochReward[key]
}

// SetEpochRewardBalance updates a value in Epoch Reward.
func (self *state1Object) SetEpochRewardBalance(key uint64, rewardAmount *big.Int) {

	if rewardAmount.Sign() < 0 {
		log.Infof("!!!amount is negative in SetEpochRewardBalance(), make it 0 by force, addr is %x", self.address)
		rewardAmount = big.NewInt(0)
	}

	epochReward := self.EpochReward()
	epochReward[key] = rewardAmount
	if rewardAmount == nil || rewardAmount.Sign() == 0 {
		delete(epochReward, key)
	}

	self.SetEpochReward(epochReward)
}

