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
	"github.com/tendermint/go-wire"
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

func GetChildTdmBlockByNumber(db ethdb.Database, blockNumber int64, chainId string) (*tdmTypes.Block, error) {

	key := calBlockKeyByNumber(blockNumber, chainId)

	r := getReader(db, key)
	if r == nil {
		return nil, nil
	}

	var n int
	var err error

	block := wire.ReadBinary(&tdmTypes.Block{}, r, 0, &n, &err).(*tdmTypes.Block)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading block : %v", err))
	}

	return block, nil
}

func GetChildTdmBlockByHash(db ethdb.Database, blockHash []byte, chainId string) (*tdmTypes.Block, error) {

	key := calBlockKeyByHash(blockHash, chainId)

	r := getReader(db, key)
	if r == nil {
		return nil, nil
	}

	var n int
	var err error

	block := wire.ReadBinary(&tdmTypes.Block{}, r, 0, &n, &err).(*tdmTypes.Block)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading block : %v", err))
	}

	return block, nil
}

//notice: the 'txHash' is the hash format of ethereum, not tendermint
func GetChildTransactionByHash(db ethdb.Database, txHash common.Hash, chainId string) (*types.Transaction, error) {

	key := calTxKey(txHash, chainId)

	r := getReader(db, key)
	if r == nil {
		return nil, nil
	}

	var n int
	var err error

	tx := wire.ReadBinary(&types.Transaction{}, r, 0, &n, &err).(*types.Transaction)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading block : %v", err))
	}

	return tx, nil
}

func DeleteTdmBlockWithDetail(db ethdb.Database, number int64, chainId string) error {
	/*
	tdmBlock, err := GetChildTdmBlockByNumber(db, number, chainId)
	if err != nil {return err}

	DeleteTdmBlock(db, number, chainId)
	if err != nil {return err}

	for _, tx := range tdmBlock.Txs {

		err = DeleteTdmTransactions(db, tx.Hash(), chainId)
		if err != nil {return err}
	}

	err = DeleteTdmExtraData(db, number, chainId)
	if err != nil {return err}

	err = DeleteTdmBlockPartSize(db, number, chainId)
	if err != nil {return err}

	return DeleteTdmCommits(db, number, chainId)
	*/
	return nil
}

func DeleteTdmBlock(db ethdb.Database, number int64, chainId string) error {

	return db.Delete(calBlockKeyByNumber(number, chainId))
}

func DeleteTdmTransactions(db ethdb.Database, txHash common.Hash, chainId string) error {

	return db.Delete(calTxKey(txHash, chainId))
}

func DeleteTdmExtraData(db ethdb.Database, number int64, chainId string) error {

	return db.Delete(calExtraDataKey(number, chainId))
}

func DeleteTdmBlockPartSize(db ethdb.Database, number int64, chainId string) error {

	return db.Delete(calBlockPartSizeKey(number, chainId))
}

func DeleteTdmCommits(db ethdb.Database, number int64, chainId string) error {

	return db.Delete(calCommitKey(number, chainId))
}


func WriteTdmBlockWithDetail(db ethdb.Database, tdmBlock *tdmTypes.Block, blockPartSize int, commit *tdmTypes.Commit) error {

	WriteTdmBlock(db, tdmBlock)
	WriteTdmTransactions(db, tdmBlock)
	WriteTdmExtraData(db, tdmBlock)
	WriteTdmBlockPartSize(db, tdmBlock, blockPartSize)
	WriteTdmCommits(db, tdmBlock, commit)

	return nil
}

func WriteTdmBlock(db ethdb.Database, tdmBlock *tdmTypes.Block) error {

	/*
	//chainId := tdmBlock.ChainID
	chainId := ""

	blockByte := wire.BinaryBytes(tdmBlock)
	key := calBlockKeyByHash(tdmBlock.EthBlock.Hash().Bytes(), chainId)

	err := db.Put(key, blockByte)
	if err != nil {
		return err
	}

	key = calBlockKeyByNumber(tdmBlock.EthBlock.Number().Int64(), chainId)
	return db.Put(key, blockByte)
	*/
	return nil
}

func WriteTdmTransactions(db ethdb.Database, tdmBlock *tdmTypes.Block) error {

	/*
	chainId := ""

	if len(tdmBlock.EthBlock.Transactions()) == 0 {
		return nil
	}

	for _, tx := range tdmBlock.EthBlock.Transactions() {

		key := calTxKey(tx.Hash(), chainId)//tdmBlock.ChainID

		err := db.Put(key, tx.Data())
		if err != nil {
			return err
		}
	}
	*/
	return nil
}

func WriteTdmExtraData(db ethdb.Database, tdmBlock *tdmTypes.Block) error {

	/*
	if len(tdmBlock.BlockExData) == 0 {
		return nil
	}

	key := calExtraDataKey(int64(tdmBlock.Height), tdmBlock.ChainID)
	return db.Put(key, tdmBlock.BlockExData)
	*/
	return nil
}

func WriteTdmBlockPartSize(db ethdb.Database, tdmBlock *tdmTypes.Block, blockPartSize int) error {
/*
	key := calBlockPartSizeKey(tdmBlock.EthBlock.Number().Int64(), "")//tdmBlock.ChainID
	bpsByte := wire.BinaryBytes(blockPartSize)

	return db.Put(key, bpsByte)
*/
	return nil
}

func WriteTdmCommits(db ethdb.Database, tdmBlock *tdmTypes.Block, commit *tdmTypes.Commit) error {
/*
	commitByte := wire.BinaryBytes(commit)
	key := calCommitKey(tdmBlock.EthBlock.Number().Int64(), "")//tdmBlock.ChainID

	return db.Put(key, commitByte)
*/
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

func calExtraDataKey(number int64, chainId string) []byte {

	return append(extraDataPrefix, []byte(fmt.Sprintf("-%v-%s", number, chainId))...)
}

func calBlockPartSizeKey(number int64, chainId string) []byte {

	return append(blockPartSizePrefix, []byte(fmt.Sprintf("-%v-%s", number, chainId))...)
}

func calCommitKey(number int64, chainId string) []byte {

	return append(commitPrefix, []byte(fmt.Sprintf("-%v-%s", number, chainId))...)
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