package core

import (
	"errors"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)


type ReceiveTxCb func() error
type CheckTxCb func(tx *ethTypes.Transaction) error
type DeliverTxCb func(tx *ethTypes.Transaction) error
type CommitCb func(brCommit BrCommit) error
type RefreshABCIResponseCb func() error

var receiveTxCbMap map[string]ReceiveTxCb = make(map[string]ReceiveTxCb)
var checkTxCbMap    map[string]CheckTxCb = make(map[string]CheckTxCb)
var deliverTxCbMap    map[string]DeliverTxCb = make(map[string]DeliverTxCb)
var commitCbMap    map[string]CommitCb = make(map[string]CommitCb)
var refreshABCIResponseCbMap    map[string]RefreshABCIResponseCb = make(map[string]RefreshABCIResponseCb)

func RegisterReceiveTxCb(etdFuncName string, receiveTxCb ReceiveTxCb) error {

	//fmt.Printf("RegisterValidateCb (name, validateCb) = (%s, %v)\n", name, validateCb)
	_, ok := receiveTxCbMap[etdFuncName]
	if ok {
		//fmt.Printf("RegisterValidateCb return (%v, %v)\n", cb, ok)
		return errors.New("the name has registered in receiveTxCbMap")
	}

	receiveTxCbMap[etdFuncName] = receiveTxCb
	return nil
}

func GetReceiveTxCb(etdFuncName string) ReceiveTxCb {

	cb, ok := receiveTxCbMap[etdFuncName]
	if ok {
		return cb
	}

	return nil
}

func RegisterCheckTxCb(etdFuncName string, checkTxCb CheckTxCb) error {

	_, ok := checkTxCbMap[etdFuncName]
	if ok {
		return errors.New("the name has registered in checkTxCbMap")
	}

	checkTxCbMap[etdFuncName] = checkTxCb

	return nil
}

func GetCheckTxCb(etdFuncName string) CheckTxCb {

	cb, ok := checkTxCbMap[etdFuncName]
	if ok {
		return cb
	}

	return nil
}

func RegisterDeliverTxCb(etdFuncName string, deliverTxCb DeliverTxCb) error {

	_, ok := deliverTxCbMap[etdFuncName]
	if ok {
		return errors.New("the name has registered in checkTxCbMap")
	}

	deliverTxCbMap[etdFuncName] = deliverTxCb

	return nil
}

func GetDeliverTxCb(etdFuncName string) DeliverTxCb {

	cb, ok := deliverTxCbMap[etdFuncName]
	if ok {
		return cb
	}

	return nil
}

func RegisterCommitCb(etdFuncName string, commitCb CommitCb) error {

	_, ok := commitCbMap[etdFuncName]
	if ok {
		return errors.New("the name has registered in checkTxCbMap")
	}

	commitCbMap[etdFuncName] = commitCb

	return nil
}

func GetCommitCb(etdFuncName string) CommitCb {

	cb, ok := commitCbMap[etdFuncName]
	if ok {
		return cb
	}

	return nil
}

func GetCommitCbMap() map[string]CommitCb{
	return commitCbMap
}

func RegisterRefreshABCIResponseCbMap(etdFuncName string, refreshABCIResponseCb RefreshABCIResponseCb) error {

	_, ok := refreshABCIResponseCbMap[etdFuncName]
	if ok {
		return errors.New("the name has registered in checkTxCbMap")
	}

	refreshABCIResponseCbMap[etdFuncName] = refreshABCIResponseCb

	return nil
}

func GetRefreshABCIResponseCb(etdFuncName string) RefreshABCIResponseCb {

	cb, ok := refreshABCIResponseCbMap[etdFuncName]
	if ok {
		return cb
	}

	return nil
}

func GetRefreshABCIResponseCbMap() map[string]RefreshABCIResponseCb{
	return refreshABCIResponseCbMap
}

