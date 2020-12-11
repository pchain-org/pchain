package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
)

// ----- Type
type Reward map[uint64]*big.Int // key = Epoch Number, value = Reward Amount

func (p Reward) String() (str string) {
	for key, value := range p {
		str += fmt.Sprintf("Epoch %v : %v\n", key, value)
	}
	return
}

func (p Reward) Copy() Reward {
	cpy := make(Reward)
	for key, value := range p {
		cpy[key] = new(big.Int).Set(value)
	}
	return cpy
}

// ----- RewardBalance

// AddRewardBalance add amount to c's RewardBalance.
func (c *stateObject) AddRewardBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}
		return
	}
	c.SetRewardBalance(new(big.Int).Add(c.RewardBalance(), amount))
}

// SubRewardBalance removes amount from c's RewardBalance.
func (c *stateObject) SubRewardBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetRewardBalance(new(big.Int).Sub(c.RewardBalance(), amount))
}

func (self *stateObject) SetRewardBalance(amount *big.Int) {

	if amount.Sign() < 0 {
		log.Infof("!!!amount is negative, not support yet, make it 0 by force")
		amount = big.NewInt(0)
	}

	self.db.journal = append(self.db.journal, rewardBalanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.RewardBalance),
	})
	self.setRewardBalance(amount)
}

func (self *stateObject) setRewardBalance(amount *big.Int) {
	self.data.RewardBalance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) RewardBalance() *big.Int {
	return self.data.RewardBalance
}

// ----- Reward Trie

func (c *stateObject) getRewardTrie(db Database) Trie {
	if c.rewardTrie == nil {
		var err error
		c.rewardTrie, err = db.OpenRewardTrie(c.addrHash, c.data.RewardRoot)
		if err != nil {
			c.rewardTrie, _ = db.OpenRewardTrie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create reward trie: %v", err))
		}
	}
	return c.rewardTrie
}

// GetEpochRewardBalance returns a value in Reward trie
func (self *stateObject) GetEpochRewardBalance(db Database, key uint64) *big.Int {
	// If we have a dirty value for this state entry, return it
	value, dirty := self.dirtyReward[key]
	if dirty {
		return value
	}
	// If we have the original value cached, return that
	value, cached := self.originReward[key]
	if cached {
		return value
	}
	// Otherwise load the value from the database
	k, _ := rlp.EncodeToBytes(key)
	enc, err := self.getRewardTrie(db).TryGet(k)
	if err != nil {
		self.setError(err)
		return nil
	}
	if len(enc) > 0 {
		value = new(big.Int)
		err := rlp.DecodeBytes(enc, value)
		if err != nil {
			self.setError(err)
		}
	}
	self.originReward[key] = value
	return value
}

// SetEpochRewardBalance updates a value in Epoch Reward.
func (self *stateObject) SetEpochRewardBalance(db Database, key uint64, rewardAmount *big.Int) {
	self.db.journal = append(self.db.journal, epochRewardBalanceChange{
		account:  &self.address,
		key:      key,
		prevalue: self.GetEpochRewardBalance(db, key),
	})
	self.setEpochRewardBalance(key, rewardAmount)
}

func (self *stateObject) setEpochRewardBalance(key uint64, rewardAmount *big.Int) {
	self.dirtyReward[key] = rewardAmount

	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// updateRewardTrie writes cached reward modifications into the object's reward trie.
func (self *stateObject) updateRewardTrie(db Database) Trie {
	tr := self.getRewardTrie(db)
	for key, value := range self.dirtyReward {
		delete(self.dirtyReward, key)

		// Skip noop changes, persist actual changes
		if self.originReward[key] != nil && value.Cmp(self.originReward[key]) == 0 {
			continue
		}
		self.originReward[key] = value

		k, _ := rlp.EncodeToBytes(key)
		if value.Sign() == 0 {
			self.setError(tr.TryDelete(k))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(value)
		self.setError(tr.TryUpdate(k, v))
	}
	return tr
}

// updateRewardRoot sets the rewardTrie root to the current root hash of
func (self *stateObject) updateRewardRoot(db Database) {
	self.updateRewardTrie(db)
	self.data.RewardRoot = self.rewardTrie.Hash()
}

// CommitRewardTrie the reward trie of the object to dwb.
// This updates the reward trie root.
func (self *stateObject) CommitRewardTrie(db Database) error {
	self.updateRewardTrie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.rewardTrie.Commit(nil)
	if err == nil {
		self.data.RewardRoot = root
	}
	return err
}
