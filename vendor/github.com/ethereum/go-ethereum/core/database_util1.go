// Copyright 2015 The go-ethereum Authors
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

package core

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

type CrossChainTxType int

const (
	MainChainToChildChain CrossChainTxType = iota
	ChildChainToMainChain
)

func (t CrossChainTxType) String() string {
	switch t {
	case MainChainToChildChain:
		return "MainChainToChildChain"
	case ChildChainToMainChain:
		return "ChildChainToMainChain"
	default:
		return "UnKnown"
	}
}

var (
	// the prefix must not conflict with variables in database_util.go
	txPrefix               = []byte("cc-tx")   // child-chain tx
	toChildChainTxPrefix   = []byte("cc-to")   // txHash that deposit to child chain
	fromChildChainTxPrefix = []byte("cc-from") // txHash that withdraw from child chain
	usedChildChainTxPrefix = []byte("cc-used") // txHash that has been used in child chain

	// errors
	NotFoundErr = errors.New("not found") // general not found error
)

func GetChildChainTransactionByHash(db ethdb.Database, chainId string, txHash common.Hash) (*types.Transaction, error) {

	key := calcChildChainTxKey(chainId, txHash)
	bs, err := db.Get(key)
	if bs == nil || err != nil {
		return nil, NotFoundErr
	}

	return decodeTx(bs)
}

