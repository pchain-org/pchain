package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	tmTypes "github.com/tendermint/tendermint/types"
	"math/big"
)

var RewardPerBlock *big.Int = big.NewInt(5e+18)

type Strategy interface {
	Receiver() common.Address
	SetValidators(validators []*tmTypes.GenesisValidator)
	GetValidators()(validators []*tmTypes.GenesisValidator)
	CollectTx(tx *ethTypes.Transaction)
	GetUpdatedValidators() []*tmTypes.GenesisValidator
	AccumulateRewards(statedb *state.StateDB, header *ethTypes.Header, uncles []*ethTypes.Header, totalUsedMoney *big.Int)
}
