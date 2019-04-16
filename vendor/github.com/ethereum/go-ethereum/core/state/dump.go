// Copyright 2014 The go-ethereum Authors
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
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type DumpAccount struct {
	Balance        string                  `json:"balance"`
	Deposit        string                  `json:"deposit_balance"`
	Delegate       string                  `json:"delegate_balance"`
	Proxied        string                  `json:"proxied_balance"`
	DepositProxied string                  `json:"deposit_proxied_balance"`
	PendingRefund  string                  `json:"pending_refund_balance"`
	ProxiedDetail  map[string]*DumpProxied `json:"proxied_detail"`

	Reward       string            `json:"reward_balance"`
	RewardDetail map[string]string `json:"reward_detail"`

	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`

	Candidate  bool  `json:"candidate"`
	Commission uint8 `json:"commission"`
}

type DumpProxied struct {
	Proxied        string `json:"proxied_balance"`
	DepositProxied string `json:"deposit_proxied_balance"`
	PendingRefund  string `json:"pending_refund_balance"`
}

type Dump struct {
	Root           string                 `json:"root"`
	Accounts       map[string]DumpAccount `json:"accounts"`
	RewardAccounts []string               `json:"reward_accounts"`
	RefundAccounts []string               `json:"refund_accounts"`
}

func (self *StateDB) RawDump() Dump {
	dump := Dump{
		Root:           fmt.Sprintf("%x", self.trie.Hash()),
		Accounts:       make(map[string]DumpAccount),
		RewardAccounts: make([]string, 0),
		RefundAccounts: make([]string, 0),
	}

	it := trie.NewIterator(self.trie.NodeIterator(nil))
	for it.Next() {
		addr := self.trie.GetKey(it.Key)
		if len(addr) == 20 {
			var data Account
			if err := rlp.DecodeBytes(it.Value, &data); err != nil {
				panic(err)
			}

			obj := newObject(nil, common.BytesToAddress(addr), data, nil)
			account := DumpAccount{
				Balance:        data.Balance.String(),
				Deposit:        data.DepositBalance.String(),
				Delegate:       data.DelegateBalance.String(),
				Proxied:        data.ProxiedBalance.String(),
				DepositProxied: data.DepositProxiedBalance.String(),
				PendingRefund:  data.PendingRefundBalance.String(),
				ProxiedDetail:  make(map[string]*DumpProxied),
				Reward:         data.RewardBalance.String(),
				RewardDetail:   make(map[string]string),

				Nonce:    data.Nonce,
				Root:     common.Bytes2Hex(data.Root[:]),
				CodeHash: common.Bytes2Hex(data.CodeHash),
				Code:     common.Bytes2Hex(obj.Code(self.db)),
				Storage:  make(map[string]string),

				Candidate:  data.Candidate,
				Commission: data.Commission,
			}
			storageIt := trie.NewIterator(obj.getTrie(self.db).NodeIterator(nil))
			for storageIt.Next() {
				account.Storage[common.Bytes2Hex(self.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
			}

			proxiedIt := trie.NewIterator(obj.getProxiedTrie(self.db).NodeIterator(nil))
			for proxiedIt.Next() {
				var apb accountProxiedBalance
				rlp.DecodeBytes(proxiedIt.Value, &apb)
				account.ProxiedDetail[common.Bytes2Hex(self.trie.GetKey(proxiedIt.Key))] = &DumpProxied{
					Proxied:        apb.ProxiedBalance.String(),
					DepositProxied: apb.DepositProxiedBalance.String(),
					PendingRefund:  apb.PendingRefundBalance.String(),
				}
			}

			rewardIt := trie.NewIterator(obj.getRewardTrie(self.db).NodeIterator(nil))
			for rewardIt.Next() {
				var key uint64
				rlp.DecodeBytes(self.trie.GetKey(rewardIt.Key), &key)
				var value big.Int
				rlp.DecodeBytes(rewardIt.Value, &value)
				account.RewardDetail[fmt.Sprintf("epoch_%d", key)] = value.String()
			}

			dump.Accounts[common.Bytes2Hex(addr)] = account
		} else {
			if bytes.Equal(addr, rewardSetKey) {
				var data RewardSet
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for rewardAddr := range data {
					dump.RewardAccounts = append(dump.RewardAccounts, rewardAddr.Hex())
				}
			} else if bytes.Equal(addr, refundSetKey) {
				var data DelegateRefundSet
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for refundAddr := range data {
					dump.RefundAccounts = append(dump.RefundAccounts, refundAddr.Hex())
				}
			}
		}
	}
	return dump
}

func (self *StateDB) Dump() []byte {
	json, err := json.MarshalIndent(self.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return json
}
