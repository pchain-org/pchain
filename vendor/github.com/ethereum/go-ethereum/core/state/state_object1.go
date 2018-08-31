package state

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// childChainDepositBalance
type childChainDepositBalance struct {
	ChainId        string
	DepositBalance *big.Int
}

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

// AddChildChainDepositBalance add amount to the child chain deposit balance
func (c *stateObject) AddChildChainDepositBalance(chainId string, amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Cmp(common.Big0) == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}

	c.SetChildChainDepositBalance(chainId, new(big.Int).Add(c.ChildChainDepositBalance(chainId), amount))
}

// SubChildChainDepositBalance removes amount from c's child chain deposit balance
func (c *stateObject) SubChildChainDepositBalance(chainId string, amount *big.Int) {
	if amount.Cmp(common.Big0) == 0 {
		return
	}
	c.SetChildChainDepositBalance(chainId, new(big.Int).Sub(c.ChildChainDepositBalance(chainId), amount))
}

func (self *stateObject) SetChildChainDepositBalance(chainId string, amount *big.Int) {
	var index = -1
	for i := range self.data.ChildChainDepositBalance {
		if self.data.ChildChainDepositBalance[i].ChainId == chainId {
			index = i
			break
		}
	}
	if index < 0 { // not found, we'll append
		self.data.ChildChainDepositBalance = append(self.data.ChildChainDepositBalance, &childChainDepositBalance{
			ChainId:        chainId,
			DepositBalance: new(big.Int),
		})
		index = len(self.data.ChildChainDepositBalance) - 1
	}

	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.ChildChainDepositBalance[index].DepositBalance),
	})
	self.setChildChainDepositBalance(index, amount)
}

func (self *stateObject) setChildChainDepositBalance(index int, amount *big.Int) {
	self.data.ChildChainDepositBalance[index].DepositBalance = amount
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

func (self *stateObject) ChildChainDepositBalance(chainId string) *big.Int {
	for i := range self.data.ChildChainDepositBalance {
		if self.data.ChildChainDepositBalance[i].ChainId == chainId {
			return self.data.ChildChainDepositBalance[i].DepositBalance
		}
	}
	return common.Big0
}

func (self *stateObject) ChainBalance() *big.Int {
	return self.data.ChainBalance
}
