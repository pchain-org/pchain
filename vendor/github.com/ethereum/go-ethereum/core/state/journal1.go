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

package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type journal1Entry interface {
	undo(*State1DB)
}

type journal1 []journal1Entry

type (
	// Changes to the account trie.
	createState1ObjectChange struct {
		account *common.Address
	}
	resetState1ObjectChange struct {
		prev *state1Object
	}

	suicideState1ObjectChange struct {
		account     *common.Address
		prev        bool // whether account had already suicided
		prevEpochReward   map[uint64]*big.Int
		prevExtractNumber uint64
	}

	state1AddLogChange struct {
		txhash common.Hash
	}

	state1EpochRewardChange struct {
		account  *common.Address
		prev     map[uint64]*big.Int
	}

	state1ExtractNumberChange struct {
		account *common.Address
		prev    uint64
	}
	state1AddPreimageChange struct {
		hash common.Hash
	}
	state1TouchChange struct {
		account   *common.Address
		prev      bool
		prevDirty bool
	}
)

func (ch createState1ObjectChange) undo(s *State1DB) {
	delete(s.state1Objects, *ch.account)
	delete(s.state1ObjectsDirty, *ch.account)
}

func (ch resetState1ObjectChange) undo(s *State1DB) {
	s.setState1Object(ch.prev)
}

func (ch state1EpochRewardChange) undo(s *State1DB) {
	s.getState1Object(*ch.account).setEpochReward(ch.prev)
}

func (ch state1ExtractNumberChange) undo(s *State1DB) {
	s.getState1Object(*ch.account).setExtractNumber(ch.prev)
}

func (ch suicideState1ObjectChange) undo(s *State1DB) {
	obj := s.getState1Object(*ch.account)
	if obj != nil {
		obj.suicided = ch.prev
		obj.setEpochReward(ch.prevEpochReward)
		obj.setExtractNumber(ch.prevExtractNumber)
	}
}

func (ch state1AddLogChange) undo(s *State1DB) {
	logs := s.logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.logs, ch.txhash)
	} else {
		s.logs[ch.txhash] = logs[:len(logs)-1]
	}
	s.logSize--
}

func (ch state1AddPreimageChange) undo(s *State1DB) {
	delete(s.preimages, ch.hash)
}

func (ch state1TouchChange) undo(s *State1DB) {
	if !ch.prev && *ch.account != ripemd {
		s.getState1Object(*ch.account).touched = ch.prev
		if !ch.prevDirty {
			delete(s.state1ObjectsDirty, *ch.account)
		}
	}
}