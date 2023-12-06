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
	"io/ioutil"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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
	ProxiedRoot    string                  `json:"proxied_root"`
	ProxiedDetail  map[string]*DumpProxied `json:"proxied_detail"`

	Reward       string            `json:"reward_balance"`
	RewardRoot   string            `json:"reward_root"`
	RewardDetail map[string]string `json:"reward_detail"`

	EpochReward   map[uint64]*big.Int `json:"epoch_reward"`
	ExtractNumber uint64              `json:"extract_number"`

	Tx1Root   string   `json:"tx1_root"`
	Tx1Detail []string `json:"tx1_detail"`
	Tx3Root   string   `json:"tx3_root"`
	Tx3Detail []string `json:"tx3_detail"`

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
	Root                     string                 `json:"root"`
	Accounts                 map[string]DumpAccount `json:"accounts"`
	RewardAccounts           []string               `json:"reward_accounts"`
	ChildChainRewardPerBlock *big.Int               `json:"child_chain_reward_per_block"`
	RefundAccounts           []string               `json:"refund_accounts"`
}
type DumpAccountBalance struct {
	Balance        string `json:"balance"`
	Deposit        string `json:"deposit_balance"`
	Delegate       string `json:"delegate_balance"`
	Proxied        string `json:"proxied_balance"`
	DepositProxied string `json:"deposit_proxied_balance"`
	PendingRefund  string `json:"pending_refund_balance"`
	Reward         string `json:"reward_balance"`
	Nonce          uint64 `json:"nonce"`
	Candidate      bool   `json:"candidate"`
}
type DumpBalance struct {
	Root     string                        `json:"root"`
	Accounts map[string]DumpAccountBalance `json:"accounts"`
}

