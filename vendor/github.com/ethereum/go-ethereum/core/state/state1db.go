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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type State1DB struct {

	stateDB *StateDB

	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	state1Objects      map[common.Address]*state1Object
	state1ObjectsDirty map[common.Address]struct{}

	// Cache of Reward Set
	//rewardSet          RewardSet
	//rewardSetDirty     bool
	
	//rewardOutsideSet map[common.Address]Reward //cache rewards of candidate&delegators for recording in diskdb
	//extractRewardSet map[common.Address]uint64 //cache rewards of different epochs when delegator does extract

	//if there is rollback, the rewards stored in diskdb for 'out-of-storage' feature should not be added again
	//remember the last block consistent with out-of-storage recording
	//oosLastBlock  *big.Int

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        journal1
	validRevisions []revision
	nextRevisionId int
}

// Create a new state from a given trie
func NewState1DB(root common.Hash, state *StateDB) (*State1DB, error) {

	db := state.Database()
	
	state1 := &State1DB{
		db:                            db,
		//trie:                          tr,
		state1Objects:                 make(map[common.Address]*state1Object),
		state1ObjectsDirty:            make(map[common.Address]struct{}),
		logs:                          make(map[common.Hash][]*types.Log),
		preimages:                     make(map[common.Hash][]byte),
	}
	
	if len(root) != 0 {
		tr, err := db.OpenTrie(root)
		if err != nil {
			return nil, err
		}
		state1.trie = tr
	}

	return state1, nil
}

// setError remembers the first non-nil error it is called with.
func (self *State1DB) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *State1DB) Error() error {
	return self.dbErr
}

// Reset clears out all emphemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *State1DB) Reset(root common.Hash) error {
	tr, err := self.db.OpenTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.state1Objects = make(map[common.Address]*state1Object)
	self.state1ObjectsDirty = make(map[common.Address]struct{})
	self.thash = common.Hash{}
	self.bhash = common.Hash{}
	self.txIndex = 0
	self.logs = make(map[common.Hash][]*types.Log)
	self.logSize = 0
	self.preimages = make(map[common.Hash][]byte)
	self.clearJournalAndRefund()
	return nil
}

