package state

import (
	"errors"
	"fmt"

	fail "github.com/ebuchman/fail-test"
	"github.com/pchain/common/plogger"
	abci "github.com/tendermint/abci/types"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/epoch"
	tdmMempool "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	rpcTxHook "github.com/tendermint/tendermint/rpc/core/txhook"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

var plog = plogger.GetLogger("tendermint/execution")

//--------------------------------------------------
// Execute the block

// ValExecBlock executes the block, but does NOT mutate State.
// + validates the block
// + executes block.Txs on the proxyAppConn
func (s *State) ValExecBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus, block *types.Block,
	cch rpcTxHook.CrossChainHelper) (*ABCIResponses, error) {
	// Validate the block.
	if err := s.validateBlock(block); err != nil {
		return nil, ErrInvalidBlock(err)
	}

	// Execute the block txs
	abciResponses, err := execBlockOnProxyApp(eventCache, proxyAppConn, s, block, cch)
	if err != nil {
		// There was some error in proxyApp
		// TODO Report error and wait for proxyApp to be available.
		return nil, ErrProxyAppConn(err)
	}

	return abciResponses, nil
}

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
// TODO: Generate a bitmap or otherwise store tx validity in state.
func execBlockOnProxyApp(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
	state *State, block *types.Block, cch rpcTxHook.CrossChainHelper) (*ABCIResponses, error) {

	abciResponses := RefreshABCIResponses(block, state, eventCache, proxyAppConn, cch)

	// Begin block
	err := proxyAppConn.BeginBlockSync(block.Hash(), types.TM2PB.Header(block.Header))
	if err != nil {
		log.Warn("Error in proxyAppConn.BeginBlock", "error", err)
		return nil, err
	}

	// Run txs of block
	for _, tx := range block.Txs {
		proxyAppConn.DeliverTxSync(tx)
		/*
			res := proxyAppConn.DeliverTxSync(tx)
			if res.IsErr() {
				return nil, errors.New(res.Error())
			}

				proxyAppConn.DeliverTxAsync(tx)
				if err := proxyAppConn.Error(); err != nil {
					return nil, err
				}
		*/
	}

	// End block
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(uint64(block.Height))
	if err != nil {
		log.Warn("Error in proxyAppConn.EndBlock", "error", err)
		return nil, err
	}

	valDiff := abciResponses.EndBlock.Diffs

	log.Info("Executed block", "height", block.Height, "valid txs", abciResponses.ValidTxs, "invalid txs", abciResponses.InvalidTxs)
	if len(valDiff) > 0 {
		log.Info("Update to validator set", "updates", abci.ValidatorsString(valDiff))
	}

	return abciResponses, nil
}

