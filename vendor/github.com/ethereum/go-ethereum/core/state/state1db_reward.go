package state

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)


// ----- Reward Trie
/*
// GetRewardBalanceByEpochNumber
func (self *State1DB) GetRewardBalanceByEpochNumber(addr common.Address, epochNo uint64) *big.Int {
	stateObject := self.getState1Object(addr)
	if stateObject != nil {
		rewardBalance := stateObject.GetEpochRewardBalance(self.db, epochNo)
		if rewardBalance == nil {
			return common.Big0
		} else {
			return rewardBalance
		}
	}
	return common.Big0
}

// AddRewardBalanceByEpochNumber adds reward amount to the account associated with epoch number
func (self *State1DB) AddRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	stateObject := self.GetOrNewState1Object(addr)
	if stateObject != nil {
		// Get EpochRewardBalance and update EpochRewardBalance
		rewardBalance := stateObject.GetEpochRewardBalance(self.db, epochNo)
		var dirtyRewardBalance *big.Int
		if rewardBalance == nil {
			dirtyRewardBalance = amount
		} else {
			dirtyRewardBalance = new(big.Int).Add(rewardBalance, amount)
		}
		stateObject.SetEpochRewardBalance(self.db, epochNo, dirtyRewardBalance)

		// Add amount to Total Reward Balance
		stateObject.AddRewardBalance(amount)
	}
}

// SubRewardBalanceByEpochNumber subtracts reward amount from the account associated with epoch number
func (self *State1DB) SubRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	stateObject := self.GetOrNewState1Object(addr)
	if stateObject != nil {
		// Get EpochRewardBalance and update EpochRewardBalance
		rewardBalance := stateObject.GetEpochRewardBalance(self.db, epochNo)
		var dirtyRewardBalance *big.Int
		if rewardBalance == nil {
			panic("you can't subtract the amount from nil balance, check the code, this should not happen")
		} else {
			dirtyRewardBalance = new(big.Int).Sub(rewardBalance, amount)
		}
		stateObject.SetEpochRewardBalance(self.db, epochNo, dirtyRewardBalance)

		// Sub amount from Total Reward Balance
		stateObject.SubRewardBalance(amount)
	}
}
*/
func (db *State1DB) ForEachReward(addr common.Address, cb func(key uint64, rewardBalance *big.Int) bool) {
	so := db.getState1Object(addr)
	if so == nil {
		return
	}

	for epoch, reward  := range so.data.EpochReward {
		cb(epoch, reward)
	}
}

/*
func (self *State1DB) GetOutsideReward() map[common.Address]Reward {
	return self.rewardOutsideSet
}

func (self *State1DB) ClearOutsideReward() {
	self.rewardOutsideSet = make(map[common.Address]Reward)
}
*/
//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (self *State1DB) GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64) *big.Int {
	//if rewardset, exist := self.rewardOutsideSet[addr]; exist {
	//	if rewardbalance, rewardexist := rewardset[epochNo]; rewardexist {
	//		return rewardbalance
	//	}
	//}
	stateObject := self.getState1Object(addr)
	if stateObject != nil {
		rewardBalance := stateObject.GetEpochRewardBalance(epochNo)
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

//get value from db directly; this will ignore current runtime-context
/*
func (self *State1DB) GetOutsideRewardBalanceByEpochNumberFromDB(addr common.Address, epochNo uint64, height uint64) *big.Int {
	rb := self.db.TrieDB().GetEpochReward(addr, epochNo, height)
	// if 0 epoch reward, try to read from trie
	if rb.Sign() == 0 {
		rb = self.stateDB.GetRewardBalanceByEpochNumber(addr, epochNo)
	}

	return rb
}
*/

func (self *State1DB) AddOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo, height)
	newReward := new(big.Int).Add(currentRewardBalance, amount)
	/*
	if rs, exist := self.rewardOutsideSet[addr]; exist {
		rs[epochNo] = newReward
	} else {
		epochReward := Reward{epochNo: newReward}
		self.rewardOutsideSet[addr] = epochReward
	}
	*/
	
	stateObject := self.GetOrNewState1Object(addr)
	rs := stateObject.GetEpochRewardBalance(epochNo)
	if rs == nil { //import all epoch rewards from statedb or diskdb
		allRewards := self.stateDB.GetAllEpochReward(addr, height)
		for epoch, reward := range allRewards {
			stateObject.SetEpochRewardBalance(self.db, epoch, reward)
		}
	}
	stateObject.SetEpochRewardBalance(self.db, epochNo, newReward)

	self.stateDB.AddRewardBalance(addr, amount)
}

