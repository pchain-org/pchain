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
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (self Code) String() string {
	return string(self) //strings.Join(Disassemble(self), " ")
}

type Storage map[common.Hash]common.Hash

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	originStorage Storage // Storage cache of original entries to dedup rewrites
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cross Chain TX trie
	tx1Trie Trie // tx1 trie, which become non-nil on first access
	tx3Trie Trie // tx3 trie, which become non-nil on first access

	dirtyTX1 map[common.Hash]struct{} // tx1 entries that need to be flushed to disk
	dirtyTX3 map[common.Hash]struct{} // tx3 entries that need to be flushed to disk

	// Delegate Trie
	proxiedTrie   Trie    // proxied trie, store the proxied balance from other user
	originProxied Proxied // cache data of proxied trie
	dirtyProxied  Proxied // dirty data of proxied trie, need to be flushed to disk later

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	touched   bool
	deleted   bool
	onDirty   func(addr common.Address) // Callback method to mark a state object newly dirty
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash) && s.data.DepositBalance.Sign() == 0 && len(s.data.ChildChainDepositBalance) == 0 && s.data.ChainBalance.Sign() == 0 && s.data.DelegateBalance.Sign() == 0 && s.data.ProxiedBalance.Sign() == 0 && s.data.DepositProxiedBalance.Sign() == 0 && s.data.PendingRefundBalance.Sign() == 0
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce                    uint64
	Balance                  *big.Int                    // for normal user
	DepositBalance           *big.Int                    // for validator, can not be consumed
	ChildChainDepositBalance []*childChainDepositBalance // only valid in main chain for child chain validator before child chain launch, can not be consumed
	ChainBalance             *big.Int                    // only valid in main chain for child chain owner, can not be consumed
	Root                     common.Hash                 // merkle root of the storage trie
	TX1Root                  common.Hash                 // merkle root of the TX1 trie
	TX3Root                  common.Hash                 // merkle root of the TX3 trie
	CodeHash                 []byte

	// Delegation
	DelegateBalance       *big.Int    // the accumulative balance which this account delegate the Balance to other user
	ProxiedBalance        *big.Int    // the accumulative balance which other user delegate to this account (this balance can be revoked, can be deposit for validator)
	DepositProxiedBalance *big.Int    // the deposit proxied balance for validator which come from ProxiedBalance (this balance can not be revoked)
	PendingRefundBalance  *big.Int    // the accumulative balance which other user try to cancel their delegate balance (this balance will be refund to user's address after epoch end)
	ProxiedRoot           common.Hash // merkle root of the Proxied trie
	// Candidate
	Candidate  bool  // flag for Account, true indicate the account has been applied for the Delegation Candidate
	Commission uint8 // commission percentage of Delegation Candidate (0-100)
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data Account, onDirty func(addr common.Address)) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.DepositBalance == nil {
		data.DepositBalance = new(big.Int)
	}
	if data.ChainBalance == nil {
		data.ChainBalance = new(big.Int)
	}
	// init delegate balance
	if data.DelegateBalance == nil {
		data.DelegateBalance = new(big.Int)
	}
	if data.ProxiedBalance == nil {
		data.ProxiedBalance = new(big.Int)
	}
	if data.DepositProxiedBalance == nil {
		data.DepositProxiedBalance = new(big.Int)
	}
	if data.PendingRefundBalance == nil {
		data.PendingRefundBalance = new(big.Int)
	}

	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
		dirtyTX1:      make(map[common.Hash]struct{}),
		dirtyTX3:      make(map[common.Hash]struct{}),
		originProxied: make(Proxied),
		dirtyProxied:  make(Proxied),
		onDirty:       onDirty,
	}
}

