package chain

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"golang.org/x/crypto/sha3"
	"math/big"
	"sync"
	"time"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 1
	OFFICIAL_MINIMUM_DEPOSIT    = "100000000000000000000000" // 100,000 * e18
)

type CrossChainHelper struct {
	mtx         sync.Mutex
	chainInfoDB dbm.DB
	//the client does only connect to main chain
	client *ethclient.Client
}

func (cch *CrossChainHelper) GetMutex() *sync.Mutex {
	return &cch.mtx
}

func (cch *CrossChainHelper) GetChainInfoDB() dbm.DB {
	return cch.chainInfoDB
}

func (cch *CrossChainHelper) GetClient() *ethclient.Client {
	return cch.client
}

// CanCreateChildChain check the condition before send the create child chain into the tx pool
func (cch *CrossChainHelper) CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error {

	if chainId == MainChain {
		return errors.New("you can't create PChain as a child chain, try use other name instead")
	}

	// Check if "chainId" has been created
	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		return fmt.Errorf("Chain %s has already exist, try use other name instead", chainId)
	}

	// Check if "chainId" has been registered
	cci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if cci != nil {
		return fmt.Errorf("Chain %s has already applied, try use other name instead", chainId)
	}

	// Check the minimum validators
	if minValidators < OFFICIAL_MINIMUM_VALIDATORS {
		return fmt.Errorf("Validators amount is not meet the minimum official validator amount (%v)", OFFICIAL_MINIMUM_VALIDATORS)
	}

	// Check the minimum deposit amount
	officialMinimumDeposit := math.MustParseBig256(OFFICIAL_MINIMUM_DEPOSIT)
	if minDepositAmount.Cmp(officialMinimumDeposit) == -1 {
		return fmt.Errorf("Deposit amount is not meet the minimum official deposit amount (%v PAI)", new(big.Int).Div(officialMinimumDeposit, big.NewInt(params.Ether)))
	}

	// Check start/end block
	if startBlock.Cmp(endBlock) >= 0 {
		return errors.New("start block number must be less than end block number")
	}

	// Check End Block already passed
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	currentBlock := ethereum.BlockChain().CurrentBlock()
	if endBlock.Cmp(currentBlock.Number()) <= 0 {
		return errors.New("end block number has already passed")
	}

	return nil
}

// CreateChildChain Save the Child Chain Data into the DB, the data will be used later during Block Commit Callback
func (cch *CrossChainHelper) CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error {
	logger.Debug("CreateChildChain - start")

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

	logger.Debug("CreateChildChain - end")
	return nil
}

// ValidateJoinChildChain check the criteria whether it meets the join child chain requirement
func (cch *CrossChainHelper) ValidateJoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error {
	logger.Debug("ValidateJoinChildChain - start")

	if chainId == MainChain {
		return errors.New("you can't join PChain as a child chain, try use other name instead")
	}

	// Check if "chainId" has been created/registered
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		return fmt.Errorf("Child Chain %s not exist, try use other name instead", chainId)
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

	logger.Debug("ValidateJoinChildChain - end")
	return nil
}

// JoinChildChain Join the Child Chain
func (cch *CrossChainHelper) JoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error {
	logger.Debugln("JoinChildChain - start")

	// Load the Child Chain first
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		logger.Errorf("JoinChildChain - Child Chain %s not exist, you can't join the chain", chainId)
		return fmt.Errorf("Child Chain %s not exist, you can't join the chain", chainId)
	}

	jv := core.JoinedValidator{
		PubKey:        crypto.EtherumPubKey(common.FromHex(pubkey)),
		Address:       from,
		DepositAmount: depositAmount,
	}

	ci.JoinedValidators = append(ci.JoinedValidators, jv)

	core.UpdatePendingChildChainData(cch.chainInfoDB, ci)

	logger.Debugln("JoinChildChain - end")
	return nil
}

func (cch *CrossChainHelper) ReadyForLaunchChildChain(height *big.Int, stateDB *state.StateDB) []string {
	logger.Debugln("ReadyForLaunchChildChain - start")

	readyId := core.GetChildChainForLaunch(cch.chainInfoDB, height, stateDB)
	if len(readyId) == 0 {
		logger.Debugf("ReadyForLaunchChildChain - No child chain to be launch in Block %v", height)
	} else {
		logger.Infof("ReadyForLaunchChildChain - %v child chain(s) to be launch in Block %v. %v\n", len(readyId), height, readyId)
		//for _, chainId := range readyId {
		//	// Convert the Chain Info from Pending to Formal
		//	cci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
		//	core.SaveChainInfo(cch.chainInfoDB, &core.ChainInfo{CoreChainInfo: *cci})
		//	// Send Post to Chain Manager
		//	cch.GetTypeMutex().Post(core.CreateChildChainEvent{ChainId: chainId})
		//}
	}

	logger.Debugln("ReadyForLaunchChildChain - end")
	return readyId
}

