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

package trial_backend

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/p2p"
	"fmt"
)

const (
	tendemrintMsg = 0x11
)

var (
	// errDecodeFailed is returned when decode message fails
	errDecodeFailed = errors.New("fail to decode tendermint message")
)

// Protocol implements consensus.Engine.Protocol
func (sb *backend) Protocol() consensus.Protocol {

	fmt.Printf("Tendermint: (sb *backend) Protocol, add logic here\n")

	return consensus.Protocol{
		Name:     "tendermint",
		Versions: []uint{9},
		Lengths:  []uint64{18},
	}
}

// HandleMsg implements consensus.Handler.HandleMsg
func (sb *backend) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()

	fmt.Printf("Tendermint: (sb *backend) HandleMsg, add logic here\n")

	return false, nil
}
