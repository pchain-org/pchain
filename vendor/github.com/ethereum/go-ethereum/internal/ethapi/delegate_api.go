package ethapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	pabi "github.com/pchain/abi"
	"math/big"
)

type PublicDelegateAPI struct {
	b Backend
}

func NewPublicDelegateAPI(b Backend) *PublicDelegateAPI {
	return &PublicDelegateAPI{
		b: b,
	}
}

var (
	defaultSelfSecurityDeposit = math.MustParseBig256("10000000000000000000000") // 10,000 * e18
	minimumDelegationAmount    = math.MustParseBig256("1000000000000000000000")  // 1000 * e18
	maxDelegationAddresses     = 1000
)

func (api *PublicDelegateAPI) Delegate(ctx context.Context, from, candidate common.Address, amount *hexutil.Big, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.Delegate.String(), candidate)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.Delegate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: gasPrice,
		Value:    amount,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) CancelDelegate(ctx context.Context, from, candidate common.Address, amount *hexutil.Big, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.CancelDelegate.String(), candidate, (*big.Int)(amount))
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.CancelDelegate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) ApplyCandidate(ctx context.Context, from common.Address, securityDeposit *hexutil.Big, commission uint8, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.Candidate.String(), commission)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.Candidate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: gasPrice,
		Value:    securityDeposit,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) CancelCandidate(ctx context.Context, from common.Address, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.CancelCandidate.String())
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.CancelCandidate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) CheckCandidate(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (map[string]interface{}, error) {
	state, _, err := api.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}

	fields := map[string]interface{}{
		"candidate":  state.IsCandidate(address),
		"commission": state.GetCommission(address),
	}
	return fields, state.Error()
}


func (api *PublicDelegateAPI) ExtractReward(ctx context.Context, from common.Address, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.ExtractReward.String(), from)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.ExtractReward.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}


func init() {
	// Delegate
	core.RegisterValidateCb(pabi.Delegate, del_ValidateCb)
	core.RegisterApplyCb(pabi.Delegate, del_ApplyCb)

	// Cancel Delegate
	core.RegisterValidateCb(pabi.CancelDelegate, cdel_ValidateCb)
	core.RegisterApplyCb(pabi.CancelDelegate, cdel_ApplyCb)

	// Candidate
	core.RegisterValidateCb(pabi.Candidate, appcdd_ValidateCb)
	core.RegisterApplyCb(pabi.Candidate, appcdd_ApplyCb)

	// Cancel Candidate
	core.RegisterValidateCb(pabi.CancelCandidate, ccdd_ValidateCb)
	core.RegisterApplyCb(pabi.CancelCandidate, ccdd_ApplyCb)

	//Extract Reward
	core.RegisterValidateCb(pabi.ExtractReward, extrRwd_ValidateCb)
	core.RegisterApplyCb(pabi.ExtractReward, extrRwd_ApplyCb)
}

func del_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	from := derivedAddressFromTx(tx)
	_, verror := delegateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}
	return nil
}

func del_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := delegateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}

	// Do job
	amount := tx.Value()
	// Move Balance to delegate balance
	state.SubBalance(from, amount)
	state.AddDelegateBalance(from, amount)
	// Add Balance to Candidate's Proxied Balance
	state.AddProxiedBalanceByUser(args.Candidate, from, amount)

	return nil
}

func cdel_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	from := derivedAddressFromTx(tx)
	_, verror := cancelDelegateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}
	return nil
}

func cdel_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := cancelDelegateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}

	// Apply Logic
	// if request amount < proxied amount, refund it immediately
	// otherwise, refund the proxied amount, and put the rest to pending refund balance
	proxiedBalance := state.GetProxiedBalanceByUser(args.Candidate, from)
	var immediatelyRefund *big.Int
	if args.Amount.Cmp(proxiedBalance) <= 0 {
		immediatelyRefund = args.Amount
	} else {
		immediatelyRefund = proxiedBalance
		restRefund := new(big.Int).Sub(args.Amount, proxiedBalance)
		state.AddPendingRefundBalanceByUser(args.Candidate, from, restRefund)
		// TODO Add Pending Refund Set, Commit the Refund Set
		state.MarkDelegateAddressRefund(args.Candidate)
	}

	state.SubProxiedBalanceByUser(args.Candidate, from, immediatelyRefund)
	state.SubDelegateBalance(from, immediatelyRefund)
	state.AddBalance(from, immediatelyRefund)

	return nil
}

