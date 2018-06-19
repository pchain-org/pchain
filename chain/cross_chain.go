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
	"encoding/json"
	"github.com/ethereum/go-ethereum/ethclient"
)

type CrossChainHelper struct {
	mtx  sync.Mutex
	typeMut *event.TypeMux
	chainInfoDB dbm.DB
	//the client does only connect to main chain
	client *ethclient.Client
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

func (cch *CrossChainHelper) GetClient() *ethclient.Client {
	return cch.client
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

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	tx, _, _, _ := core.GetTransaction(chainDb, txHash)

	return tx
}

//should return varified transaction
func (cch *CrossChainHelper) GetTxFromChildChain(txHash common.Hash, chainId string) *ethTypes.Transaction {

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	tx, _ := core.GetChildTransactionByHash(chainDb, txHash, chainId)

	return tx

}

//get one child chain's block by number
func (cch *CrossChainHelper) GetChildBlockByNumber(number int64, chainId string) *tdmTypes.Block {

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	block, _ := core.GetChildTdmBlockByNumber(chainDb, number, chainId)

	return block
}

//get one child chain's block by hash
func (cch *CrossChainHelper) GetChildBlockByHash(hash []byte, chainId string) *tdmTypes.Block {

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	block, _ := core.GetChildTdmBlockByHash(chainDb, hash, chainId)

	return block
}

func (cch *CrossChainHelper) VerifyTdmBlock(from common.Address, block string) error {

	var tdmBlock tdmTypes.Block
	err := json.Unmarshal([]byte(block), &tdmBlock)
	if err != nil {
		return err

	}

	return nil
}

func (cch *CrossChainHelper) SaveTdmBlock2MainBlock(block string) error {

	var tdmBlock tdmTypes.Block
	err := json.Unmarshal([]byte(block), &tdmBlock)
	if err != nil {
		return err

	}

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	return core.WriteTdmBlockWithDetail(chainDb, &tdmBlock)
}
