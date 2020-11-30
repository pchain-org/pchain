package state

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"io"
	"math/big"
	"sort"
)

// ----- RewardBalance (Total)

// GetTotalRewardBalance Retrieve the reward balance from the given address or 0 if object not found
func (self *StateDB) GetTotalRewardBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.RewardBalance()
	}
	return common.Big0
}

func (self *StateDB) AddRewardBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Add amount to Total Reward Balance
		stateObject.AddRewardBalance(amount)
	}
}

func (self *StateDB) SubRewardBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Add amount to Total Reward Balance
		stateObject.SubRewardBalance(amount)
	}
}


// ----- Reward Trie

// GetRewardBalanceByEpochNumber
func (self *StateDB) GetRewardBalanceByEpochNumber(addr common.Address, epochNo uint64) *big.Int {
	stateObject := self.getStateObject(addr)
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
func (self *StateDB) AddRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
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
func (self *StateDB) SubRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
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

func (db *StateDB) ForEachReward(addr common.Address, cb func(key uint64, rewardBalance *big.Int) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}
	it := trie.NewIterator(so.getRewardTrie(db.db).NodeIterator(nil))
	for it.Next() {
		var key uint64
		rlp.DecodeBytes(db.trie.GetKey(it.Key), &key)
		if value, dirty := so.dirtyReward[key]; dirty {
			cb(key, value)
			continue
		}
		var value big.Int
		rlp.DecodeBytes(it.Value, &value)
		cb(key, &value)
	}
}

func (self *StateDB) GetOutsideReward() map[common.Address]Reward {
	return self.rewardOutsideSet
}

func (self *StateDB) ClearOutsideReward() {
	self.rewardOutsideSet = make(map[common.Address]Reward)
}

func (self *StateDB) GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64) *big.Int {
	if rewardset, exist := self.rewardOutsideSet[addr]; exist {
		if rewardbalance, rewardexist := rewardset[epochNo]; rewardexist {
			return rewardbalance
		}
	}
	rb := self.db.TrieDB().GetEpochReward(addr, epochNo)
	// if 0 epoch reward, try to read from trie
	if rb.Sign() == 0 {
		rb = self.GetRewardBalanceByEpochNumber(addr, epochNo)
	}

	return rb
}

func (self *StateDB) AddOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo)
	newReward := new(big.Int).Add(currentRewardBalance, amount)
	if rs, exist := self.rewardOutsideSet[addr]; exist {
		rs[epochNo] = newReward
	} else {
		epochReward := Reward{epochNo: newReward}
		self.rewardOutsideSet[addr] = epochReward
	}

	self.AddRewardBalance(addr, amount)
}

func (self *StateDB) SubOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, amount *big.Int) {
	currentRewardBalance := self.GetOutsideRewardBalanceByEpochNumber(addr, epochNo)
	newReward := new(big.Int).Sub(currentRewardBalance, amount)
	if rs, exist := self.rewardOutsideSet[addr]; exist {
		rs[epochNo] = newReward
	} else {
		epochReward := Reward{epochNo: newReward}
		self.rewardOutsideSet[addr] = epochReward
	}

	self.SubRewardBalance(addr, amount)
}


func (self *StateDB) GetEpochReward(address common.Address, epoch uint64) *big.Int {
	return self.db.TrieDB().GetEpochReward(address, epoch)
}

func (self *StateDB) GetAllEpochReward(address common.Address) map[uint64]*big.Int {
	return self.db.TrieDB().GetAllEpochReward(address)
}

func (self *StateDB) GetExtractRewardSet() map[common.Address]uint64 {
	return self.extractRewardSet
}

func (self *StateDB) ClearExtractRewardSet() {
	self.extractRewardSet = make(map[common.Address]uint64)
}

func (self *StateDB) MarkEpochRewardExtracted(address common.Address, epoch uint64) {
	self.extractRewardSet[address] = epoch
}

func (self *StateDB) GetEpochRewardExtracted(address common.Address) (uint64, error) {
	return self.db.TrieDB().GetEpochRewardExtracted(address)
}

func (self *StateDB) WriteEpochRewardExtracted(address common.Address, epoch uint64) error {
	return self.db.TrieDB().WriteEpochRewardExtracted(address, epoch)
}


//record candidate's last proposed block which brings reward
func (self *StateDB) ReadOOSLastBlock() (*big.Int, error) {
	return self.db.TrieDB().ReadOOSLastBlock()
}