func (cch *CrossChainHelper) ValidateVoteNextEpoch(chainId string) (*epoch.Epoch, error) {

	var ethereum *eth.Ethereum
	if chainId == MainChain {
		ethereum = MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	} else {
		ethereum = MustGetEthereumFromNode(chainMgr.childChains[chainId].EthNode)
	}

	var ep *epoch.Epoch
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}

	if ep == nil {
		return nil, errors.New("epoch is nil, are you running on Tendermint Consensus Engine")
	}

	// Check Epoch in Hash Vote stage
	if ep.NextEpoch == nil {
		return nil, errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 75% - 85%
	height := ethereum.BlockChain().CurrentBlock().NumberU64()
	if !ep.CheckInHashVoteStage(height) {
		return nil, errors.New(fmt.Sprintf("you can't send the hash vote during this time, current height %v", height))
	}
	return ep, nil
}

func (cch *CrossChainHelper) ValidateRevealVote(chainId string, from common.Address, pubkey string, depositAmount *big.Int, salt string) (*epoch.Epoch, error) {
	// Check PubKey match the Address
	pubkeySlice := ethcrypto.FromECDSAPub(ethcrypto.ToECDSAPub(common.FromHex(pubkey)))
	if pubkeySlice == nil {
		return nil, errors.New("your Public Key is not valid, please provide a valid Public Key")
	}

	validatorPubkey := crypto.EtherumPubKey(pubkeySlice)
	if !bytes.Equal(validatorPubkey.Address(), from.Bytes()) {
		return nil, errors.New("your Public Key is not match with your Address, please provide a valid Public Key and Address")
	}

	var ethereum *eth.Ethereum
	if chainId == MainChain {
		ethereum = MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	} else {
		ethereum = MustGetEthereumFromNode(chainMgr.childChains[chainId].EthNode)
	}

	var ep *epoch.Epoch
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}

	if ep == nil {
		return nil, errors.New("epoch is nil, are you running on Tendermint Consensus Engine")
	}

	// Check Epoch in Reveal Vote stage
	if ep.NextEpoch == nil {
		return nil, errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 85% - 95%
	height := ethereum.BlockChain().CurrentBlock().NumberU64()
	if !ep.CheckInRevealVoteStage(height) {
		return nil, errors.New(fmt.Sprintf("you can't send the reveal vote during this time, current height %v", height))
	}

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	// Check Vote exist
	if !exist {
		return nil, errors.New(fmt.Sprintf("Can not found the vote for Address %x", from))
	}

	if len(vote.VoteHash) == 0 {
		return nil, errors.New(fmt.Sprintf("Address %x doesn't has vote hash", from))
	}

	// Check Vote Hash
	byte_data := [][]byte{
		from.Bytes(),
		common.FromHex(pubkey),
		depositAmount.Bytes(),
		[]byte(salt),
	}
	voteHash := sha3.Sum256(concatCopyPreAllocate(byte_data))
	if vote.VoteHash != voteHash {
		return nil, errors.New("your vote doesn't match your vote hash, please check your vote")
	}

	return ep, nil
}

func (cch *CrossChainHelper) GetTxFromMainChain(txHash common.Hash) *types.Transaction {

	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	tx, _, _, _ := core.GetTransaction(chainDb, txHash)
	return tx
}

func (cch *CrossChainHelper) GetTxFromChildChain(txHash common.Hash, chainId string) *types.Transaction {

	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	tx, _ := core.GetChildChainTransactionByHash(chainDb, chainId, txHash)
	return tx
}