// EncodeRLP implements rlp.Encoder.
func (c *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *stateObject) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *stateObject) markSuicided() {
	self.suicided = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (c *stateObject) touch() {
	c.db.journal = append(c.db.journal, touchChange{
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

func (c *stateObject) getTX1Trie(db Database) Trie {
	if c.tx1Trie == nil {
		var err error
		c.tx1Trie, err = db.OpenTX1Trie(c.addrHash, c.data.TX1Root)
		if err != nil {
			c.tx1Trie, _ = db.OpenTX1Trie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create TX1 trie: %v", err))
		}
	}
	return c.tx1Trie
}

// HasTX1 returns true if tx1 is in account TX1 trie.
func (self *stateObject) HasTX1(db Database, txHash common.Hash) bool {
	// check the dirtyTX1 firstly.
	_, ok := self.dirtyTX1[txHash]
	if ok {
		return true
	}

	// Load from DB in case it is missing.
	enc, err := self.getTX1Trie(db).TryGet(txHash[:])
	if err != nil {
		return false
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
			return false
		}
		if !bytes.Equal(content, txHash[:]) {
			self.setError(fmt.Errorf("content mismatch the tx hash"))
			return false
		}

		return true
	}

	return false
}

// AddTX1 adds a tx1 in account tx1 trie.
func (self *stateObject) AddTX1(db Database, txHash common.Hash) {
	self.db.journal = append(self.db.journal, addTX1Change{
		account: &self.address,
		txHash:  txHash,
	})
	self.addTX1(txHash)
}

func (self *stateObject) addTX1(txHash common.Hash) {
	self.dirtyTX1[txHash] = struct{}{}
}

func (self *stateObject) removeTX1(txHash common.Hash) {
	delete(self.dirtyTX1, txHash)
}

// updateTX1Trie writes cached tx1 modifications into the object's tx1 trie.
func (self *stateObject) updateTX1Trie(db Database) Trie {
	tr := self.getTX1Trie(db)

	for tx1 := range self.dirtyTX1 {
		delete(self.dirtyTX1, tx1)

		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(tx1[:], "\x00"))
		self.setError(tr.TryUpdate(tx1[:], v))
	}
	return tr
}

// updateTX1Root sets the trie root to the current root hash
func (self *stateObject) updateTX1Root(db Database) {
	self.updateTX1Trie(db)
	self.data.TX1Root = self.tx1Trie.Hash()
}

// CommitTX1Trie the tx1 trie of the object to dwb.
// This updates the trie root.
func (self *stateObject) CommitTX1Trie(db Database) error {
	self.updateTX1Trie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.tx1Trie.Commit(nil)
	if err == nil {
		self.data.TX1Root = root
	}
	return err
}

func (c *stateObject) getTX3Trie(db Database) Trie {
	if c.tx3Trie == nil {
		var err error
		c.tx3Trie, err = db.OpenTX3Trie(c.addrHash, c.data.TX3Root)
		if err != nil {
			c.tx3Trie, _ = db.OpenTX3Trie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create TX3 trie: %v", err))
		}
	}
	return c.tx3Trie
}

// HasTX3 returns true if tx3 is in account TX3 trie.
func (self *stateObject) HasTX3(db Database, txHash common.Hash) bool {
	// check the dirtyTX3 firstly.
	_, ok := self.dirtyTX3[txHash]
	if ok {
		return true
	}

	// Load from DB in case it is missing.
	enc, err := self.getTX3Trie(db).TryGet(txHash[:])
	if err != nil {
		return false
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
			return false
		}
		if !bytes.Equal(content, txHash[:]) {
			self.setError(fmt.Errorf("content mismatch the tx hash"))
			return false
		}

		return true
	}

	return false
}

// AddTX3 adds a tx3 in account tx3 trie.
func (self *stateObject) AddTX3(db Database, txHash common.Hash) {
	self.db.journal = append(self.db.journal, addTX3Change{
		account: &self.address,
		txHash:  txHash,
	})
	self.addTX3(txHash)
}

func (self *stateObject) addTX3(txHash common.Hash) {
	self.dirtyTX3[txHash] = struct{}{}
}

func (self *stateObject) removeTX3(txHash common.Hash) {
	delete(self.dirtyTX3, txHash)
}

// updateTX3Trie writes cached tx3 modifications into the object's tx3 trie.
func (self *stateObject) updateTX3Trie(db Database) Trie {
	tr := self.getTX3Trie(db)

	for tx3 := range self.dirtyTX3 {
		delete(self.dirtyTX3, tx3)

		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(tx3[:], "\x00"))
		self.setError(tr.TryUpdate(tx3[:], v))
	}
	return tr
}

// updateTX3Root sets the trie root to the current root hash
func (self *stateObject) updateTX3Root(db Database) {
	self.updateTX3Trie(db)
	self.data.TX3Root = self.tx3Trie.Hash()
}

// CommitTX3Trie the tx3 trie of the object to dwb.
// This updates the trie root.
func (self *stateObject) CommitTX3Trie(db Database) error {
	self.updateTX3Trie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.tx3Trie.Commit(nil)
	if err == nil {
		self.data.TX3Root = root
	}
	return err
}

