package core

import (
	"github.com/tendermint/tendermint/types"
)

type BrCommit interface {
	GetChainId() string
	GetValidators() (*types.ValidatorSet, *types.ValidatorSet, error)
	GetCurrentBlock() (*types.Block)
	GetCrossChainHelper() CrossChainHelper
}