// verify the signature of validators who voted for the block
// most of the logic here is from 'VerifyHeader'
func (cch *CrossChainHelper) VerifyChildChainBlock(from common.Address, bs []byte) error {

	logger.Debugln("VerifyChildChainBlock - start")

	var block types.Block
	err := rlp.DecodeBytes(bs, &block)
	if err != nil {
		return err
	}

	header := block.Header()
	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return errors.New("block in the future")
	}

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain {
		return fmt.Errorf("invalid child chain id: %s", chainId)
	}

	if header.Nonce != (types.TendermintEmptyNonce) && !bytes.Equal(header.Nonce[:], types.TendermintNonce) {
		return errors.New("invalid nonce")
	}

	if header.MixDigest != types.TendermintDigest {
		return errors.New("invalid mix digest")
	}

	if header.UncleHash != types.TendermintNilUncleHash {
		return errors.New("invalid uncle Hash")
	}

	if header.Difficulty == nil || header.Difficulty.Cmp(types.TendermintDefaultDifficulty) != 0 {
		return errors.New("invalid difficulty")
	}

	ci := core.GetChainInfo(cch.chainInfoDB, chainId)
	if ci == nil {
		return fmt.Errorf("chain info %s not found", chainId)
	}
	epoch := ci.GetEpochByBlockNumber(tdmExtra.Height)
	if epoch == nil {
		return fmt.Errorf("could not get epoch for block height %v", tdmExtra.Height)
	}
	valSet := epoch.Validators
	if !bytes.Equal(valSet.Hash(), tdmExtra.ValidatorsHash) {
		return errors.New("inconsistent validator set")
	}

	seenCommit := tdmExtra.SeenCommit
	if !bytes.Equal(tdmExtra.SeenCommitHash, seenCommit.Hash()) {
		return errors.New("invalid committed seals")
	}

	if err = valSet.VerifyCommit(tdmExtra.ChainID, tdmExtra.Height, seenCommit); err != nil {
		return err
	}

	logger.Debugln("VerifyChildChainBlock - end")
	return nil
}

func (cch *CrossChainHelper) SaveChildChainBlockToMainChain(bs []byte) error {

	logger.Debugln("SaveChildChainBlockToMainChain - start")

	var block types.Block
	err := rlp.DecodeBytes(bs, &block)
	if err != nil {
		return err
	}

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(block.Header())
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain {
		return fmt.Errorf("invalid child chain id: %s", chainId)
	}

	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()
	err = core.WriteChildChainBlock(chainDb, &block)
	if err != nil {
		return err
	}

	//here is epoch update; should be a more general mechanism
	if len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil {
			ci := core.GetChainInfo(cch.chainInfoDB, tdmExtra.ChainID)
			if ep.Number == 0 || ep.Number > ci.EpochNumber {
				ci.EpochNumber = ep.Number
				ci.Epoch = ep
				core.SaveChainInfo(cch.chainInfoDB, ci)
			}
		}
	}

	logger.Debugln("SaveChildChainBlockToMainChain - end")
	return nil
}

func (cch *CrossChainHelper) AddToChildChainTx(chainId string, account common.Address, txHash common.Hash) error {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.AddCrossChainTx(chainDb, core.MainChainToChildChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) RemoveToChildChainTx(chainId string, account common.Address, txHash common.Hash) error {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.RemoveCrossChainTx(chainDb, core.MainChainToChildChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) HasToChildChainTx(chainId string, account common.Address, txHash common.Hash) bool {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.HasCrossChainTx(chainDb, core.MainChainToChildChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) AddFromChildChainTx(chainId string, account common.Address, txHash common.Hash) error {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.AddCrossChainTx(chainDb, core.ChildChainToMainChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) RemoveFromChildChainTx(chainId string, account common.Address, txHash common.Hash) error {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.RemoveCrossChainTx(chainDb, core.ChildChainToMainChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) HasFromChildChainTx(chainId string, account common.Address, txHash common.Hash) bool {
	chainMgr := GetCMInstance(nil)
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.HasCrossChainTx(chainDb, core.ChildChainToMainChain, chainId, account, txHash)
}

func (cch *CrossChainHelper) AppendUsedChildChainTx(chainId string, account common.Address, txHash common.Hash) error {
	chainMgr := GetCMInstance(nil)
	chain := chainMgr.childChains[chainId]
	ethereum := MustGetEthereumFromNode(chain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.AppendUsedChildChainTx(chainDb, chainId, account, txHash)
}

func (cch *CrossChainHelper) HasUsedChildChainTx(chainId string, account common.Address, txHash common.Hash) bool {
	chainMgr := GetCMInstance(nil)
	chain := chainMgr.childChains[chainId]
	ethereum := MustGetEthereumFromNode(chain.EthNode)
	chainDb := ethereum.ChainDb()

	return core.HasUsedChildChainTx(chainDb, chainId, account, txHash)
}

func MustGetEthereumFromNode(node *node.Node) *eth.Ethereum {
	ethereum, err := getEthereumFromNode(node)
	if err != nil {
		panic("getEthereumFromNode error: " + err.Error())
	}
	return ethereum
}

func getEthereumFromNode(node *node.Node) (*eth.Ethereum, error) {
	var ethereum *eth.Ethereum
	if err := node.Service(&ethereum); err != nil {
		return nil, err
	}

	return ethereum, nil
}

func concatCopyPreAllocate(slices [][]byte) []byte {
	var totalLen int
	for _, s := range slices {
		totalLen += len(s)
	}
	tmp := make([]byte, totalLen)
	var i int
	for _, s := range slices {
		i += copy(tmp[i:], s)
	}
	return tmp
}