// return a bit array of validators that signed the last commit
// NOTE: assumes commits have already been authenticated
func commitBitArrayFromBlock(block *types.Block) *BitArray {
	signed := NewBitArray(len(block.LastCommit.Precommits))
	for i, precommit := range block.LastCommit.Precommits {
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
	if block.Height == 1 {
		if len(block.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		/*
			if len(block.LastCommit.Precommits) != s.LastValidators.Size() {
				fmt.Printf("validateBlock(), LastCommit.Precommits are: %v, LastValidators are: %v\n",
					block.LastCommit.Precommits, s.LastValidators)
				return errors.New(Fmt("Invalid block commit size. Expected %v, got %v",
					s.LastValidators.Size(), len(block.LastCommit.Precommits)))
			}
		*/
		//fmt.Printf("(s *State) validateBlock(), avoid LastValidators and LastCommit.Precommits size check for validatorset change\n")
		lastValidators, _, err := s.GetValidators()

		if err != nil && lastValidators != nil {
			err = lastValidators.VerifyCommit(
				s.ChainID, s.LastBlockID, block.Height-1, block.LastCommit)
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
func (s *State) ApplyBlock(eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, partsHeader types.PartSetHeader, mempool types.Mempool, cch rpcTxHook.CrossChainHelper) error {

	abciResponses, err := s.ValExecBlock(eventCache, proxyAppConn, block, cch)
	if err != nil {
		return fmt.Errorf("Exec failed for application: %v", err)
	}

	fail.Fail() // XXX

	// index txs. This could run in the background
	s.indexTxs(abciResponses)

	// save the results before we commit
	saveABCIResponses(s.db, block.Height, abciResponses)

	fail.Fail() // XXX

	//here handles the proposed next epoch
	nextEpochInBlock := epoch.FromBytes(block.ExData.BlockExData)
	if nextEpochInBlock != nil && nextEpochInBlock.Number != 0 {

		nextEpochInBlock.RS = s.Epoch.RS
		nextEpochInBlock.SetEpochValidatorVoteSet(epoch.NewEpochValidatorVoteSet())
		nextEpochInBlock.Status = epoch.EPOCH_VOTED_NOT_SAVED
		s.Epoch.SetNextEpoch(nextEpochInBlock)

		s.Epoch.Save()
	}

	fail.Fail() // XXX

	plog.Infof("Apply Block height %d, Epoch No %d", block.Height, s.Epoch.Number)
	// Check if need to enter next epoch
	ok, err := s.Epoch.ShouldEnterNewEpoch(block.Height)
	if err != nil {
		log.Warn(Fmt("ApplyBlock(%v): Check Enter new Epoch Failed. Current epoch: %v, error: %v", block.Height, s.Epoch, err))
		return err
	}
	var refund []*abci.RefundValidatorAmount
	var nextEpoch *epoch.Epoch
	if ok {
		// Yes, let create the next epoch and calculate the new validator set and refund list
		nextEpoch, refund, err = s.Epoch.EnterNewEpoch(block.Height)
		if err != nil {
			log.Warn(Fmt("ApplyBlock(%v): Enter New Epoch Failed. Current epoch: %v, error: %v", block.Height, s.Epoch, err))
			return err
		}
	}
	plog.Infof("Apply Block after switch epoch. height %d, Epoch No %d", block.Height, s.Epoch.Number)

	// lock mempool, commit state, update mempoool
	err = s.CommitStateUpdateMempool(proxyAppConn, block, mempool, refund)
	if err != nil {
		return fmt.Errorf("Commit failed for application: %v", err)
	}

	fail.Fail() // XXX

	//here handles when enter new epoch
	s.SetBlockAndEpoch(block.Header, partsHeader)

	// if next Epoch is not nil, let's move to the next Epoch
	if nextEpoch != nil {
		s.Epoch = nextEpoch
		// Update the Epoch in Mempool
		if mem, ok := mempool.(*tdmMempool.Mempool); ok {
			mem.SetEpoch(nextEpoch)
		}
	}

	// save the state
	s.Epoch.Save()
	s.Save()
	return nil
}

// mempool must be locked during commit and update
// because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid
func (s *State) CommitStateUpdateMempool(proxyAppConn proxy.AppConnConsensus, block *types.Block, mempool types.Mempool,
	refundList []*abci.RefundValidatorAmount) error {
	mempool.Lock()
	defer mempool.Unlock()

	// Commit block, get hash back
	lastValidators, _, _ := s.GetValidators()
	res := proxyAppConn.CommitSync(lastValidators.ToAbciValidators(), s.Epoch.RewardPerBlock, refundList)
	if res.IsErr() {
		log.Warn("Error in proxyAppConn.CommitSync", "error", res)
		return res
	}
	if res.Log != "" {
		log.Debug("Commit.Log: " + res.Log)
	}

	log.Info("Committed state", "hash", res.Data)
	// Set the state's new AppHash
	s.AppHash = res.Data

	// Update mempool.
	mempool.Update(block.Height, block.Txs)

	return nil
}

func (s *State) indexTxs(abciResponses *ABCIResponses) {
	// save the tx results using the TxIndexer
	// NOTE: these may be overwriting, but the values should be the same.
	batch := txindex.NewBatch(len(abciResponses.DeliverTx))
	for i, d := range abciResponses.DeliverTx {
		tx := abciResponses.txs[i]
		batch.Add(types.TxResult{
			Height: uint64(abciResponses.Height),
			Index:  uint32(i),
			Tx:     tx,
			Result: *d,
		})
	}
	s.TxIndexer.AddBatch(batch)
}

// Exec and commit a block on the proxyApp without validating or mutating the state
// Returns the application root hash (result of abci.Commit)
func ExecCommitBlock(appConnConsensus proxy.AppConnConsensus, state *State, block *types.Block, cch rpcTxHook.CrossChainHelper) ([]byte, error) {
	var eventCache types.Fireable // nil
	_, err := execBlockOnProxyApp(eventCache, appConnConsensus, state, block, cch)
	if err != nil {
		log.Warn("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// TODO Add Epoch Logic in Replay process

	// Commit block, get hash back
	lastValidators, _, _ := state.GetValidators()
	res := appConnConsensus.CommitSync(lastValidators.ToAbciValidators(), state.Epoch.RewardPerBlock, nil)
	if res.IsErr() {
		log.Warn("Error in proxyAppConn.CommitSync", "error", res)
		return nil, res
	}
	if res.Log != "" {
		log.Info("Commit.Log: " + res.Log)
	}
	return res.Data, nil
}
