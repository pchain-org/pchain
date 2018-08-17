package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/abci/types"
	"math/big"
)

type Strategy interface {
	Receiver() common.Address
	SetValidators(validators []*types.Validator)
	GetValidators() (validators []*types.Validator)
	CollectTx(tx *ethTypes.Transaction)
	GetUpdatedValidators() []*types.Validator
	AccumulateRewards(statedb *state.StateDB, header *ethTypes.Header, uncles []*ethTypes.Header, totalUsedMoney *big.Int, rewardPerBlock *big.Int)
}

