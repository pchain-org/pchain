package core

import (
	"math/big"
	"sync"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	dbm "github.com/tendermint/go-db"
	tdmTypes "github.com/tendermint/tendermint/types"
)

type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetTypeMutex() *event.TypeMux
	CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error
	GetChainInfoDB() dbm.DB
	GetTxFromMainChain(txHash common.Hash) *ethTypes.Transaction	//should return varified transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *ethTypes.Transaction	//should return varified transaction
	SaveBlock(block *tdmTypes.Block, chainId string) error	//Save child chain's block to main block
	GetBlockByNumber(number int64, chainId string) *tdmTypes.Block	//get one chain's block by number
	GetBlockByHash(hash common.Hash, chainId string) *tdmTypes.Block	//get one chain's block by hash
}
