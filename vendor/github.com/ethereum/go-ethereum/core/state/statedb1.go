package state

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// Retrieve the deposit balance from the given address or 0 if object not found
func (self *StateDB) GetDepositBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.DepositBalance()
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

// AddDepositBalance adds amount to the deposit balance associated with addr
func (self *StateDB) AddDepositBalance(addr common.Address, amount *big.Int) {

	//        fmt.Printf("StateDB_AddLockedBalance : value to lock %d\n", amount)
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddDepositBalance(amount)
	}
}

// SubDepositBalance adds amount to the deposit balance associated with addr
func (self *StateDB) SubDepositBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubDepositBalance(amount)
	}
}

func (self *StateDB) SetDepositBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetDepositBalance(amount)
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
