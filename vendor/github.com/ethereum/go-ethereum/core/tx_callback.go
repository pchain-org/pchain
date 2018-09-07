package core

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	dbm "github.com/tendermint/go-db"
	"math/big"
	"sync"
)

type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetClient() *ethclient.Client
	GetChainInfoDB() dbm.DB

	CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error
	CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error
	ValidateJoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	JoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	ReadyForLaunchChildChain(height *big.Int, stateDB *state.StateDB) []string

	ValidateVoteNextEpoch(chainId string) (*epoch.Epoch, error)
	ValidateRevealVote(chainId string, from common.Address, pubkey string, depositAmount *big.Int, salt string) (*epoch.Epoch, error)

	GetTxFromMainChain(txHash common.Hash) *types.Transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *types.Transaction
	VerifyChildChainBlock(bs []byte) error
	SaveChildChainBlockToMainChain(bs []byte) error

	// these should operate on the main chain db
	AddToChildChainTx(chainId string, account common.Address, txHash common.Hash) error
	RemoveToChildChainTx(chainId string, account common.Address, txHash common.Hash) error
	HasToChildChainTx(chainId string, account common.Address, txHash common.Hash) bool
	AddFromChildChainTx(chainId string, account common.Address, txHash common.Hash) error
	RemoveFromChildChainTx(chainId string, account common.Address, txHash common.Hash) error
	HasFromChildChainTx(chainId string, account common.Address, txHash common.Hash) bool
	// these should operate on the child chain db
	AppendUsedChildChainTx(chainId string, account common.Address, txHash common.Hash) error
	HasUsedChildChainTx(chainId string, account common.Address, txHash common.Hash) bool
}

type EtdValidateCb func(tx *types.Transaction, state *state.StateDB, cch CrossChainHelper) error
type EtdApplyCb func(tx *types.Transaction, state *state.StateDB, cch CrossChainHelper) error
type EtdInsertBlockCb func(block *types.Block)

var validateCbMap = make(map[string]EtdValidateCb)
var applyCbMap = make(map[string]EtdApplyCb)
var insertBlockCbMap = make(map[string]EtdInsertBlockCb)

func RegisterValidateCb(name string, validateCb EtdValidateCb) error {

	_, ok := validateCbMap[name]
	if ok {
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
