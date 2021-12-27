package state

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

func (db *State1DB) ForEachReward(addr common.Address, cb func(key uint64, rewardBalance *big.Int) bool) {
	so := db.getState1Object(addr)
	if so == nil {
		return
	}

	for epoch, reward  := range so.data.EpochReward {
		cb(epoch, reward)
	}
}


//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (self *State1DB) GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64) *big.Int {

	state1Object := self.getState1Object(addr)
	if state1Object != nil {
		rewardBalance := state1Object.GetEpochRewardBalance(epochNo)
		if rewardBalance != nil {
			return rewardBalance
		}
	}
	
	rb := self.db.TrieDB().GetEpochReward(addr, epochNo,height-1)
	// if 0 epoch reward, try to read from trie
	if rb.Sign() == 0 {
		rb = self.stateDB.GetRewardBalanceByEpochNumber(addr, epochNo)
	}

	return rb
}

func (self *State1DB) AddOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo, height)
	newReward := new(big.Int).Add(currentRewardBalance, amount)
	
	state1Object := self.GetOrNewState1Object(addr)
	// TODO：here are some pi buried
	/*
	rs := state1Object.GetEpochRewardBalance(epochNo)
	if rs == nil { //import all epoch rewards from statedb or diskdb
		allRewards := self.stateDB.GetAllEpochReward(addr, height)
		for epoch, reward := range allRewards {
			state1Object.SetEpochRewardBalance(self.db, epoch, reward)
		}
	}
	*/

	state1Object.SetEpochRewardBalance(epochNo, newReward)
}

func (self *State1DB) SubOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo, height)
	newReward := new(big.Int).Sub(currentRewardBalance, amount)
	
	state1Object := self.GetOrNewState1Object(addr)
	// TODO：here are some pi buried
	/*
	rs := state1Object.GetEpochRewardBalance(epochNo)
	if rs == nil { //import all epoch rewards from statedb or diskdb
		allRewards := self.stateDB.GetAllEpochReward(addr, height)
		for epoch, reward := range allRewards {
			state1Object.SetEpochRewardBalance(self.db, epoch, reward)
		}
	}
	*/
	state1Object.SetEpochRewardBalance(epochNo, newReward)
}

func (self *State1DB) GetAllEpochReward(address common.Address, height uint64) map[uint64]*big.Int {

	result := self.db.TrieDB().GetAllEpochReward(address, height-1)
	self.ForEachReward(address, func(key uint64, rewardBalance *big.Int) bool {
		result[key] = rewardBalance
		return true
	})

	return result
}

//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (self *State1DB) GetEpochRewardExtracted(address common.Address, height uint64) (uint64, error) {

	state1Object := self.getState1Object(address)
	if state1Object != nil {
		extractNumber := state1Object.ExtractNumber()
		if extractNumber == 0 {
			if oldExtractNumber, err := self.db.TrieDB().GetEpochRewardExtracted(address, height-1); err == nil {
				return oldExtractNumber, nil
			}
		}
		return extractNumber, nil
	}

	return 0, errors.New("no extract number found")
}
