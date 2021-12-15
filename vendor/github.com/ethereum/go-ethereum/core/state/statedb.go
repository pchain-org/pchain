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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	
	state1DB *State1DB
	
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// Cache of Delegate Refund Set
	delegateRefundSet      DelegateRefundSet
	delegateRefundSetDirty bool

	// Cache of Reward Set
	rewardSet      RewardSet
	rewardSetDirty bool

	// Cache of Child Chain Reward Per Block
	childChainRewardPerBlock      *big.Int
	childChainRewardPerBlockDirty bool

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
	journal        journal
	validRevisions []revision
	nextRevisionId int
}

// Create a new state from a given trie
func New___(root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	
	return &StateDB{
		db:                            db,
		trie:                          tr,
		stateObjects:                  make(map[common.Address]*stateObject),
		stateObjectsDirty:             make(map[common.Address]struct{}),
		delegateRefundSet:             make(DelegateRefundSet),
		delegateRefundSetDirty:        false,
		rewardSet:                     make(RewardSet),
		rewardSetDirty:                false,
		childChainRewardPerBlock:      nil,
		childChainRewardPerBlockDirty: false,
		logs:                          make(map[common.Hash][]*types.Log),
		preimages:                     make(map[common.Hash][]byte),
	}, nil
}

//parameter root is from header, parameter root1 is from block
func NewFromRoots(root, root1 common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	
	state := &StateDB{
		db:                            db,
		trie:                          tr,
		stateObjects:                  make(map[common.Address]*stateObject),
		stateObjectsDirty:             make(map[common.Address]struct{}),
		delegateRefundSet:             make(DelegateRefundSet),
		delegateRefundSetDirty:        false,
		rewardSet:                     make(RewardSet),
		rewardSetDirty:                false,
		childChainRewardPerBlock:      nil,
		childChainRewardPerBlockDirty: false,
		logs:                          make(map[common.Hash][]*types.Log),
		preimages:                     make(map[common.Hash][]byte),
	}

	state1, err1 := NewState1DB(root1, state)
	if  err1 != nil {
		return nil, err1
	}

	state.state1DB = state1
	state1.stateDB = state
	
	return state, nil
}

// setError remembers the first non-nil error it is called with.
func (self *StateDB) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateDB) Error() error {
	return self.dbErr
}

// Reset clears out all emphemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *StateDB) Reset(root common.Hash) error {
	tr, err := self.db.OpenTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.stateObjects = make(map[common.Address]*stateObject)
	self.stateObjectsDirty = make(map[common.Address]struct{})
	self.delegateRefundSet = make(DelegateRefundSet)
	self.rewardSet = make(RewardSet)
	self.childChainRewardPerBlock = nil
	self.thash = common.Hash{}
	self.bhash = common.Hash{}
	self.txIndex = 0
	self.logs = make(map[common.Hash][]*types.Log)
	self.logSize = 0
	self.preimages = make(map[common.Hash][]byte)
	self.clearJournalAndRefund()
	return nil
}

func (self *StateDB) AddLog(log *types.Log) {
	self.journal = append(self.journal, addLogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.BlockHash = self.bhash
	log.TxIndex = uint(self.txIndex)
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return self.logs[hash]
}

func (self *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (self *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := self.preimages[hash]; !ok {
		self.journal = append(self.journal, addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		self.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (self *StateDB) Preimages() map[common.Hash][]byte {
	return self.preimages
}

func (self *StateDB) AddRefund(gas uint64) {
	self.journal = append(self.journal, refundChange{prev: self.refund})
	self.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (self *StateDB) SubRefund(gas uint64) {
	self.journal = append(self.journal, refundChange{prev: self.refund})
	if gas > self.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, self.refund))
	}
	self.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (self *StateDB) Exist(addr common.Address) bool {
	return self.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (self *StateDB) Empty(addr common.Address) bool {
	so := self.getStateObject(addr)
	return so == nil || so.empty()
}

// Retrieve the balance from the given address or 0 if object not found
func (self *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

func (self *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

// TxIndex returns the current transaction index set by Prepare.
func (self *StateDB) TxIndex() int {
	return self.txIndex
}

// BlockHash returns the current block hash set by Prepare.
func (self *StateDB) BlockHash() common.Hash {
	return self.bhash
}

func (self *StateDB) GetCode(addr common.Address) []byte {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(self.db)
	}
	return nil
}

func (self *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := self.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		self.setError(err)
	}
	return size
}

func (self *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

func (self *StateDB) GetState(a common.Address, b common.Hash) common.Hash {
	stateObject := self.getStateObject(a)
	if stateObject != nil {
		return stateObject.GetState(self.db, b)
	}
	return common.Hash{}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(s.db, hash)
	}
	return common.Hash{}
}

func (self *StateDB) HasTX1(a common.Address, txHash common.Hash) bool {
	stateObject := self.getStateObject(a)
	if stateObject != nil {
		return stateObject.HasTX1(self.db, txHash)
	}
	return false
}

func (self *StateDB) HasTX3(a common.Address, txHash common.Hash) bool {
	stateObject := self.getStateObject(a)
	if stateObject != nil {
		return stateObject.HasTX3(self.db, txHash)
	}
	return false
}

// Database retrieves the low level database supporting the lower level trie ops.
func (self *StateDB) Database() Database {
	return self.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) StorageTrie(a common.Address) Trie {
	stateObject := self.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self, nil)
	return cpy.updateTrie(self.db)
}

// TX1Trie returns the TX1 trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) TX1Trie(a common.Address) Trie {
	stateObject := self.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self, nil)
	return cpy.updateTX1Trie(self.db)
}

// TX3Trie returns the TX3 trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) TX3Trie(a common.Address) Trie {
	stateObject := self.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self, nil)
	return cpy.updateTX3Trie(self.db)
}

