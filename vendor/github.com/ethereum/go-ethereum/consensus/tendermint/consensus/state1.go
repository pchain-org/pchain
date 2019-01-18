package consensus

import (
	consss "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"

	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	sm "github.com/ethereum/go-ethereum/consensus/tendermint/state"
	cmn "github.com/tendermint/go-common"
	"time"
)

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) GetChainReader() consss.ChainReader {
	return bs.backend.ChainReader()
}

//this function is called when the system starts or a block has been inserted into
//the insert could be self/other triggered
//anyway, we start/restart a new height with the latest block update
func (cs *ConsensusState) StartNewHeight() {

	//start locking
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	//reload the block
	cr := cs.backend.ChainReader()
	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	cs.logger.Infof("StartNewHeight. current block height is %v", curHeight)

	state := cs.InitState(cs.Epoch)
	cs.UpdateToState(state)

	cs.newStep()
	cs.scheduleRound0(cs.getRoundState()) //not use cs.GetRoundState to avoid dead-lock
}

func (cs *ConsensusState) InitState(epoch *ep.Epoch) *sm.State {

	state := sm.NewState(cs.logger)

	state.TdmExtra, _ = cs.LoadLastTendermintExtra()
	if state.TdmExtra == nil { //means it it the first block

		state = sm.MakeGenesisState( /*stateDB, */ cs.chainConfig.PChainId, cs.logger)
		//state.Save()

		if state.TdmExtra.EpochNumber != uint64(epoch.Number) {
			cmn.Exit(cmn.Fmt("InitStateAndEpoch(), initial state error"))
		}
		state.Epoch = epoch

		cs.logger.Infof("InitStateAndEpoch. genesis state extra: %#v, epoch validators: %v", state.TdmExtra, epoch.Validators)
	} else {
		state.Epoch = epoch
		cs.ReconstructLastCommit(state)

		cs.logger.Infof("InitStateAndEpoch. state extra: %#v, epoch validators: %v", state.TdmExtra, epoch.Validators)
	}

	return state
}

func (cs *ConsensusState) Initialize() {

	//initialize state
	cs.Height = 0
	cs.blockFromMiner = nil

	//initialize round state
	cs.proposer = nil
	cs.ProposerPeerKey = ""
	cs.Validators = nil
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.Votes = nil
	cs.VoteSignAggr = nil
	cs.PrevoteMaj23SignAggr = nil
	cs.PrecommitMaj23SignAggr = nil
	cs.CommitRound = -1
	cs.state = nil
}

// Updates ConsensusState and increments height to match thatRewardScheme of state.
// The round becomes 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) UpdateToState(state *sm.State) {

	cs.Initialize()

	height := state.TdmExtra.Height + 1
	// Next desired block height
	cs.Height = height

	// RoundState fields
	cs.updateRoundStep(0, RoundStepNewHeight)
	if cs.CommitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.StartTime = cs.timeoutParams.Commit(time.Now())
	} else {
		cs.StartTime = cs.timeoutParams.Commit(cs.CommitTime)
	}

	// Reset fields based on state.
	_, validators, _ := state.GetValidators()
	cs.Validators = validators
	cs.Votes = NewHeightVoteSet(cs.chainConfig.PChainId, height, validators, cs.logger)
	cs.VoteSignAggr = NewHeightVoteSignAggr(cs.chainConfig.PChainId, height, validators, cs.logger)

	cs.state = state

	cs.newStep()
}

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) LoadBlock(height uint64) *types.TdmBlock {

	cr := bs.GetChainReader()

	ethBlock := cr.GetBlockByNumber(height)
	if ethBlock == nil {
		return nil
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	if header == nil {
		return nil
	}
	TdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		return nil
	}

	return &types.TdmBlock{
		Block:    ethBlock,
		TdmExtra: TdmExtra,
	}
}

func (bs *ConsensusState) LoadLastTendermintExtra() (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	if curHeight == 0 {
		return nil, 0
	}

	return bs.LoadTendermintExtra(curHeight)
}

func (bs *ConsensusState) LoadTendermintExtra(height uint64) (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	ethBlock := cr.GetBlockByNumber(height)
	if ethBlock == nil {
		bs.logger.Warn("LoadTendermintExtra. nil block")
		return nil, 0
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		bs.logger.Warnf("LoadTendermintExtra. error: %v", err)
		return nil, 0
	}

	return tdmExtra, tdmExtra.Height
}
