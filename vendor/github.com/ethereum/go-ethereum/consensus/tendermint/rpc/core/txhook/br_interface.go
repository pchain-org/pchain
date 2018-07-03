package core

import (
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

type BrCommit interface {
	GetChainId() string
	GetValidators() (*types.ValidatorSet, *types.ValidatorSet, error)
	SaveCurrentBlock2MainChain()
	GetCurrentBlock() (*types.Block)
	GetCrossChainHelper() CrossChainHelper
}