func (self *State1DB) SubOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo, height)
	newReward := new(big.Int).Sub(currentRewardBalance, amount)
	
	/*if rs, exist := self.rewardOutsideSet[addr]; exist {
		rs[epochNo] = newReward
	} else {
		epochReward := Reward{epochNo: newReward}
		self.rewardOutsideSet[addr] = epochReward
	}
	*/
	
	stateObject := self.getState1Object(addr)
	if stateObject == nil {
		stateObject, _ = self.createState1Object(addr)
	}
	rs := stateObject.GetEpochRewardBalance(epochNo)
	if rs == nil { //import all epoch rewards from statedb or diskdb
		allRewards := self.stateDB.GetAllEpochReward(addr, height)
		for epoch, reward := range allRewards {
			stateObject.SetEpochRewardBalance(self.db, epoch, reward)
		}
	}
	stateObject.SetEpochRewardBalance(self.db, epochNo, newReward)
	
	
	self.stateDB.SubRewardBalance(addr, amount)
}


//func (self *StateDB) GetEpochReward(address common.Address, epoch uint64) *big.Int {
//	return self.db.TrieDB().GetEpochReward(address, epoch)
//}

func (self *State1DB) GetAllEpochReward(address common.Address, height uint64) map[uint64]*big.Int {

	result := make(map[uint64]*big.Int)
	self.ForEachReward(address, func(key uint64, rewardBalance *big.Int) bool {
		result[key] = rewardBalance
		return true
	})

	if len(result) != 0 {
		return result
	}

	return self.db.TrieDB().GetAllEpochReward(address, height-1)
}
/*
//get value from db directly; this will ignore current runtime-context
func (self *State1DB) GetAllEpochRewardFromDB(address common.Address, height uint64) map[uint64]*big.Int {
	return self.db.TrieDB().GetAllEpochReward(address, height)
}

func (self *State1DB) GetExtractRewardSet() map[common.Address]uint64 {
	return self.extractRewardSet
}

func (self *State1DB) ClearExtractRewardSet() {
	self.extractRewardSet = make(map[common.Address]uint64)
}

func (self *State1DB) MarkEpochRewardExtracted(address common.Address, epoch uint64) {
	self.extractRewardSet[address] = epoch
}
*/
//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (self *State1DB) GetEpochRewardExtracted(address common.Address, height uint64) (uint64, error) {
	stateObject := self.getState1Object(address)
	if stateObject != nil {
		return stateObject.ExtractNumber(), nil
	}

	return self.db.TrieDB().GetEpochRewardExtracted(address, height-1)
}
/*
//get value from db directly; this will ignore current runtime-context
func (self *State1DB) GetEpochRewardExtractedFromDB(address common.Address, height uint64) (uint64, error) {
	return self.db.TrieDB().GetEpochRewardExtracted(address, height)
}
*/

func (self *State1DB) WriteEpochRewardExtracted(address common.Address, epoch uint64, height uint64) error {
	return self.db.TrieDB().WriteEpochRewardExtracted(address, epoch, height)
}
/*
//record candidate's last proposed block which brings reward
func (self *State1DB) ReadOOSLastBlock() (*big.Int, error) {
	return self.db.TrieDB().ReadOOSLastBlock()
}

func (self *State1DB) WriteOOSLastBlock(blockNumber *big.Int) error {
	return self.db.TrieDB().WriteOOSLastBlock(blockNumber)
}
*/