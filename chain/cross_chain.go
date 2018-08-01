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
	//tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"encoding/json"
	"github.com/ethereum/go-ethereum/ethclient"
	//"bytes"
	//"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
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
	//chainMgr := GetCMInstance(nil)
	//epoch := chainMgr.mainChain.EthNode.ConsensusState().Epoch
	//found := epoch.Validators.HasAddress(from.Bytes())
	found := true
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

	ci = &core.ChainInfo {}
	ci.Owner = from
	ci.ChainId = chainId

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
func (cch *CrossChainHelper) GetChildBlockByNumber(number int64, chainId string) *ethTypes.Block {

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	block, _ := core.GetChildBlockByNumber(chainDb, number, chainId)

	return block
}

//get one child chain's block by hash
func (cch *CrossChainHelper) GetChildBlockByHash(hash []byte, chainId string) *ethTypes.Block {

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	block, _ := core.GetChildBlockByHash(chainDb, hash, chainId)

	return block
}

//verify the signature of validators who voted for the block
func (cch *CrossChainHelper) VerifyBlock(from common.Address, block string) error {

	return nil
	/*
	var intBlock tdmTypes.IntegratedBlock
	err := json.Unmarshal([]byte(block), &intBlock)
	if err != nil {
		return err
	}

	tdmBlock := intBlock.Block
	commit := intBlock.Commit
	blockPartSize := intBlock.BlockPartSize

	chainId := tdmBlock.ChainID
	//1, check from is the validator of child chain
	//   and check the validator hash
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci == nil {
		return errors.New(fmt.Sprintf("chain %s not exist", chainId))
	}

	epoch := ci.GetEpochByBlockNumber(tdmBlock.Height)
	if epoch == nil {
		return errors.New(fmt.Sprintf("could not get epoch for block height %v", tdmBlock.Height))
	}

	valSet := epoch.Validators
	found := valSet.HasAddress(from.Bytes())
	if !found {
		return errors.New(fmt.Sprint("%x is not a validator of chain %s", from, chainId))
	}

	if !bytes.Equal(epoch.Validators.Hash(), tdmBlock.ValidatorsHash) {
		return errors.New("validator set gets wrong")
	}

	//2, block header check:
	//   *block hash
	// must fail here, because the blockid is for the current block
	firstParts := tdmBlock.MakePartSet(blockPartSize)
	firstPartsHeader := firstParts.Header()
	blockId :=  tdmTypes.BlockID {
		tdmBlock.Hash(),
		firstPartsHeader,
	}
	if !blockId.Equals(tdmBlock.LastBlockID) {
		return errors.New("block id gets wrong")
	}

	//3, *data hash & num(tx)
	if !bytes.Equal(tdmBlock.Data.Hash(), tdmBlock.DataHash) {
		return errors.New("data hash gets wrong")
	}

	if tdmBlock.NumTxs != len(tdmBlock.Data.Txs) {
		return errors.New("transaction number gets wrong")
	}

	//4, commit and vote check
	if !bytes.Equal(tdmBlock.LastCommitHash, tdmBlock.LastCommit.Hash()) {
		return errors.New("transaction number gets wrong")
	}

	return valSet.VerifyCommit(chainId, blockId, tdmBlock.Height, commit)
	*/
}

func (cch *CrossChainHelper) SaveBlock2MainBlock(blockStr string) error {

	block := ethTypes.Block{}
	err := json.Unmarshal([]byte(blockStr), &block)
	if err != nil { return err }

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	err = core.WriteChildBlockWithDetail(chainDb, &block)
	if err != nil { return err }
	/*
	//here is epoch update; should be a more general mechanism
	if tdmBlock.BlockExData != nil && len(tdmBlock.BlockExData) != 0 {

		ep := epoch.FromBytes(tdmBlock.BlockExData)
		if ep != nil {
			ci := core.GetChainInfo(cch.chainInfoDB, tdmBlock.ChainID)
			if ep.Number > ci.EpochNumber {
				ci.EpochNumber = ep.Number
				ci.Epoch = ep
				core.SaveChainInfo(cch.chainInfoDB, ci)
			}
		}
	}
	*/
	return nil
}
