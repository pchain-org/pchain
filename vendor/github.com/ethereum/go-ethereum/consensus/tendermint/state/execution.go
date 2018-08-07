package state

import (
	"errors"
	//"fmt"

	. "github.com/tendermint/go-common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
)


//--------------------------------------------------
// Execute the block

// ValExecBlock executes the block, but does NOT mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
func (s *State) ValExecBlock(eventCache types.Fireable, block *types.TdmBlock, cch core.CrossChainHelper) error {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return ErrInvalidBlock(err)
	}

	return nil
}

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
	epoch := s.Epoch.GetEpochByBlockNumber(int(block.TdmExtra.Height))
	if epoch == nil || epoch.Validators == nil{
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
// ApplyBlock validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.
// Transaction results are optionally indexed.

// Validate, execute, and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache types.Fireable, block *types.TdmBlock, partsHeader types.PartSetHeader, cch core.CrossChainHelper) error {

	//here handles the proposed next epoch

	nextEpochInBlock := epoch.FromBytes(block.TdmExtra.EpochBytes)
	if nextEpochInBlock != nil {
		s.Epoch.SetNextEpoch(nextEpochInBlock)
		s.Epoch.NextEpoch.RS = s.Epoch.RS
		s.Epoch.NextEpoch.Status = epoch.EPOCH_VOTED_NOT_SAVED
		s.Epoch.Save()
	}

	//here handles if need to enter next epoch
	ok, err := s.Epoch.ShouldEnterNewEpoch(int(block.TdmExtra.Height))
	if ok && err == nil {

		// now update the block and validators
		// fmt.Println("Diffs before:", abciResponses.EndBlock.Diffs)
		// types.ValidatorChannel <- abciResponses.EndBlock.Diffs
		// types.ValidatorChannel <- s.Epoch.Number
		// abciResponses.EndBlock.Diffs = <-types.EndChannel
		// fmt.Println("Diffs after:", abciResponses.EndBlock.Diffs)

		s.Epoch, _ = s.Epoch.EnterNewEpoch(int(block.TdmExtra.Height))

	} else if err != nil {
		log.Warn(Fmt("ApplyBlock(%v): Invalid epoch. Current epoch: %v, error: %v",
			block.TdmExtra.Height, s.Epoch, err))
		return err
	}

	//here handles when enter new epoch
	s.SetBlockAndEpoch(block.TdmExtra, partsHeader)

	// save the state
	s.Epoch.Save()
	//s.Save()
	return nil
}
