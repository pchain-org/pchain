package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
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

// ----- PendingRefundBalance (Total)

// GetTotalPendingRefundBalance Retrieve the pending refund balance from the given address or 0 if object not found
func (self *StateDB) GetTotalPendingRefundBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.PendingRefundBalance()
	}
	return common.Big0
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

// SubProxiedBalanceByUser subtracts proxied amount from the account associated with addr
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

// GetDepositProxiedBalanceByUser
func (self *StateDB) GetDepositProxiedBalanceByUser(addr, user common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		if apb == nil {
			return common.Big0
		} else {
			return apb.DepositProxiedBalance
		}
	}
	return common.Big0
}

// AddDepositProxiedBalanceByUser adds proxied amount to the account associated with addr
func (self *StateDB) AddDepositProxiedBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update DepositProxiedBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.DepositProxiedBalance = new(big.Int).Add(dirtyApb.DepositProxiedBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Add amount to Total Proxied Balance
		stateObject.AddDepositProxiedBalance(amount)
	}
}

// SubDepositProxiedBalanceByUser subtracts proxied amount from the account associated with addr
func (self *StateDB) SubDepositProxiedBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update DepositProxiedBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.DepositProxiedBalance = new(big.Int).Sub(dirtyApb.DepositProxiedBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Sub amount from Total Proxied Balance
		stateObject.SubDepositProxiedBalance(amount)
	}
}

// GetPendingRefundBalanceByUser
func (self *StateDB) GetPendingRefundBalanceByUser(addr, user common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		if apb == nil {
			return common.Big0
		} else {
			return apb.PendingRefundBalance
		}
	}
	return common.Big0
}

// AddPendingRefundBalanceByUser adds pending refund amount to the account associated with addr
func (self *StateDB) AddPendingRefundBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update PendingRefundBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.PendingRefundBalance = new(big.Int).Add(dirtyApb.PendingRefundBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Add amount to Total Proxied Balance
		stateObject.AddPendingRefundBalance(amount)
	}
}

// SubPendingRefundBalanceByUser subtracts pending refund amount from the account associated with addr
func (self *StateDB) SubPendingRefundBalanceByUser(addr, user common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		// Get AccountProxiedBalance and update PendingRefundBalance
		apb := stateObject.GetAccountProxiedBalance(self.db, user)
		var dirtyApb *accountProxiedBalance
		if apb == nil {
			dirtyApb = NewAccountProxiedBalance()
		} else {
			dirtyApb = apb.Copy()
		}
		dirtyApb.PendingRefundBalance = new(big.Int).Sub(dirtyApb.PendingRefundBalance, amount)
		stateObject.SetAccountProxiedBalance(self.db, user, dirtyApb)

		// Sub amount from Total Proxied Balance
		stateObject.SubPendingRefundBalance(amount)
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

func (self *StateDB) IsCleanAddress(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return !stateObject.IsCandidate() && stateObject.IsEmptyTrie()
	}
	return true
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
func (self *StateDB) CancelCandidate(addr common.Address, allRefund bool) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCandidate(false)
		if allRefund {
			stateObject.SetCommission(0)
		}
	}
}

// ClearCommission Set the Candidate commission to 0
func (self *StateDB) ClearCommission(addr common.Address) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCommission(0)
	}
}

// ----- Refund Set

// MarkDelegateAddressRefund adds the specified object to the dirty map to avoid
func (self *StateDB) MarkDelegateAddressRefund(addr common.Address) {
	self.delegateRefundSet[addr] = struct{}{}
	self.delegateRefundSetDirty = true
}

func (self *StateDB) GetDelegateAddressRefundSet() DelegateRefundSet {
	if len(self.delegateRefundSet) != 0 {
		return self.delegateRefundSet
	}
	// Try to get from Trie
	enc, err := self.trie.TryGet(refundSetKey)
	if err != nil {
		self.setError(err)
		return nil
	}
	var value DelegateRefundSet
	if len(enc) > 0 {
		err := rlp.DecodeBytes(enc, &value)
		if err != nil {
			self.setError(err)
		}
	}
	self.delegateRefundSet = value
	return value
}

func (self *StateDB) commitDelegateRefundSet() {
	data, err := rlp.EncodeToBytes(self.delegateRefundSet)
	if err != nil {
		panic(fmt.Errorf("can't encode delegate refund set : %v", err))
	}
	self.setError(self.trie.TryUpdate(refundSetKey, data))
}

func (self *StateDB) ClearDelegateRefundSet() {
	self.setError(self.trie.TryDelete(refundSetKey))
	self.delegateRefundSet = make(DelegateRefundSet)
	self.delegateRefundSetDirty = false
}

// Store the Delegate Refund Set

var refundSetKey = []byte("DelegateRefundSet")

type DelegateRefundSet map[common.Address]struct{}

func (set DelegateRefundSet) EncodeRLP(w io.Writer) error {
	var list []common.Address
	for addr := range set {
		list = append(list, addr)
	}
	return rlp.Encode(w, list)
}

func (set *DelegateRefundSet) DecodeRLP(s *rlp.Stream) error {
	var list []common.Address
	if err := s.Decode(&list); err != nil {
		return err
	}
	refundSet := make(DelegateRefundSet, len(list))
	for _, addr := range list {
		refundSet[addr] = struct{}{}
	}
	*set = refundSet
	return nil
}
