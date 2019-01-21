package ethapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	pabi "github.com/pchain/abi"
	"github.com/tendermint/go-crypto"
	"math/big"
)

type PublicTdmAPI struct {
	b Backend
}

func NewPublicTdmAPI(b Backend) *PublicTdmAPI {
	return &PublicTdmAPI{
		b: b,
	}
}

func (api *PublicTdmAPI) VoteNextEpoch(ctx context.Context, from common.Address, voteHash common.Hash, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.VoteNextEpoch.String(), voteHash)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.VoteNextEpoch.RequiredGas()

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

func (api *PublicTdmAPI) RevealVote(ctx context.Context, from common.Address, pubkey crypto.BLSPubKey, amount *hexutil.Big, salt string, signature hexutil.Bytes, gasPrice *hexutil.Big) (common.Hash, error) {

	input, err := pabi.ChainABI.Pack(pabi.RevealVote.String(), pubkey.Bytes(), (*big.Int)(amount), salt, signature)
	if err != nil {
		return common.Hash{}, err
	}

	defaultGas := pabi.RevealVote.RequiredGas()

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
	// Vote for Next Epoch
	core.RegisterValidateCb(pabi.VoteNextEpoch, vne_ValidateCb)
	core.RegisterApplyCb(pabi.VoteNextEpoch, vne_ApplyCb)

	// Reveal Vote
	core.RegisterValidateCb(pabi.RevealVote, rev_ValidateCb)
	core.RegisterApplyCb(pabi.RevealVote, rev_ApplyCb)
}

func vne_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {

	_, verror := voteNextEpochValidation(tx, bc)
	if verror != nil {
		return verror
	}

	return nil
}

func vne_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {
	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := voteNextEpochValidation(tx, bc)
	if verror != nil {
		return verror
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

func rev_ValidateCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) error {
	from := derivedAddressFromTx(tx)
	_, verror := revealVoteValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}
	return nil
}

func rev_ApplyCb(tx *types.Transaction, state *state.StateDB, bc *core.BlockChain, ops *types.PendingOps) error {

	// Validate first
	from := derivedAddressFromTx(tx)
	args, verror := revealVoteValidation(from, tx, state, bc)
	if verror != nil {
		return verror
	}

	// Apply Logic
	if state.IsCandidate(from) {
		// Move delegate amount first if Candidate
		state.ForEachProxied(from, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
			// Move Proxied Amount to Deposit Proxied Amount
			state.SubProxiedBalanceByUser(from, key, proxiedBalance)
			state.AddDepositProxiedBalanceByUser(from, key, proxiedBalance)
			return true
		})
	}

	// Rest Vote Amount
	proxiedBalance := state.GetTotalProxiedBalance(from)
	depositProxiedBalance := state.GetTotalDepositProxiedBalance(from)
	pendingRefundBalance := state.GetTotalPendingRefundBalance(from)
	netProxied := new(big.Int).Sub(new(big.Int).Add(proxiedBalance, depositProxiedBalance), pendingRefundBalance)
	netSelfAmount := new(big.Int).Sub(args.Amount, netProxied)

	// if lock balance less than net self amount, then add enough amount to locked balance
	if state.GetDepositBalance(from).Cmp(netSelfAmount) == -1 {
		difference := new(big.Int).Sub(netSelfAmount, state.GetDepositBalance(from))
		state.SubBalance(from, difference)
		state.AddDepositBalance(from, difference)
	}

	var pub crypto.BLSPubKey
	copy(pub[:], args.PubKey)

	op := types.RevealVoteOp{
		From:   from,
		Pubkey: pub,
		Amount: args.Amount,
		Salt:   args.Salt,
		TxHash: tx.Hash(),
	}

	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	return nil
}

// Validation

func voteNextEpochValidation(tx *types.Transaction, bc *core.BlockChain) (*pabi.VoteNextEpochArgs, error) {
	var args pabi.VoteNextEpochArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.VoteNextEpoch.String(), data[4:]); err != nil {
		return nil, err
	}

	// Check Epoch Height
	if err := checkEpochInHashVoteStage(bc); err != nil {
		return nil, err
	}

	return &args, nil
}

