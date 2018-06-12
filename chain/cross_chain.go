package chain

import (
	"sync"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
	"github.com/ethereum/go-ethereum/event"
	dbm "github.com/tendermint/go-db"
	"errors"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	tdmTypes "github.com/tendermint/tendermint/types"
)

type CrossChainHelper struct {
	mtx  sync.Mutex
	typeMut *event.TypeMux
	chainInfoDB dbm.DB
}

func (cch *CrossChainHelper) GetMutex() *sync.Mutex {
	return &cch.mtx
}

func (cch *CrossChainHelper) GetTypeMutex() *event.TypeMux {
	return cch.typeMut
}

func (cch *CrossChainHelper) GetChainInfoDB() dbm.DB {
	return cch.chainInfoDB
}

//TODO multi-chain
func (cch *CrossChainHelper) CanCreateChildChain(from common.Address, chainId string) error {

	fmt.Printf("cch CanCreateChildChain called")

	//check if "chainId" has been created/registered
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		return errors.New(fmt.Sprintf("chain %s does exist, can't create again", chainId))
	}

	//check if "from" is a legal validator in main chain
	chainMgr := GetCMInstance(nil)
	epoch := chainMgr.mainChain.TdmNode.ConsensusState().Epoch
	found := epoch.Validators.HasAddress(from.Bytes())
	if !found {
		return errors.New(fmt.Sprint("You are not a validator in Main Chain, therefore child chain creation is forbidden"))
	}

	// TODO Add More check
	return nil
}

func (cch *CrossChainHelper) CreateChildChain(from common.Address, chainId string) error {

	fmt.Printf("cch CreateChildChain called\n")

	//write the child chain info to "multi-chain" db
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		fmt.Printf("chain %s does exist, can't create again\n", chainId)
		//return nil, because this could be executed for the same TX!!!
		return nil
	}

	ci = &core.ChainInfo {Owner: from, ChainId: chainId}

	core.SaveChainInfo(cch.chainInfoDB, ci)

	return nil
}

//should return varified transaction
func (cch *CrossChainHelper) GetTxFromMainChain(txHash common.Hash) *ethTypes.Transaction {
	return nil
}

//should return varified transaction
func (cch *CrossChainHelper) GetTxFromChildChain(txHash common.Hash, chainId string) *ethTypes.Transaction {
	return nil
}

//Save child chain's block to main block
func (cch *CrossChainHelper) SaveBlock(block *tdmTypes.Block, chainId string) error {
	return nil
}

//get one chain's block by number
func (cch *CrossChainHelper) GetBlockByNumber(number int64, chainId string) *tdmTypes.Block {
	return nil
}

//get one chain's block by hash
func (cch *CrossChainHelper) GetBlockByHash(hash common.Hash, chainId string) *tdmTypes.Block {
	return nil
}
