// Copyright 2017 The go-ethereum Authors
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
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
	"math/big"
)

const (
	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root common.Hash) (Trie, error)

	// OpenStorageTrie opens the storage trie of an account.
	OpenStorageTrie(addrHash, root common.Hash) (Trie, error)

	// OpenTX1Trie opens the tx1 trie of an account
	OpenTX1Trie(addrHash, root common.Hash) (Trie, error)

	// OpenTX3Trie opens the tx3 trie of an account
	OpenTX3Trie(addrHash, root common.Hash) (Trie, error)

	// OpenProxiedTrie opens the proxied trie of an account
	OpenProxiedTrie(addrHash, root common.Hash) (Trie, error)

	// OpenRewardTrie opens the reward trie of an account
	OpenRewardTrie(addrHash, root common.Hash) (Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(Trie) Trie

	// ContractCode retrieves a particular contract's code.
	ContractCode(addrHash, codeHash common.Hash) ([]byte, error)

	// ContractCodeSize retrieves a particular contracts code's size.
	ContractCodeSize(addrHash, codeHash common.Hash) (int, error)

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *trie.Database

	GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64) *big.Int
	GetAllEpochReward(address common.Address, height uint64) map[uint64]*big.Int
	UpdateOutsideRewardBalance(addr common.Address, reward trie.Reward)

	GetOOSLastBlock() (*big.Int, error)
	UpdateOOSLastBlock(oosBlock *big.Int)
	
	GetEpochRewardExtracted(address common.Address, height uint64) (uint64, error)
	UpdateEpochRewardExtracted(address common.Address, epoch uint64)
	
	FlushCache(blockNr uint64)
}

// Trie is a Ethereum Merkle Patricia trie.
type Trie interface {
	// GetKey returns the sha3 preimage of a hashed key that was previously used
	// to store a value.
	//
	// TODO(fjl): remove this when SecureTrie is removed
	GetKey([]byte) []byte

	// TryGet returns the value for key stored in the trie. The value bytes must
	// not be modified by the caller. If a node was not found in the database, a
	// trie.MissingNodeError is returned.
	TryGet(key []byte) ([]byte, error)

	// TryUpdate associates key with value in the trie. If value has length zero, any
	// existing value is deleted from the trie. The value bytes must not be modified
	// by the caller while they are stored in the trie. If a node was not found in the
	// database, a trie.MissingNodeError is returned.
	TryUpdate(key, value []byte) error

	// TryDelete removes any existing value for key from the trie. If a node was not
	// found in the database, a trie.MissingNodeError is returned.
	TryDelete(key []byte) error

	// Hash returns the root hash of the trie. It does not write to the database and
	// can be used even if the trie doesn't have one.
	Hash() common.Hash

	// Commit writes all nodes to the trie's memory database, tracking the internal
	// and external (for account tries) references.
	Commit(onleaf trie.LeafCallback) (common.Hash, error)

	// NodeIterator returns an iterator that returns nodes of the trie. Iteration
	// starts at the key after the given start key.
	NodeIterator(startKey []byte) trie.NodeIterator

	// Prove constructs a Merkle proof for key. The result contains all encoded nodes
	// on the path to the value at key. The value itself is also included in the last
	// node and can be retrieved by verifying the proof.
	//
	// If the trie does not contain a value for key, the returned proof contains all
	// nodes of the longest existing prefix of the key (at least the root), ending
	// with the node that proves the absence of the key.
	Prove(key []byte, fromLevel uint, proofDb ethdb.Writer) error

	GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64) *big.Int
	GetAllEpochReward(address common.Address, height uint64) map[uint64]*big.Int
	UpdateOutsideRewardBalance(addr common.Address, reward trie.Reward)
	
	GetOOSLastBlock() (*big.Int, error)
	UpdateOOSLastBlock(oosBlock *big.Int)
	
	GetEpochRewardExtracted(address common.Address, height uint64) (uint64, error)
	UpdateEpochRewardExtracted(address common.Address, epoch uint64)
	
	FlushCache(blockNr uint64)
}

// NewDatabase creates a backing store for state. The returned database is safe for
// concurrent use, but does not retain any recent trie nodes in memory. To keep some
// historical state in memory, use the NewDatabaseWithCache constructor.
func NewDatabase(db ethdb.Database) Database {
	return NewDatabaseWithCache(db, 0)
}

// NewDatabaseWithCache creates a backing store for state. The returned database
// is safe for concurrent use and retains a lot of collapsed RLP trie nodes in a
// large memory cache.
func NewDatabaseWithCache(db ethdb.Database, cache int) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:                            trie.NewDatabaseWithCache(db, cache),
		codeSizeCache:                 csc,
		rewardOutsideSet:              make(map[common.Address]trie.Reward),
		extractRewardSet:              make(map[common.Address]uint64),
		oosLastBlock:                  nil,
	}
}

type cachingDB struct {
	db                 *trie.Database
	codeSizeCache      *lru.Cache
	rewardOutsideSet   map[common.Address]trie.Reward
	extractRewardSet   map[common.Address]uint64
	oosLastBlock       *big.Int
}

