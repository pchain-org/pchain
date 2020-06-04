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

	input, err := pabi.ChainABI.Pack(pabi.ExtractReward.String())
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
			if bc.Config().PChainId != "pchain" && bc.Config().PChainId != "testnet" {
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

		from := derivedAddressFromTx(tx)

		epoch := tdm.GetEpoch().GetEpochByBlockNumber(bc.CurrentBlock().NumberU64())
		currentEpochNumber := epoch.Number
		noExtractMark := false
		extractEpochNumber, err := state.GetEpochRewardExtracted(from)
		if err != nil {
			noExtractMark = true
		}
		maxExtractEpochNumber := uint64(0)

		rewards := state.GetAllEpochReward(from)

		log.Debugf("extrRwd_ApplyCb currentEpochNumber, noExtractMark, extractEpochNumber is %v, %v, %v\n", currentEpochNumber, noExtractMark, extractEpochNumber)
		log.Debugf("extrRwd_ApplyCb rewards is %v\n", rewards)

		//feature 'ExtractReward' is after 'OutOfStorage', so just operate on reward directly
		for epNumber, reward := range rewards{
			if (noExtractMark || extractEpochNumber < epNumber) && epNumber < currentEpochNumber {
				state.SubOutsideRewardBalanceByEpochNumber(from, epNumber, reward)
				state.AddBalance(from, reward)

				if maxExtractEpochNumber < epNumber {
					maxExtractEpochNumber = epNumber
					state.MarkEpochRewardExtracted(from, maxExtractEpochNumber)
				}
			}
		}
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
	signer := types.NewEIP155Signer(tx.ChainId())
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
