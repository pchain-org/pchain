// Copyright 2016 The go-ethereum Authors
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

package light

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/hashicorp/golang-lru"
)

var (
	bodyCacheLimit  = 256
	blockCacheLimit = 256
)

// LightChain represents a canonical chain that by default only handles block
// headers, downloading block bodies and receipts on demand through an ODR
// interface. It only does header validation during chain insertion.
type LightChain struct {
	hc            *core.HeaderChain
	chainDb       ethdb.Database
	engine        consensus.Engine
	odr           OdrBackend
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	bodyCache    *lru.Cache // Cache for the most recent block bodies
	bodyRLPCache *lru.Cache // Cache for the most recent block bodies in RLP encoded format
	blockCache   *lru.Cache // Cache for the most recent entire blocks

	chainmu sync.RWMutex // protects header inserts
	quit    chan struct{}
	wg      sync.WaitGroup

	// Atomic boolean switches:
	running       int32 // whether LightChain is running or stopped
	procInterrupt int32 // interrupts chain insert
}

// NewLightChain returns a fully initialised light chain using information
// available in the database. It initialises the default Ethereum header
// validator.
func NewLightChain(odr OdrBackend, config *params.ChainConfig, engine consensus.Engine) (*LightChain, error) {
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)

	lc := &LightChain{
		chainDb:      odr.Database(),
		odr:          odr,
		quit:         make(chan struct{}),
		bodyCache:    bodyCache,
		bodyRLPCache: bodyRLPCache,
		blockCache:   blockCache,
		engine:       engine,
	}
	var err error
	lc.hc, err = core.NewHeaderChain(odr.Database(), config, lc.engine, lc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	lc.genesisBlock, _ = lc.GetBlockByNumber(NoOdr, 0)
	if lc.genesisBlock == nil {
		return nil, core.ErrNoGenesis
	}
	if cp, ok := trustedCheckpoints[lc.genesisBlock.Hash()]; ok {
		lc.addTrustedCheckpoint(cp)
	}
	if err := lc.loadLastState(); err != nil {
		return nil, err
	}
	// Check the current state of the block hashes and make sure that we do not have any of the bad blocks in our chain
	for hash := range core.BadHashes {
		if header := lc.GetHeaderByHash(hash); header != nil {
			log.Error("Found bad hash, rewinding chain", "number", header.Number, "hash", header.ParentHash)
			lc.SetHead(header.Number.Uint64() - 1)
			log.Error("Chain rewind was successful, resuming normal operation")
		}
	}
	return lc, nil
}

// addTrustedCheckpoint adds a trusted checkpoint to the blockchain
func (lc *LightChain) addTrustedCheckpoint(cp trustedCheckpoint) {
	if lc.odr.ChtIndexer() != nil {
		StoreChtRoot(lc.chainDb, cp.sectionIdx, cp.sectionHead, cp.chtRoot)
		lc.odr.ChtIndexer().AddKnownSectionHead(cp.sectionIdx, cp.sectionHead)
	}
	if lc.odr.BloomTrieIndexer() != nil {
		StoreBloomTrieRoot(lc.chainDb, cp.sectionIdx, cp.sectionHead, cp.bloomTrieRoot)
		lc.odr.BloomTrieIndexer().AddKnownSectionHead(cp.sectionIdx, cp.sectionHead)
	}
	if lc.odr.BloomIndexer() != nil {
		lc.odr.BloomIndexer().AddKnownSectionHead(cp.sectionIdx, cp.sectionHead)
	}
	log.Info("Added trusted checkpoint", "chain", cp.name, "block", (cp.sectionIdx+1)*CHTFrequencyClient-1, "hash", cp.sectionHead)
}

func (lc *LightChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&lc.procInterrupt) == 1
}

// Odr returns the ODR backend of the chain
func (lc *LightChain) Odr() OdrBackend {
	return lc.odr
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (lc *LightChain) loadLastState() error {
	if head := rawdb.ReadHeadHeaderHash(lc.chainDb); head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		lc.Reset()
	} else {
		if header := lc.GetHeaderByHash(head); header != nil {
			lc.hc.SetCurrentHeader(header)
		}
	}

	// Issue a status log and return
	header := lc.hc.CurrentHeader()
	headerTd := lc.GetTd(header.Hash(), header.Number.Uint64())
	log.Info("Loaded most recent local header", "number", header.Number, "hash", header.Hash(), "td", headerTd)

	return nil
}

// SetHead rewinds the local chain to a new head. Everything above the new
// head will be deleted and the new one set.
func (lc *LightChain) SetHead(head uint64) {
	lc.chainmu.Lock()
	defer lc.chainmu.Unlock()

	lc.hc.SetHead(head, nil)
	lc.loadLastState()
}