func revealVoteValidation(from common.Address, tx *types.Transaction, state *state.StateDB, bc *core.BlockChain) (*pabi.RevealVoteArgs, error) {
	var args pabi.RevealVoteArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.RevealVote.String(), data[4:]); err != nil {
		return nil, err
	}

	var netProxied *big.Int
	if state.IsCandidate(from) {
		// is Candidate? Check Proxied Balance (Amount >= (proxiedBalance + depositProxiedBalance - pendingRefundBalance))
		proxiedBalance := state.GetTotalProxiedBalance(from)
		depositProxiedBalance := state.GetTotalDepositProxiedBalance(from)
		pendingRefundBalance := state.GetTotalPendingRefundBalance(from)
		netProxied = new(big.Int).Sub(new(big.Int).Add(proxiedBalance, depositProxiedBalance), pendingRefundBalance)
	} else {
		netProxied = common.Big0
	}
	if args.Amount == nil || args.Amount.Sign() < 0 || args.Amount.Cmp(netProxied) == -1 {
		return nil, core.ErrVoteAmountTooLow
	}

	// Check Amount (Amount <= net proxied + balance + deposit)
	balance := state.GetBalance(from)
	deposit := state.GetDepositBalance(from)
	maximumAmount := new(big.Int).Add(new(big.Int).Add(balance, deposit), netProxied)
	if args.Amount.Cmp(maximumAmount) == 1 {
		return nil, core.ErrVoteAmountTooHight
	}

	// Check Signature of the PubKey matched against the Address
	if err := crypto.CheckConsensusPubKey(from, args.PubKey, args.Signature); err != nil {
		return nil, err
	}

	// Check Epoch Height
	ep, err := checkEpochInRevealVoteStage(bc)
	if err != nil {
		return nil, err
	}

	// Check Vote
	voteSet := ep.GetNextEpoch().GetEpochValidatorVoteSet()
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
		args.PubKey,
		args.Amount.Bytes(),
		[]byte(args.Salt),
	}
	voteHash := ethcrypto.Keccak256Hash(concatCopyPreAllocate(byte_data))
	if vote.VoteHash != voteHash {
		return nil, errors.New("your vote doesn't match your vote hash, please check your vote")
	}

	// Check Logic - Amount can't be 0 for new Validator
	if !ep.Validators.HasAddress(from.Bytes()) && args.Amount.Sign() <= 0 {
		return nil, errors.New("invalid vote!!! new validator's vote amount must be greater than 0")
	}

	return &args, nil
}

// Common

func checkEpochInHashVoteStage(bc *core.BlockChain) error {
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}

	if ep == nil {
		return errors.New("epoch is nil, are you running on Tendermint Consensus Engine")
	}

	// Check Epoch in Hash Vote stage
	if ep.GetNextEpoch() == nil {
		return errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 75% - 85%
	height := bc.CurrentBlock().NumberU64()
	if !ep.CheckInHashVoteStage(height) {
		return errors.New(fmt.Sprintf("you can't send the hash vote during this time, current height %v", height))
	}
	return nil
}

func checkEpochInRevealVoteStage(bc *core.BlockChain) (*epoch.Epoch, error) {
	var ep *epoch.Epoch
	if tdm, ok := bc.Engine().(consensus.Tendermint); ok {
		ep = tdm.GetEpoch()
	}

	if ep == nil {
		return nil, errors.New("epoch is nil, are you running on Tendermint Consensus Engine")
	}

	// Check Epoch in Reveal Vote stage
	if ep.GetNextEpoch() == nil {
		return nil, errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 85% - 95%
	height := bc.CurrentBlock().NumberU64()
	if !ep.CheckInRevealVoteStage(height) {
		return nil, errors.New(fmt.Sprintf("you can't send the reveal vote during this time, current height %v", height))
	}
	return ep, nil
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
