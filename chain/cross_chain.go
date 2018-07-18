package chain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/pchain/ethermint/ethereum"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/epoch"
	tdmTypes "github.com/tendermint/tendermint/types"
	"math/big"
	"sync"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 1
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

	// Check if "chainId" has been created
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		return errors.New(fmt.Sprintf("Chain %s has already exist, try use other name instead", chainId))
	}

	// Check if "chainId" has been registered
	cci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if cci != nil {
		return errors.New(fmt.Sprintf("Chain %s has already applied, try use other name instead", chainId))
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

	cci := &core.CoreChainInfo{
		Owner:            from,
		ChainId:          chainId,
		MinValidators:    minValidators,
		MinDepositAmount: minDepositAmount,
		StartBlock:       startBlock,
		EndBlock:         endBlock,
		JoinedValidators: make([]core.JoinedValidator, 0),
	}
	core.CreatePendingChildChainData(cch.chainInfoDB, cci)

	return nil
}

// ValidateJoinChildChain check the criteria whether it meets the join child chain requirement
func (cch *CrossChainHelper) ValidateJoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error {
	plog.Debugln("ValidateJoinChildChain - start")

	if chainId == MainChain {
		return errors.New("you can't join PChain as a child chain, try use other name instead")
	}

	// Check if "chainId" has been created/registered
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		return errors.New(fmt.Sprintf("Child Chain %s not exist, try use other name instead", chainId))
	}

	// Check PubKey match the Address
	pubkeySlice := ethcrypto.FromECDSAPub(ethcrypto.ToECDSAPub(common.FromHex(pubkey)))
	if pubkeySlice == nil {
		return errors.New("your Public Key is not valid, please provide a valid Public Key")
	}

	validatorPubkey := crypto.EtherumPubKey(pubkeySlice)
	if !bytes.Equal(validatorPubkey.Address(), from.Bytes()) {
		return errors.New("your Public Key is not match with your Address, please provide a valid Public Key and Address")
	}

	// Check if already joined the chain
	find := false
	for _, joined := range ci.JoinedValidators {
		if from == joined.Address {
			find = true
			break
		}
	}

	if find {
		return errors.New(fmt.Sprintf("You have already joined the Child Chain %s", chainId))
	}

	// Check the deposit amount
	if !(depositAmount != nil && depositAmount.Sign() == 1) {
		return errors.New("deposit amount must be greater than 0")
	}

	plog.Debugln("ValidateJoinChildChain - end")
	return nil
}

// JoinChildChain Join the Child Chain
func (cch *CrossChainHelper) JoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error {
	plog.Debugln("JoinChildChain - start")

	// Load the Child Chain first
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		plog.Infof("JoinChildChain - Child Chain %s not exist, you can't join the chain\n", chainId)
		return errors.New(fmt.Sprintf("Child Chain %s not exist, you can't join the chain", chainId))
	}

	jv := core.JoinedValidator{
		PubKey:        crypto.EtherumPubKey(common.FromHex(pubkey)),
		Address:       from,
		DepositAmount: depositAmount,
	}

	ci.JoinedValidators = append(ci.JoinedValidators, jv)

	core.UpdatePendingChildChainData(cch.chainInfoDB, ci)
	plog.Debugln("JoinChildChain - end")
	return nil
}

func (cch *CrossChainHelper) ReadyForLaunchChildChain(height uint64, stateDB *state.StateDB) {
	plog.Debugln("ReadyForLaunchChildChain - start")

	readyId := core.GetChildChainForLaunch(cch.chainInfoDB, height, stateDB)
	if len(readyId) == 0 {
		plog.Infof("ReadyForLaunchChildChain - No child chain to be launch in Block %v", height)
	} else {
		plog.Infof("ReadyForLaunchChildChain - %v child chain(s) to be launch in Block %v. %v\n", len(readyId), height, readyId)
		for _, chainId := range readyId {
			cch.GetTypeMutex().Post(core.CreateChildChainEvent{ChainId: chainId})
			// Convert the Chain Info from Pending to Formal
			cci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
			core.SaveChainInfo(cch.chainInfoDB, &core.ChainInfo{CoreChainInfo: *cci})
			core.DeletePendingChildChainData(cch.chainInfoDB, chainId)
		}
	}

	plog.Debugln("ReadyForLaunchChildChain - end")
}

//should return varified transaction
func (cch *CrossChainHelper) GetTxFromMainChain(txHash common.Hash) *ethTypes.Transaction {

	chainMgr := GetCMInstance(nil)

	stack := chainMgr.mainChain.EthNode
	var backend *ethereum.Backend
	if err := stack.Service(&backend); err != nil {
		utils.Fatalf("backend service not running: %v", err)
	}
	chainDb := backend.Ethereum().ChainDb()

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
	blockId := tdmTypes.BlockID{
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
	if err != nil {
		return err
	}

	tdmBlock := intBlock.Block
	blockPartSize := intBlock.BlockPartSize
	commit := intBlock.Commit

	chainMgr := GetCMInstance(nil)
	chainDb := chainMgr.mainChain.EthNode.Backend().ChainDb()

	err = core.WriteTdmBlockWithDetail(chainDb, tdmBlock, blockPartSize, commit)
	if err != nil {
		return err
	}

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

func (cch *CrossChainHelper) RecordCrossChainTx(from common.Address, txHash common.Hash) error {
	plog.Infof("RecordCrossChainTx - from: %s, txHash: %s\n", from.Hex(), txHash.Hex())

	cch.chainInfoDB.SetSync(txHash.Bytes(), from.Bytes())
	return nil
}

func (cch *CrossChainHelper) DeleteCrossChainTx(txHash common.Hash) error {
	plog.Infof("DeleteCrossChainTx - txHash: %s\n", txHash.Hex())

	cch.chainInfoDB.DeleteSync(txHash.Bytes())
	return nil
}

func (cch *CrossChainHelper) VerifyCrossChainTx(txHash common.Hash) bool {
	return cch.chainInfoDB.Get(txHash.Bytes()) != nil
}