func appcdd_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	from := derivedAddressFromTx(tx)
	_, verror := candidateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}
	return nil
}

func appcdd_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := candidateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}

	amount := tx.Value()
	// Add security deposit to self
	state.SubBalance(from, amount)
	state.AddDelegateBalance(from, amount)
	state.AddProxiedBalanceByUser(from, from, amount)
	// Become a Candidate
	state.ApplyForCandidate(from, args.Commission)

	return nil
}

func ccdd_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	from := derivedAddressFromTx(tx)
	verror := cancelCandidateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}
	return nil
}

func ccdd_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	verror := cancelCandidateValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}

	// Do job
	allRefund := true
	// Refund all the amount back to users
	state.ForEachProxied(from, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
		// Refund Proxied Amount
		state.SubProxiedBalanceByUser(from, key, proxiedBalance)
		state.SubDelegateBalance(key, proxiedBalance)
		state.AddBalance(key, proxiedBalance)

		// Refund Deposit to PendingRefund if deposit > 0
		if depositProxiedBalance.Sign() > 0 {
			allRefund = false
			mainChainHeight := bc.CurrentHeader().Number
			if !bc.Config().IsMainChain() {
				mainChainHeight = bc.CurrentHeader().MainChainNumber
			}
			if !bc.Config().IsChildSd2mcWhenEpochEndsBlock(mainChainHeight) {
				state.AddPendingRefundBalanceByUser(from, key, depositProxiedBalance)
			} else {
				//Calculate the refunding amount user canceled by oneself before
				refunded := state.GetPendingRefundBalanceByUser(from, key)
				//Add the rest to refunding balance
				state.AddPendingRefundBalanceByUser(from, key, new(big.Int).Sub(depositProxiedBalance, refunded))
			}
			// TODO Add Pending Refund Set, Commit the Refund Set
			state.MarkDelegateAddressRefund(from)
		}
		return true
	})

	state.CancelCandidate(from, allRefund)

	return nil
}

func extrRwd_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {

	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {

		selfRetrieveReward := consensus.IsSelfRetrieveReward(tdm.GetEpoch(), bc, bc.CurrentBlock().Header())
		log.Debugf("extrRwd_ValidateCb selfRetrieveReward is %v\n", selfRetrieveReward)
		if !selfRetrieveReward {
			return errors.New("not enabled yet")
		}

		return nil
	} else {
		return errors.New("not pdbft engine")
	}
}

func extrRwd_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {

	//validate again
	if err := extrRwd_ValidateCb(tx, state, bc); err != nil {
		return err
	}

	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {

		chainId := bc.Config().PChainId
		from := derivedAddressFromTx(tx)

		curBlockHeight := bc.CurrentBlock().NumberU64()
		height := curBlockHeight + 1

		log.Infof("extrRwd_ApplyCb, (chainId, height, from) is (%v, %v, %x\n", chainId, height, from)
		if chainId == "child_0" && height == 43635343 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			fmt.Printf("debug here")
		}

		if patchNoRun(chainId, height, from) {
			return nil
		}

		epoch := tdm.GetEpoch().GetEpochByBlockNumber(curBlockHeight)
		currentEpochNumber := epoch.Number
		extractEpochNumber, err := state.GetEpochRewardExtracted(from, height)
		noExtractMark := false
		if extractEpochNumber == 0 || err != nil {
			noExtractMark = true
		}
		maxExtractEpochNumber := uint64(0)

		log.Infof("extrRwd_ApplyCb begin, (from， balance, rewardBalance, currentEpochNumber, noExtractMark, extractEpochNumber) is (%x, %v, %v, %v, %v, %v\n",
			from, state.GetBalance(from), state.GetRewardBalance(from).String(), currentEpochNumber, noExtractMark, extractEpochNumber)

		rewards := state.GetAllEpochReward(from, height)
		log.Infof("extrRwd_ApplyCb before patchStep1, rewards is %v\n", rewards)
		extractEpochNumber, noExtractMark, rewards = patchStep1(chainId, height, from, currentEpochNumber, extractEpochNumber, noExtractMark, rewards)
		log.Infof("extrRwd_ApplyCb after patchStep1, rewards is %v\n", rewards)

		//feature 'ExtractReward' is after 'OutOfStorage', so just operate on reward directly
		for epNumber, reward := range rewards {
			if (noExtractMark || extractEpochNumber < epNumber) && epNumber < currentEpochNumber {

				state.SubOutsideRewardBalanceByEpochNumber(from, epNumber, height, reward)
				state.AddBalance(from, reward)

				if maxExtractEpochNumber < epNumber {
					maxExtractEpochNumber = epNumber
					state.SetEpochRewardExtracted(from, maxExtractEpochNumber)
				}
			}
		}

		patchStep2(chainId, height, from, state, currentEpochNumber)

		log.Infof("extrRwd_ApplyCb end, (from， balance, rewardBalance, currentEpochNumber, noExtractMark, extractEpochNumber) is (%x, %v, %v, %v, %v, %v\n",
			from, state.GetBalance(from), state.GetRewardBalance(from).String(), currentEpochNumber, noExtractMark, extractEpochNumber)
	}

	return nil
}

