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

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      nil,
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

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      nil,
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
}

func del_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {

	//var args pabi.DelegateArgs
	//data := tx.Data()
	//if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.Delegate.String(), data[4:]); err != nil {
	//	return err
	//}

	// Check Candidate

	return nil
}

func del_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	amount := tx.Value()
	if state.GetBalance(from).Cmp(amount) == -1 {
		return core.ErrInsufficientFunds
	}

	var args pabi.DelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.Delegate.String(), data[4:]); err != nil {
		return err
	}

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

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.CancelDelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CancelDelegate.String(), data[4:]); err != nil {
		return err
	}

	// Check Proxied Amount in Candidate Balance
	existProxiedBalance := state.GetProxiedBalanceByUser(args.Candidate, from)
	if args.Amount.Cmp(existProxiedBalance) == 1 {
		return core.ErrInsufficientProxiedBalance
	}

	//err = cch.ValidateRevealVote(args.ChainId, from, args.PubKey, args.Amount, args.Salt, args.Signature)
	return nil
}

func cdel_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.CancelDelegateArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CancelDelegate.String(), data[4:]); err != nil {
		return err
	}

	// Check Proxied Amount in Candidate Balance
	existProxiedBalance := state.GetProxiedBalanceByUser(args.Candidate, from)
	if args.Amount.Cmp(existProxiedBalance) == 1 {
		return core.ErrInsufficientProxiedBalance
	}

	//err = cch.ValidateRevealVote(args.ChainId, from, args.PubKey, args.Amount, args.Salt, args.Signature)
	//if err != nil {
	//	return err
	//}

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