// OpenTrie opens the main account trie.
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenStorageTrie opens the storage trie of an account.
func (db *cachingDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenTX1Trie opens the tx1 trie of an account
func (db *cachingDB) OpenTX1Trie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenTX3Trie opens the tx3 trie of an account
func (db *cachingDB) OpenTX3Trie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenProxiedTrie opens the proxied trie of an account
func (db *cachingDB) OpenProxiedTrie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenRewardTrie opens the reward trie of an account
func (db *cachingDB) OpenRewardTrie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// CopyTrie returns an independent copy of the given trie.
func (db *cachingDB) CopyTrie(t Trie) Trie {
	switch t := t.(type) {
	case *trie.SecureTrie:
		return t.Copy()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// ContractCode retrieves a particular contract's code.
func (db *cachingDB) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	code, err := db.db.Node(codeHash)
	if err == nil {
		db.codeSizeCache.Add(codeHash, len(code))
	}
	return code, err
}

// ContractCodeSize retrieves a particular contracts code's size.
func (db *cachingDB) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	if cached, ok := db.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := db.ContractCode(addrHash, codeHash)
	if err == nil {
		db.codeSizeCache.Add(codeHash, len(code))
	}
	return len(code), err
}

// TrieDB retrieves any intermediate trie-node caching layer.
func (db *cachingDB) TrieDB() *trie.Database {
	return db.db
}

//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (db *cachingDB) GetOutsideRewardBalanceByEpochNumber(addr common.Address, epochNo uint64, height uint64) *big.Int {
	if rewardset, exist := db.rewardOutsideSet[addr]; exist {
		if rewardbalance, rewardexist := rewardset[epochNo]; rewardexist {
			return rewardbalance
		}
	}

	rb := db.db.GetOutsideRewardBalanceByEpochNumber(addr, epochNo, height-1)
	if rewardset, exist := db.rewardOutsideSet[addr]; exist {
		rewardset[epochNo] = rb
	} else {
		epochReward := trie.Reward{epochNo: rb}
		db.rewardOutsideSet[addr] = epochReward
	}

	return rb
}

func (db *cachingDB) GetAllEpochReward(address common.Address, height uint64) map[uint64]*big.Int {
	
	//read value from lastest block
	rewardsFromDB := db.db.GetAllEpochReward(address, height-1)
	
	rewardsCache, exist := db.rewardOutsideSet[address]
	if !exist {
		db.rewardOutsideSet[address] = rewardsFromDB
	} else {
		//refresh result with lastest value
		for epoch, reward := range rewardsFromDB {
			if _, exist1 := rewardsCache[epoch]; !exist1 {
				rewardsCache[epoch] = reward
			}
		}
	}

	return db.rewardOutsideSet[address]
}

func (db *cachingDB) UpdateOutsideRewardBalance(addr common.Address, reward trie.Reward) {
	_, exist := db.rewardOutsideSet[addr]; 
	if !exist {
		db.rewardOutsideSet[addr] = reward
	} else {
		for epoch, oneReward := range reward {
			db.rewardOutsideSet[addr][epoch] = oneReward
		}
	}
}

func (db *cachingDB) GetOOSLastBlock() (*big.Int, error) {
	if db.oosLastBlock != nil {
		return db.oosLastBlock, nil	
	}
	
	oosBlock, err := db.db.ReadOOSLastBlock()
	if err != nil {
		return oosBlock, err
	}
	
	db.oosLastBlock = oosBlock
	return oosBlock, nil
}

func (db *cachingDB) UpdateOOSLastBlock(oosBlock *big.Int) {
	if db.oosLastBlock == nil || db.oosLastBlock.Cmp(oosBlock) < 0 {
		db.oosLastBlock = oosBlock
	}
}

//if there is a cache in memory, then return it; or return the lastest value from older block(height-1)
func (db *cachingDB) GetEpochRewardExtracted(address common.Address, height uint64) (uint64, error) {
	if epoch, exist := db.extractRewardSet[address]; exist {
		return epoch, nil
	}
	
	epoch, err := db.db.GetEpochRewardExtracted(address, height-1)
	if err != nil {
		return epoch, err
	}
	
	db.extractRewardSet[address] = epoch
	return epoch, nil
}

func (db *cachingDB) UpdateEpochRewardExtracted(address common.Address, epoch uint64) {
	db.extractRewardSet[address] = epoch
}

func (db *cachingDB) FlushCache(blockNr uint64) {
	for addr, reward := range db.rewardOutsideSet {
		for epoch, rewardAmount := range reward {
			db.db.WriteOutsideRewardBalanceByEpochNumber(addr, epoch, blockNr, rewardAmount)
		}
	}
	
	if db.oosLastBlock != nil {
		db.db.WriteOOSLastBlock(db.oosLastBlock)
	}

	for addr, epoch := range db.extractRewardSet {
		db.db.WriteEpochRewardExtracted(addr, epoch, blockNr)
	}
}