// Validation

func delegateValidation(from common.Address, tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) (*pabi.DelegateArgs, error) {
	// Check minimum delegate amount
	if tx.Value().Cmp(minimumDelegationAmount) < 0 {
		return nil, core.ErrDelegateAmount
	}

	var args pabi.DelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.Delegate.String(), data[4:]); err != nil {
		return nil, err
	}

	// Check Candidate
	if !state.IsCandidate(args.Candidate) {
		return nil, core.ErrNotCandidate
	}

	depositBalance := state.GetDepositProxiedBalanceByUser(args.Candidate, from)
	if depositBalance.Sign() == 0 {
		// Check if exceed the limit of delegated addresses
		// if exceed the limit of delegation address number, return error
		delegatedAddressNumber := state.GetProxiedAddressNumber(args.Candidate)
		if delegatedAddressNumber >= maxDelegationAddresses {
			return nil, core.ErrExceedDelegationAddressLimit
		}
	}

	// If Candidate is supernode, only allow to increase the stack(whitelist proxied list), not allow to create the new stack
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
	}
	if _, supernode := ep.Validators.GetByAddress(args.Candidate.Bytes()); supernode != nil && supernode.RemainingEpoch > 0 {
		if depositBalance.Sign() == 0 {
			return nil, core.ErrCannotDelegate
		}
	}

	// Check Epoch Height
	if err := checkEpochInNormalStage(bc); err != nil {
		return nil, err
	}
	return &args, nil
}

func cancelDelegateValidation(from common.Address, tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) (*pabi.CancelDelegateArgs, error) {

	var args pabi.CancelDelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CancelDelegate.String(), data[4:]); err != nil {
		return nil, err
	}

	// Check Self Address
	if from == args.Candidate {
		return nil, core.ErrCancelSelfDelegate
	}

	// Super node Candidate can't decrease balance
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
	}
	if _, supernode := ep.Validators.GetByAddress(args.Candidate.Bytes()); supernode != nil && supernode.RemainingEpoch > 0 {
		return nil, core.ErrCannotCancelDelegate
	}

	// Check Proxied Amount in Candidate Balance
	proxiedBalance := state.GetProxiedBalanceByUser(args.Candidate, from)
	depositProxiedBalance := state.GetDepositProxiedBalanceByUser(args.Candidate, from)
	pendingRefundBalance := state.GetPendingRefundBalanceByUser(args.Candidate, from)
	// net = deposit - pending refund
	netDeposit := new(big.Int).Sub(depositProxiedBalance, pendingRefundBalance)
	// available = proxied + net
	availableRefundBalance := new(big.Int).Add(proxiedBalance, netDeposit)
	if args.Amount.Cmp(availableRefundBalance) == 1 {
		return nil, core.ErrInsufficientProxiedBalance
	}

	remainingBalance := new(big.Int).Sub(availableRefundBalance, args.Amount)
	if remainingBalance.Sign() == 1 && remainingBalance.Cmp(minimumDelegationAmount) == -1 {
		return nil, core.ErrDelegateAmount
	}

	// Check Epoch Height
	if err := checkEpochInNormalStage(bc); err != nil {
		return nil, err
	}

	return &args, nil
}

