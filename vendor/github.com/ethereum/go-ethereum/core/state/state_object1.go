package state

import (
	"math/big"
	"github.com/ethereum/go-ethereum/common"
)

// AddLockedBalance add amount to the locked balance.
func (c *stateObject) AddLockedBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Cmp(common.Big0) == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}

	c.SetLockedBalance(new(big.Int).Add(c.LockedBalance(), amount))
}

// SubLockedBalance removes amount from c's locked balance.
func (c *stateObject) SubLockedBalance(amount *big.Int) {
	if amount.Cmp(common.Big0) == 0 {
		return
	}
	c.SetLockedBalance(new(big.Int).Sub(c.LockedBalance(), amount))
}

func (self *stateObject) SetLockedBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.LockedBalance),
	})
	self.setLockedBalance(amount)
}

func (self *stateObject) setLockedBalance(amount *big.Int) {
	self.data.LockedBalance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// AddChainBalance add amount to the locked balance.
func (c *stateObject) AddChainBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Cmp(common.Big0) == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}

	c.SetChainBalance(new(big.Int).Add(c.ChainBalance(), amount))
}

// SubChainBalance removes amount from c's chain balance.
func (c *stateObject) SubChainBalance(amount *big.Int) {
	if amount.Cmp(common.Big0) == 0 {
		return
	}
	c.SetChainBalance(new(big.Int).Sub(c.ChainBalance(), amount))
}

func (self *stateObject) SetChainBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.ChainBalance),
	})
	self.setChainBalance(amount)
}

func (self *stateObject) setChainBalance(amount *big.Int) {
	self.data.ChainBalance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}


func (self *stateObject) LockedBalance() *big.Int {
	return self.data.LockedBalance
}

func (self *stateObject) ChainBalance() *big.Int {
	return self.data.ChainBalance
}
