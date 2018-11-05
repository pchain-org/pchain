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

package tendermint

import (
	"errors"
	"github.com/ethereum/go-ethereum/consensus"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
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
	if sb.chainConfig.PChainId == "pchain" {
		protocolName = sb.chainConfig.PChainId
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

	sb.logger.Info("Tendermint (backend) HandleMsg, add logic here")

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

func (sb *backend) NewChainHead() error {
	sb.coreMu.RLock()
	defer sb.coreMu.RUnlock()
	if !sb.coreStarted {
		return ErrStoppedEngine
	}
	go tdmTypes.FireEventFinalCommitted(sb.core.EventSwitch(), tdmTypes.EventDataFinalCommitted{})
	return nil
}

func (sb *backend) GetLogger() log.Logger {
	return sb.logger
}

func (sb *backend) AddPeer(src consensus.Peer) {
	sb.core.consensusReactor.AddPeer(src)
}

func (sb *backend) RemovePeer(src consensus.Peer) {
	sb.core.consensusReactor.RemovePeer(src, nil)
}