func (self *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr
func (self *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr
func (self *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (self *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (self *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (self *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (self *StateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(self.db, key, value)
	}
}

func (self *StateDB) AddTX1(addr common.Address, txHash common.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddTX1(self.db, txHash)
	}
}

func (self *StateDB) AddTX3(addr common.Address, txHash common.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddTX3(self.db, txHash)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (self *StateDB) Suicide(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	self.journal = append(self.journal, suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (self *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (self *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.setError(self.trie.TryDelete(addr[:]))
}

// Retrieve a state object given my the address. Returns nil if not found.
func (self *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := self.stateObjects[addr]; obj != nil {
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
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(self, addr, data, self.MarkStateObjectDirty)
	self.setStateObject(obj)
	return obj
}

func (self *StateDB) setStateObject(object *stateObject) {
	self.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil
func (self *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := self.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = self.createObject(addr)
	}
	return stateObject
}

// MarkStateObjectDirty adds the specified object to the dirty map to avoid costly
// state object cache iteration to find a handful of modified ones.
func (self *StateDB) MarkStateObjectDirty(addr common.Address) {
	self.stateObjectsDirty[addr] = struct{}{}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = self.getStateObject(addr)
	newobj = newObject(self, addr, Account{}, self.MarkStateObjectDirty)
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		self.journal = append(self.journal, createObjectChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetObjectChange{prev: prev})
	}
	self.setStateObject(newobj)
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
func (self *StateDB) CreateAccount(addr common.Address) {
	new, prev := self.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

func (db *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
	so := db.getStateObject(addr)
	if so == nil {
		return nil
	}
	it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))

	for it.Next() {
		key := common.BytesToHash(db.trie.GetKey(it.Key))
		if value, dirty := so.dirtyStorage[key]; dirty {
			if !cb(key, value) {
				return nil
			}
			continue
		}

		if len(it.Value) > 0 {
			_, content, _, err := rlp.Split(it.Value)
			if err != nil {
				return err
			}
			if !cb(key, common.BytesToHash(content)) {
				return nil
			}
		}
	}
	return nil
}

func (db *StateDB) ForEachTX1(addr common.Address, cb func(tx1 common.Hash) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}

	it := trie.NewIterator(so.getTX1Trie(db.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToHash(db.trie.GetKey(it.Key)) // key is the tx1 hash
		if ret := cb(key); !ret {
			break
		}
	}
}

func (db *StateDB) ForEachTX3(addr common.Address, cb func(tx3 common.Hash) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}

	it := trie.NewIterator(so.getTX3Trie(db.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToHash(db.trie.GetKey(it.Key)) // key is the tx3 hash
		if ret := cb(key); !ret {
			break
		}
	}
}

func (db *StateDB) ForEachProxied(addr common.Address, cb func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}
	it := trie.NewIterator(so.getProxiedTrie(db.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToAddress(db.trie.GetKey(it.Key))
		if value, dirty := so.dirtyProxied[key]; dirty {
			cb(key, value.ProxiedBalance, value.DepositProxiedBalance, value.PendingRefundBalance)
			continue
		}
		var apb accountProxiedBalance
		rlp.DecodeBytes(it.Value, &apb)
		cb(key, apb.ProxiedBalance, apb.DepositProxiedBalance, apb.PendingRefundBalance)
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) Copy() *StateDB {
	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                            self.db,
		trie:                          self.db.CopyTrie(self.trie),
		stateObjects:                  make(map[common.Address]*stateObject, len(self.stateObjectsDirty)),
		stateObjectsDirty:             make(map[common.Address]struct{}, len(self.stateObjectsDirty)),
		delegateRefundSet:             make(DelegateRefundSet, len(self.delegateRefundSet)),
		delegateRefundSetDirty:        self.delegateRefundSetDirty,
		rewardSet:                     make(RewardSet, len(self.rewardSet)),
		rewardSetDirty:                self.rewardSetDirty,
		childChainRewardPerBlockDirty: self.childChainRewardPerBlockDirty,
		refund:                        self.refund,
		logs:                          make(map[common.Hash][]*types.Log, len(self.logs)),
		logSize:                       self.logSize,
		preimages:                     make(map[common.Hash][]byte, len(self.preimages)),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range self.stateObjectsDirty {
		state.stateObjects[addr] = self.stateObjects[addr].deepCopy(state, state.MarkStateObjectDirty)
		state.stateObjectsDirty[addr] = struct{}{}
	}
	for addr := range self.delegateRefundSet {
		state.delegateRefundSet[addr] = struct{}{}
	}
	for addr := range self.rewardSet {
		state.rewardSet[addr] = struct{}{}
	}
	if self.childChainRewardPerBlock != nil {
		state.childChainRewardPerBlock = new(big.Int).Set(self.childChainRewardPerBlock)
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
func (self *StateDB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, len(self.journal)})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *StateDB) RevertToSnapshot(revid int) {
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
func (self *StateDB) GetRefund() uint64 {
	return self.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]
		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			stateObject.updateTX1Root(s.db)
			stateObject.updateTX3Root(s.db)
			stateObject.updateProxiedRoot(s.db)
			stateObject.updateRewardRoot(s.db)
			s.updateStateObject(stateObject)
		}
	}

	// Update Delegate Refund Set if something changed
	if s.delegateRefundSetDirty {
		s.commitDelegateRefundSet()
	}

	// Update Reward Set if something changed
	if s.rewardSetDirty {
		s.commitRewardSet()
	}

	// Update Child Chain Reward per Block if something changed
	if s.childChainRewardPerBlockDirty {
		s.commitChildChainRewardPerBlock()
	}

	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
	
	//s.state1DB.Finalise(deleteEmptyObjects)
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (self *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	self.thash = thash
	self.bhash = bhash
	self.txIndex = ti
}

// DeleteSuicides flags the suicided objects for deletion so that it
// won't be referenced again when called / queried up on.
//
// DeleteSuicides should not be used for consensus related updates
// under any circumstances.
func (s *StateDB) DeleteSuicides() {
	// Reset refund so that any used-gas calculations can use this method.
	s.clearJournalAndRefund()

	for addr := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]

		// If the object has been removed by a suicide
		// flag the object as deleted.
		if stateObject.suicided {
			stateObject.deleted = true
		}
		delete(s.stateObjectsDirty, addr)
	}
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = nil
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Write any TX1 changes in the state object to its TX1 trie.
			if err := stateObject.CommitTX1Trie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Write any TX3 changes in the state object to its TX3 trie.
			if err := stateObject.CommitTX3Trie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Write any Proxied Delegate Balance changes in the state object to its proxied trie.
			if err := stateObject.CommitProxiedTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Write any Reward Balance changes in the state object to its reward trie.
			if err := stateObject.CommitRewardTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}

	// Commit Delegate Refund Set to the trie
	if s.delegateRefundSetDirty {
		s.commitDelegateRefundSet()
		s.delegateRefundSetDirty = false
	}

	// Commit Reward Set to the trie
	if s.rewardSetDirty {
		s.commitRewardSet()
		s.rewardSetDirty = false
	}

	// Commit Reward Per Block to the trie
	if s.childChainRewardPerBlockDirty {
		s.commitChildChainRewardPerBlock()
		s.childChainRewardPerBlockDirty = false
	}

	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyRoot {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		if account.TX1Root != emptyRoot {
			s.db.TrieDB().Reference(account.TX1Root, parent)
		}
		if account.TX3Root != emptyRoot {
			s.db.TrieDB().Reference(account.TX3Root, parent)
		}
		if account.ProxiedRoot != emptyRoot {
			s.db.TrieDB().Reference(account.ProxiedRoot, parent)
		}
		if account.RewardRoot != emptyRoot {
			s.db.TrieDB().Reference(account.RewardRoot, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	return root, err
}

func (s *StateDB) GetState1DB() *State1DB {
	return s.state1DB
}