func (self *StateDB) WriteOOSLastBlock(blockNumber *big.Int) error {
	return self.db.TrieDB().WriteOOSLastBlock(blockNumber)
}

// ----- Reward Set

// MarkAddressReward adds the specified object to the dirty map to avoid
func (self *StateDB) MarkAddressReward(addr common.Address) {
	if _, exist := self.GetRewardSet()[addr]; !exist {
		self.rewardSet[addr] = struct{}{}
		self.rewardSetDirty = true
	}
}

func (self *StateDB) GetRewardSet() RewardSet {
	if len(self.rewardSet) != 0 {
		return self.rewardSet
	}
	// Try to get from Trie
	enc, err := self.trie.TryGet(rewardSetKey)
	if err != nil {
		self.setError(err)
		return nil
	}
	var value RewardSet
	if len(enc) > 0 {
		err := rlp.DecodeBytes(enc, &value)
		if err != nil {
			self.setError(err)
		}
		self.rewardSet = value
	}
	return value
}

func (self *StateDB) commitRewardSet() {
	data, err := rlp.EncodeToBytes(self.rewardSet)
	if err != nil {
		panic(fmt.Errorf("can't encode reward set : %v", err))
	}
	self.setError(self.trie.TryUpdate(rewardSetKey, data))
}

func (self *StateDB) ClearRewardSetByAddress(addr common.Address) {
	delete(self.rewardSet, addr)
	self.rewardSetDirty = true
}

// Store the Reward Address Set

var rewardSetKey = []byte("RewardSet")

type RewardSet map[common.Address]struct{}

func (set RewardSet) EncodeRLP(w io.Writer) error {
	var list []common.Address
	for addr := range set {
		list = append(list, addr)
	}
	sort.Slice(list, func(i, j int) bool {
		return bytes.Compare(list[i].Bytes(), list[j].Bytes()) == 1
	})
	return rlp.Encode(w, list)
}

func (set *RewardSet) DecodeRLP(s *rlp.Stream) error {
	var list []common.Address
	if err := s.Decode(&list); err != nil {
		return err
	}
	rewardSet := make(RewardSet, len(list))
	for _, addr := range list {
		rewardSet[addr] = struct{}{}
	}
	*set = rewardSet
	return nil
}

// ----- Child Chain Reward Per Block

func (self *StateDB) SetChildChainRewardPerBlock(rewardPerBlock *big.Int) {
	self.childChainRewardPerBlock = rewardPerBlock
	self.childChainRewardPerBlockDirty = true
}

func (self *StateDB) GetChildChainRewardPerBlock() *big.Int {
	if self.childChainRewardPerBlock != nil {
		return self.childChainRewardPerBlock
	}
	// Try to get from Trie
	enc, err := self.trie.TryGet(childChainRewardPerBlockKey)
	if err != nil {
		self.setError(err)
		return nil
	}
	value := new(big.Int)
	if len(enc) > 0 {
		err := rlp.DecodeBytes(enc, value)
		if err != nil {
			self.setError(err)
		}
		self.childChainRewardPerBlock = value
	}
	return value
}

func (self *StateDB) commitChildChainRewardPerBlock() {
	data, err := rlp.EncodeToBytes(self.childChainRewardPerBlock)
	if err != nil {
		panic(fmt.Errorf("can't encode child chain reward per block : %v", err))
	}
	self.setError(self.trie.TryUpdate(childChainRewardPerBlockKey, data))
}

// Child Chain Reward Per Block
var childChainRewardPerBlockKey = []byte("RewardPerBlock")

// ----- Mark if one address proposed in one specific epoch
func (self *StateDB)MarkProposedInEpoch(address common.Address, epoch uint64) error {

	return self.db.TrieDB().MarkProposedInEpoch(address, epoch)
}

func (self *StateDB) CheckProposedInEpoch(address common.Address, epoch uint64) bool {

	return self.db.TrieDB().CheckProposedInEpoch(address, epoch)
}


func (self *StateDB) MarkProposalStartInEpoch(epoch uint64) error {
	return self.db.TrieDB().MarkProposalStartInEpoch(epoch)
}

func (self *StateDB) GetProposalStartInEpoch() (uint64, error) {
	return self.db.TrieDB().GetProposalStartInEpoch()
}
