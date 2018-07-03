package trial_backend

import (
	"sync"
	"github.com/hashicorp/golang-lru"
	"github.com/ethereum/go-ethereum/event"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/consensus/tendermint"
)


// New creates an Ethereum backend for Istanbul core engine.
func New(config *tendermint.Config, privateKey *ecdsa.PrivateKey, db ethdb.Database) consensus.Tendermint {
	// Allocate the snapshot caches and create the engine
	//recents, _ := lru.NewARC(inmemorySnapshots)
	//recentMessages, _ := lru.NewARC(inmemoryPeers)
	//knownMessages, _ := lru.NewARC(inmemoryMessages)
	backend := &backend{
		//config:           config,
		//istanbulEventMux: new(event.TypeMux),
		privateKey:       privateKey,
		//address:          crypto.PubkeyToAddress(privateKey.PublicKey),
		//logger:           log.New(),
		db:               db,
		commitCh:         make(chan *types.Block, 1),
		//recents:          recents,
		candidates:       make(map[common.Address]bool),
		coreStarted:      false,
		//recentMessages:   recentMessages,
		//knownMessages:    knownMessages,
	}
	//backend.core = istanbulCore.New(backend, backend.config)
	return backend
}

type backend struct {
	//config           *istanbul.Config
	istanbulEventMux *event.TypeMux
	privateKey       *ecdsa.PrivateKey
	address          common.Address
	//core             istanbulCore.Engine
	//logger           log.Logger
	db               ethdb.Database
	chain            consensus.ChainReader
	currentBlock     func() *types.Block
	hasBadBlock      func(hash common.Hash) bool

	// the channels for istanbul engine notifications
	commitCh          chan *types.Block
	proposedBlockHash common.Hash
	sealMu            sync.Mutex
	coreStarted       bool
	coreMu            sync.RWMutex

	// Current list of candidates we are pushing
	candidates map[common.Address]bool
	// Protects the signer fields
	candidatesLock sync.RWMutex
	// Snapshots for recent block to speed up reorgs
	recents *lru.ARCCache

	// event subscription for ChainHeadEvent event
	broadcaster consensus.Broadcaster

	recentMessages *lru.ARCCache // the cache of peer's messages
	knownMessages  *lru.ARCCache // the cache of self messages
}

