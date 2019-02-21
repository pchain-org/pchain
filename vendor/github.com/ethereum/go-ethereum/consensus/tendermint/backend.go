package tendermint

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"gopkg.in/urfave/cli.v1"
	"sync"
)

// New creates an Ethereum backend for Tendermint core engine.
func New(chainConfig *params.ChainConfig, cliCtx *cli.Context,
	privateKey *ecdsa.PrivateKey, db ethdb.Database,
	cch core.CrossChainHelper) consensus.Tendermint {
	// Allocate the snapshot caches and create the engine
	//recents, _ := lru.NewARC(inmemorySnapshots)
	//recentMessages, _ := lru.NewARC(inmemoryPeers)
	//knownMessages, _ := lru.NewARC(inmemoryMessages)

	config := GetTendermintConfig(chainConfig.PChainId, cliCtx)

	backend := &backend{
		//config:           config,
		chainConfig:        chainConfig,
		tendermintEventMux: new(event.TypeMux),
		privateKey:         privateKey,
		//address:          crypto.PubkeyToAddress(privateKey.PublicKey),
		//core:             node,
		logger:    chainConfig.ChainLogger,
		db:        db,
		commitCh:  make(chan *ethTypes.Block, 1),
		vcommitCh: make(chan *types.IntermediateBlockResult, 1),
		//recents:          recents,
		//candidates:  make(map[common.Address]bool),
		coreStarted: false,
		//recentMessages:   recentMessages,
		//knownMessages:    knownMessages,
	}
	backend.core = MakeTendermintNode(backend, config, chainConfig, cch)
	return backend
}

type backend struct {
	//config           *istanbul.Config
	chainConfig        *params.ChainConfig
	tendermintEventMux *event.TypeMux
	privateKey         *ecdsa.PrivateKey
	address            common.Address
	core               *Node
	logger             log.Logger
	db                 ethdb.Database
	chain              consensus.ChainReader
	currentBlock       func() *ethTypes.Block
	hasBadBlock        func(hash common.Hash) bool

	// the channels for istanbul engine notifications
	commitCh          chan *ethTypes.Block
	vcommitCh         chan *types.IntermediateBlockResult
	proposedBlockHash common.Hash
	sealMu            sync.Mutex
	shouldStart       bool
	coreStarted       bool
	coreMu            sync.RWMutex

	// Current list of candidates we are pushing
	//candidates map[common.Address]bool
	// Protects the signer fields
	//candidatesLock sync.RWMutex
	// Snapshots for recent block to speed up reorgs
	//recents *lru.ARCCache

	// event subscription for ChainHeadEvent event
	broadcaster consensus.Broadcaster

	//recentMessages *lru.ARCCache // the cache of peer's messages
	//knownMessages  *lru.ARCCache // the cache of self messages
}
