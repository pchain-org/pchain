package consensus

import (
	"fmt"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	consss "github.com/ethereum/go-ethereum/consensus"
	//"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	sm "github.com/ethereum/go-ethereum/consensus/tendermint/state"
	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	cmn "github.com/tendermint/go-common"
	"io/ioutil"
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
	cs.nhMtx.Lock()
	defer cs.nhMtx.Unlock()

	//reload the block
	cr := cs.backend.ChainReader()
	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	fmt.Printf("(cs *ConsensusState) StartNewHeight, current block height is %v\n", curHeight)

	state, epoch := cs.InitStateAndEpoch()
	cs.Initialize()
	cs.ApplyBlockEx(curEthBlock, state, epoch)
	cs.UpdateToStateAndEpoch(state, epoch)

	cs.newStep()
	cs.scheduleRound0(cs.GetRoundState())
}

func (cs *ConsensusState) InitStateAndEpoch() (*sm.State, *ep.Epoch) {

	state := &sm.State{}
	var epoch *ep.Epoch = nil
	epochDB := cs.node.EpochDB()

	state.TdmExtra, _ = cs.LoadLastTendermintExtra()

	if state.TdmExtra == nil { //means it it the first block

		genDocFile := cs.node.Config().GetString("genesis_file")
		if !cmn.FileExists(genDocFile) {
			cmn.Exit(cmn.Fmt("InitStateAndEpoch(), Couldn't find GenesisDoc file"))
		}

		jsonBlob, err := ioutil.ReadFile(genDocFile)
		if err != nil {
			cmn.Exit(cmn.Fmt("InitStateAndEpoch(), Couldn't read GenesisDoc file: %v", err))
		}

		genDoc, err := types.GenesisDocFromJSON(jsonBlob)
		if err != nil {
			cmn.PanicSanity(cmn.Fmt("InitStateAndEpoch(), Genesis doc parse json error: %v", err))
		}

		state = sm.MakeGenesisState(/*stateDB, */genDoc)
		//state.Save()

		rewardScheme := ep.MakeRewardScheme(epochDB, &genDoc.RewardScheme)
		epoch = ep.MakeOneEpoch(epochDB, &genDoc.CurrentEpoch)
		epoch.RS = rewardScheme
		fmt.Printf("0 epoch.Validators: %v\n", epoch.Validators)

		if state.TdmExtra.EpochNumber != uint64(epoch.Number) {
			cmn.Exit(cmn.Fmt("InitStateAndEpoch(), initial state error"))
		}
		state.Epoch = epoch
		fmt.Printf("0 state.Epoch.Validators: %v\n", state.Epoch.Validators)
		rewardScheme.Save()
		epoch.Save()

	} else {
		epoch = ep.LoadOneEpoch(epochDB, int(state.TdmExtra.EpochNumber))
		state.Epoch = epoch
		cs.ReconstructLastCommit(state)
	}

	return state, epoch
}


func (cs *ConsensusState) Initialize() {

	//initialize state
	cs.Height = 0
	cs.blockFromMiner = nil

	//initialize round state
	cs.Validators = nil
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.Votes = nil
	cs.CommitRound = -1
	cs.LastCommit = nil
	cs.Epoch = nil
	cs.state = nil
	cs.epoch = nil
}

//apply latest information such as epoch to the new height of consensus
func (bs *ConsensusState) ApplyBlockEx(block *ethTypes.Block, state *sm.State, epoch *ep.Epoch) error {
	return nil
}

// Updates ConsensusState and increments height to match thatRewardScheme of state.
// The round becomes 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) UpdateToStateAndEpoch(state *sm.State, epoch *ep.Epoch) {

	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != state.TdmExtra.Height {
		cmn.PanicSanity(cmn.Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.TdmExtra.Height))
	}

	// Reset fields based on state.
	_, validators, _ := state.GetValidators()
	lastPrecommits := (*types.VoteSet)(nil)
	if cs.CommitRound > -1 && cs.Votes != nil {
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			cmn.PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = cs.Votes.Precommits(cs.CommitRound)
	}

	//Re-Initialized, really need this?
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

	cs.Validators = validators
	cs.Votes = NewHeightVoteSet(cs.config.GetString("chain_id"), height, validators)
	cs.LastCommit = lastPrecommits
	cs.Epoch = epoch

	cs.state = state
	cs.epoch = epoch

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
		Block: ethBlock,
		TdmExtra: TdmExtra,
	}
}

func (bs *ConsensusState) LoadLastTendermintExtra() (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit, current block height is %v\n", curHeight)
	if curHeight == 0 {
		return nil, 0
	}

	return bs.LoadTendermintExtra(curHeight)
}

func (bs *ConsensusState) LoadTendermintExtra(height uint64) (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	fmt.Printf("(cs *ConsensusState) LoadSeenCommit height is %v\n", height)
	ethBlock := cr.GetBlockByNumber(height)
	if ethBlock == nil {
		fmt.Printf("(cs *ConsensusState) LoadSeenCommit nil block\n")
		return nil, 0
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit header is %v\n", header)
	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		fmt.Printf("(cs *ConsensusState) LoadSeenCommit got error: %v\n", err)
		return nil, 0
	}
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit got error: %v\n", err)

	return tdmExtra, tdmExtra.Height
}
