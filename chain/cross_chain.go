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
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	pabi "github.com/pchain/abi"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"math/big"
	"sync"
	"time"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 1
	OFFICIAL_MINIMUM_DEPOSIT    = "100000000000000000000000" // 100,000 * e18
)

type CrossChainHelper struct {
	mtx             sync.Mutex
	chainInfoDB     dbm.DB
	localTX3CacheDB ethdb.Database
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
		return fmt.Errorf("Deposit amount is not meet the minimum official deposit amount (%v PI)", new(big.Int).Div(officialMinimumDeposit, big.NewInt(params.PI)))
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
	log.Debug("CreateChildChain - start")

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

	log.Debug("CreateChildChain - end")
	return nil
}

// ValidateJoinChildChain check the criteria whether it meets the join child chain requirement
func (cch *CrossChainHelper) ValidateJoinChildChain(from common.Address, consensusPubkey []byte, chainId string, depositAmount *big.Int, signature []byte) error {
	log.Debug("ValidateJoinChildChain - start")

	if chainId == MainChain {
		return errors.New("you can't join PChain as a child chain, try use other name instead")
	}

	// Check Signature of the PubKey matched against the Address
	if err := crypto.CheckConsensusPubKey(from, consensusPubkey, signature); err != nil {
		return err
	}

	// Check if "chainId" has been created/registered
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		if core.GetChainInfo(cch.chainInfoDB, chainId) != nil {
			return fmt.Errorf("chain %s has already created/started, try use other name instead", chainId)
		} else {
			return fmt.Errorf("child chain %s not exist, try use other name instead", chainId)
		}
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

	log.Debug("ValidateJoinChildChain - end")
	return nil
}

// JoinChildChain Join the Child Chain
func (cch *CrossChainHelper) JoinChildChain(from common.Address, pubkey crypto.PubKey, chainId string, depositAmount *big.Int) error {
	log.Debug("JoinChildChain - start")

	// Load the Child Chain first
	ci := core.GetPendingChildChainData(cch.chainInfoDB, chainId)
	if ci == nil {
		log.Errorf("JoinChildChain - Child Chain %s not exist, you can't join the chain", chainId)
		return fmt.Errorf("Child Chain %s not exist, you can't join the chain", chainId)
	}

	for _, joined := range ci.JoinedValidators {
		if from == joined.Address {
			return nil
		}
	}

	jv := core.JoinedValidator{
		PubKey:        pubkey,
		Address:       from,
		DepositAmount: depositAmount,
	}

	ci.JoinedValidators = append(ci.JoinedValidators, jv)

	core.UpdatePendingChildChainData(cch.chainInfoDB, ci)

	log.Debug("JoinChildChain - end")
	return nil
}

func (cch *CrossChainHelper) ReadyForLaunchChildChain(height *big.Int, stateDB *state.StateDB) ([]string, []byte, []string) {
	log.Debug("ReadyForLaunchChildChain - start")

	readyId, updateBytes, removedId := core.GetChildChainForLaunch(cch.chainInfoDB, height, stateDB)
	if len(readyId) == 0 {
		log.Debugf("ReadyForLaunchChildChain - No child chain to be launch in Block %v", height)
	} else {
		log.Infof("ReadyForLaunchChildChain - %v child chain(s) to be launch in Block %v. %v", len(readyId), height, readyId)
	}

	log.Debug("ReadyForLaunchChildChain - end")
	return readyId, updateBytes, removedId
}

func (cch *CrossChainHelper) ProcessPostPendingData(newPendingIdxBytes []byte, deleteChildChainIds []string) {
	core.ProcessPostPendingData(cch.chainInfoDB, newPendingIdxBytes, deleteChildChainIds)
}

func (cch *CrossChainHelper) VoteNextEpoch(ep *epoch.Epoch, from common.Address, voteHash common.Hash, txHash common.Hash) error {

	voteSet := ep.GetNextEpoch().GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Overwrite the Previous Hash Vote
		vote.VoteHash = voteHash
		vote.TxHash = txHash
	} else {
		// Create a new Hash Vote
		vote = &epoch.EpochValidatorVote{
			Address:  from,
			VoteHash: voteHash,
			TxHash:   txHash,
		}
		voteSet.StoreVote(vote)
	}
	// Save the VoteSet
	epoch.SaveEpochVoteSet(ep.GetDB(), ep.GetNextEpoch().Number, voteSet)
	return nil
}

func (cch *CrossChainHelper) RevealVote(ep *epoch.Epoch, from common.Address, pubkey crypto.PubKey, depositAmount *big.Int, salt string, txHash common.Hash) error {

	voteSet := ep.GetNextEpoch().GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Update the Hash Vote with Real Data
		vote.PubKey = pubkey
		vote.Amount = depositAmount
		vote.Salt = salt
		vote.TxHash = txHash
	}
	// Save the VoteSet
	epoch.SaveEpochVoteSet(ep.GetDB(), ep.GetNextEpoch().Number, voteSet)
	return nil
}

func (cch *CrossChainHelper) GetHeightFromMainChain() *big.Int {
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	return ethereum.BlockChain().CurrentBlock().Number()
}

func (cch *CrossChainHelper) GetTxFromMainChain(txHash common.Hash) *types.Transaction {
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	chainDb := ethereum.ChainDb()

	tx, _, _, _ := core.GetTransaction(chainDb, txHash)
	return tx
}

func (cch *CrossChainHelper) GetEpochFromMainChain() *epoch.Epoch {
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	var ep *epoch.Epoch
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}
	return ep
}