func (self *StateDB) RawDump() Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", self.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
		//RewardAccounts: make([]string, 0),
		//RefundAccounts: make([]string, 0),
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
				ProxiedRoot:    data.ProxiedRoot.String(),
				ProxiedDetail:  make(map[string]*DumpProxied),
				Reward:         data.RewardBalance.String(),
				RewardRoot:     data.RewardRoot.String(),
				RewardDetail:   make(map[string]string),
				EpochReward:    make(map[uint64]*big.Int),

				Tx1Root: data.TX1Root.String(),
				Tx3Root: data.TX3Root.String(),

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

			tx1It := trie.NewIterator(obj.getTX1Trie(self.db).NodeIterator(nil))
			for tx1It.Next() {
				tx1hash := common.BytesToHash(self.trie.GetKey(tx1It.Key))
				account.Tx1Detail = append(account.Tx1Detail, tx1hash.Hex())
			}

			tx3It := trie.NewIterator(obj.getTX3Trie(self.db).NodeIterator(nil))
			for tx3It.Next() {
				tx3hash := common.BytesToHash(self.trie.GetKey(tx3It.Key))
				account.Tx3Detail = append(account.Tx3Detail, tx3hash.Hex())
			}

			if data.ProxiedRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
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
			}

			if data.RewardRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
				rewardIt := trie.NewIterator(obj.getRewardTrie(self.db).NodeIterator(nil))
				for rewardIt.Next() {
					var key uint64
					rlp.DecodeBytes(self.trie.GetKey(rewardIt.Key), &key)
					var value big.Int
					rlp.DecodeBytes(rewardIt.Value, &value)
					account.RewardDetail[fmt.Sprintf("epoch_%d", key)] = value.String()
				}
			}

			dump.Accounts[common.Bytes2Hex(addr)] = account
		} else {
			if bytes.Equal(addr, rewardSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, rewardAddr := range data {
					dump.RewardAccounts = append(dump.RewardAccounts, rewardAddr.Hex())
				}
			} else if bytes.Equal(addr, childChainRewardPerBlockKey) {
				data := new(big.Int)
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				dump.ChildChainRewardPerBlock = data
			} else if bytes.Equal(addr, refundSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, refundAddr := range data {
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

func (self *StateDB) RawDumpToFile(height uint64, filename string) Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", self.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
		//RewardAccounts: make([]string, 0),
		//RefundAccounts: make([]string, 0),
	}

	//icount := 0
	it := trie.NewIterator(self.trie.NodeIterator(nil))
	for it.Next() {

		addr := self.trie.GetKey(it.Key)
		if len(addr) == 20 {
			var data Account
			if err := rlp.DecodeBytes(it.Value, &data); err != nil {
				panic(err)
			}

			commonAddr := common.BytesToAddress(addr)
			obj := newObject(nil, commonAddr, data, nil)
			account := DumpAccount{
				Balance:        data.Balance.String(),
				Deposit:        data.DepositBalance.String(),
				Delegate:       data.DelegateBalance.String(),
				Proxied:        data.ProxiedBalance.String(),
				DepositProxied: data.DepositProxiedBalance.String(),
				PendingRefund:  data.PendingRefundBalance.String(),
				ProxiedRoot:    data.ProxiedRoot.String(),
				ProxiedDetail:  make(map[string]*DumpProxied),
				Reward:         data.RewardBalance.String(),
				RewardRoot:     data.RewardRoot.String(),
				RewardDetail:   make(map[string]string),
				EpochReward:    make(map[uint64]*big.Int),

				Tx1Root: data.TX1Root.String(),
				Tx3Root: data.TX3Root.String(),

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

			tx1It := trie.NewIterator(obj.getTX1Trie(self.db).NodeIterator(nil))
			for tx1It.Next() {
				tx1hash := common.BytesToHash(self.trie.GetKey(tx1It.Key))
				account.Tx1Detail = append(account.Tx1Detail, tx1hash.Hex())
			}

			tx3It := trie.NewIterator(obj.getTX3Trie(self.db).NodeIterator(nil))
			for tx3It.Next() {
				tx3hash := common.BytesToHash(self.trie.GetKey(tx3It.Key))
				account.Tx3Detail = append(account.Tx3Detail, tx3hash.Hex())
			}

			if data.ProxiedRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
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
			}

			if data.RewardRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
				rewardIt := trie.NewIterator(obj.getRewardTrie(self.db).NodeIterator(nil))
				for rewardIt.Next() {
					var key uint64
					rlp.DecodeBytes(self.trie.GetKey(rewardIt.Key), &key)
					var value big.Int
					rlp.DecodeBytes(rewardIt.Value, &value)
					account.RewardDetail[fmt.Sprintf("epoch_%d", key)] = value.String()
				}
			}

			account.EpochReward = self.GetAllEpochReward(commonAddr, height, false)
			for epoch, reward := range account.EpochReward {
				if reward == nil || reward.Sign() == 0 {
					delete(account.EpochReward, epoch)
				}
			}
			if extractNumber, err1 := self.GetEpochRewardExtracted(commonAddr, height); err1 == nil {
				account.ExtractNumber = extractNumber
				for epoch, _ := range account.EpochReward {
					if epoch <= account.ExtractNumber {
						delete(account.EpochReward, epoch)
					}
				}
			}

			dump.Accounts[common.Bytes2Hex(addr)] = account
		} else {
			if bytes.Equal(addr, rewardSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, rewardAddr := range data {
					dump.RewardAccounts = append(dump.RewardAccounts, rewardAddr.Hex())
				}
			} else if bytes.Equal(addr, childChainRewardPerBlockKey) {
				data := new(big.Int)
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				dump.ChildChainRewardPerBlock = data
			} else if bytes.Equal(addr, refundSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, refundAddr := range data {
					dump.RefundAccounts = append(dump.RefundAccounts, refundAddr.Hex())
				}
			}
		}
	}

	json, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}
	ioutil.WriteFile(filename, json, 0644)
	return dump
}

func (self *StateDB) RawDumpBalanceToFile(height uint64, filename string) DumpBalance {
	dump := DumpBalance{
		Root:     fmt.Sprintf("%x", self.trie.Hash()),
		Accounts: make(map[string]DumpAccountBalance),
	}

	it := trie.NewIterator(self.trie.NodeIterator(nil))
	for it.Next() {
		addr := self.trie.GetKey(it.Key)
		if len(addr) == 20 {
			var data Account
			if err := rlp.DecodeBytes(it.Value, &data); err != nil {
				panic(err)
			}

			account := DumpAccountBalance{
				Balance:        data.Balance.String(),
				Deposit:        data.DepositBalance.String(),
				Delegate:       data.DelegateBalance.String(),
				Proxied:        data.ProxiedBalance.String(),
				DepositProxied: data.DepositProxiedBalance.String(),
				PendingRefund:  data.PendingRefundBalance.String(),
				Reward:         data.RewardBalance.String(),
				Nonce:          data.Nonce,
				Candidate:      data.Candidate,
			}

			dump.Accounts[common.Bytes2Hex(addr)] = account
		}
	}

	json, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}
	ioutil.WriteFile(filename, json, 0644)
	return dump
}

