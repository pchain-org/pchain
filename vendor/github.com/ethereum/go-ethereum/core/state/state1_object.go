// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)


type state1Object struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account1
	db       *State1DB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	rewardTrie   Trie   // Reward Trie, store the pending reward balance for this account
	originReward Reward // cache data of Reward trie
	dirtyReward  Reward // dirty data of Reward trie, need to be flushed to disk later
	
	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	suicided  bool
	touched   bool
	deleted   bool
	onDirty   func(addr common.Address) // Callback method to mark a state object newly dirty
}

// empty returns whether the account is considered empty.
func (s *state1Object) empty() bool {
	return s.data.Nonce == 0 && s.data.RewardRoot == common.Hash{} 
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account1 struct {
	Nonce                    uint64

	// Reward
	RewardRoot    common.Hash // merkle root of the Reward trie

	// From which epoch number the rewards has been extracted
	ExtractNumber uint64
}

// newObject creates a state object.
func newState1Object(db *State1DB, address common.Address, data Account1, onDirty func(addr common.Address)) *state1Object {

	return &state1Object{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		onDirty:       onDirty,
		originReward:  make(Reward),
		dirtyReward:   make(Reward),
	}
}

// EncodeRLP implements rlp.Encoder.
func (c *state1Object) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *state1Object) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *state1Object) markSuicided() {
	self.suicided = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (c *state1Object) touch() {
	c.db.journal = append(c.db.journal, touchState1Change{
		account:   &c.address,
		prev:      c.touched,
		prevDirty: c.onDirty == nil,
	})
	if c.onDirty != nil {
		c.onDirty(c.Address())
		c.onDirty = nil
	}
	c.touched = true
}

// Returns the address of the contract/account
func (c *state1Object) Address() common.Address {
	return c.address
}

func (self *state1Object) SetNonce(nonce uint64) {
	self.db.journal = append(self.db.journal, nonceState1Change{
		account: &self.address,
		prev:    self.data.Nonce,
	})
	self.setNonce(nonce)
}

func (self *state1Object) setNonce(nonce uint64) {
	self.data.Nonce = nonce
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *state1Object) setRewardRoot(rewardRoot common.Hash) {
	self.data.RewardRoot = rewardRoot
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *state1Object) SetExtractNumber(extractNumber uint64) {
	self.db.journal = append(self.db.journal, extractNumberState1Change{
		account: &self.address,
		prev:    self.data.ExtractNumber,
	})
	self.setExtractNumber(extractNumber)
}

func (self *state1Object) setExtractNumber(extractNumber uint64) {
	self.data.ExtractNumber = extractNumber
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *state1Object) Nonce() uint64 {
	return self.data.Nonce
}

func (self *state1Object) RewardRoot() common.Hash {
	return self.data.RewardRoot
}

func (self *state1Object) ExtractNumber() uint64 {
	return self.data.ExtractNumber
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (self *state1Object) Value() *big.Int {
	panic("Value on stateObject should never be called")
}

func (self *state1Object) deepCopy(db *State1DB, onDirty func(addr common.Address)) *state1Object {
	stateObject := newState1Object(db, self.address, self.data, onDirty)
	if self.rewardTrie != nil {
		stateObject.rewardTrie = db.db.CopyTrie(self.rewardTrie)
	}
	stateObject.suicided = self.suicided
	stateObject.deleted = self.deleted
	stateObject.dirtyReward = self.dirtyReward.Copy()
	stateObject.originReward = self.originReward.Copy()
	return stateObject
}
