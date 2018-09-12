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

	"github.com/ethereum/go-ethereum/common"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pchain/common/plogger"
)

var logger = plogger.GetLogger("core")

type CrossChainTxType byte

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

type CrossChainTxState byte

const (
	CrossChainTxNotFound CrossChainTxState = iota
	CrossChainTxReady
	CrossChainTxAlreadyUsed
	CrossChainTxInvalid
)

func (s CrossChainTxState) String() string {
	switch s {
	case CrossChainTxNotFound:
		return "NotFound"
	case CrossChainTxReady:
		return "Ready"
	case CrossChainTxAlreadyUsed:
		return "AlreadyUsed"
	case CrossChainTxInvalid:
		return "Invalid"
	default:
		return "UnKnown"
	}
}

var (
	// the prefix must not conflict with variables in database_util.go
	txPrefix               = []byte("cc-tx")   // child-chain tx
	toChildChainTxPrefix   = []byte("cc-to")   // txHash that deposit to child chain
	fromChildChainTxPrefix = []byte("cc-from") // txHash that withdraw from child chain
	childChainTxUsedPrefix = []byte("cc-used") // txHash that has been used in child chain

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

			// mark 'child chain to main chain' tx.
			from, _ := etd.GetAddress("from")
			err = MarkCrossChainTx(db, ChildChainToMainChain, from, tdmExtra.ChainID, txHash, false)
			if err != nil {
				return err
			}
		} else if etd.FuncName == "DepositInChildChain" {
			from, _ := etd.GetAddress("from")
			txHash, _ := etd.GetHash("txHash")
			// mark 'main chain to child chain' tx as used.
			err = MarkCrossChainTx(db, MainChainToChildChain, from, tdmExtra.ChainID, txHash, true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func MarkCrossChainTx(db ethdb.Database, t CrossChainTxType, from common.Address, chainId string, txHash common.Hash, used bool) error {
	logger.Infof("MarkCrossChainTx %v: account: %x, chain: %s, tx: %x, used: %v", t, from, chainId, txHash, used)

	key := calcCrossChainTxKey(t, from, chainId, txHash)
	var value []byte
	if used {
		// sanity check
		s := ValidateCrossChainTx(db, t, from, chainId, txHash)
		if s != CrossChainTxReady {
			return fmt.Errorf("inconsistent state")
		}
		value = []byte{byte(CrossChainTxAlreadyUsed)}
	} else {
		value = []byte{byte(CrossChainTxReady)}
	}
	err := db.Put(key, value)
	if err != nil {
		logger.Warnf("MarkCrossChainTx db put error: %v", err)
		return err
	}
	return nil
}

func ValidateCrossChainTx(db ethdb.Database, t CrossChainTxType, from common.Address, chainId string, txHash common.Hash) CrossChainTxState {
	logger.Infof("ValidateCrossChainTx %v: account: %x, chain: %s, tx: %x", t, from, chainId, txHash)

	key := calcCrossChainTxKey(t, from, chainId, txHash)
	value, err := db.Get(key)
	if err != nil {
		return CrossChainTxNotFound
	}

	if len(value) != 1 {
		return CrossChainTxInvalid
	}

	if value[0] != byte(CrossChainTxReady) && value[0] != byte(CrossChainTxAlreadyUsed) {
		return CrossChainTxInvalid
	}

	return CrossChainTxState(value[0])
}

func MarkTxUsedOnChildChain(db ethdb.Database, from common.Address, chainId string, txHash common.Hash) error {
	logger.Infof("MarkChildChainTxUsed %v: account: %x, chain: %s, tx: %x", from, chainId, txHash)

	key := calcChildChainTxUsedKey(from, chainId, txHash)
	err := db.Put(key, []byte{byte(CrossChainTxAlreadyUsed)})
	if err != nil {
		logger.Warnf("MarkChildChainTxUsed db put error: %v", err)
		return err
	}
	return nil
}

func IsTxUsedOnChildChain(db ethdb.Database, from common.Address, chainId string, txHash common.Hash) bool {
	logger.Infof("IsChildChainTxUsed %v: account: %x, chain: %s, tx: %x", from, chainId, txHash)

	key := calcChildChainTxUsedKey(from, chainId, txHash)
	value, err := db.Get(key)
	if err != nil {
		return false
	}

	if len(value) != 1 || value[0] != byte(CrossChainTxAlreadyUsed) {
		return false
	}

	return true
}

func calcChildChainTxKey(chainId string, txHash common.Hash) []byte {
	return append(txPrefix, []byte(fmt.Sprintf("-%s-%x", chainId, txHash))...)
}

func calcCrossChainTxKey(t CrossChainTxType, from common.Address, chainId string, txHash common.Hash) []byte {
	if t == MainChainToChildChain {
		return append(toChildChainTxPrefix, []byte(fmt.Sprintf("%x-%s-%x", from, chainId, txHash))...)
	} else { // ChildChainToMainChain
		return append(fromChildChainTxPrefix, []byte(fmt.Sprintf("%x-%s-%x", from, chainId, txHash))...)
	}
}

func calcChildChainTxUsedKey(from common.Address, chainId string, txHash common.Hash) []byte {
	return append(childChainTxUsedPrefix, []byte(fmt.Sprintf("%x-%s-%x", from, chainId, txHash))...)
}

func decodeTx(txBytes []byte) (*types.Transaction, error) {

	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}
