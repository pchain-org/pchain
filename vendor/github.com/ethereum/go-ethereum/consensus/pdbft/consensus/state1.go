package consensus

import (
	consss "github.com/ethereum/go-ethereum/consensus"
	ep "github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	sm "github.com/ethereum/go-ethereum/consensus/pdbft/state"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
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

	//initialize round state
	cs.proposer = nil
	cs.isProposer = false
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

	if cs.blockFromMiner != nil && cs.blockFromMiner.NumberU64() >= cs.Height {
		log.Debugf("block %v has been received from miner, not set to nil", cs.blockFromMiner.NumberU64())
	} else {
		cs.blockFromMiner = nil
	}

	// RoundState fields
	cs.updateRoundStep(0, RoundStepNewHeight)
	//cs.StartTime = cs.timeoutParams.Commit(cs.CommitTime)
	if state.TdmExtra.ChainID == params.MainnetChainConfig.PChainId ||
		state.TdmExtra.ChainID == params.TestnetChainConfig.PChainId {

		cs.StartTime = cs.timeoutParams.Commit(time.Now())

	} else {

		if cs.CommitTime.IsZero() {
			cs.StartTime = cs.timeoutParams.Commit(time.Now())
		} else {
			cs.StartTime = cs.timeoutParams.Commit(cs.CommitTime)
		}
	}

	// Reset fields based on state.
	_, validators, _ := state.GetValidators()
	cs.Validators = validators
	cs.Votes = NewHeightVoteSet(cs.chainConfig.PChainId, height, validators, cs.logger)
	cs.VoteSignAggr = NewHeightVoteSignAggr(cs.chainConfig.PChainId, height, validators, cs.logger)

	cs.vrfValIndex = -1
	cs.pastRoundStates = make(map[int]int)

	cs.state = state

	cs.newStep()
}

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (cs *ConsensusState) LoadBlock(height uint64) *types.TdmBlock {

	cr := cs.GetChainReader()

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

func (cs *ConsensusState) LoadLastTendermintExtra() (*types.TendermintExtra, uint64) {

	cr := cs.backend.ChainReader()

	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	if curHeight == 0 {
		return nil, 0
	}

	return cs.LoadTendermintExtra(curHeight)
}

func (cs *ConsensusState) LoadTendermintExtra(height uint64) (*types.TendermintExtra, uint64) {

	cr := cs.backend.ChainReader()

	ethBlock := cr.GetBlockByNumber(height)
	if ethBlock == nil {
		cs.logger.Warn("LoadTendermintExtra. nil block")
		return nil, 0
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		cs.logger.Warnf("LoadTendermintExtra. error: %v", err)
		return nil, 0
	}

	blockHeight := ethBlock.NumberU64()
	if tdmExtra.Height != blockHeight {
		cs.logger.Warnf("extra.height:%v, block.Number %v, reset it", tdmExtra.Height, blockHeight)
		tdmExtra.Height = blockHeight
	}

	return tdmExtra, tdmExtra.Height
}