func (c *stateObject) getTrie(db Database) Trie {
	if c.trie == nil {
		var err error
		c.trie, err = db.OpenStorageTrie(c.addrHash, c.data.Root)
		if err != nil {
			c.trie, _ = db.OpenStorageTrie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return c.trie
}

// GetState returns a value in account storage.
func (self *stateObject) GetState(db Database, key common.Hash) common.Hash {
	// If we have the original value cached, return that
	value, cached := self.originStorage[key]
	if cached {
		return value
	}
	// Otherwise load the value from the database
	enc, err := self.getTrie(db).TryGet(key[:])
	if err != nil {
		self.setError(err)
		return common.Hash{}
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
		}
		value.SetBytes(content)
	}
	self.originStorage[key] = value
	return value
}

// SetState updates a value in account storage.
func (self *stateObject) SetState(db Database, key, value common.Hash) {
	self.db.journal = append(self.db.journal, storageChange{
		account:  &self.address,
		key:      key,
		prevalue: self.GetState(db, key),
	})
	self.setState(key, value)
}

func (self *stateObject) setState(key, value common.Hash) {
	self.dirtyStorage[key] = value

	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (self *stateObject) updateTrie(db Database) Trie {
	tr := self.getTrie(db)
	for key, value := range self.dirtyStorage {
		delete(self.dirtyStorage, key)

		// Skip noop changes, persist actual changes
		if value == self.originStorage[key] {
			continue
		}
		self.originStorage[key] = value

		if (value == common.Hash{}) {
			self.setError(tr.TryDelete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		self.setError(tr.TryUpdate(key[:], v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (self *stateObject) updateRoot(db Database) {
	self.updateTrie(db)
	self.data.Root = self.trie.Hash()
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *stateObject) CommitTrie(db Database) error {
	self.updateTrie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.trie.Commit(nil)
	if err == nil {
		self.data.Root = root
	}
	return err
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (c *stateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (c *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))
}

func (self *stateObject) SetBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.Balance),
	})
	self.setBalance(amount)
}

func (self *stateObject) setBalance(amount *big.Int) {
	self.data.Balance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (c *stateObject) ReturnGas(gas *big.Int) {}

func (self *stateObject) deepCopy(db *StateDB, onDirty func(addr common.Address)) *stateObject {
	stateObject := newObject(db, self.address, self.data, onDirty)
	if self.trie != nil {
		stateObject.trie = db.db.CopyTrie(self.trie)
	}
	if self.tx1Trie != nil {
		stateObject.tx1Trie = db.db.CopyTrie(self.tx1Trie)
	}
	if self.tx3Trie != nil {
		stateObject.tx3Trie = db.db.CopyTrie(self.tx3Trie)
	}
	if self.proxiedTrie != nil {
		stateObject.proxiedTrie = db.db.CopyTrie(self.proxiedTrie)
	}
	stateObject.code = self.code
	stateObject.dirtyStorage = self.dirtyStorage.Copy()
	stateObject.originStorage = self.originStorage.Copy()
	stateObject.suicided = self.suicided
	stateObject.dirtyCode = self.dirtyCode
	stateObject.deleted = self.deleted
	stateObject.dirtyTX1 = make(map[common.Hash]struct{})
	for tx1 := range self.dirtyTX1 {
		stateObject.dirtyTX1[tx1] = struct{}{}
	}
	stateObject.dirtyTX3 = make(map[common.Hash]struct{})
	for tx3 := range self.dirtyTX3 {
		stateObject.dirtyTX3[tx3] = struct{}{}
	}
	stateObject.dirtyProxied = self.dirtyProxied.Copy()
	stateObject.originProxied = self.originProxied.Copy()
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (c *stateObject) Address() common.Address {
	return c.address
}

// Code returns the contract code associated with this object, if any.
func (self *stateObject) Code(db Database) []byte {
	if self.code != nil {
		return self.code
	}
	if bytes.Equal(self.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(self.addrHash, common.BytesToHash(self.CodeHash()))
	if err != nil {
		self.setError(fmt.Errorf("can't load code hash %x: %v", self.CodeHash(), err))
	}
	self.code = code
	return code
}

func (self *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := self.Code(self.db.db)
	self.db.journal = append(self.db.journal, codeChange{
		account:  &self.address,
		prevhash: self.CodeHash(),
		prevcode: prevcode,
	})
	self.setCode(codeHash, code)
}

func (self *stateObject) setCode(codeHash common.Hash, code []byte) {
	self.code = code
	self.data.CodeHash = codeHash[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) SetNonce(nonce uint64) {
	self.db.journal = append(self.db.journal, nonceChange{
		account: &self.address,
		prev:    self.data.Nonce,
	})
	self.setNonce(nonce)
}

func (self *stateObject) setNonce(nonce uint64) {
	self.data.Nonce = nonce
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) CodeHash() []byte {
	return self.data.CodeHash
}

func (self *stateObject) Balance() *big.Int {
	return self.data.Balance
}

func (self *stateObject) Nonce() uint64 {
	return self.data.Nonce
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (self *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
