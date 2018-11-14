package ethapi

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	pabi "github.com/pchain/abi"
	"math/big"
)

type PublicTdmAPI struct {
	am *accounts.Manager
	b  Backend
}

func NewPublicTdmAPI(b Backend) *PublicTdmAPI {
	return &PublicTdmAPI{
		am: b.AccountManager(),
		b:  b,
	}
}

func (api *PublicTdmAPI) VoteNextEpoch(ctx context.Context, from common.Address, voteHash common.Hash) (common.Hash, error) {

	chainId := api.b.ChainConfig().PChainId

	input, err := pabi.ChainABI.Pack(pabi.VoteNextEpoch.String(), chainId, voteHash)
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

func (api *PublicTdmAPI) RevealVote(ctx context.Context, from common.Address, pubkey string, amount *hexutil.Big, salt string) (common.Hash, error) {

	chainId := api.b.ChainConfig().PChainId

	input, err := pabi.ChainABI.Pack(pabi.RevealVote.String(), chainId, pubkey, (*big.Int)(amount), salt)
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
	// Vote for Next Epoch
	core.RegisterValidateCb(pabi.VoteNextEpoch, vne_ValidateCb)
	core.RegisterApplyCb(pabi.VoteNextEpoch, vne_ApplyCb)

	// Reveal Vote
	core.RegisterValidateCb(pabi.RevealVote, rev_ValidateCb)
	core.RegisterApplyCb(pabi.RevealVote, rev_ApplyCb)
}

func vne_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {

	var args pabi.VoteNextEpochArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.VoteNextEpoch.String(), data[4:]); err != nil {
		return err
	}

	err := cch.ValidateVoteNextEpoch(args.ChainId)
	return err
}

func vne_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.VoteNextEpochArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.VoteNextEpoch.String(), data[4:]); err != nil {
		return err
	}

	err = cch.ValidateVoteNextEpoch(args.ChainId)
	if err != nil {
		return err
	}

	op := types.VoteNextEpochOp{
		From:     from,
		VoteHash: args.VoteHash,
		TxHash:   tx.Hash(),
	}

	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	return nil
}

func rev_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.RevealVoteArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.RevealVote.String(), data[4:]); err != nil {
		return err
	}

	// Check Balance (Available + Lock)
	total := new(big.Int).Add(state.GetBalance(from), state.GetDepositBalance(from))
	if total.Cmp(args.Amount) == -1 {
		return core.ErrInsufficientFunds
	}

	err = cch.ValidateRevealVote(args.ChainId, from, args.PubKey, args.Amount, args.Salt)
	return err
}

func rev_ApplyCb(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper, mining bool) error {

	signer := types.NewEIP155Signer(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.RevealVoteArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.RevealVote.String(), data[4:]); err != nil {
		return err
	}

	// Check Balance (Available + Lock)
	total := new(big.Int).Add(state.GetBalance(from), state.GetDepositBalance(from))
	if total.Cmp(args.Amount) == -1 {
		return core.ErrInsufficientFunds
	}

	err = cch.ValidateRevealVote(args.ChainId, from, args.PubKey, args.Amount, args.Salt)
	if err != nil {
		return err
	}

	// Apply Logic
	// if lock balance less than deposit amount, then add enough amount to locked balance
	if state.GetDepositBalance(from).Cmp(args.Amount) == -1 {
		difference := new(big.Int).Sub(args.Amount, state.GetDepositBalance(from))
		if state.GetBalance(from).Cmp(difference) == -1 {
			return core.ErrInsufficientFunds
		} else {
			state.SubBalance(from, difference)
			state.AddDepositBalance(from, difference)
		}
	}

	op := types.RevealVoteOp{
		From:   from,
		Pubkey: args.PubKey,
		Amount: args.Amount,
		Salt:   args.Salt,
		TxHash: tx.Hash(),
	}

	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	return nil
}
