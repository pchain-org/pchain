package core

import (
	"github.com/tendermint/tendermint/types"
)

type BrState interface {
	GetValidators() (*types.ValidatorSet, *types.ValidatorSet, error)
}
