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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	"math/big"
)

// Constants to match up protocol versions and messages
const (
	Eth62 = 62
	Eth63 = 63
)

var (
	EthProtocol = Protocol{
		Name:     "eth",
		Versions: []uint{Eth62, Eth63},
		Lengths:  []uint64{17, 8},
	}
)

// Protocol defines the protocol of the consensus
type Protocol struct {
	// Official short name of the protocol used during capability negotiation.
	Name string
	// Supported versions of the eth protocol (first is primary).
	Versions []uint
	// Number of implemented message corresponding to different protocol versions.
	Lengths []uint64
}

// Broadcaster defines the interface to enqueue blocks to fetcher and find peer
type Broadcaster interface {
	// Enqueue add a block into fetcher queue
	Enqueue(id string, block *types.Block)
	// FindPeers retrives peers by addresses
	FindPeers(map[common.Address]bool) map[common.Address]Peer
	// BroadcastBlock broadcast Block
	BroadcastBlock(block *types.Block, propagate bool)
	// BroadcastMessage broadcast Message to P2P network
	BroadcastMessage(msgcode uint64, data interface{})
	// Find the Bad Preimages and send request to best peer for correction
	TryFixBadPreimages()
}

// Peer defines the interface to communicate with peer
type Peer interface {
	// Send sends the message to this peer
	Send(msgcode uint64, data interface{}) error
	//Send block to this peer
	SendNewBlock(block *types.Block, td *big.Int) error
	// GetPeerState return the Peer State during consensus
	GetPeerState() PeerState
	// GetKey return the short Public Key of peer
	GetKey() string
	// PeerState set the Peer State
	SetPeerState(ps PeerState)
	// P2PPeer return p2p.Peer
	P2PPeer() *p2p.Peer
}

type PeerState interface {
	GetHeight() uint64
	Disconnect()
}
