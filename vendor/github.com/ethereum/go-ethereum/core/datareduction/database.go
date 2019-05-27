package datareduction

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

// PruneDatabase wraps access to prune tries.
type PruneDatabase interface {
	// OpenPruneTrie opens the prune trie.
	OpenPruneTrie(root common.Hash) (state.Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(state.Trie) state.Trie

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *trie.Database
}

// NewDatabase creates a backing store for prune trie. The returned database is safe for
// concurrent use, but does not retain any recent trie nodes in memory. To keep some
// historical state in memory, use the NewDatabaseWithCache constructor.
func NewDatabase(db ethdb.Database) PruneDatabase {
	return NewDatabaseWithCache(db, 0)
}

// NewDatabaseWithCache creates a backing store for prune trie. The returned database
// is safe for concurrent use and retains a lot of collapsed RLP trie nodes in a
// large memory cache.
func NewDatabaseWithCache(db ethdb.Database, cache int) PruneDatabase {
	return &pruneDB{
		db: trie.NewDatabaseWithCache(db, cache),
	}
}



type pruneDB struct {
	db *trie.Database
}

// OpenTrie opens the Prune trie.
func (db *pruneDB) OpenPruneTrie(root common.Hash) (state.Trie, error) {
	return trie.NewSecure(root, db.db)
}

// CopyTrie returns an independent copy of the given trie.
func (db *pruneDB) CopyTrie(t state.Trie) state.Trie {
	switch t := t.(type) {
	case *trie.SecureTrie:
		return t.Copy()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// TrieDB retrieves any intermediate trie-node caching layer.
func (db *pruneDB) TrieDB() *trie.Database {
	return db.db
}
