package ethapi

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

const (
	VNEFuncName = "VoteNextEpoch"
	REVFuncName = "RevealVote"

	// Reveal Vote Parameters
	REV_ARGS_FROM    = "from"
	REV_ARGS_PUBKEY  = "pubkey"
	REV_ARGS_DEPOSIT = "amount"
	REV_ARGS_SALT    = "salt"
)

type PublicTdmAPI struct {
	am *accounts.Manager
	b  Backend
}

func NewPublicTdmAPI(b Backend) *PublicChainAPI {
	return &PublicChainAPI{
		am: b.AccountManager(),
		b:  b,
	}
}

func (api *PublicTdmAPI) VoteNextEpoch(ctx context.Context, from common.Address, voteHash common.Hash) (common.Hash, error) {

	params := types.MakeKeyValueSet()
	params.Set("from", from)
	params.Set("voteHash", voteHash)

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