// verify the signature of validators who voted for the block
// most of the logic here is from 'VerifyHeader'
func (cch *CrossChainHelper) VerifyChildChainProofData(bs []byte) error {

	log.Debug("VerifyChildChainProofData - start")

	var proofData types.ChildChainProofData
	err := rlp.DecodeBytes(bs, &proofData)
	if err != nil {
		return err
	}

	header := proofData.Header
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

	// special case: epoch 0 update
	// TODO: how to verify this block which includes epoch 0?
	if tdmExtra.EpochBytes != nil && len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil && ep.Number == 0 {
			return nil
		}
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

	log.Debug("VerifyChildChainProofData - end")
	return nil
}

func (cch *CrossChainHelper) SaveChildChainProofDataToMainChain(bs []byte) error {
	log.Debug("SaveChildChainProofDataToMainChain - start")

	var proofData types.ChildChainProofData
	err := rlp.DecodeBytes(bs, &proofData)
	if err != nil {
		return err
	}

	header := proofData.Header
	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain {
		return fmt.Errorf("invalid child chain id: %s", chainId)
	}

	// here is epoch update; should be a more general mechanism
	if len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil {
			ci := core.GetChainInfo(cch.chainInfoDB, tdmExtra.ChainID)
			// ChainInfo is nil means we need to wait for Child Chain to be launched, this could happened during catch-up scenario
			if ci == nil {
				for {
					// wait for 3 sec and try again
					time.Sleep(3 * time.Second)
					ci = core.GetChainInfo(cch.chainInfoDB, tdmExtra.ChainID)
					if ci != nil {
						break
					}
				}
			}

			if ep.Number == 0 || ep.Number > ci.EpochNumber {
				ci.EpochNumber = ep.Number
				ci.Epoch = ep
				core.SaveChainInfo(cch.chainInfoDB, ci)
				log.Infof("Epoch saved from chain: %s, epoch: %v", chainId, ep)
			}
		}
	}

	log.Debug("SaveChildChainProofDataToMainChain - end")
	return nil
}

func (cch *CrossChainHelper) ValidateTX3ProofData(proofData *types.TX3ProofData) error {
	log.Debug("ValidateTX3ProofData - start")

	header := proofData.Header
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

	// special case: epoch 0 update
	// TODO: how to verify this block which includes epoch 0?
	if tdmExtra.EpochBytes != nil && len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil && ep.Number == 0 {
			return nil
		}
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

	// tx merkle proof verify
	keybuf := new(bytes.Buffer)
	for i, txIndex := range proofData.TxIndexs {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(txIndex))
		_, err, _ := trie.VerifyProof(header.TxHash, keybuf.Bytes(), proofData.TxProofs[i])
		if err != nil {
			return err
		}
	}

	log.Debug("ValidateTX3ProofData - end")
	return nil
}

func (cch *CrossChainHelper) ValidateTX4WithInMemTX3ProofData(tx4 *types.Transaction, tx3ProofData *types.TX3ProofData) error {
	// TX4
	signer := types.NewEIP155Signer(tx4.ChainId())
	from, err := types.Sender(signer, tx4)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.WithdrawFromMainChainArgs

	if !pabi.IsPChainContractAddr(tx4.To()) {
		return errors.New("invalid TX4: wrong To()")
	}

	data := tx4.Data()
	function, err := pabi.FunctionTypeFromId(data[:4])
	if err != nil {
		return err
	}

	if function != pabi.WithdrawFromMainChain {
		return errors.New("invalid TX4: wrong function")
	}

	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromMainChain.String(), data[4:]); err != nil {
		return err
	}

	// TX3
	header := tx3ProofData.Header
	if err != nil {
		return err
	}
	keybuf := new(bytes.Buffer)
	rlp.Encode(keybuf, tx3ProofData.TxIndexs[0])
	val, err, _ := trie.VerifyProof(header.TxHash, keybuf.Bytes(), tx3ProofData.TxProofs[0])
	if err != nil {
		return err
	}

	var tx3 types.Transaction
	err = rlp.DecodeBytes(val, &tx3)
	if err != nil {
		return err
	}

	signer2 := types.NewEIP155Signer(tx3.ChainId())
	tx3From, err := types.Sender(signer2, &tx3)
	if err != nil {
		return core.ErrInvalidSender
	}

	var tx3Args pabi.WithdrawFromChildChainArgs
	tx3Data := tx3.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&tx3Args, pabi.WithdrawFromChildChain.String(), tx3Data[4:]); err != nil {
		return err
	}

	// Does TX3 & TX4 Match
	if from != tx3From || args.ChainId != tx3Args.ChainId || args.Amount.Cmp(tx3.Value()) != 0 {
		return errors.New("params are not consistent with tx in child chain")
	}

	return nil
}

// TX3LocalCache start
func (cch *CrossChainHelper) GetTX3(chainId string, txHash common.Hash) *types.Transaction {
	return core.GetTX3(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) DeleteTX3(chainId string, txHash common.Hash) {
	core.DeleteTX3(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) WriteTX3ProofData(proofData *types.TX3ProofData) error {
	return core.WriteTX3ProofData(cch.localTX3CacheDB, proofData)
}

func (cch *CrossChainHelper) GetTX3ProofData(chainId string, txHash common.Hash) *types.TX3ProofData {
	return core.GetTX3ProofData(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) GetAllTX3ProofData() []*types.TX3ProofData {
	return core.GetAllTX3ProofData(cch.localTX3CacheDB)
}

// TX3LocalCache end

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