func (self *StateDB) DoSnapshot(sn trie.Snapshot) error {

	dump := Dump{
		Root:     fmt.Sprintf("%x", self.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
		//RewardAccounts: make([]string, 0),
		//RefundAccounts: make([]string, 0),
	}

	soCount := 0
	backLen := 0
	content := fmt.Sprintf("%v state object copied", soCount)
	replacePrint(backLen, content)
	backLen = len(content)

	it := trie.NewIterator(self.trie.NodeIterator(nil))
	for it.Next() {

		addr := self.trie.GetKey(it.Key)

		//checkForAddress(it.Key, addr, "0xf45ced1304c7dddd6a7012361f4a3d8f9ea03b7f")

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
				ProxiedRoot:    data.ProxiedRoot.String(),
				ProxiedDetail:  make(map[string]*DumpProxied),
				Reward:         data.RewardBalance.String(),
				RewardRoot:     data.RewardRoot.String(),
				RewardDetail:   make(map[string]string),

				Tx1Root: data.TX1Root.String(),
				Tx3Root: data.TX3Root.String(),

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

			tx1It := trie.NewIterator(obj.getTX1Trie(self.db).NodeIterator(nil))
			for tx1It.Next() {
				tx1hash := common.BytesToHash(self.trie.GetKey(tx1It.Key))
				account.Tx1Detail = append(account.Tx1Detail, tx1hash.Hex())
			}

			tx3It := trie.NewIterator(obj.getTX3Trie(self.db).NodeIterator(nil))
			for tx3It.Next() {
				tx3hash := common.BytesToHash(self.trie.GetKey(tx3It.Key))
				account.Tx3Detail = append(account.Tx3Detail, tx3hash.Hex())
			}

			if data.ProxiedRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
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
			}

			if data.RewardRoot.String() != "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421" {
				rewardIt := trie.NewIterator(obj.getRewardTrie(self.db).NodeIterator(nil))
				for rewardIt.Next() {
					var key uint64
					rlp.DecodeBytes(self.trie.GetKey(rewardIt.Key), &key)
					var value big.Int
					rlp.DecodeBytes(rewardIt.Value, &value)
					account.RewardDetail[fmt.Sprintf("epoch_%d", key)] = value.String()
				}
			}

			dump.Accounts[common.Bytes2Hex(addr)] = account
		} else {
			if bytes.Equal(addr, rewardSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, rewardAddr := range data {
					dump.RewardAccounts = append(dump.RewardAccounts, rewardAddr.Hex())
				}
			} else if bytes.Equal(addr, childChainRewardPerBlockKey) {
				data := new(big.Int)
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				dump.ChildChainRewardPerBlock = data
			} else if bytes.Equal(addr, refundSetKey) {
				var data []common.Address
				if err := rlp.DecodeBytes(it.Value, &data); err != nil {
					panic(err)
				}
				for _, refundAddr := range data {
					dump.RefundAccounts = append(dump.RefundAccounts, refundAddr.Hex())
				}
			}
		}

		soCount++
		content = fmt.Sprintf("%v state object copied", soCount)
		replacePrint(backLen, content)
		backLen = len(content)

		giveABreak()
	}

	fmt.Println("")

	return nil
}

func replacePrint(backlen int, content string) {

	backString := ""
	for i := 0; i < backlen; i++ {
		backString += "\b"
	}
	fmt.Print(backString)
	fmt.Print(content)
}

// for debug; to check if doing snapshot for the specific strAddr
func checkForAddress(key, addr []byte, strAddr string) bool {

	keyHex := common.Bytes2Hex(key)
	addrHex := common.Bytes2Hex(addr)
	fmt.Printf("it.Key %v, addrHex %v\n", keyHex, addrHex)

	addr0 := common.Address{}
	addr0.SetBytes(addr)
	addr1 := common.Address{}
	addr1.SetBytes(common.FromHex(strAddr))
	if addr0 == addr1 {
		return true
	}

	shex := common.Bytes2Hex(crypto.Keccak256Hash(common.FromHex(strAddr)).Bytes())
	fmt.Printf("sh: %v\n", shex)
	if shex == keyHex {
		return true
	}

	return false
}

func giveABreak() {
	time.Sleep(time.Millisecond)
}
