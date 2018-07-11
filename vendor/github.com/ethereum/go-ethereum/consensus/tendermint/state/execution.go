package state

import (
	"errors"
	//"fmt"

	fail "github.com/ebuchman/fail-test"
	//abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	//"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	rpcTxHook "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/txhook"
)


//--------------------------------------------------
// Execute the block

// ValExecBlock executes the block, but does NOT mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
func (s *State) ValExecBlock(eventCache types.Fireable, block *types.Block, cch rpcTxHook.CrossChainHelper) error {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return ErrInvalidBlock(err)
	}

	return nil
}

// return a bit array of validators that signed the last commit
// NOTE: assumes commits have already been authenticated
func commitBitArrayFromBlock(block *types.Block) *BitArray {

	signed := NewBitArray(len(block.TdmExtra.LastCommit.Precommits))
	for i, precommit := range block.TdmExtra.LastCommit.Precommits {
		if precommit != nil {
			signed.SetIndex(i, true) // val_.LastCommitHeight = block.Height - 1
		}
	}
	return signed
}

//-----------------------------------------------------
// Validate block

func (s *State) ValidateBlock(block *types.Block) error {
	return s.validateBlock(block)
}

func (s *State) validateBlock(block *types.Block) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockID, s.LastBlockTime, s.AppHash)
	if err != nil {
		return err
	}

	// Validate block LastCommit.
	if block.TdmExtra.Header.Height == 1 {
		if len(block.TdmExtra.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		//fmt.Printf("(s *State) validateBlock(), avoid LastValidators and LastCommit.Precommits size check for validatorset change\n")
		lastValidators, _, err := s.GetValidators()

		if err != nil && lastValidators != nil {
			err = lastValidators.VerifyCommit(
				s.ChainID, s.LastBlockID, block.TdmExtra.Header.Height - 1, block.TdmExtra.LastCommit)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

//-----------------------------------------------------------------------------
// ApplyBlock validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.
// Transaction results are optionally indexed.

// Validate, execute, and commit block against app, save block and state
func (s *State) ApplyBlock(eventCache types.Fireable, block *types.Block, partsHeader types.PartSetHeader, cch rpcTxHook.CrossChainHelper) error {

	//here handles the proposed next epoch
	/*
	nextEpochInBlock := epoch.FromBytes(block.ExData.BlockExData)
	if nextEpochInBlock != nil {
		s.Epoch.SetNextEpoch(nextEpochInBlock)
		s.Epoch.NextEpoch.RS = s.Epoch.RS
		s.Epoch.NextEpoch.Status = epoch.EPOCH_VOTED_NOT_SAVED
		s.Epoch.Save()
	}

	fail.Fail() // XXX

	//here handles if need to enter next epoch
	ok, err := s.Epoch.ShouldEnterNewEpoch(block.Height)
	if ok && err == nil {

		// now update the block and validators
		// fmt.Println("Diffs before:", abciResponses.EndBlock.Diffs)
		// types.ValidatorChannel <- abciResponses.EndBlock.Diffs
		types.ValidatorChannel <- s.Epoch.Number
		abciResponses.EndBlock.Diffs = <-types.EndChannel
		// fmt.Println("Diffs after:", abciResponses.EndBlock.Diffs)

		s.Epoch, _ = s.Epoch.EnterNewEpoch(block.Height)

	} else if err != nil {
		log.Warn(Fmt("ApplyBlock(%v): Invalid epoch. Current epoch: %v, error: %v",
			block.Height, s.Epoch, err))
		return err
	}
	*/
	//here handles when enter new epoch
	s.SetBlockAndEpoch(block.TdmExtra.Header, partsHeader)

	fail.Fail() // XXX

	// save the state
	s.Epoch.Save()
	s.Save()
	return nil
}
