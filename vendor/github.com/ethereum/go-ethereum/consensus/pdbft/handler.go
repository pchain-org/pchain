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

package pdbft

import (
	"errors"
	"github.com/ethereum/go-ethereum/consensus"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	// errDecodeFailed is returned when decode message fails
	errDecodeFailed = errors.New("fail to decode tendermint message")
)

// Protocol implements consensus.Engine.Protocol
func (sb *backend) Protocol() consensus.Protocol {

	sb.logger.Info("Tendermint (backend) Protocol, add logic here")

	var protocolName string
	if sb.chainConfig.IsMainChain() {
		protocolName = "pchain" //we also use "pchain" if the net is "testnet"
	} else {
		protocolName = "pchain_" + sb.chainConfig.PChainId
	}

	return consensus.Protocol{
		Name:     protocolName,
		Versions: []uint{64},
		Lengths:  []uint64{64},
	}
}

// HandleMsg implements consensus.Handler.HandleMsg
func (sb *backend) HandleMsg(chID uint64, src consensus.Peer, msgBytes []byte) (bool, error) {
	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()

	sb.core.consensusReactor.Receive(chID, src, msgBytes)

	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (sb *backend) SetBroadcaster(broadcaster consensus.Broadcaster) {

	sb.logger.Infof("Tendermint (backend) SetBroadcaster: %p", broadcaster)
	sb.broadcaster = broadcaster
}

func (sb *backend) GetBroadcaster() consensus.Broadcaster {

	sb.logger.Infof("Tendermint (backend) GetBroadcaster: %p", sb.broadcaster)
	return sb.broadcaster
}

func (sb *backend) NewChainHead(block *types.Block) error {
	sb.coreMu.RLock()
	defer sb.coreMu.RUnlock()
	if !sb.coreStarted {
		return ErrStoppedEngine
	}
	go tdmTypes.FireEventFinalCommitted(sb.core.EventSwitch(), tdmTypes.EventDataFinalCommitted{block.NumberU64()})
	return nil
}

func (sb *backend) GetLogger() log.Logger {
	return sb.logger
}

func (sb *backend) AddPeer(src consensus.Peer) {

	sb.core.consensusReactor.AddPeer(src)
	sb.logger.Debug("Peer successful added into Consensus Reactor")

	/*
	if !sb.shouldStart {
		sb.logger.Debug("Consensus Engine (Tendermint) does not plan to start")
		return
	}


	for i := 0; i < 10; i++ {
		sb.coreMu.RLock()
		started := sb.coreStarted
		sb.coreMu.RUnlock()

		if started {
			sb.core.consensusReactor.AddPeer(src)
			sb.logger.Debug("Peer successful added into Consensus Reactor")
			return
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	sb.logger.Error("Wait for 10 sec, Consensus Engine (Tendermint) still not start, unable to add the peer to Engine")
	*/
}

func (sb *backend) RemovePeer(src consensus.Peer) {
	sb.core.consensusReactor.RemovePeer(src, nil)
}