// GasLimit returns the gas limit of the current HEAD block.
func (lc *LightChain) GasLimit() uint64 {
	return lc.hc.CurrentHeader().GasLimit
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (lc *LightChain) Reset() {
	lc.ResetWithGenesisBlock(lc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (lc *LightChain) ResetWithGenesisBlock(genesis *types.Block) {
	// Dump the entire block chain and purge the caches
	lc.SetHead(0)

	lc.chainmu.Lock()
	defer lc.chainmu.Unlock()

	// Prepare the genesis block and reinitialise the chain
	rawdb.WriteTd(lc.chainDb, genesis.Hash(), genesis.NumberU64(), genesis.Difficulty())
	rawdb.WriteBlock(lc.chainDb, genesis)

	lc.genesisBlock = genesis
	lc.hc.SetGenesis(lc.genesisBlock.Header())
	lc.hc.SetCurrentHeader(lc.genesisBlock.Header())
}

// Accessors

// Engine retrieves the light chain's consensus engine.
func (lc *LightChain) Engine() consensus.Engine { return lc.engine }

// Genesis returns the genesis block
func (lc *LightChain) Genesis() *types.Block {
	return lc.genesisBlock
}

func (lc *LightChain) StateCache() state.Database {
	panic("not implemented")
}

// GetBody retrieves a block body (transactions and uncles) from the database
// or ODR service by hash, caching it if found.
func (lc *LightChain) GetBody(ctx context.Context, hash common.Hash) (*types.Body, error) {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := lc.bodyCache.Get(hash); ok {
		body := cached.(*types.Body)
		return body, nil
	}
	number := lc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	body, err := GetBody(ctx, lc.odr, hash, *number)
	if err != nil {
		return nil, err
	}
	// Cache the found body for next time and return
	lc.bodyCache.Add(hash, body)
	return body, nil
}

// GetBodyRLP retrieves a block body in RLP encoding from the database or
// ODR service by hash, caching it if found.
func (lc *LightChain) GetBodyRLP(ctx context.Context, hash common.Hash) (rlp.RawValue, error) {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := lc.bodyRLPCache.Get(hash); ok {
		return cached.(rlp.RawValue), nil
	}
	number := lc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	body, err := GetBodyRLP(ctx, lc.odr, hash, *number)
	if err != nil {
		return nil, err
	}
	// Cache the found body for next time and return
	lc.bodyRLPCache.Add(hash, body)
	return body, nil
}

// HasBlock checks if a block is fully present in the database or not, caching
// it if present.
func (lc *LightChain) HasBlock(hash common.Hash, number uint64) bool {
	blk, _ := lc.GetBlock(NoOdr, hash, number)
	return blk != nil
}

// GetBlock retrieves a block from the database or ODR service by hash and number,
// caching it if found.
func (lc *LightChain) GetBlock(ctx context.Context, hash common.Hash, number uint64) (*types.Block, error) {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := lc.blockCache.Get(hash); ok {
		return block.(*types.Block), nil
	}
	block, err := GetBlock(ctx, lc.odr, hash, number)
	if err != nil {
		return nil, err
	}
	// Cache the found block for next time and return
	lc.blockCache.Add(block.Hash(), block)
	return block, nil
}

// GetBlockByHash retrieves a block from the database or ODR service by hash,
// caching it if found.
func (lc *LightChain) GetBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	number := lc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	return lc.GetBlock(ctx, hash, *number)
}

// GetBlockByNumber retrieves a block from the database or ODR service by
// number, caching it (associated with its hash) if found.
func (lc *LightChain) GetBlockByNumber(ctx context.Context, number uint64) (*types.Block, error) {
	hash, err := GetCanonicalHash(ctx, lc.odr, number)
	if hash == (common.Hash{}) || err != nil {
		return nil, err
	}
	return lc.GetBlock(ctx, hash, number)
}

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the procInterrupt.
func (lc *LightChain) Stop() {
	if !atomic.CompareAndSwapInt32(&lc.running, 0, 1) {
		return
	}
	close(lc.quit)
	atomic.StoreInt32(&lc.procInterrupt, 1)

	lc.wg.Wait()
	log.Info("Blockchain manager stopped")
}

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (lc *LightChain) Rollback(chain []common.Hash) {
	lc.chainmu.Lock()
	defer lc.chainmu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		if head := lc.hc.CurrentHeader(); head.Hash() == hash {
			lc.hc.SetCurrentHeader(lc.GetHeader(head.ParentHash, head.Number.Uint64()-1))
		}
	}
}

// postChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
func (lc *LightChain) postChainEvents(events []interface{}) {
	for _, event := range events {
		switch ev := event.(type) {
		case core.ChainEvent:
			if lc.CurrentHeader().Hash() == ev.Hash {
				lc.chainHeadFeed.Send(core.ChainHeadEvent{Block: ev.Block})
			}
			lc.chainFeed.Send(ev)
		case core.ChainSideEvent:
			lc.chainSideFeed.Send(ev)
		}
	}
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verfy nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
//
// In the case of a light chain, InsertHeaderChain also creates and posts light
// chain events when necessary.
func (lc *LightChain) InsertHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := lc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

	// Make sure only one thread manipulates the chain at once
	lc.chainmu.Lock()
	defer func() {
		lc.chainmu.Unlock()
		time.Sleep(time.Millisecond * 10) // ugly hack; do not hog chain lock in case syncing is CPU-limited by validation
	}()

	lc.wg.Add(1)
	defer lc.wg.Done()

	var events []interface{}
	whFunc := func(header *types.Header) error {
		status, err := lc.hc.WriteHeader(header)

		switch status {
		case core.CanonStatTy:
			log.Debug("Inserted new header", "number", header.Number, "hash", header.Hash())
			events = append(events, core.ChainEvent{Block: types.NewBlockWithHeader(header), Hash: header.Hash()})

		case core.SideStatTy:
			log.Debug("Inserted forked header", "number", header.Number, "hash", header.Hash())
			events = append(events, core.ChainSideEvent{Block: types.NewBlockWithHeader(header)})
		}
		return err
	}
	i, err := lc.hc.InsertHeaderChain(chain, whFunc, start)
	lc.postChainEvents(events)
	return i, err
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (lc *LightChain) CurrentHeader() *types.Header {
	return lc.hc.CurrentHeader()
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (lc *LightChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return lc.hc.GetTd(hash, number)
}

// GetTdByHash retrieves a block's total difficulty in the canonical chain from the
// database by hash, caching it if found.
func (lc *LightChain) GetTdByHash(hash common.Hash) *big.Int {
	return lc.hc.GetTdByHash(hash)
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (lc *LightChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return lc.hc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (lc *LightChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return lc.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (lc *LightChain) HasHeader(hash common.Hash, number uint64) bool {
	return lc.hc.HasHeader(hash, number)
}

// GetCanonicalHash returns the canonical hash for a given block number
func (lc *LightChain) GetCanonicalHash(number uint64) common.Hash {
	return lc.hc.GetCanonicalHash(number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (lc *LightChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return lc.hc.GetBlockHashesFromHash(hash, max)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (lc *LightChain) GetHeaderByNumber(number uint64) *types.Header {
	return lc.hc.GetHeaderByNumber(number)
}

// GetHeaderByNumberOdr retrieves a block header from the database or network
// by number, caching it (associated with its hash) if found.
func (lc *LightChain) GetHeaderByNumberOdr(ctx context.Context, number uint64) (*types.Header, error) {
	if header := lc.hc.GetHeaderByNumber(number); header != nil {
		return header, nil
	}
	return GetHeaderByNumber(ctx, lc.odr, number)
}

// Config retrieves the header chain's chain configuration.
func (lc *LightChain) Config() *params.ChainConfig { return lc.hc.Config() }

func (lc *LightChain) SyncCht(ctx context.Context) bool {
	if lc.odr.ChtIndexer() == nil {
		return false
	}
	headNum := lc.CurrentHeader().Number.Uint64()
	chtCount, _, _ := lc.odr.ChtIndexer().Sections()
	if headNum+1 < chtCount*CHTFrequencyClient {
		num := chtCount*CHTFrequencyClient - 1
		header, err := GetHeaderByNumber(ctx, lc.odr, num)
		if header != nil && err == nil {
			lc.chainmu.Lock()
			if lc.hc.CurrentHeader().Number.Uint64() < header.Number.Uint64() {
				lc.hc.SetCurrentHeader(header)
			}
			lc.chainmu.Unlock()
			return true
		}
	}
	return false
}

// LockChain locks the chain mutex for reading so that multiple canonical hashes can be
// retrieved while it is guaranteed that they belong to the same version of the chain
func (lc *LightChain) LockChain() {
	lc.chainmu.RLock()
}

// UnlockChain unlocks the chain mutex
func (lc *LightChain) UnlockChain() {
	lc.chainmu.RUnlock()
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (lc *LightChain) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return lc.scope.Track(lc.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (lc *LightChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return lc.scope.Track(lc.chainHeadFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (lc *LightChain) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return lc.scope.Track(lc.chainSideFeed.Subscribe(ch))
}

// SubscribeLogsEvent implements the interface of filters.Backend
// LightChain does not send logs events, so return an empty subscription.
func (lc *LightChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return lc.scope.Track(new(event.Feed).Subscribe(ch))
}

// SubscribeRemovedLogsEvent implements the interface of filters.Backend
// LightChain does not send core.RemovedLogsEvent, so return an empty subscription.
func (lc *LightChain) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return lc.scope.Track(new(event.Feed).Subscribe(ch))
}

// SubscribePendingLogs starts delivering logs from pending transactions
// to the given channel.
func (lc *LightChain) SubscribePendingLogs(ch chan<- []*types.Log) event.Subscription {
	return lc.scope.Track(new(event.Feed).Subscribe(ch))
}