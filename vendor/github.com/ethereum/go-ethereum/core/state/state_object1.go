package state

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// AddDepositBalance add amount to the deposit balance.
func (c *stateObject) AddDepositBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Cmp(common.Big0) == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}

	c.SetDepositBalance(new(big.Int).Add(c.DepositBalance(), amount))
}

// SubDepositBalance removes amount from c's deposit balance.
func (c *stateObject) SubDepositBalance(amount *big.Int) {
	if amount.Cmp(common.Big0) == 0 {
		return
	}
	c.SetDepositBalance(new(big.Int).Sub(c.DepositBalance(), amount))
}

func (self *stateObject) SetDepositBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.DepositBalance),
	})
	self.setDepositBalance(amount)
}

func (self *stateObject) setDepositBalance(amount *big.Int) {
	self.data.DepositBalance = amount
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

func (self *stateObject) DepositBalance() *big.Int {
	return self.data.DepositBalance
}

func (self *stateObject) ChainBalance() *big.Int {
	return self.data.ChainBalance
}
