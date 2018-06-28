package ethereum

import (
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

// NullBlockProcessor does not validate anything
type NullBlockProcessor struct{}

// ValidateBody validates the given block's content.
func (NullBlockProcessor) ValidateBody(block *ethTypes.Block) error {return nil}

// ValidateState validates the given statedb and optionally the receipts and
// gas used.
func (NullBlockProcessor) ValidateState(block, parent *ethTypes.Block, state *state.StateDB, receipts ethTypes.Receipts, usedGas uint64) error {return nil}
