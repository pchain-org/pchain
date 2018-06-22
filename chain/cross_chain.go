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
	"math/big"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 10
	OFFICIAL_MINIMUM_DEPOSIT = "100000000000000000000000" // 10,000 * e18

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
