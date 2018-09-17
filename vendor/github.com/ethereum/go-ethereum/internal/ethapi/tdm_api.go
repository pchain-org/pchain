package ethapi

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/go-crypto"
	"math/big"
)

const (
	VNEFuncName = "VoteNextEpoch"
	REVFuncName = "RevealVote"

	// Vote Next Epoch Parameters
	VNE_ARGS_FROM    = "from"
	VNE_ARGS_HASH    = "voteHash"
	VNE_ARGS_CHAINID = "currentChainId"

	// Reveal Vote Parameters
	REV_ARGS_FROM    = "from"
	REV_ARGS_PUBKEY  = "pubkey"
	REV_ARGS_DEPOSIT = "amount"
	REV_ARGS_SALT    = "salt"
	REV_ARGS_CHAINID = "currentChainId"
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

	params := types.MakeKeyValueSet()
	params.Set(VNE_ARGS_FROM, from)
	params.Set(VNE_ARGS_HASH, voteHash)
	params.Set(VNE_ARGS_CHAINID, api.b.ChainConfig().PChainId)

	etd := &types.ExtendTxData{
		FuncName: VNEFuncName,
		Params:   params,
	}

	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          nil,
		GasPrice:     nil,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (api *PublicTdmAPI) RevealVote(ctx context.Context, from common.Address, pubkey string, amount *big.Int, salt string) (common.Hash, error) {

	params := types.MakeKeyValueSet()
	params.Set(REV_ARGS_FROM, from)
	params.Set(REV_ARGS_PUBKEY, pubkey)
	params.Set(REV_ARGS_DEPOSIT, amount)
	params.Set(REV_ARGS_SALT, salt)
	params.Set(REV_ARGS_CHAINID, api.b.ChainConfig().PChainId)

	etd := &types.ExtendTxData{
		FuncName: REVFuncName,
		Params:   params,
	}

	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          nil,
		GasPrice:     nil,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return api.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func init() {
	// Vote for Next Epoch
	core.RegisterValidateCb(VNEFuncName, vne_ValidateCb)
	core.RegisterApplyCb(VNEFuncName, vne_ApplyCb)

	// Reveal Vote
	core.RegisterValidateCb(REVFuncName, rev_ValidateCb)
	core.RegisterApplyCb(REVFuncName, rev_ApplyCb)
}

func vne_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()
	chainId, _ := etd.GetString(VNE_ARGS_CHAINID)

	_, err := cch.ValidateVoteNextEpoch(chainId)
	return err
}

func vne_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()
	from, _ := etd.GetAddress(VNE_ARGS_FROM)
	voteHash, _ := etd.GetHash(VNE_ARGS_HASH)
	chainId, _ := etd.GetString(VNE_ARGS_CHAINID)
	txhash := tx.Hash()

	ep, err := cch.ValidateVoteNextEpoch(chainId)
	if err != nil {
		return err
	}

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Overwrite the Previous Hash Vote
		vote.VoteHash = voteHash
		vote.TxHash = txhash
	} else {
		// Create a new Hash Vote
		vote = &epoch.EpochValidatorVote{
			Address:  from,
			VoteHash: voteHash,
			TxHash:   txhash,
		}
		voteSet.StoreVote(vote)
	}

	return nil
}

func rev_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(REV_ARGS_FROM)
	pubkey, _ := etd.GetString(REV_ARGS_PUBKEY)
	depositAmount, _ := etd.GetBigInt(REV_ARGS_DEPOSIT)
	salt, _ := etd.GetString(REV_ARGS_SALT)
	chainId, _ := etd.GetString(REV_ARGS_CHAINID)

	// Check Balance (Available + Lock)
	total := new(big.Int).Add(state.GetBalance(from), state.GetDepositBalance(from))
	if total.Cmp(depositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	_, err := cch.ValidateRevealVote(chainId, from, pubkey, depositAmount, salt)
	return err
}

func rev_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(REV_ARGS_FROM)
	pubkey, _ := etd.GetString(REV_ARGS_PUBKEY)
	depositAmount, _ := etd.GetBigInt(REV_ARGS_DEPOSIT)
	salt, _ := etd.GetString(REV_ARGS_SALT)
	chainId, _ := etd.GetString(REV_ARGS_CHAINID)

	// Check Balance (Available + Lock)
	total := new(big.Int).Add(state.GetBalance(from), state.GetDepositBalance(from))
	if total.Cmp(depositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	ep, err := cch.ValidateRevealVote(chainId, from, pubkey, depositAmount, salt)
	if err != nil {
		return err
	}

	// Apply Logic
	// if lock balance less than deposit amount, then add enough amount to locked balance
	if state.GetDepositBalance(from).Cmp(depositAmount) == -1 {
		difference := new(big.Int).Sub(depositAmount, state.GetDepositBalance(from))
		if state.GetBalance(from).Cmp(difference) == -1 {
			return core.ErrInsufficientFunds
		} else {
			state.SubBalance(from, difference)
			state.AddDepositBalance(from, difference)
		}
	}

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Update the Hash Vote with Real Data
		vote.PubKey = crypto.EthereumPubKey(common.FromHex(pubkey))
		vote.Amount = depositAmount
		vote.Salt = salt
		vote.TxHash = tx.Hash()
	}

	return nil
}
