package chain

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
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
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type CrossChainHelper struct {
	mtx             sync.Mutex
	chainInfoDB     dbm.DB
	localTX3CacheDB ethdb.Database
	mainChainUrl    string
	mainChainId     string
}

func (cch *CrossChainHelper) GetMutex() *sync.Mutex {
	return &cch.mtx
}

func (cch *CrossChainHelper) GetChainInfoDB() dbm.DB {
	return cch.chainInfoDB
}

func (cch *CrossChainHelper) GetMainChainUrl() string {
	return cch.mainChainUrl
}

func (cch *CrossChainHelper) GetMainChainId() string {
	return cch.mainChainId
}

// CanCreateChildChain check the condition before send the create child chain into the tx pool
func (cch *CrossChainHelper) CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount, startupCost *big.Int, startBlock, endBlock *big.Int) error {

	if chainId == "" || strings.Contains(chainId, ";") {
		return errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	pass, _ := regexp.MatchString("^[a-z]+[a-z0-9_]*$", chainId)
	if !pass {
		return errors.New("chainId must be start with letter (a-z) and contains alphanumeric(lower case) or underscore, try use other name instead")
	}

	if utf8.RuneCountInString(chainId) > 30 {
		return errors.New("max characters of chain id is 30, try use other name instead")
	}

	if chainId == MainChain || chainId == TestnetChain {
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
	if minValidators < core.OFFICIAL_MINIMUM_VALIDATORS {
		return fmt.Errorf("Validators count is not meet the minimum official validator count (%v)", core.OFFICIAL_MINIMUM_VALIDATORS)
	}

	// Check the minimum deposit amount
	officialMinimumDeposit := math.MustParseBig256(core.OFFICIAL_MINIMUM_DEPOSIT)
	if minDepositAmount.Cmp(officialMinimumDeposit) == -1 {
		return fmt.Errorf("Deposit amount is not meet the minimum official deposit amount (%v PI)", new(big.Int).Div(officialMinimumDeposit, big.NewInt(params.PI)))
	}

	// Check the startup cost
	if startupCost.Cmp(officialMinimumDeposit) != 0 {
		return fmt.Errorf("Startup cost is not meet the required amount (%v PI)", new(big.Int).Div(officialMinimumDeposit, big.NewInt(params.PI)))
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

	if chainId == MainChain || chainId == TestnetChain {
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
	if voteSet == nil {
		voteSet = epoch.NewEpochValidatorVoteSet()
	}

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

	tx, _, _, _ := rawdb.ReadTransaction(chainDb, txHash)
	return tx
}

func (cch *CrossChainHelper) GetEpochFromMainChain() (string, *epoch.Epoch) {
	ethereum := MustGetEthereumFromNode(chainMgr.mainChain.EthNode)
	var ep *epoch.Epoch
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}
	return ethereum.ChainConfig().PChainId, ep
}

func (cch *CrossChainHelper) ChangeValidators(chainId string) {

	if chainMgr == nil {
		return
	}

	var chain *Chain = nil
	if chainId == MainChain || chainId == TestnetChain {
		chain = chainMgr.mainChain
	} else if chn, ok := chainMgr.childChains[chainId]; ok {
		chain = chn
	}

	if chain == nil || chain.EthNode == nil {
		return
	}

	if address, ok := chainMgr.getNodeValidator(chain.EthNode); ok {
		chainMgr.server.AddLocalValidator(chainId, address)
	}
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
		//return errors.New("block in the future")
	}

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain || chainId == TestnetChain {
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

	// Bypass the validator check for official child chain 0
	if chainId != "child_0" {
		ci := core.GetChainInfo(cch.chainInfoDB, chainId)
		if ci == nil {
			return fmt.Errorf("chain info %s not found", chainId)
		}
		epoch := ci.GetEpochByBlockNumber(tdmExtra.Height)
		if epoch == nil {
			return fmt.Errorf("could not get epoch for block height %v", tdmExtra.Height)
		}

		if epoch.Number > ci.EpochNumber {
			ci.EpochNumber = epoch.Number
			ci.Epoch = epoch
			core.SaveChainInfo(cch.chainInfoDB, ci)
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
	}

	log.Debug("VerifyChildChainProofData - end")
	return nil
}

func (cch *CrossChainHelper) SaveChildChainProofDataToMainChain(proofData *types.ChildChainProofData) error {
	log.Debug("SaveChildChainProofDataToMainChain - start")

	header := proofData.Header
	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain || chainId == TestnetChain {
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

			futureEpoch := ep.Number > ci.EpochNumber && tdmExtra.Height < ep.StartBlock
			if futureEpoch {
				// Future Epoch, just save the Epoch into Chain Info DB
				core.SaveFutureEpoch(cch.chainInfoDB, ep, chainId)
			} else if ep.Number == 0 || ep.Number >= ci.EpochNumber {
				// New Epoch, save or update the Epoch into Chain Info DB
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
		//return errors.New("block in the future")
	}

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain || chainId == TestnetChain {
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

	if epoch.Number > ci.EpochNumber {
		ci.EpochNumber = epoch.Number
		ci.Epoch = epoch
		core.SaveChainInfo(cch.chainInfoDB, ci)
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
		_, _, err := trie.VerifyProof(header.TxHash, keybuf.Bytes(), proofData.TxProofs[i])
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
	val, _, err := trie.VerifyProof(header.TxHash, keybuf.Bytes(), tx3ProofData.TxProofs[0])
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

//SaveDataToMainV1 acceps both epoch and tx3
func (cch *CrossChainHelper) VerifyChildChainProofDataV1(proofData *types.ChildChainProofDataV1) error {

	log.Debug("VerifyChildChainProofDataV1 - start")

	header := proofData.Header
	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		//return errors.New("block in the future")
	}

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain || chainId == TestnetChain {
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

	isSd2mc := params.IsSd2mc(cch.GetMainChainId(), cch.GetHeightFromMainChain())
	// Bypass the validator check for official child chain 0
	if chainId != "child_0" || isSd2mc {

		getValidatorsFromChainInfo := false
		if tdmExtra.EpochBytes != nil && len(tdmExtra.EpochBytes) != 0 {
			ep := epoch.FromBytes(tdmExtra.EpochBytes)
			if ep != nil && ep.Number == 0 {
				//Child chain just created and save the epoch info, get validators from chain info
				getValidatorsFromChainInfo = true
			}
		}

		var valSet *tdmTypes.ValidatorSet = nil
		if !getValidatorsFromChainInfo {
			ep := ci.GetEpochByBlockNumber(tdmExtra.Height)
			if ep == nil {
				return fmt.Errorf("could not get epoch for block height %v", tdmExtra.Height)
			}

			if ep.Number > ci.EpochNumber {
				ci.EpochNumber = ep.Number
				ci.Epoch = ep
				core.SaveChainInfo(cch.chainInfoDB, ci)
			}

			valSet = ep.Validators
			log.Debugf("ep>>>>>>>>>>>>>>>>>>>> Ep: %v, Validators: %v", ep.String(), ep.Validators.String())
			log.Debugf("tdmextra>>>>>>>>>>>>>>>>>> tdmExtra: %v", tdmExtra.String())
		} else {
			_, tdmGenesis := core.LoadChainGenesis(cch.chainInfoDB, chainId)
			if tdmGenesis == nil {
				return errors.New(fmt.Sprintf("unable to retrieve the genesis file for child chain %s", chainId))
			}
			coreGenesis, err := tdmTypes.GenesisDocFromJSON(tdmGenesis)
			if err != nil {
				return err
			}

			ep := epoch.MakeOneEpoch(nil, &coreGenesis.CurrentEpoch, nil)
			if ep == nil {
				return fmt.Errorf("could not get epoch for genesis information")
			}
			valSet = ep.Validators
		}

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
	}

	//Verify Tx3
	// tx merkle proof verify
	keybuf := new(bytes.Buffer)
	for i, txIndex := range proofData.TxIndexs {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(txIndex))
		_, _, err := trie.VerifyProof(header.TxHash, keybuf.Bytes(), proofData.TxProofs[i])
		if err != nil {
			return err
		}
	}

	log.Debug("VerifyChildChainProofDataV1 - end")
	return nil
}

func (cch *CrossChainHelper) SaveChildChainProofDataToMainChainV1(proofData *types.ChildChainProofDataV1) error {
	log.Info("SaveChildChainProofDataToMainChainV1 - start")

	header := proofData.Header
	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return err
	}

	chainId := tdmExtra.ChainID
	if chainId == "" || chainId == MainChain || chainId == TestnetChain {
		return fmt.Errorf("invalid child chain id: %s", chainId)
	}

	// here is epoch update; should be a more general mechanism
	if len(tdmExtra.EpochBytes) != 0 {
		log.Info("SaveChildChainProofDataToMainChainV1 - Save Epoch")
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil {
			ci := core.GetChainInfo(cch.chainInfoDB, tdmExtra.ChainID)
			// ChainInfo is nil means we need to wait for Child Chain to be launched, this could happened during catch-up scenario
			if ci == nil {
				return fmt.Errorf("not possible to pass verification")
			}

			futureEpoch := ep.Number > ci.EpochNumber && tdmExtra.Height < ep.StartBlock
			if futureEpoch {
				// Future Epoch, just save the Epoch into Chain Info DB
				core.SaveFutureEpoch(cch.chainInfoDB, ep, chainId)
			} else if ep.Number == 0 || ep.Number >= ci.EpochNumber {
				// New Epoch, save or update the Epoch into Chain Info DB
				ci.EpochNumber = ep.Number
				ci.Epoch = ep
				core.SaveChainInfo(cch.chainInfoDB, ci)
				log.Infof("Epoch saved from chain: %s, epoch: %v", chainId, ep)
			}
		}
	}

	// Write the TX3ProofData
	if len(proofData.TxIndexs) != 0 {

		log.Infof("SaveChildChainProofDataToMainChainV1 - Save Tx3, count is %v", len(proofData.TxIndexs))
		tx3ProofData := &types.TX3ProofData{
			Header:   proofData.Header,
			TxIndexs: proofData.TxIndexs,
			TxProofs: proofData.TxProofs,
		}
		if err := cch.WriteTX3ProofData(tx3ProofData); err != nil {
			log.Error("TX3ProofDataMsg write error", "error", err)
		}
	}

	log.Info("SaveChildChainProofDataToMainChainV1 - end")
	return nil
}

// TX3LocalCache start
func (cch *CrossChainHelper) GetTX3(chainId string, txHash common.Hash) *types.Transaction {
	return rawdb.GetTX3(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) DeleteTX3(chainId string, txHash common.Hash) {
	rawdb.DeleteTX3(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) WriteTX3ProofData(proofData *types.TX3ProofData) error {
	return rawdb.WriteTX3ProofData(cch.localTX3CacheDB, proofData)
}

func (cch *CrossChainHelper) GetTX3ProofData(chainId string, txHash common.Hash) *types.TX3ProofData {
	return rawdb.GetTX3ProofData(cch.localTX3CacheDB, chainId, txHash)
}

func (cch *CrossChainHelper) GetAllTX3ProofData() []*types.TX3ProofData {
	return rawdb.GetAllTX3ProofData(cch.localTX3CacheDB)
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
