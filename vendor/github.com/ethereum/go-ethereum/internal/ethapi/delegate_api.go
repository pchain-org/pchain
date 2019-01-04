package ethapi

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
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

func (api *PublicDelegateAPI) Delegate(ctx context.Context, from, candidate common.Address, amount *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.Delegate.String(), candidate)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.Delegate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: nil,
		Value:    amount,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) CancelDelegate(ctx context.Context, from, candidate common.Address, amount *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.CancelDelegate.String(), candidate, (*big.Int)(amount))
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.CancelDelegate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: nil,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) ApplyCandidate(ctx context.Context, from common.Address, commission uint8) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.Candidate.String(), commission)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.Candidate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: nil,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}
	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicDelegateAPI) CancelCandidate(ctx context.Context, from common.Address) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.CancelCandidate.String())
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.CancelCandidate.RequiredGas()

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      (*hexutil.Uint64)(&defaultGas),
		GasPrice: nil,
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
}

func del_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	from := derivedAddressFromTx(tx)
	_, verror := delegateValidation(from, tx, state)
	if verror != nil {
		return verror
	}
	return nil
}

func del_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := delegateValidation(from, tx, state)
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

	//op := types.DelegateOp{
	//	From:      from,
	//	Candidate: args.Candidate,
	//	Amount:    tx.Value(),
	//}
	//
	//if ok := ops.Append(&op); !ok {
	//	return fmt.Errorf("pending ops conflict: %v", op)
	//}

	return nil
}

func cdel_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	from := derivedAddressFromTx(tx)
	_, verror := cancelDelegateValidation(from, tx, state)
	if verror != nil {
		return verror
	}
	return nil
}

func cdel_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := cancelDelegateValidation(from, tx, state)
	if verror != nil {
		return verror
	}

	// Apply Logic
	state.SubProxiedBalanceByUser(args.Candidate, from, args.Amount)
	state.SubDelegateBalance(from, args.Amount)
	state.AddBalance(from, args.Amount)

	//op := types.CancelDelegateOp{
	//	From:      from,
	//	Candidate: args.Candidate,
	//	Amount:    args.Amount,
	//}
	//
	//if ok := ops.Append(&op); !ok {
	//	return fmt.Errorf("pending ops conflict: %v", op)
	//}

	return nil
}

func appcdd_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	from := derivedAddressFromTx(tx)
	_, verror := candidateValidation(from, tx, state)
	if verror != nil {
		return verror
	}
	return nil
}

func appcdd_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := candidateValidation(from, tx, state)
	if verror != nil {
		return verror
	}

	// Do job
	state.ApplyForCandidate(from, args.Commission)
	return nil
}

func ccdd_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	from := derivedAddressFromTx(tx)
	verror := cancelCandidateValidation(from, tx, state)
	if verror != nil {
		return verror
	}
	return nil
}

func ccdd_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	verror := cancelCandidateValidation(from, tx, state)
	if verror != nil {
		return verror
	}

	// Do job
	state.CancelCandidate(from)
	//TODO Refund all the amount back to users

	return nil
}

// Validation

func delegateValidation(from common.Address, tx *types.Transaction, state *state.StateDB) (*pabi.DelegateArgs, error) {
	// Check Amount
	amount := tx.Value()
	if state.GetBalance(from).Cmp(amount) == -1 {
		return nil, core.ErrInsufficientFunds
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

	// Check Epoch Height

	return &args, nil
}

func cancelDelegateValidation(from common.Address, tx *types.Transaction, state *state.StateDB) (*pabi.CancelDelegateArgs, error) {

	var args pabi.CancelDelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CancelDelegate.String(), data[4:]); err != nil {
		return nil, err
	}

	// Check Proxied Amount in Candidate Balance
	existProxiedBalance := state.GetProxiedBalanceByUser(args.Candidate, from)
	if args.Amount.Cmp(existProxiedBalance) == 1 {
		return nil, core.ErrInsufficientProxiedBalance
	}

	// Check Epoch Height

	return &args, nil
}

func candidateValidation(from common.Address, tx *types.Transaction, state *state.StateDB) (*pabi.CandidateArgs, error) {
	// Check already Candidate
	if state.IsCandidate(from) {
		return nil, core.ErrAlreadyCandidate
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

	return &args, nil
}

func cancelCandidateValidation(from common.Address, tx *types.Transaction, state *state.StateDB) error {
	// Check already Candidate
	if !state.IsCandidate(from) {
		return core.ErrNotCandidate
	}

	// Check Epoch Height

	return nil
}

// Common
func derivedAddressFromTx(tx *types.Transaction) (from common.Address) {
	signer := types.NewEIP155Signer(tx.ChainId())
	from, _ = types.Sender(signer, tx)
	return
}
