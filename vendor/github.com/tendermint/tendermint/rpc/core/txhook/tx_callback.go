package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/pchain/common/plogger"
	dbm "github.com/tendermint/go-db"
	tdmTypes "github.com/tendermint/tendermint/types"
	"math/big"
	"sync"
)

type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetTypeMutex() *event.TypeMux
	CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	ReadyForLaunchChildChain(height uint64, state *state.StateDB)
	GetChainInfoDB() dbm.DB
	GetClient() *ethclient.Client
	GetTxFromMainChain(txHash common.Hash) *ethTypes.Transaction                  //should return varified transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *ethTypes.Transaction //should return varified transaction
	GetChildBlockByNumber(number int64, chainId string) *tdmTypes.Block           //get one chain's block by number
	GetChildBlockByHash(hash []byte, chainId string) *tdmTypes.Block              //get one chain's block by hash
}

var plog = plogger.GetLogger("TendermintTxCallback")
