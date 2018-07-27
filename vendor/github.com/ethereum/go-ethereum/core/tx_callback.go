package core

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	dbm "github.com/tendermint/go-db"
	"math/big"
	"sync"
)

type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetTypeMutex() *event.TypeMux
	CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	ValidateJoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	JoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	ReadyForLaunchChildChain(height uint64, state *st.StateDB)
	GetChainInfoDB() dbm.DB
	GetTxFromMainChain(txHash common.Hash) *types.Transaction                  //should return varified transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *types.Transaction //should return varified transaction
	VerifyTdmBlock(from common.Address, block []byte) error
	SaveTdmBlock2MainBlock(block []byte) error
	RecordCrossChainTx(from common.Address, txHash common.Hash) error
	DeleteCrossChainTx(txHash common.Hash) error
	VerifyCrossChainTx(txHash common.Hash) bool
}

type EtdValidateCb func(tx *types.Transaction, state *st.StateDB, cch CrossChainHelper) error
type EtdApplyCb func(tx *types.Transaction, state *st.StateDB, cch CrossChainHelper) error

var validateCbMap map[string]EtdValidateCb = make(map[string]EtdValidateCb)
var applyCbMap map[string]EtdApplyCb = make(map[string]EtdApplyCb)

func RegisterValidateCb(name string, validateCb EtdValidateCb) error {

	//fmt.Printf("RegisterValidateCb (name, validateCb) = (%s, %v)\n", name, validateCb)
	_, ok := validateCbMap[name]
	if ok {
		//fmt.Printf("RegisterValidateCb return (%v, %v)\n", cb, ok)
		return errors.New("the name has registered in ValidateCbMap")
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
		return errors.New("the name has registered in ValidateCbMap")
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