func (self *State1DB) AddLog(log *types.Log) {
	self.journal = append(self.journal, addState1LogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.BlockHash = self.bhash
	log.TxIndex = uint(self.txIndex)
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *State1DB) GetLogs(hash common.Hash) []*types.Log {
	return self.logs[hash]
}

func (self *State1DB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (self *State1DB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := self.preimages[hash]; !ok {
		self.journal = append(self.journal, addState1PreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		self.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (self *State1DB) Preimages() map[common.Hash][]byte {
	return self.preimages
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (self *State1DB) Exist(addr common.Address) bool {
	return self.getState1Object(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (self *State1DB) Empty(addr common.Address) bool {
	so := self.getState1Object(addr)
	return so == nil || so.empty()
}

// TxIndex returns the current transaction index set by Prepare.
func (self *State1DB) TxIndex() int {
	return self.txIndex
}

// BlockHash returns the current block hash set by Prepare.
func (self *State1DB) BlockHash() common.Hash {
	return self.bhash
}

// Database retrieves the low level database supporting the lower level trie ops.
func (self *State1DB) Database() Database {
	return self.db
}


func (self *State1DB) HasSuicided(addr common.Address) bool {
	state1Object := self.getState1Object(addr)
	if state1Object != nil {
		return state1Object.suicided
	}
	return false
}

func (self *State1DB) SetEpochRewardExtracted(addr common.Address, extractNumber uint64) {
	stateObject := self.GetOrNewState1Object(addr)
	if stateObject != nil {
		stateObject.SetExtractNumber(extractNumber)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (self *State1DB) Suicide(addr common.Address) bool {
	stateObject := self.getState1Object(addr)
	if stateObject == nil {
		return false
	}
	self.journal = append(self.journal, suicideState1ObjectChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevEpochReward:   stateObject.EpochReward(),
		prevExtractNumber: stateObject.ExtractNumber(),
	})
	stateObject.markSuicided()
	stateObject.data.EpochReward = make(map[uint64]*big.Int)
	stateObject.data.ExtractNumber = 0
	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (self *State1DB) updateState1Object(stateObject *state1Object) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (self *State1DB) deleteState1Object(stateObject *state1Object) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.setError(self.trie.TryDelete(addr[:]))
}

// Retrieve a state object given my the address. Returns nil if not found.
func (self *State1DB) getState1Object(addr common.Address) (stateObject *state1Object) {
	// Prefer 'live' objects.
	if obj := self.state1Objects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := self.trie.TryGet(addr[:])
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var data Account1
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newState1Object(self, addr, data, self.MarkState1ObjectDirty)
	self.setState1Object(obj)
	return obj
}

func (self *State1DB) setState1Object(object *state1Object) {
	self.state1Objects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil
func (self *State1DB) GetOrNewState1Object(addr common.Address) *state1Object {
	state1Object := self.getState1Object(addr)
	if state1Object == nil || state1Object.deleted {
		state1Object, _ = self.createState1Object(addr)
	}
	return state1Object
}

// MarkStateObjectDirty adds the specified object to the dirty map to avoid costly
// state object cache iteration to find a handful of modified ones.
func (self *State1DB) MarkState1ObjectDirty(addr common.Address) {
	self.state1ObjectsDirty[addr] = struct{}{}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *State1DB) createState1Object(addr common.Address) (newobj, prev *state1Object) {
	prev = self.getState1Object(addr)
	newobj = newState1Object(self, addr, Account1{}, self.MarkState1ObjectDirty)
	if prev == nil {
		self.journal = append(self.journal, createState1ObjectChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetState1ObjectChange{prev: prev})
	}
	self.setState1Object(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (self *State1DB) CreateAccount1(addr common.Address) {
	new, prev := self.createState1Object(addr)
	if prev != nil {
		new.setEpochReward(prev.EpochReward())
		new.setExtractNumber(prev.ExtractNumber())
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *State1DB) Copy() *State1DB {
	// Copy all the basic fields, initialize the memory ones
	state := &State1DB{
		db:                            self.db,
		trie:                          self.db.CopyTrie(self.trie),
		state1Objects:                 make(map[common.Address]*state1Object, len(self.state1ObjectsDirty)),
		state1ObjectsDirty:            make(map[common.Address]struct{}, len(self.state1ObjectsDirty)),
		refund:                        self.refund,
		logs:                          make(map[common.Hash][]*types.Log, len(self.logs)),
		logSize:                       self.logSize,
		preimages:                     make(map[common.Hash][]byte, len(self.preimages)),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range self.state1ObjectsDirty {
		state.state1Objects[addr] = self.state1Objects[addr].deepCopy(state, state.MarkState1ObjectDirty)
		state.state1ObjectsDirty[addr] = struct{}{}
	}
	for hash, logs := range self.logs {
		state.logs[hash] = make([]*types.Log, len(logs))
		copy(state.logs[hash], logs)
	}
	for hash, preimage := range self.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (self *State1DB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, len(self.journal)})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *State1DB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(self.validRevisions), func(i int) bool {
		return self.validRevisions[i].id >= revid
	})
	if idx == len(self.validRevisions) || self.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := self.validRevisions[idx].journalIndex

	// Replay the journal to undo changes.
	for i := len(self.journal) - 1; i >= snapshot; i-- {
		self.journal[i].undo(self)
	}
	self.journal = self.journal[:snapshot]

	// Remove invalidated snapshots from the stack.
	self.validRevisions = self.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (self *State1DB) GetRefund() uint64 {
	return self.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *State1DB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.state1ObjectsDirty {
		stateObject := s.state1Objects[addr]
		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteState1Object(stateObject)
		} else {
			s.updateState1Object(stateObject)
		}
	}

	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *State1DB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (self *State1DB) Prepare(thash, bhash common.Hash, ti int) {
	self.thash = thash
	self.bhash = bhash
	self.txIndex = ti
}

// DeleteSuicides flags the suicided objects for deletion so that it
// won't be referenced again when called / queried up on.
//
// DeleteSuicides should not be used for consensus related updates
// under any circumstances.
func (s *State1DB) DeleteSuicides() {
	// Reset refund so that any used-gas calculations can use this method.
	s.clearJournalAndRefund()

	for addr := range s.state1ObjectsDirty {
		stateObject := s.state1Objects[addr]

		// If the object has been removed by a suicide
		// flag the object as deleted.
		if stateObject.suicided {
			stateObject.deleted = true
		}
		delete(s.state1ObjectsDirty, addr)
	}
}

func (s *State1DB) clearJournalAndRefund() {
	s.journal = nil
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *State1DB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	// Commit objects to the trie.
	for addr, stateObject := range s.state1Objects {
		_, isDirty := s.state1ObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteState1Object(stateObject)
		case isDirty:
			s.updateState1Object(stateObject)
		}
		delete(s.state1ObjectsDirty, addr)
	}

	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account1
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		return nil
	})
	return root, err
}

