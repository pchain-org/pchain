package state

import (
	"math/big"
	"github.com/ethereum/go-ethereum/common"
)

// Retrieve the locked balance from the given address or 0 if object not found
func (self *StateDB) GetLockedBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.LockedBalance()
	}
	return common.Big0
}

// Retrieve the chain balance from the given address or 0 if object not found
func (self *StateDB) GetChainBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.ChainBalance()
	}
	return common.Big0
}

// AddLockedBalance adds amount to the locked balance associated with addr
func (self *StateDB) AddLockedBalance(addr common.Address, amount *big.Int) {

	//        fmt.Printf("StateDB_AddLockedBalance : value to lock %d\n", amount)
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddLockedBalance(amount)
	}
}

// SubLockedBalance adds amount to the locked balance associated with addr
func (self *StateDB) SubLockedBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubLockedBalance(amount)
	}
}

func (self *StateDB) SetLockedBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetLockedBalance(amount)
	}
}

// AddChainBalance adds amount to the account associated with addr
func (self *StateDB) AddChainBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddChainBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr
func (self *StateDB) SubChainBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubChainBalance(amount)
	}
}

func (self *StateDB) SetChainBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetChainBalance(amount)
	}
}
