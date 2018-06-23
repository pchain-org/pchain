package chain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/epoch"
	tdmTypes "github.com/tendermint/tendermint/types"
	"math/big"
	"sync"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 10
	OFFICIAL_MINIMUM_DEPOSIT    = "100000000000000000000000" // 100,000 * e18
)

type CrossChainHelper struct {
	mtx         sync.Mutex
	typeMut     *event.TypeMux
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

// CanCreateChildChain check the condition before send the create child chain into the tx pool
func (cch *CrossChainHelper) CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error {

	if chainId == MainChain {
		return errors.New("you can't create PChain as a child chain, try use other name instead")
	}

	// Check if "chainId" has been created/registered
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		return errors.New(fmt.Sprintf("Chain %s has already exist, try use other name instead", chainId))
	}

	// Check the minimum validators
	if minValidators < OFFICIAL_MINIMUM_VALIDATORS {
		return errors.New(fmt.Sprintf("Validators amount is not meet the minimum official validator amount (%v)", OFFICIAL_MINIMUM_VALIDATORS))
	}

	// Check the minimum deposit amount
	officialMinimumDeposit := common.String2Big(OFFICIAL_MINIMUM_DEPOSIT)
	if minDepositAmount.Cmp(officialMinimumDeposit) == -1 {
		return errors.New(fmt.Sprintf("Deposit amount is not meet the minimum official deposit amount (%v PAI)", new(big.Int).Div(officialMinimumDeposit, common.Ether)))
	}

	// Check start/end block
	if startBlock >= endBlock {
		return errors.New("start block number must be greater than end block number")
	}

	// Check End Block already passed
	lastBlockHeight := chainMgr.mainChain.TdmNode.ConsensusState().GetState().LastBlockHeight
	if endBlock < uint64(lastBlockHeight) {
		return errors.New("end block number has already passed")
	}

	// TODO Check Full node
	//chainMgr := GetCMInstance(nil)
	//epoch := chainMgr.mainChain.TdmNode.ConsensusState().Epoch
	//found := epoch.Validators.HasAddress(from.Bytes())
	//if !found {
	//	return errors.New(fmt.Sprint("You are not a validator in Main Chain, therefore child chain creation is forbidden"))
	//}

	return nil
}

// CreateChildChain Save the Child Chain Data into the DB, the data will be used later during Block Commit Callback
func (cch *CrossChainHelper) CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) error {

	fmt.Printf("cch CreateChildChain called\n")

	//write the child chain info to "multi-chain" db
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		fmt.Printf("chain %s does exist, can't create again\n", chainId)
		//return nil, because this could be executed for the same TX!!!
		return nil
	}

	coreChainInfo := core.CoreChainInfo{
		Owner:            from,
		ChainId:          chainId,
		MinValidators:    minValidators,
		MinDepositAmount: minDepositAmount,
		StartBlock:       startBlock,
		EndBlock:         endBlock,
	}

	ci = &core.ChainInfo{}
	ci.CoreChainInfo = coreChainInfo

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

//verify the signature of validators who voted for the block
func (cch *CrossChainHelper) VerifyTdmBlock(from common.Address, block string) error {

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

	//3��*data hash & num(tx)
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
}

func (cch *CrossChainHelper) SaveTdmBlock2MainBlock(block string) error {

	var intBlock tdmTypes.IntegratedBlock
	err := json.Unmarshal([]byte(block), &intBlock)
	if err != nil { return err }

	tdmBlock := intBlock.Block
	blockPartSize := intBlock.BlockPartSize
	commit := intBlock.Commit

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	err = core.WriteTdmBlockWithDetail(chainDb, tdmBlock, blockPartSize, commit)
	if err != nil { return err }

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

	return nil
}