func WriteChildChainBlock(db ethdb.Database, block *types.Block) error {

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(block.Header())
	if err != nil {
		return err
	}

	txs := block.Transactions()
	for _, tx := range txs {
		etd := tx.ExtendTxData()
		if etd.FuncName == "WithdrawFromChildChain" {
			// write the entire tx
			bs, err := rlp.EncodeToBytes(tx)
			if err != nil {
				return err
			}
			txHash := tx.Hash()
			key := calcChildChainTxKey(tdmExtra.ChainID, txHash)
			err = db.Put(key, bs)
			if err != nil {
				return err
			}

			// add 'child chain to main chain' tx.
			from, _ := etd.GetAddress("from")
			err = AddCrossChainTx(db, ChildChainToMainChain, tdmExtra.ChainID, from, txHash)
			if err != nil {
				return err
			}
		} else if etd.FuncName == "DepositInChildChain" {
			from, _ := etd.GetAddress("from")
			txHash, _ := etd.GetHash("txHash")
			// remove 'main chain to child chain' tx.
			err = RemoveCrossChainTx(db, MainChainToChildChain, tdmExtra.ChainID, from, txHash)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func AddCrossChainTx(db ethdb.Database, t CrossChainTxType, chainId string, account common.Address, txHash common.Hash) error {

	log.Infof("AddCrossChainTx %v: chain: %s, account: %x, tx: %x", t, chainId, account, txHash)
	var hashes []common.Hash
	key := calcCrossChainTxKey(t, chainId, account)
	bs, err := db.Get(key)
	if err == nil { // already exists
		err = rlp.DecodeBytes(bs, &hashes)
		if err != nil {
			log.Warnf("AddCrossChainTx decode error: %v", err)
			return err
		}
	}
	hashes = append(hashes, txHash)
	bs, err = rlp.EncodeToBytes(hashes)
	if err != nil {
		log.Warnf("AddCrossChainTx encode error: %v", err)
		return err
	}
	err = db.Put(key, bs)
	if err != nil {
		log.Warnf("AddCrossChainTx db put error: %v", err)
		return err
	}
	log.Infof("AddCrossChainTx %v: chain: %s, account: %x, txs: %x", t, chainId, account, hashes)
	return nil
}

func RemoveCrossChainTx(db ethdb.Database, t CrossChainTxType, chainId string, account common.Address, txHash common.Hash) error {

	log.Infof("RemoveCrossChainTx %v: chain: %s, account: %x, tx: %x", t, chainId, account, txHash)
	var hashes []common.Hash
	key := calcCrossChainTxKey(t, chainId, account)
	bs, err := db.Get(key)
	if err != nil {
		log.Warnf("RemoveCrossChainTx db get error: %v", err)
		return err
	}
	err = rlp.DecodeBytes(bs, &hashes)
	if err != nil {
		log.Warnf("RemoveCrossChainTx decode error: %v", err)
		return err
	}
	for i := range hashes {
		if hashes[i] == txHash {
			// remove element at index i
			hashes[i] = hashes[len(hashes)-1]
			hashes = hashes[:len(hashes)-1]

			bs, err = rlp.EncodeToBytes(hashes)
			if err != nil {
				log.Warnf("RemoveCrossChainTx encode error: %v", err)
				return err
			}

			err = db.Put(key, bs)
			if err != nil {
				log.Warnf("RemoveCrossChainTx db put error: %v", err)
				return err
			}

			log.Infof("RemoveCrossChainTx %v: chain: %s, account: %x, txs: %x", t, chainId, account, hashes)
			return nil
		}
	}
	return fmt.Errorf("tx not found: %x", txHash)
}

func HasCrossChainTx(db ethdb.Database, t CrossChainTxType, chainId string, account common.Address, txHash common.Hash) bool {
	var hashes []common.Hash
	key := calcCrossChainTxKey(t, chainId, account)
	bs, err := db.Get(key)
	if err != nil {
		return false
	}
	err = rlp.DecodeBytes(bs, &hashes)
	if err != nil {
		return false
	}
	for i := range hashes {
		if hashes[i] == txHash {
			return true
		}
	}
	return false
}

func AppendUsedChildChainTx(db ethdb.Database, chainId string, account common.Address, txHash common.Hash) error {

	log.Infof("AppendUsedChildChainTx chain: %s, account: %x, tx: %x", chainId, account, txHash)
	var hashes []common.Hash
	key := calcUsedChildChainTxKey(chainId, account)
	bs, err := db.Get(key)
	if err == nil { // already exists
		err = rlp.DecodeBytes(bs, &hashes)
		if err != nil {
			log.Warnf("AppendUsedChildChainTx decode error: %v", err)
			return err
		}
	}
	hashes = append(hashes, txHash)
	bs, err = rlp.EncodeToBytes(hashes)
	if err != nil {
		log.Warnf("AppendUsedChildChainTx encode error: %v", err)
		return err
	}
	err = db.Put(key, bs)
	if err != nil {
		log.Warnf("AppendUsedChildChainTx db put error: %v", err)
		return err
	}
	log.Infof("AppendUsedChildChainTx chain: %s, account: %x, txs: %x", chainId, account, hashes)
	return nil
}

func HasUsedChildChainTx(db ethdb.Database, chainId string, account common.Address, txHash common.Hash) bool {
	var hashes []common.Hash
	key := calcUsedChildChainTxKey(chainId, account)
	bs, err := db.Get(key)
	if err != nil {
		return false
	}
	err = rlp.DecodeBytes(bs, &hashes)
	if err != nil {
		return false
	}
	for i := range hashes {
		if hashes[i] == txHash {
			return true
		}
	}
	return false
}

func calcChildChainTxKey(chainId string, hash common.Hash) []byte {
	return append(txPrefix, []byte(fmt.Sprintf("-%s-%x", chainId, hash))...)
}

func calcUsedChildChainTxKey(chainId string, account common.Address) []byte {
	return append(usedChildChainTxPrefix, []byte(fmt.Sprintf("-%s-%x", chainId, account))...)
}

func calcCrossChainTxKey(t CrossChainTxType, chainId string, account common.Address) []byte {
	if t == MainChainToChildChain {
		return append(toChildChainTxPrefix, []byte(fmt.Sprintf("-%s-%x", chainId, account))...)
	} else { // ChildChainToMainChain
		return append(fromChildChainTxPrefix, []byte(fmt.Sprintf("-%s-%x", chainId, account))...)
	}
}

func decodeTx(txBytes []byte) (*types.Transaction, error) {

	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}
