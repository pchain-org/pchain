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

// Package consensus implements different Ethereum consensus engines.
package consensus

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
)

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *types.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *types.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *types.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) *types.Header

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) *types.Block

	// GetBlockByNumber retrieves a block from the database by number, caching it
	// (associated with its hash) if found.
	GetBlockByNumber(number uint64) *types.Block

	// GetTd retrieves a block's total difficulty in the canonical chain from the
	// database by hash and number, caching it if found.
	GetTd(hash common.Hash, number uint64) *big.Int

	// CurrentBlock retrieves the current head block of the canonical chain. The
	// block is retrieved from the blockchain's internal cache.
	CurrentBlock() *types.Block

	// State retrieves the current state of the canonical chain.
	State() (*state.StateDB, error)
}

// ChainValidator execute and validate the block with the current latest block as parent.
type ChainValidator interface {
	ValidateBlock(block *types.Block) (*state.StateDB, types.Receipts, *types.PendingOps, error)
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header *types.Header) (common.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainReader, header *types.Header, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error)

	// VerifyUncles verifies that the given block's uncles conform to the consensus
	// rules of a given engine.
	VerifyUncles(chain ChainReader, block *types.Block) error

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain ChainReader, header *types.Header) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainReader, header *types.Header) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// and assembles the final block.
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	Finalize(chain ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, totalGasFee *big.Int,
		uncles []*types.Header, receipts []*types.Receipt, ops *types.PendingOps) (*types.Block, error)

	// Seal generates a new block for the given input block with the local miner's
	// seal place on top.
	Seal(chain ChainReader, block *types.Block, stop <-chan struct{}) (interface{}, error)

	// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
	// that a new block should have.
	CalcDifficulty(chain ChainReader, time uint64, parent *types.Header) *big.Int

	// APIs returns the RPC APIs this consensus engine provides.
	APIs(chain ChainReader) []rpc.API

	// Close terminates any background threads maintained by the consensus engine.
	Close() error

	// Protocol returns the protocol for this consensus
	Protocol() Protocol
}

// Handler should be implemented is the consensus needs to handle and send peer's message
type Handler interface {
	// NewChainHead handles a new head block comes
	NewChainHead(block *types.Block) error

	// HandleMsg handles a message from peer
	HandleMsg(chID uint64, src Peer, msgBytes []byte) (bool, error)

	// SetBroadcaster sets the broadcaster to send message to peers
	SetBroadcaster(Broadcaster)

	// GetBroadcaster gets the broadcaster to send message to peers
	GetBroadcaster() Broadcaster

	AddPeer(src Peer)

	RemovePeer(src Peer)
}

// PoW is a consensus engine based on proof-of-work.
type PoW interface {
	Engine

	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
}

type EngineStartStop interface {
	// Start starts the engine
	Start(chain ChainReader, currentBlock func() *types.Block, hasBadBlock func(hash common.Hash) bool) error

	// Stop stops the engine
	Stop() error
}

// Istanbul is a consensus engine to avoid byzantine failure
type Istanbul interface {
	Engine

	EngineStartStop
}

// Tendermint is a consensus engine to avoid byzantine failure
type Tendermint interface {
	Engine

	EngineStartStop

	ShouldStart() bool

	IsStarted() bool

	// Normally Should Start flag will be set depends on the validator set
	// Force Start only set the Should Start Flag to true, when node join the validator before epoch switch
	ForceStart()

	GetEpoch() *epoch.Epoch

	SetEpoch(ep *epoch.Epoch)

	//check if need refresh validator's total voting power; true means it needs
	CheckAndRefreshVotingPowerForValidators(state *state.StateDB, ep *epoch.Epoch) bool

	PrivateValidator() common.Address

	// VerifyHeader checks whether a header conforms to the consensus rules of a given engine.
	VerifyHeaderBeforeConsensus(chain ChainReader, header *types.Header, seal bool) error
}
