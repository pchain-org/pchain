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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"io"
)

var (
	//the prefix must not conflict with variables in database_util.go
	blockPrefix = []byte("cc-block") //child-chain block
	txPrefix  = []byte("cc-tx")	//child-chain tx
	extraDataPrefix   = []byte("cc-ex") //child-chain extra data
	blockPartSizePrefix = []byte("cc-bps") //child-chain blockPartSize
	commitPrefix = []byte("cc-cm") //child-chain commits

	NotFoundErr = errors.New("not found") // general not found error
)

func GetChildBlockByNumber(db ethdb.Database, blockNumber int64, chainId string) (*types.Block, error) {

	key := calBlockKeyByNumber(blockNumber, chainId)

	value, err := db.Get(key)
	if err != nil {
		return nil, err
	}

	block := &types.Block{}
	return block.DecodeRLP1(value)
}

func GetChildBlockByHash(db ethdb.Database, blockHash []byte, chainId string) (*types.Block, error) {

	key := calBlockKeyByHash(blockHash, chainId)

	value, err := db.Get(key)
	if err != nil {
		return nil, err
	}

	block := &types.Block{}
	return block.DecodeRLP1(value)
}

//notice: the 'txHash' is the hash format of ethereum, not tendermint
func GetChildTransactionByHash(db ethdb.Database, txHash common.Hash, chainId string) (*types.Transaction, error) {

	key := calTxKey(txHash, chainId)

	value, err := db.Get(key)
	if err != nil {
		return nil, err
	}

	return decodeTx(value)
}

func DeleteChildBlockWithDetail(db ethdb.Database, number int64, chainId string) error {

	block, err := GetChildBlockByNumber(db, number, chainId)
	if err != nil {return err}

	DeleteChildBlock(db, number, chainId)
	if err != nil {return err}

	for _, tx := range block.Transactions() {

		err = DeleteChildTransaction(db, tx.Hash(), chainId)
		if err != nil {return err}
	}

	return nil
}

func DeleteChildBlock(db ethdb.Database, number int64, chainId string) error {

	return db.Delete(calBlockKeyByNumber(number, chainId))
}

func DeleteChildTransaction(db ethdb.Database, txHash common.Hash, chainId string) error {

	return db.Delete(calTxKey(txHash, chainId))
}

func WriteChildBlockWithDetail(db ethdb.Database, block *types.Block) error {

	WriteChildBlock(db, block)
	WriteChildTransactions(db, block)

	return nil
}

func WriteChildBlock(db ethdb.Database, block *types.Block) error {

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(block.Header())
	if err != nil {
		return err
	}

	blockByte, err := block.EncodeRLP1()
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	key := calBlockKeyByHash(block.Hash().Bytes(), chainId)
	err = db.Put(key, blockByte)
	if err != nil {
		return err
	}

	key = calBlockKeyByNumber(block.Number().Int64(), chainId)
	return db.Put(key, blockByte)
}

func WriteChildTransactions(db ethdb.Database, block *types.Block) error {

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(block.Header())
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID

	if len(block.Transactions()) == 0 {
		return nil
	}

	for _, tx := range block.Transactions() {

		key := calTxKey(tx.Hash(), chainId)//tdmBlock.ChainID

		err := db.Put(key, tx.Data())
		if err != nil {
			return err
		}
	}

	return nil
}

func calBlockKeyByNumber(number int64, chainId string) []byte {

	return append(blockPrefix, []byte(fmt.Sprintf("-%v-%s", number, chainId))...)
}

func calBlockKeyByHash(hash []byte, chainId string) []byte {

	return append(blockPrefix, []byte(fmt.Sprintf("-%x-%s", hash, chainId))...)
}

func calTxKey(hash common.Hash, chainId string) []byte {

	return append(txPrefix, []byte(fmt.Sprintf("-%x-%s", hash, chainId))...)
}

func getReader(db ethdb.Database, key []byte) io.Reader {
	bytez, err := db.Get(key)
	if bytez == nil || err != nil{
		return nil
	}
	return bytes.NewReader(bytez)
}

func decodeTx(txBytes []byte) (*types.Transaction, error) {

	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}