func candidateValidation(from common.Address, tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) (*pabi.CandidateArgs, error) {
	// Check cleaned Candidate
	if !state.IsCleanAddress(from) {
		return nil, core.ErrAlreadyCandidate
	}

	// Check minimum Security Deposit
	if tx.Value().Cmp(defaultSelfSecurityDeposit) == -1 {
		return nil, core.ErrMinimumSecurityDeposit
	}

	var args pabi.CandidateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.Candidate.String(), data[4:]); err != nil {
		return nil, err
	}

	// Check Commission Range
	if args.Commission > 100 {
		return nil, core.ErrCommission
	}

	// Check Epoch Height
	if err := checkEpochInNormalStage(bc); err != nil {
		return nil, err
	}

	// Annual/SemiAnnual supernode can not become candidate
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
	}
	if _, supernode := ep.Validators.GetByAddress(from.Bytes()); supernode != nil && supernode.RemainingEpoch > 0 {
		return nil, core.ErrCannotCandidate
	}

	return &args, nil
}

func cancelCandidateValidation(from common.Address, tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	// Check already Candidate
	if !state.IsCandidate(from) {
		return core.ErrNotCandidate
	}

	// Super node can't cancel Candidate
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
	}
	if _, supernode := ep.Validators.GetByAddress(from.Bytes()); supernode != nil && supernode.RemainingEpoch > 0 {
		return core.ErrCannotCancelCandidate
	}

	// Check Epoch Height
	if err := checkEpochInNormalStage(bc); err != nil {
		return err
	}

	return nil
}

// Common
func derivedAddressFromTx(tx *types.Transaction) (from common.Address) {
	signer := types.LatestSignerForChainID(tx.ChainId())
	from, _ = types.Sender(signer, tx)
	return
}

func checkEpochInNormalStage(bc *core.BlockChain) error {
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
	}

	if ep == nil {
		return errors.New("epoch is nil, are you running on Tendermint Consensus Engine")
	}

	// Vote is valid between height 0% - 75%
	height := bc.CurrentBlock().NumberU64()
	if !ep.CheckInNormalStage(height) {
		return errors.New(fmt.Sprintf("you can't send this tx during this time, current height %v", height))
	}
	return nil
}


type patchStruct struct {
	chainId            string
	blockNumber        uint64
	from               common.Address
	extractEpochNumber uint64
	noExtractMark      bool
	rewards            map[uint64]*big.Int
}

func (ps *patchStruct) reset(chainId string, blockNumber uint64, from common.Address) {
	if chainId != ps.chainId || blockNumber != ps.blockNumber || from != ps.from {
		ps.chainId = ""
		ps.blockNumber = 0
		ps.from = common.Address{}
		ps.extractEpochNumber = 0
		ps.noExtractMark = false
		ps.rewards = make(map[uint64]*big.Int)
	}
}

var patchData = patchStruct {
	chainId: "",
	blockNumber: 0,
	extractEpochNumber: 0,
	noExtractMark:      false,
	rewards:            make(map[uint64]*big.Int),
}

func patchNoRun(chainId string, blockNumber uint64, from common.Address) bool {

	log.Infof("patchNoRun; chainId is: %v, height: %v, from: %x", chainId, blockNumber, from)

	if chainId == "child_0" {
		if (blockNumber == 32110529 && from == common.HexToAddress("0x5e48674176e2cdc663b38cc0aeea1f92a3082db7")) ||
			(blockNumber == 32132151 && from == common.HexToAddress("0x8128f3e133c565ccc6ca0a8d206d5e2b2ba36868")) ||
			(blockNumber == 32132151 && from == common.HexToAddress("0xf49d2ee4e9217ae347dfddc740b7475bbceef6be")) ||
			(blockNumber == 32132151 && from == common.HexToAddress("0xfff9b142b8e4c6aff9adbf17beec53a414c5f068")) {
			return true
		}
	}

	return false
}

