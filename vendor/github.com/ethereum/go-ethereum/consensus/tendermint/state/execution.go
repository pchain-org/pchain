package state

import (
	"errors"
	"github.com/ethereum/go-ethereum/consensus"

	//"fmt"

	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	. "github.com/tendermint/go-common"
)

//--------------------------------------------------

// return a bit array of validators that signed the last commit
// NOTE: assumes commits have already been authenticated
func commitBitArrayFromBlock(block *types.TdmBlock) *BitArray {

	signed := NewBitArray(uint64(len(block.TdmExtra.SeenCommit.Precommits)))
	for i, precommit := range block.TdmExtra.SeenCommit.Precommits {
		if precommit != nil {
			signed.SetIndex(uint64(i), true) // val_.LastCommitHeight = block.Height - 1
		}
	}
	return signed
}

//-----------------------------------------------------
// Validate block

func (s *State) ValidateBlock(block *types.TdmBlock) error {
	return s.validateBlock(block)
}

//Very current block
func (s *State) validateBlock(block *types.TdmBlock) error {
	// Basic block validation.
	err := block.ValidateBasic(s.TdmExtra)
	if err != nil {
		return err
	}

	// Validate block SeenCommit.
	epoch := s.Epoch.GetEpochByBlockNumber(block.TdmExtra.Height)
	if epoch == nil || epoch.Validators == nil {
		return errors.New("no epoch for current block height")
	}

	valSet := epoch.Validators
	err = valSet.VerifyCommit(block.TdmExtra.ChainID, block.TdmExtra.Height,
		block.TdmExtra.SeenCommit)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------

func init() {
	core.RegisterInsertBlockCb("InsertNextEpoch", insertNextEpoch)
}

func insertNextEpoch(bc *core.BlockChain, block *ethTypes.Block) {
	if block.NumberU64() == 0 {
		return
	}

	tdmExtra, _ := types.ExtractTendermintExtra(block.Header())
	//here handles the proposed next epoch
	nextEpochInBlock := ep.FromBytes(tdmExtra.EpochBytes)
	if nextEpochInBlock != nil && nextEpochInBlock.Number != 0 {
		// Update Engine -> Epoch
		eng := bc.Engine().(consensus.Tendermint)
		currentEpoch := eng.GetEpoch()
		nextEpochInBlock.Status = ep.EPOCH_VOTED_NOT_SAVED
		currentEpoch.SetNextEpoch(nextEpochInBlock)
		currentEpoch.Save()
	}
}
