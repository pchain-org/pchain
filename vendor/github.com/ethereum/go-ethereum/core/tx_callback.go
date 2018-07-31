package core

import (

	"errors"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"sync"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	dbm "github.com/tendermint/go-db"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/ethclient"
)


type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetTypeMutex() *event.TypeMux
	CanCreateChildChain(from common.Address, chainId string) error
	CreateChildChain(from common.Address, chainId string) error
	GetChainInfoDB() dbm.DB
	GetClient() *ethclient.Client
	GetTxFromMainChain(txHash common.Hash) *types.Transaction	//should return varified transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *types.Transaction	//should return varified transaction
	VerifyTdmBlock(from common.Address, block string) error
	SaveTdmBlock2MainBlock(block string) error
	GetChildBlockByNumber(number int64, chainId string) *tdmTypes.Block	//get one chain's block by number
	GetChildBlockByHash(hash []byte, chainId string) *tdmTypes.Block	//get one chain's block by hash
}


type EtdValidateCb func(tx *types.Transaction, state *st.StateDB, cch CrossChainHelper) error
type EtdApplyCb func(tx *types.Transaction, state *st.StateDB, cch CrossChainHelper) error
type EtdInsertBlockCb func(block *types.Block)

var validateCbMap 		map[string]EtdValidateCb = make(map[string]EtdValidateCb)
var applyCbMap    		map[string]EtdApplyCb = make(map[string]EtdApplyCb)
var insertBlockCbMap    map[string]EtdInsertBlockCb = make(map[string]EtdInsertBlockCb)

func RegisterValidateCb(name string, validateCb EtdValidateCb) error {

	//fmt.Printf("RegisterValidateCb (name, validateCb) = (%s, %v)\n", name, validateCb)
	_, ok := validateCbMap[name]
	if ok {
		//fmt.Printf("RegisterValidateCb return (%v, %v)\n", cb, ok)
		return errors.New("the name has registered in validateCbMap")
	}

	validateCbMap[name] = validateCb
	return nil
}

func GetValidateCb(name string) EtdValidateCb {

	cb, ok := validateCbMap[name]
	if ok {
		return cb
	}

	return nil
}

func RegisterApplyCb(name string, applyCb EtdApplyCb) error {

	_, ok := applyCbMap[name]
	if ok {
		return errors.New("the name has registered in applyCbMap")
	}

	applyCbMap[name] = applyCb

	return nil
}

func GetApplyCb(name string) EtdApplyCb {

	cb, ok := applyCbMap[name]
	if ok {
		return cb
	}

	return nil
}


func RegisterInsertBlockCb(name string, insertBlockCb EtdInsertBlockCb) error {

	_, ok := insertBlockCbMap[name]
	if ok {
		return errors.New("the name has registered in insertBlockCbMap")
	}

	insertBlockCbMap[name] = insertBlockCb

	return nil
}

func GetInsertBlockCb(name string) EtdInsertBlockCb {

	cb, ok := insertBlockCbMap[name]
	if ok {
		return cb
	}

	return nil
}

func GetInsertBlockCbMap() map[string]EtdInsertBlockCb {

	return insertBlockCbMap
}