func patchStep1(chainId string, blockNumber uint64, from common.Address, currentEpochNumber, extractEpochNumber uint64,
	noExtractMark bool, rewards map[uint64]*big.Int) (uint64, bool, map[uint64]*big.Int) {

	log.Infof("patchStep1; height: %v, from: %x", blockNumber, from)

	patchData.reset(chainId, blockNumber, from)

	if (chainId == "pchain" && blockNumber == 13311677 && from == common.HexToAddress("0x8be8a44943861279377a693b51c0703420087480")) ||
		(chainId == "child_0" && blockNumber == 22094435 && from == common.HexToAddress("0xf5005b496dff7b1ba3ca06294f8f146c9afbe09d")) {
		if len(patchData.rewards) == 0 {
			patchData.chainId = chainId
			patchData.blockNumber = blockNumber
			patchData.from = from
			patchData.extractEpochNumber = extractEpochNumber
			patchData.noExtractMark = noExtractMark
			patchData.rewards = rewards
		}
		return patchData.extractEpochNumber, patchData.noExtractMark, patchData.rewards
	}

	if chainId == "child_0" {
		
		if blockNumber == 33389535 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
		}
		if blockNumber == 35182438 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
		}
		if blockNumber == 36492381 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 38070487 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 41975759 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 45115772 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
		}
		if blockNumber == 49769059 && from == common.HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") {
			rewardDiff := new(big.Int).SetUint64(121128901091097)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}


		if blockNumber == 33611723 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
		}
		if blockNumber == 37967696 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 40807203 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}
		if blockNumber == 42394831 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 45190833 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
		}
		if blockNumber == 47305502 && from == common.HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") {
			rewardDiff := new(big.Int).SetUint64(1041442624722396)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
		}


		if blockNumber == 33612352 && from == common.HexToAddress("0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58") {
			rewardDiff := new(big.Int).SetUint64(19900557187202)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
		}
		if blockNumber == 36624513 && from == common.HexToAddress("0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58") {
			rewardDiff := new(big.Int).SetUint64(19900557187202)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 40260767 && from == common.HexToAddress("0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58") {
			rewardDiff := new(big.Int).SetUint64(19900557187202)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}
		if blockNumber == 42600734 && from == common.HexToAddress("0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58") {
			rewardDiff := new(big.Int).SetUint64(19900557187202)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}


		if blockNumber == 34132176 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
		}
		//where is rewards[21] ???
		if blockNumber == 38004652 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 39482991 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}
		if blockNumber == 41970025 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 44569259 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
		}
		if blockNumber == 46688263 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
		}
		if blockNumber == 48756650 && from == common.HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") {
			rewardDiff := new(big.Int).SetUint64(8124004097323)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}

		
		if blockNumber == 34517913 && from == common.HexToAddress("0xd5e6619291b2384b5b7da595a9bd78ec7ea30785") {
			rewardDiff := new(big.Int).SetUint64(6320382574071211)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
		}
		if blockNumber == 38183507 && from == common.HexToAddress("0xd5e6619291b2384b5b7da595a9bd78ec7ea30785") {
			rewardDiff := new(big.Int).SetUint64(6320382574071211)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 42712099 && from == common.HexToAddress("0xd5e6619291b2384b5b7da595a9bd78ec7ea30785") {
			rewardDiff := new(big.Int).SetUint64(6320382574071211)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		
		
		if blockNumber == 36553835 && from == common.HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") {
			rewardDiff := new(big.Int).SetUint64(734409970398072)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 38000080 && from == common.HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") {
			rewardDiff := new(big.Int).SetUint64(734409970398072)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 40241404 && from == common.HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") {
			rewardDiff := new(big.Int).SetUint64(734409970398072)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}
		if blockNumber == 41960852 && from == common.HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") {
			rewardDiff := new(big.Int).SetUint64(734409970398072)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 48221867 && from == common.HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") {
			rewardDiff := new(big.Int).SetUint64(734409970398072)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
		}

		
		if blockNumber == 36677302 && from == common.HexToAddress("0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902") {
			rewardDiff := new(big.Int).SetUint64(458798981837926)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 38853970 && from == common.HexToAddress("0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902") {
			rewardDiff := new(big.Int).SetUint64(458798981837926)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 42459583 && from == common.HexToAddress("0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902") {
			rewardDiff := new(big.Int).SetUint64(458798981837926)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 50525758 && from == common.HexToAddress("0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902") {
			rewardDiff := new(big.Int).SetUint64(458798981837926)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}


		if blockNumber == 36816616 && from == common.HexToAddress("0x9a4eb75fc8db5680497ac33fd689b536334292b0") {
			rewardDiff := new(big.Int).SetUint64(812400409732380)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 38430645 && from == common.HexToAddress("0x9a4eb75fc8db5680497ac33fd689b536334292b0") {
			rewardDiff := new(big.Int).SetUint64(812400409732380)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
		}
		if blockNumber == 41639657 && from == common.HexToAddress("0x9a4eb75fc8db5680497ac33fd689b536334292b0") {
			rewardDiff := new(big.Int).SetUint64(812400409732380)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}

		
		if blockNumber == 37056299 && from == common.HexToAddress("0xae6bde77bc386d2cb6492f824ded9147d0926512") {
			rewardDiff := new(big.Int).SetUint64(3327665194300706)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 42138348 && from == common.HexToAddress("0xae6bde77bc386d2cb6492f824ded9147d0926512") {
			rewardDiff := new(big.Int).SetUint64(3327665194300706)
			//rewards[22] = new(big.Int).Add(rewards[22], rewardDiff) why need omit one????
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 50287799 && from == common.HexToAddress("0xae6bde77bc386d2cb6492f824ded9147d0926512") {
			rewardDiff := new(big.Int).SetUint64(3327665194300706)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}

		
		if blockNumber == 37682064 && from == common.HexToAddress("0xef470c3a63343585651808b8187bba0e277bc3c8") {
			rewardDiff := new(big.Int).SetUint64(10268741179017)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
		}
		if blockNumber == 49512860 && from == common.HexToAddress("0xef470c3a63343585651808b8187bba0e277bc3c8") {
			rewardDiff := new(big.Int).SetUint64(10268741179017)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}
		if blockNumber == 52081898 && from == common.HexToAddress("0xef470c3a63343585651808b8187bba0e277bc3c8") {
			rewardDiff := new(big.Int).SetUint64(10268741179017)
			rewards[28] = new(big.Int).Add(rewards[28], rewardDiff)
		}


		if blockNumber == 39602464 && from == common.HexToAddress("0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa") {
			rewardDiff := new(big.Int).SetUint64(1676043787709040)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
		}
		if blockNumber == 42479918 && from == common.HexToAddress("0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa") {
			rewardDiff := new(big.Int).SetUint64(1676043787709040)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 44774842 && from == common.HexToAddress("0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa") {
			rewardDiff := new(big.Int).SetUint64(1676043787709040)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
		}


		if blockNumber == 43635343 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			rewardDiff := new(big.Int).SetUint64(40072897040696)
			rewards[19] = new(big.Int).Add(rewards[19], rewardDiff)
			rewards[20] = new(big.Int).Add(rewards[20], rewardDiff)
			rewards[21] = new(big.Int).Add(rewards[21], rewardDiff)
			rewards[22] = new(big.Int).Add(rewards[22], rewardDiff)
			rewards[23] = new(big.Int).Add(rewards[23], rewardDiff)
			rewards[24] = new(big.Int).Add(rewards[24], rewardDiff)
		}
		if blockNumber == 44574635 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			rewardDiff := new(big.Int).SetUint64(40072897040696)
			rewards[25] = new(big.Int).Add(rewards[25], rewardDiff)
		}
		if blockNumber == 47946932 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			rewardDiff := new(big.Int).SetUint64(40072897040696)
			rewards[26] = new(big.Int).Add(rewards[26], rewardDiff)
		}
		if blockNumber == 49779509 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			rewardDiff := new(big.Int).SetUint64(40072897040696)
			rewards[27] = new(big.Int).Add(rewards[27], rewardDiff)
		}
		if blockNumber == 51389901 && from == common.HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") {
			rewardDiff := new(big.Int).SetUint64(40072897040696)
			rewards[28] = new(big.Int).Add(rewards[28], rewardDiff)
		}
	}

	return extractEpochNumber, noExtractMark, rewards
}


func patchStep2(chainId string, blockNumber uint64, from common.Address, state *state.StateDB, currentEpochNumber uint64) {

	if (chainId == "pchain" && blockNumber == 13311677 && from == common.HexToAddress("0x8be8a44943861279377a693b51c0703420087480")) ||
		(chainId == "child_0" && blockNumber == 22094435 && from == common.HexToAddress("0xf5005b496dff7b1ba3ca06294f8f146c9afbe09d")) {

		state1Object := state.GetState1DB().GetOrNewState1Object(from)
		for epoch, reward := range state1Object.EpochReward() {
			if reward.Sign() < 0 {
				reward = reward.Abs(reward)
				state1Object.SetEpochRewardBalance(epoch, reward)
			}
		}
	}
}
