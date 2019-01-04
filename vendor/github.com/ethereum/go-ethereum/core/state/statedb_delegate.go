package state

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// ----- DelegateBalance

// GetDelegateBalance Retrieve the delegate balance from the given address or 0 if object not found
func (self *StateDB) GetDelegateBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.DelegateBalance()
	}
	return common.Big0
}

// AddDelegateBalance adds delegate amount to the account associated with addr
func (self *StateDB) AddDelegateBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddDelegateBalance(amount)
	}
}

// SubDelegateBalance subtracts delegate amount from the account associated with addr
func (self *StateDB) SubDelegateBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubDelegateBalance(amount)
	}
}

// ----- ProxiedBalance (Total)

// GetTotalProxiedBalance Retrieve the proxied balance from the given address or 0 if object not found
func (self *StateDB) GetTotalProxiedBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.ProxiedBalance()
	}
	return common.Big0
}

// ----- DepositProxiedBalance (Total)

// GetTotalDepositProxiedBalance Retrieve the deposit proxied balance from the given address or 0 if object not found
func (self *StateDB) GetTotalDepositProxiedBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.DepositProxiedBalance()
	}
	return common.Big0
}

// To be removed
// AddDepositProxiedBalance adds deposit proxied amount to the account associated with addr
func (self *StateDB) AddTotalDepositProxiedBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddDepositProxiedBalance(amount)
	}
}

// To be removed
// SubDepositProxiedBalance subtracts deposit proxied amount from the account associated with addr
func (self *StateDB) SubTotalDepositProxiedBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubDepositProxiedBalance(amount)
	}
}

// ----- Proxied Trie

// GetProxiedBalanceByUser
func (self *StateDB) GetProxiedBalanceByUser(addr, user common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		if apb == nil {
			return common.Big0
		} else {
			return apb.ProxiedBalance
		}
	}
	return common.Big0
}

// AddProxiedBalanceByUser adds proxied amount to the account associated with addr
func (self *StateDB) AddProxiedBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update ProxiedBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.ProxiedBalance = new(big.Int).Add(dirtyApb.ProxiedBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Add amount to Total Proxied Balance
		stateObject.AddProxiedBalance(amount)
	}
}

// SubProxiedBalance subtracts proxied amount from the account associated with addr
func (self *StateDB) SubProxiedBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update ProxiedBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.ProxiedBalance = new(big.Int).Sub(dirtyApb.ProxiedBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Sub amount from Total Proxied Balance
		stateObject.SubProxiedBalance(amount)
	}
}

// ----- Candidate

// IsCandidate Retrieve the candidate flag of the given address or false if object not found
func (self *StateDB) IsCandidate(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.IsCandidate()
	}
	return false
}

// GetCommission Retrieve the commission percentage of the given address or 0 if object not found
func (self *StateDB) GetCommission(addr common.Address) uint8 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Commission()
	}
	return 0
}

// ApplyForCandidate Set the Candidate Flag of the given address to true and commission to given value
func (self *StateDB) ApplyForCandidate(addr common.Address, commission uint8) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCandidate(true)
		stateObject.SetCommission(commission)
	}
}

// CancelCandidate Set the Candidate Flag of the given address to false and commission to 0
func (self *StateDB) CancelCandidate(addr common.Address) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCandidate(false)
		stateObject.SetCommission(0)
	}
}
