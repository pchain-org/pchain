package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"reflect"
	"sync"
	"time"
	//"runtime/debug"

	"github.com/ebuchman/fail-test"
	p2p "github.com/tendermint/go-p2p"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/go-crypto"
	ep "github.com/tendermint/tendermint/epoch"
	//ethTypes "github.com/ethereum/go-ethereum/core/types"
	//"github.com/ethereum/go-ethereum/common"
)

const (
	newHeightChangeSleepDuration     = 2000 * time.Millisecond
)

//-----------------------------------------------------------------------------
// Timeout Parameters

// TimeoutParams holds timeouts and deltas for each round step.
// All timeouts and deltas in milliseconds.
type TimeoutParams struct {
	Propose0          int
	ProposeDelta      int
	Prevote0          int
	PrevoteDelta      int
	Precommit0        int
	PrecommitDelta    int
	Commit0           int
	SkipTimeoutCommit bool
}

// Wait this long for a proposal
func (tp *TimeoutParams) Propose(round int) time.Duration {
	return time.Duration(tp.Propose0+tp.ProposeDelta*round) * time.Millisecond
}

// After receiving any +2/3 prevote, wait this long for stragglers
func (tp *TimeoutParams) Prevote(round int) time.Duration {
	return time.Duration(tp.Prevote0+tp.PrevoteDelta*round) * time.Millisecond
}

// After receiving any +2/3 precommits, wait this long for stragglers
func (tp *TimeoutParams) Precommit(round int) time.Duration {
	return time.Duration(tp.Precommit0+tp.PrecommitDelta*round) * time.Millisecond
}

// After receiving +2/3 precommits for a single block (a commit), wait this long for stragglers in the next height's RoundStepNewHeight
func (tp *TimeoutParams) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(tp.Commit0) * time.Millisecond)
}

// InitTimeoutParamsFromConfig initializes parameters from config
func InitTimeoutParamsFromConfig(config cfg.Config) *TimeoutParams {
	return &TimeoutParams{
		Propose0:          config.GetInt("timeout_propose"),
		ProposeDelta:      config.GetInt("timeout_propose_delta"),
		Prevote0:          config.GetInt("timeout_prevote"),
		PrevoteDelta:      config.GetInt("timeout_prevote_delta"),
		Precommit0:        config.GetInt("timeout_precommit"),
		PrecommitDelta:    config.GetInt("timeout_precommit_delta"),
		Commit0:           config.GetInt("timeout_commit"),
		SkipTimeoutCommit: config.GetBool("skip_timeout_commit"),
	}
}

//-----------------------------------------------------------------------------
// Errors

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

//-----------------------------------------------------------------------------
// RoundStepType enum type

type RoundStepType uint8 // These must be numeric, ordered.

const (
	RoundStepNewHeight     = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit
	RoundStepNewRound      = RoundStepType(0x02) // Setup new round and go to RoundStepPropose
	RoundStepPropose       = RoundStepType(0x03) // Did propose, gossip proposal
	RoundStepPrevote       = RoundStepType(0x04) // Did prevote, gossip prevotes
	RoundStepPrevoteWait   = RoundStepType(0x05) // Did receive any +2/3 prevotes, start timeout
	RoundStepPrecommit     = RoundStepType(0x06) // Did precommit, gossip precommits
	RoundStepPrecommitWait = RoundStepType(0x07) // Did receive any +2/3 precommits, start timeout
	RoundStepCommit        = RoundStepType(0x08) // Entered commit state machine
	RoundStepTest          = RoundStepType(0x09) // for test author@liaoyd
	// NOTE: RoundStepNewHeight acts as RoundStepCommitWait.
)

func (rs RoundStepType) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPropose:
		return "RoundStepPropose"
	case RoundStepPrevote:
		return "RoundStepPrevote"
	case RoundStepPrevoteWait:
		return "RoundStepPrevoteWait"
	case RoundStepPrecommit:
		return "RoundStepPrecommit"
	case RoundStepPrecommitWait:
		return "RoundStepPrecommitWait"
	case RoundStepCommit:
		return "RoundStepCommit"
	case RoundStepTest:
		return "RoundStepTest"
	default:
		return "RoundStepUnknown" // Cannot panic.
	}
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
// TODO: Actually, only the top pointer is copied,
// so access to field pointers is still racey
type RoundState struct {
	Height             int // Height we are working on
	Round              int
	Step               RoundStepType
	StartTime          time.Time
	CommitTime         time.Time // Subjective time when +2/3 precommits for Block at Round were found
	Epoch              *ep.Epoch
	Validators         *types.ValidatorSet
	Proposal           *types.Proposal
	ProposalBlock      *types.Block
	ProposalBlockParts *types.PartSet
	ProposerPeerKey	   string		// Proposer's peer key
	LockedRound        int
	LockedBlock        *types.Block
	LockedBlockParts   *types.PartSet
	Votes              *HeightVoteSet
	CommitRound        int            //
	LastCommit         *types.VoteSet // Last precommits at Height-1
	LastValidators     *types.ValidatorSet
	PrevoteAggr        *types.VotesAggr
	PrevoteMaj23       *types.Maj23VoteSet
	PrevoteMaj23Parts  *types.PartSet
	PrecommitAggr      *types.VotesAggr
	PrecommitMaj23     *types.Maj23VoteSet
	PrecommitMaj23Parts *types.PartSet
}

func (rs *RoundState) RoundStateEvent() types.EventDataRoundState {
	edrs := types.EventDataRoundState{
		Height:     rs.Height,
		Round:      rs.Round,
		Step:       rs.Step.String(),
		RoundState: rs,
	}
	return edrs
}

func (rs *RoundState) String() string {
	return rs.StringIndented("")
}

func (rs *RoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`RoundState{
%s  H:%v R:%v S:%v
%s  StartTime:     %v
%s  CommitTime:    %v
%s  Validators:    %v
%s  Proposal:      %v
%s  ProposalBlock: %v %v
%s  LockedRound:   %v
%s  LockedBlock:   %v %v
%s  Votes:         %v
%s  LastCommit: %v
%s  LastValidators:    %v
%s}`,
		indent, rs.Height, rs.Round, rs.Step,
		indent, rs.StartTime,
		indent, rs.CommitTime,
		indent, rs.Validators.StringIndented(indent+"    "),
		indent, rs.Proposal,
		indent, rs.ProposalBlockParts.StringShort(), rs.ProposalBlock.StringShort(),
		indent, rs.LockedRound,
		indent, rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort(),
		indent, rs.Votes.StringIndented(indent+"    "),
		indent, rs.LastCommit.StringShort(),
		indent, rs.LastValidators.StringIndented(indent+"    "),
		indent)
}

func (rs *RoundState) StringShort() string {
	return fmt.Sprintf(`RoundState{H:%v R:%v S:%v ST:%v}`,
		rs.Height, rs.Round, rs.Step, rs.StartTime)
}

//-----------------------------------------------------------------------------

var (
	msgQueueSize = 1000
)

// msgs from the reactor which may update the state
type msgInfo struct {
	Msg     ConsensusMessage `json:"msg"`
	PeerKey string           `json:"peer_key"`
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration `json:"duration"`
	Height   int           `json:"height"`
	Round    int           `json:"round"`
	Step     RoundStepType `json:"step"`
}

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v", ti.Duration, ti.Height, ti.Round, ti.Step)
}

// Tracks consensus state across block heights and rounds.
type PrivValidator interface {
	GetAddress() []byte
	GetPubKey() crypto.PubKey
	SignVote(chainID string, vote *types.Vote) error
	SignProposal(chainID string, proposal *types.Proposal) error
	SignValidatorMsg(chainID string, msg *types.ValidatorMsg) error
}

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	BaseService

	config       cfg.Config
	proxyAppConn proxy.AppConnConsensus
	blockStore   types.BlockStore
	mempool      types.Mempool
	privValidator PrivValidator // for signing votes

	nodeInfo	*p2p.NodeInfo	// Validator's node info (ip, port, etc)

	mtx sync.Mutex
	RoundState
	epoch *ep.Epoch
	state *sm.State // State until height-1.

	peerMsgQueue     chan msgInfo   // serializes msgs affecting state (proposals, block parts, votes)
	internalMsgQueue chan msgInfo   // like peerMsgQueue but for our own proposals, parts, votes
	timeoutTicker    TimeoutTicker  // ticker for timeouts
	timeoutParams    *TimeoutParams // parameters and functions for timeout intervals

	evsw types.EventSwitch

	wal        *WAL
	replayMode bool // so we don't log signing errors during replay

	nSteps int // used for testing to limit the number of transitions the state makes

	// allow certain function to be overwritten for testing
	decideProposal func(height, round int)
	doPrevote      func(height, round int)
	setProposal    func(proposal *types.Proposal) error

	done chan struct{}
}

func NewConsensusState(config cfg.Config, state *sm.State, proxyAppConn proxy.AppConnConsensus,
	blockStore types.BlockStore, mempool types.Mempool, epoch *ep.Epoch) *ConsensusState {
	// fmt.Println("state.Validator in newconsensus:", state.Validators)
	cs := &ConsensusState{
		config:           config,
		proxyAppConn:     proxyAppConn,
		blockStore:       blockStore,
		mempool:          mempool,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(),
		timeoutParams:    InitTimeoutParamsFromConfig(config),
		done:             make(chan struct{}),
	}
	// set function defaults (may be overwritten before calling Start)
	cs.decideProposal = cs.defaultDecideProposal
	cs.doPrevote = cs.defaultDoPrevote
	cs.setProposal = cs.defaultSetProposal

	cs.updateToStateAndEpoch(state, epoch)

	// Don't call scheduleRound0 yet.
	// We do that upon Start().
	cs.reconstructLastCommit(state)
	cs.BaseService = *NewBaseService(logger, "ConsensusState", cs)
	return cs
}

//----------------------------------------
// Public interface

// SetEventSwitch implements events.Eventable
func (cs *ConsensusState) SetEventSwitch(evsw types.EventSwitch) {
	cs.evsw = evsw
}

func (cs *ConsensusState) String() string {
	// better not to access shared variables
	return Fmt("ConsensusState") //(H:%v R:%v S:%v", cs.Height, cs.Round, cs.Step)
}

func (cs *ConsensusState) GetState() *sm.State {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.state.Copy()
}

func (cs *ConsensusState) GetRoundState() *RoundState {
//	cs.mtx.Lock()
//	defer cs.mtx.Unlock()
	return cs.getRoundState()
}

func (cs *ConsensusState) getRoundState() *RoundState {
	rs := cs.RoundState // copy
	return &rs
}

func (cs *ConsensusState) GetValidators() (int, []*types.Validator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	_, val, _ := cs.state.GetValidators()
	return cs.state.LastBlockHeight, val.Copy().Validators
}

// Sets our private validator account for signing votes.
func (cs *ConsensusState) SetPrivValidator(priv PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

// Sets our private validator account for signing votes.
func (cs *ConsensusState) GetProposer() (*types.Validator) {
	return cs.Validators.GetProposer()
}

// Returns true if this validator is the proposer.
func (cs *ConsensusState) IsProposer() bool {
	if bytes.Equal(cs.Validators.GetProposer().Address, cs.privValidator.GetAddress()) {
		return true
	} else {
		return false
	}
}

// Set the local timer
func (cs *ConsensusState) SetTimeoutTicker(timeoutTicker TimeoutTicker) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.timeoutTicker = timeoutTicker
}

func (cs *ConsensusState) LoadCommit(height int) *types.Commit {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if height == cs.blockStore.Height() {
		return cs.blockStore.LoadSeenCommit(height)
	}
	return cs.blockStore.LoadBlockCommit(height)
}

func (cs *ConsensusState) OnStart() error {
	if cs.nodeInfo == nil {
		panic("cs.nodeInfo is nil\n")
	}
	
	walFile := cs.config.GetString("cs_wal_file")
	if err := cs.OpenWAL(walFile); err != nil {
		logger.Error("Error loading ConsensusState wal", " error:", err.Error())
		return err
	}

	// we need the timeoutRoutine for replay so
	//  we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	//  firing on the tockChan until the receiveRoutine is started
	//  to deal with them (by that point, at most one will be valid)
	cs.timeoutTicker.Start()

	// we may have lost some votes if the process crashed
	// reload from consensus log to catchup
	if err := cs.catchupReplay(cs.Height); err != nil {
		logger.Error("Error on catchup replay. Proceeding to start ConsensusState anyway", " error:", err.Error())
		// NOTE: if we ever do return an error here,
		// make sure to stop the timeoutTicker
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())

	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
func (cs *ConsensusState) startRoutines(maxSteps int) {
	cs.timeoutTicker.Start()
	go cs.receiveRoutine(maxSteps)
}

func (cs *ConsensusState) OnStop() {
	cs.BaseService.OnStop()

	cs.timeoutTicker.Stop()

	// Make BaseService.Wait() wait until cs.wal.Wait()
	if cs.wal != nil && cs.IsRunning() {
		cs.wal.Wait()
	}
}

// NOTE: be sure to Stop() the event switch and drain
// any event channels or this may deadlock
func (cs *ConsensusState) Wait() {
	<-cs.done
}

// Open file to log all consensus messages and timeouts for deterministic accountability
func (cs *ConsensusState) OpenWAL(walFile string) (err error) {
	err = EnsureDir(path.Dir(walFile), 0700)
	if err != nil {
		logger.Error("Error ensuring ConsensusState wal dir", " error:", err.Error())
		return err
	}

	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	wal, err := NewWAL(walFile, cs.config.GetBool("cs_wal_light"))
	if err != nil {
		return err
	}
	cs.wal = wal
	return nil
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state,
// possibly causing a state transition
// TODO: should these return anything or let callers just use events?

// May block on send if queue is full.
func (cs *ConsensusState) AddVote(vote *types.Vote, peerKey string) (added bool, err error) {
	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerKey}
	}

	// TODO: wait for event?!
	return false, nil
}

// May block on send if queue is full.
func (cs *ConsensusState) SetProposal(proposal *types.Proposal, peerKey string) error {

	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerKey}
	}

	// TODO: wait for event?!
	return nil
}

// May block on send if queue is full.
func (cs *ConsensusState) AddProposalBlockPart(height, round int, part *types.Part, peerKey string) error {

	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerKey}
	}

	// TODO: wait for event?!
	return nil
}

// May block on send if queue is full.
func (cs *ConsensusState) SetProposalAndBlock(proposal *types.Proposal, block *types.Block, parts *types.PartSet, peerKey string) error {
	cs.SetProposal(proposal, peerKey)
	for i := 0; i < parts.Total(); i++ {
		part := parts.GetPart(i)
		cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerKey)
	}
	return nil // TODO errors
}

// Set node info wich is about current validator's peer info
func (cs *ConsensusState) SetNodeInfo(nodeInfo *p2p.NodeInfo) {
	cs.nodeInfo = nodeInfo
}

//------------------------------------------------------------
// internal functions for managing the state

func (cs *ConsensusState) updateHeight(height int) {
	cs.Height = height
}

func (cs *ConsensusState) updateRoundStep(round int, step RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *RoundState) {
	//logger.Info("scheduleRound0", "now", time.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(time.Now())
	cs.scheduleTimeout(sleepDuration, rs.Height, 0, RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height, round int, step RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (cs *ConsensusState) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		// TODO: use CList here for strict determinism and
		// attempt push to internalMsgQueue in receiveRoutine
		logger.Warn("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(state *sm.State) {
	if state.LastBlockHeight == 0 {
		return
	}
	seenCommit := cs.blockStore.LoadSeenCommit(state.LastBlockHeight)
	lastValidators, _, _ := state.GetValidators()
	lastPrecommits := types.NewVoteSet(cs.config.GetString("chain_id"), state.LastBlockHeight, seenCommit.Round(), types.VoteTypePrecommit, lastValidators)

	fmt.Printf("seenCommit are: %v\n", seenCommit)
	fmt.Printf("lastPrecommits are: %v\n", lastPrecommits)

	for _, precommit := range seenCommit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := lastPrecommits.AddVote(precommit)
		if !added || err != nil {
			PanicCrisis(Fmt("Failed to reconstruct LastCommit: %v", err))
		}
	}
	if !lastPrecommits.HasTwoThirdsMajority() {
		PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}
	cs.LastCommit = lastPrecommits
}

// Updates ConsensusState and increments height to match thatRewardScheme of state.
// The round becomes 0 and cs.Step becomes RoundStepNewHeight.
func (cs *ConsensusState) updateToStateAndEpoch(state *sm.State, epoch *ep.Epoch) {
	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != state.LastBlockHeight {
		PanicSanity(Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}
	if cs.state != nil && cs.state.LastBlockHeight+1 != cs.Height {
		// This might happen when someone else is mutating cs.state.
		// Someone forgot to pass in state.Copy() somewhere?!
		PanicSanity(Fmt("Inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
			cs.state.LastBlockHeight+1, cs.Height))
	}

	// If state isn't further out than cs.state, just ignore.
	// This happens when SwitchToConsensus() is called in the reactor.
	// We don't want to reset e.g. the Votes.
	if cs.state != nil && (state.LastBlockHeight <= cs.state.LastBlockHeight) {
		logger.Info("Ignoring updateToState()", " newHeight:", state.LastBlockHeight+1, " oldHeight:", cs.state.LastBlockHeight+1)
		return
	}

	// Reset fields based on state.
	_, validators, _ := state.GetValidators()
	//liaoyd
	// fmt.Println("validators:", validators)
	lastPrecommits := (*types.VoteSet)(nil)
	if cs.CommitRound > -1 && cs.Votes != nil {
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = cs.Votes.Precommits(cs.CommitRound)
	}

	// Next desired block height
	height := state.LastBlockHeight + 1

	// RoundState fields
	cs.updateHeight(height)
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
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.PrevoteAggr = nil
	cs.PrevoteMaj23 = nil
	cs.PrevoteMaj23Parts = nil
	cs.PrecommitAggr = nil
	cs.PrecommitMaj23 = nil
	cs.PrecommitMaj23Parts = nil
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.Votes = NewHeightVoteSet(cs.config.GetString("chain_id"), height, validators)
	cs.CommitRound = -1
	cs.LastCommit = lastPrecommits
	cs.Epoch = epoch

	//fmt.Printf("State.Copy(), cs.LastValidators are: %v, state.LastValidators are: %v\n",
	//	cs.LastValidators, state.LastValidators)
	//debug.PrintStack()

	cs.LastValidators, _, _ = state.GetValidators()

	cs.state = state

	cs.epoch = epoch

	cs.newStep()
}

func (cs *ConsensusState) newStep() {
	rs := cs.RoundStateEvent()
	cs.wal.Save(rs)
	cs.nSteps += 1
	// newStep is called by updateToStep in NewConsensusState before the evsw is set!
	if cs.evsw != nil {
		types.FireEventNewRoundStep(cs.evsw, rs)
	}
}

//-----------------------------------------
// the main go routines

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities
func (cs *ConsensusState) receiveRoutine(maxSteps int) {
	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				logger.Warn("reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		rs := cs.RoundState
		var mi msgInfo

		select {
		case mi = <-cs.peerMsgQueue:
			logger.Debug(Fmt("Got msg from peer queue: %+v\n", mi))
			cs.wal.Save(mi)
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(mi, rs)
		case mi = <-cs.internalMsgQueue:
			logger.Debug(Fmt("Got msg from internal queue %+v\n", mi))
			cs.wal.Save(mi)
			// handles proposals, block parts, votes
			cs.handleMsg(mi, rs)
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			cs.wal.Save(ti)
			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		case <-cs.Quit:

			// NOTE: the internalMsgQueue may have signed messages from our
			// priv_val that haven't hit the WAL, but its ok because
			// priv_val tracks LastSig

			// close wal now that we're done writing to it
			if cs.wal != nil {
				cs.wal.Stop()
			}

			close(cs.done)
			return
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo, rs RoundState) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var err error
	msg, peerKey := mi.Msg, mi.PeerKey
	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		logger.Debug(Fmt("handleMsg: Received proposal message %+v\n", msg))
		err = cs.setProposal(msg.Proposal)
	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		logger.Debug(Fmt("handleMsg: Received proposal block part message %+v\n", msg.Part))
		_, err = cs.addProposalBlockPart(msg.Height, msg.Part, peerKey != "")
		if err != nil && msg.Round != cs.Round {
			err = nil
		}
        case *Maj23VotesAggrMessage:
		// Msg saying a set of 2/3+ votes had been received
		logger.Debug(Fmt("handleMsg1: Received Maj23VotesAggrMessage %#v\n", (msg.Maj23VotesAggr)))
		err = cs.setMaj23VotesAggr(msg.Maj23VotesAggr)
        case *VotesAggrPartMessage:
		// Major 2/3+ vote (prevote/precommit) part message, if the votes are complete,
		// we'll enterPrecommit or tryFinalizeCommit
		_, err = cs.addMaj23VotesPart(msg.Height, msg.Part, msg.Type, peerKey != "")
		if err != nil && msg.Round != cs.Round {
			err = nil
		}
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		err := cs.tryAddVote(msg.Vote, peerKey)
		if err == ErrAddingVote {
			// TODO: punish peer
		}

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.
		// We could make note of this and help filter in broadcastHasVoteMessage().
	default:
		logger.Warn("Unknown msg type ", reflect.TypeOf(msg))
	}
	if err != nil {
		logger.Error("Error with msg", " type:", reflect.TypeOf(msg), " peer:", peerKey, " error:", err, " msg:", msg)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs RoundState) {
	logger.Debug("Received tock", " timeout:", ti.Duration, " height:", ti.Height, " round:", ti.Round, " step:", ti.Step)

	// timeouts must be for current height, round, step
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) {
		logger.Debug("Ignoring tock because we're ahead", " height:", rs.Height, " round:", rs.Round, " step:", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		// XXX: should we fire timeout here (for timeout commit)?
		cs.enterNewRound(ti.Height, 0)
	case RoundStepPropose:
		types.FireEventTimeoutPropose(cs.evsw, cs.RoundStateEvent())
		cs.enterPrevote(ti.Height, ti.Round)
	case RoundStepPrevoteWait:
		types.FireEventTimeoutWait(cs.evsw, cs.RoundStateEvent())
		cs.enterPrecommit(ti.Height, ti.Round)
	case RoundStepPrecommitWait:
		types.FireEventTimeoutWait(cs.evsw, cs.RoundStateEvent())
		cs.enterNewRound(ti.Height, ti.Round+1)
	default:
		panic(Fmt("Invalid timeout step: %v", ti.Step))
	}

}

//-----------------------------------------------------------------------------
// State functions
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: `startTime = commitTime+timeoutCommit` from NewHeight(height)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != RoundStepNewHeight) {
		logger.Debug(Fmt("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}



	if now := time.Now(); cs.StartTime.After(now) {
		logger.Warn("Need to set a buffer and logger.Warn() here for sanity.", "startTime", cs.StartTime, "now", now)
	}

	logger.Info(Fmt("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	//liaoyd
	// fmt.Println("in func (cs *ConsensusState) enterNewRound(height int, round int)")
	fmt.Println(cs.Validators)
	// Increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		validators = validators.Copy()
		validators.IncrementAccum(round - cs.Round)
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, RoundStepNewRound)
	cs.Validators = validators
	if round == 0 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
		cs.PrevoteAggr = nil
		cs.PrevoteMaj23 = nil
		cs.PrevoteMaj23Parts = nil
		cs.PrecommitAggr = nil
		cs.PrecommitMaj23 = nil
		cs.PrecommitMaj23Parts = nil
	}
	cs.Votes.SetRound(round + 1) // also track next round (round+1) to allow round-skipping

	types.FireEventNewRound(cs.evsw, cs.RoundStateEvent())

	// Immediately go to enterPropose.
	cs.enterPropose(height, round)
}

// Enter: from NewRound(height,round).
func (cs *ConsensusState) enterPropose(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPropose <= cs.Step) {
		logger.Debug(Fmt("enterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(Fmt("enterPropose(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, RoundStepPropose)
		cs.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}()

	// If we don't get the proposal and all block parts quick enough, enterPrevote
	cs.scheduleTimeout(cs.timeoutParams.Propose(round), height, round, RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		fmt.Println("we are not validator yet!!!!!!!!saaaaaaad")
		return
	}

	if !bytes.Equal(cs.Validators.GetProposer().Address, cs.privValidator.GetAddress()) {
		fmt.Println("we are not proposer!!!")
		logger.Info("enterPropose: Not our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
	} else {
		fmt.Println("we are proposer!!!")
		logger.Info("enterPropose: Our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
		cs.decideProposal(height, round)

	}
}

func (cs *ConsensusState) defaultDecideProposal(height, round int) {
	var block *types.Block
	var blockParts *types.PartSet
	var proposerPeerKey string

//logger.Debug(Fmt("defaultDecideProposal: ConsensusState %+v\n", cs))

	// Decide on block
	if cs.LockedBlock != nil {
		// Use current validator or last proposer's peer key?
		proposerPeerKey = "peer key from locked block"

		// If we're locked onto a block, just choose that.
		block, blockParts = cs.LockedBlock, cs.LockedBlockParts
	} else {
		// Need to find some way to get current validators nodeInfo
		if cs.nodeInfo != nil {
			proposerPeerKey = cs.nodeInfo.ListenAddres()
		} else {
			panic("cs.nodeInfo is nil\n")
		}

		// fmt.Println("defaultDecideProposal: cs nodeInfo %#v\n", cs.nodeInfo)
		logger.Debug(Fmt("defaultDecideProposal: Proposer peer key %s", proposerPeerKey))

		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
		if block == nil { // on error
			return
		}
	}

	// Make proposal
	polRound, polBlockID := cs.Votes.POLInfo()
	proposal := types.NewProposal(height, round, blockParts.Header(), polRound, polBlockID, proposerPeerKey)
	err := cs.privValidator.SignProposal(cs.state.ChainID, proposal)
	if err == nil {
		// Set fields
		/*  fields set by setProposal and addBlockPart
		cs.Proposal = proposal
		cs.ProposalBlock = block
		cs.ProposalBlockParts = blockParts
		cs.ProposerPeerKey = proposerPeerKey
		*/

		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})

		logger.Debug(Fmt("defaultDecideProposal: Proposal to send is %#v\n", proposal))
		//logger.Debug(Fmt("ProposalMessage to send is %+v\n", msgInfo{&ProposalMessage{proposal}, ""}))

		for i := 0; i < blockParts.Total(); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
		logger.Info("Signed proposal", " height:", height, " round:", round, " proposal:", proposal)
		//logger.Debug(Fmt("Signed proposal block: %v", block))
	} else {
		if !cs.replayMode {
			logger.Warn("enterPropose: Error signing proposal", " height:", height, " round:", round, " error:", err)
		}
	}
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
func (cs *ConsensusState) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound < 0 {
		return true
	} else {
		// if this is false the proposer is lying or we haven't received the POL yet
		return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()
	}
}

// Create the next block to propose and return it.
// Returns nil block upon error.
// NOTE: keep it side-effect free for clarity.
func (cs *ConsensusState) createProposalBlock() (block *types.Block, blockParts *types.PartSet) {
	var commit *types.Commit
	if cs.Height == 1 {
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = &types.Commit{}
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()
	} else {
		// This shouldn't happen.
		logger.Error("enterPropose: Cannot propose anything: No commit for the previous block.")
		return
	}

	// Mempool validated transactions
	txs := cs.mempool.Reap(cs.config.GetInt("block_size"))

	epTxs, err := cs.Epoch.ProposeTransactions("proposer", cs.Height)
	if err != nil {
		return nil, nil
	}

	if len(epTxs) != 0 {
		fmt.Printf("createProposalBlock(), epoch propose %v txs\n", len(epTxs))
		txs = append(txs, epTxs...)
	}

	var epochBytes []byte = []byte{}
	shouldProposeEpoch := cs.Epoch.ShouldProposeNextEpoch(cs.Height)
	if shouldProposeEpoch {
		cs.Epoch.SetNextEpoch(cs.Epoch.ProposeNextEpoch(cs.Height))
		epochBytes = cs.Epoch.NextEpoch.Bytes()
	}

	_, val, _ := cs.state.GetValidators()

	return types.MakeBlock(cs.Height, cs.state.ChainID, txs, commit,
		cs.state.LastBlockID, val.Hash(), cs.state.AppHash,
		epochBytes, cs.config.GetInt("block_part_size"))
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevote <= cs.Step) {
		logger.Debug(Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, RoundStepPrevote)
		cs.newStep()
	}()

	// fire event for how we got here
	if cs.isProposalComplete() {
		types.FireEventCompleteProposal(cs.evsw, cs.RoundStateEvent())
	} else {
		// we received +2/3 prevotes for a future round
		// TODO: catchup event?
	}

	logger.Info(Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) defaultDoPrevote(height int, round int) {
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Warn("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Valdiate proposal block
	err := cs.state.ValidateBlock(cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Warn("enterPrevote: ProposalBlock is invalid", " error:", err)
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Valdiate proposal block
	proposedNextEpoch := ep.FromBytes(cs.ProposalBlock.ExData.BlockExData)
	if proposedNextEpoch != nil {
		err = cs.RoundState.Epoch.ValidateNextEpoch(proposedNextEpoch, height)
		if err != nil {
			// ProposalBlock is invalid, prevote nil.
			logger.Warn("enterPrevote: Proposal reward scheme is invalid", "error", err)
			cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
			return
		}
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	cs.signAddVote(types.VoteTypePrevote, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	return
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevoteWait <= cs.Step) {
		logger.Debug(Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		PanicSanity(Fmt("enterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}
	logger.Info(Fmt("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.timeoutParams.Prevote(round), height, round, RoundStepPrevoteWait)
}

// Enter: +2/3 precomits for block or nil.
// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *ConsensusState) enterPrecommit(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommit <= cs.Step) {
		logger.Debug(Fmt("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	logger.Info(Fmt("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, RoundStepPrecommit)
		cs.newStep()
	}()

	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil
	types.FireEventPolka(cs.evsw, cs.RoundStateEvent())

	// the latest POLRound should be this round
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		PanicSanity(Fmt("This POLRound should be %v but got %", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = 0
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		types.FireEventRelock(cs.evsw, cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID.Hash, blockID.PartsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", " hash:", blockID.Hash)
		// Validate the block.
		if err := cs.state.ValidateBlock(cs.ProposalBlock); err != nil {
			PanicConsensus(Fmt("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		types.FireEventLock(cs.evsw, cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID.Hash, blockID.PartsHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
	}
	types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
	cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
	return
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height int, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommitWait <= cs.Step) {
		logger.Debug(Fmt("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		PanicSanity(Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	logger.Info(Fmt("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.updateRoundStep(round, RoundStepPrecommitWait)
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.timeoutParams.Precommit(round), height, round, RoundStepPrecommitWait)

}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height int, commitRound int) {
	if cs.Height != height || RoundStepCommit <= cs.Step {
		logger.Debug(Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = time.Now()
		cs.newStep()

		// Maybe finalize immediately.
		cs.tryFinalizeCommit(height)
	}()

	blockID, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		PanicSanity("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToState.
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
		} else {
			// We just need to keep waiting.
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height int) {
	if cs.Height != height {
		PanicSanity(Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || len(blockID.Hash) == 0 {
		logger.Warn("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.", " height:", height)
		return
	}
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		// TODO: this happens every time if we're not a validator (ugly logs)
		// TODO: ^^ wait, why does it matter that we're a validator?
		logger.Warn("Attempt to finalize failed. We don't have the commit block.", " height:", height, "proposal-block", cs.ProposalBlock.Hash(), "commit-block", blockID.Hash)
		return
	}
	//	go
	cs.finalizeCommit(height)
}

// Increment height and goto RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height int) {
	if cs.Height != height || cs.Step != RoundStepCommit {
		logger.Debug(Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

logger.Info("finalizeCommit: beginning", "cur height", cs.Height, "cur round", cs.Round)

	// fmt.Println("precommits:", cs.Votes.Precommits(cs.CommitRound))
	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		PanicSanity(Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !blockParts.HasHeader(blockID.PartsHeader) {
		PanicSanity(Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !block.HashesTo(blockID.Hash) {
		PanicSanity(Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if err := cs.state.ValidateBlock(block); err != nil {
		PanicConsensus(Fmt("+2/3 committed an invalid block: %v", err))
	}

	logger.Info(Fmt("Finalizing commit of block with %d txs", block.NumTxs),
		"height", block.Height, "hash", block.Hash(), "root", block.AppHash)
	logger.Info(Fmt("%v", block))

logger.Info("finalizeCommit: Wait for 15 minues before new height", "cur height", cs.Height, "cur round", cs.Round)
//logger.Info(Fmt("finalizeCommit: cs.State: %#v\n", cs.GetRoundState()))
time.Sleep(newHeightChangeSleepDuration)

	fail.Fail() // XXX

	// Save to blockStore.
	if cs.blockStore.Height() < block.Height {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound)
		seenCommit := precommits.MakeCommit()
		cs.blockStore.SaveBlock(block, blockParts, seenCommit)
	} else {
		// Happens during replay if we already saved the block but didn't commit
		logger.Info("Calling finalizeCommit on already stored block", " height:", block.Height)
	}

	fail.Fail() // XXX

	// Finish writing to the WAL for this height.
	// NOTE: If we fail before writing this, we'll never write it,
	// and just recover by running ApplyBlock in the Handshake.
	// If we moved it before persisting the block, we'd have to allow
	// WAL replay for blocks with an #ENDHEIGHT
	// As is, ConsensusState should not be started again
	// until we successfully call ApplyBlock (ie. here or in Handshake after restart)
	if cs.wal != nil {
		cs.wal.writeEndHeight(height)
	}

	fail.Fail() // XXX

	// Create a copy of the state for staging
	// and an event cache for txs
	stateCopy := cs.state.Copy()
	eventCache := types.NewEventCache(cs.evsw)

	// epochCopy := cs.epoch.Copy()
	// Execute and commit the block, update and save the state, and update the mempool.
	// All calls to the proxyAppConn come here.
	// NOTE: the block.AppHash wont reflect these txs until the next block
	err := stateCopy.ApplyBlock(eventCache, cs.proxyAppConn, block, blockParts.Header(), cs.mempool)
	if err != nil {
		logger.Error("Error on ApplyBlock. Did the application crash? Please restart tendermint", " error:", err)
		return
	}

	fail.Fail() // XXX

	// Fire event for new block.
	// NOTE: If we fail before firing, these events will never fire
	//
	// TODO: Either
	// 	* Fire before persisting state, in ApplyBlock
	//	* Fire on start up if we haven't written any new WAL msgs
	//   Both options mean we may fire more than once. Is that fine ?
	types.FireEventNewBlock(cs.evsw, types.EventDataNewBlock{block})
	types.FireEventNewBlockHeader(cs.evsw, types.EventDataNewBlockHeader{block.Header})
	eventCache.Flush()

	fail.Fail() // XXX

	// NewHeightStep!
	cs.updateToStateAndEpoch(stateCopy, stateCopy.Epoch)

	fail.Fail() // XXX

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(&cs.RoundState)

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
	return
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) defaultSetProposal(proposal *types.Proposal) error {
	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// We don't care about the proposal if we're already in RoundStepCommit.
	if RoundStepCommit <= cs.Step {
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if proposal.POLRound != -1 &&
		(proposal.POLRound < 0 || proposal.Round <= proposal.POLRound) {
		return ErrInvalidProposalPOLRound
	}

	// Verify signature
	if !cs.Validators.GetProposer().PubKey.VerifyBytes(types.SignBytes(cs.state.ChainID, proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	cs.ProposerPeerKey = proposal.ProposerPeerKey
	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit, once we have the full block.
func (cs *ConsensusState) addProposalBlockPart(height int, part *types.Part, verify bool) (added bool, err error) {
	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.ProposalBlockParts.AddPart(part, verify)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalBlockParts.IsComplete() {
		// Added and completed!
		var n int
		var err error
		cs.ProposalBlock = wire.ReadBinary(&types.Block{}, cs.ProposalBlockParts.GetReader(), types.MaxBlockSize, &n, &err).(*types.Block)
		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		logger.Info("Received complete proposal block", " height:", cs.ProposalBlock.Height, " hash:", cs.ProposalBlock.Hash())
		fmt.Printf("Received complete proposal block is %v\n", cs.ProposalBlock.String())
		fmt.Printf("block.LastCommit is %v\n", cs.ProposalBlock.LastCommit)
		if cs.Step == RoundStepPropose && cs.isProposalComplete() {
			// Move onto the next step
			cs.enterPrevote(height, cs.Round)
		} else if cs.Step == RoundStepCommit {
			// If we're waiting on the proposal block...
			cs.tryFinalizeCommit(height)
		}
		return true, err
	}
	return added, nil
}

// -----------------------------------------------------------------------------
func (cs *ConsensusState) setMaj23VotesAggr(votesAggr *types.VotesAggr) error {
	// Does not apply
	if votesAggr.Height != cs.Height || votesAggr.Round != cs.Round {
		return nil
	}

	//fmt.Printf("Received VotesAggr %#v\n", votesAggr)
/*
	// TODO : Need following check

	// We don't care about the votes if we're already in RoundStepCommit.
	if RoundStepCommit <= cs.Step {
		return nil
	}

	// TODO : Verify signature, which is not added now?
	if !cs.Validators.GetProposer().PubKey.VerifyBytes(types.SignBytes(cs.state.ChainID, votesAggr), proposal.Signature) {
		return ErrInvalidProposalSignature
	}
*/

	if votesAggr.Type == types.VoteTypePrevote {
		if cs.PrevoteAggr != nil {
			return nil
		}
		
		cs.PrevoteAggr = votesAggr
		cs.PrevoteMaj23Parts = types.NewPartSetFromHeader(votesAggr.VotePartsHeader)
		fmt.Printf("setMaj23VotesAggr:prevote aggr %#v\n", cs.PrevoteAggr)
	} else if votesAggr.Type == types.VoteTypePrecommit {
		if cs.PrecommitAggr != nil {
			return nil
		}
		
		cs.PrecommitAggr = votesAggr
		cs.PrecommitMaj23Parts = types.NewPartSetFromHeader(votesAggr.VotePartsHeader)
		fmt.Printf("setMaj23VotesAggr:precommit aggr %#v\n", cs.PrecommitAggr)
	}


	return nil
}

func (cs *ConsensusState) addMaj23VotesPart(height int, part *types.Part, votetype byte, verify bool) (added bool, err error) {
	fmt.Printf("Received Maj23VotesPart %+v\n", part)

	if votetype == types.VoteTypePrevote {
		return cs.addPrevotesAggrPart(height, part, true)
	} else {
		if votetype != types.VoteTypePrecommit {
			panic(Fmt("Invalid VotesAggrPart type %d", votetype))
		}

		return cs.addPrecommitsAggrPart(height, part, true)
	}
}

func (cs *ConsensusState) addPrevotesAggrPart(height int, part *types.Part, verify bool) (added bool, err error) {
	fmt.Printf("Enter addPrevotesAggrPart\n")

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if cs.PrevoteMaj23Parts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	logger.Debug(Fmt("addPrevotesAggrPart:part add result %d  add part %#v\n", added, part))

	added, err = cs.PrevoteMaj23Parts.AddPart(part, verify)
	if err != nil {
		logger.Info(Fmt("addPrevotesAggrPart:failed to add part %#v\n", part))
		return added, err
	}

	if added && cs.PrevoteMaj23Parts.IsComplete() {
		// Added and completed!
		var n int
		var err error

		cs.PrevoteMaj23 = wire.ReadBinary(&types.Maj23VoteSet{}, cs.PrevoteMaj23Parts.GetReader(), types.MaxVoteSetSize, &n, &err).(*types.Maj23VoteSet)

		logger.Debug(Fmt("addPrevotesAggrPart:Received complete prevote set aggr %#v\n", cs.PrevoteMaj23))
		logger.Debug(Fmt("addPrevotesAggrPart:Current prevote set %+v\n", cs.Votes.Prevotes(cs.Round)))

		for _, vote := range cs.PrevoteMaj23.Votes {
			if (vote.Height != cs.Height || vote.Round != cs.Round) {
				logger.Warn(Fmt("addPrevotesAggrPart:Invalid Vote (H %d R %d) current state (H %d %d)\n", vote.Height, vote.Round, cs.Height, cs.Round))
			}

			logger.Debug(Fmt("addPrevotesAggrPart:Try add Vote (H %d R %d T %d VALIDX %d) current state (H %d %d)\n", vote.Height, vote.Round, vote.Type, vote.ValidatorIndex, cs.Height, cs.Round))
			
			logger.Debug(Fmt("addPrevotesAggrPart:Vote to add %s\n", vote.String()))

			added, err = cs.Votes.AddVoteNoPeer(vote)

			if added == false {
				logger.Warn(Fmt("addPrevotesAggrPart: failed to added vote %#v\n", vote))
			}

			prevotes := cs.Votes.Prevotes(cs.Round)

			if prevotes.HasTwoThirdsMajority() {
				logger.Debug(Fmt("addPrevotesAggrPart:Received 2/3+ prevotes, enter precommit\n"))
				cs.enterPrecommit(height, vote.Round)
			}
		}

		return true, err
	}

	//logger.Debug(Fmt("addPrevotesAggrPart:part set header %#v\n", cs.PrevoteMaj23Parts))

	logger.Debug(Fmt("addPrevotesAggrPart:added but not complete for part %#v\n", part))

	return added, nil
}

func (cs *ConsensusState) addPrecommitsAggrPart(height int, part *types.Part, verify bool) (added bool, err error) {
//	logger.Info("Eneter addPrecommitsAggrPart")

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if cs.PrecommitMaj23Parts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = cs.PrecommitMaj23Parts.AddPart(part, verify)
	if err != nil {
		return added, err
	}
	if added && cs.PrecommitMaj23Parts.IsComplete() {
		// Added and completed!
		var n int
		var err error

		cs.PrecommitMaj23 = wire.ReadBinary(&types.Maj23VoteSet{}, cs.PrecommitMaj23Parts.GetReader(), types.MaxVoteSetSize, &n, &err).(*types.Maj23VoteSet)

		logger.Debug(Fmt("Received complete precommit vote set %v\n", cs.PrecommitAggr.String()))

		for _, vote := range cs.PrecommitMaj23.Votes {
			if (vote.Height != cs.Height || vote.Round != cs.Round) {
				logger.Warn(Fmt("addPrecommitsAggrPart:Invalid Vote (H %d R %d) current state (H %d %d)\n", vote.Height, vote.Round, cs.Height, cs.Round))
			}

			logger.Debug(Fmt("addPrecommitsAggrPart:Add Vote (H %d R %d T %d VALIDX %d) current state (H %d %d)\n", vote.Height, vote.Round, vote.Type, vote.ValidatorIndex, cs.Height, cs.Round))
			
			if cs.IsProposer() == false {
				added, err = cs.Votes.AddVoteNoPeer(vote)

				if added == false {
					fmt.Printf("addPrecommitsAggrPart: failed to add vote %#v\n", vote)
				}
			} else {
				logger.Debug("addPrevotesAggrPart: Proposer skip insert 2/3+ precommits votes\n")
			}

			precommits := cs.Votes.Precommits(cs.Round)

			if precommits.HasTwoThirdsMajority() {
				logger.Debug(Fmt("addPrevotesAggrPart:Received 2/3+ precommits, enter commit\n"))

				// TODO : Shall go to this state?
				// cs.tryFinalizeCommit(height)
				cs.enterCommit(height, cs.Round)
			}
		}

		return true, err
	}

	logger.Debug(Fmt("addPrecommitsAggrPart: added but not complete for part %#v\n", part))

	return added, nil
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(vote *types.Vote, peerKey string) error {
	_, err := cs.addVote(vote, peerKey)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, broadcast evidence tx for slashing.
		// If it's otherwise invalid, punish peer.
		if err == ErrVoteHeightMismatch {
			return err
		} else if _, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if peerKey == "" {
				logger.Warn("Found conflicting vote from ourselves. Did you unsafe_reset a validator?", " height:", vote.Height, " round:", vote.Round, " type:", vote.Type)
				return err
			}
			logger.Warn("Found conflicting vote. Publish evidence (TODO)")
			/* TODO
			evidenceTx := &types.DupeoutTx{
				Address: address,
				VoteA:   *errDupe.VoteA,
				VoteB:   *errDupe.VoteB,
			}
			cs.mempool.BroadcastTx(struct{???}{evidenceTx}) // shouldn't need to check returned err
			*/
			return err
		} else {
			// Probably an invalid signature. Bad peer.
			logger.Warn("Error attempting to add vote", " error:", err)
			return ErrAddingVote
		}
	}
	return nil
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(vote *types.Vote, peerKey string) (added bool, err error) {
	logger.Debug("addVote", "voteHeight", vote.Height, "voteType", vote.Type, "csHeight", cs.Height)

	logger.Debug(Fmt("addVote: add vote %s\n", vote.String()))
/*
	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height+1 == cs.Height {
		if !(cs.Step == RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
			// TODO: give the reason ..
			// fmt.Errorf("tryAddVote: Wrong height, not a LastCommit straggler commit.")
			return added, ErrVoteHeightMismatch
		}
		added, err = cs.LastCommit.AddVote(vote)
		if added {
			logger.Info(Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
			types.FireEventVote(cs.evsw, types.EventDataVote{vote})

			// if we can skip timeoutCommit and have all the votes now,
			if cs.timeoutParams.SkipTimeoutCommit && cs.LastCommit.HasAll() {
				// go straight to new round (skip timeout commit)
				// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, RoundStepNewHeight)
				cs.enterNewRound(cs.Height, 0)
			}
		}

		return
	}
*/

	// A prevote/precommit for this height?
	if vote.Height == cs.Height {
//		height := cs.Height
		added, err = cs.Votes.AddVote(vote, peerKey)
		if added {
			if vote.Type == types.VoteTypePrevote {
				// If 2/3+ votes received, send them to other validators
				if cs.Votes.Prevotes(cs.Round).HasTwoThirdsMajority() {
					logger.Debug(Fmt("addVote: Got 2/3+ prevotes %+v\n", cs.Votes.Prevotes(cs.Round)))
					cs.sendMaj23Vote(vote.Type)
				}
			} else if vote.Type == types.VoteTypePrecommit {
				if cs.Votes.Precommits(cs.Round).HasTwoThirdsMajority() {
					logger.Debug(Fmt("addVote: Got 2/3+ precommits %+v\n", cs.Votes.Prevotes(cs.Round)))
					cs.sendMaj23Vote(vote.Type)
				}
			}

/*
			types.FireEventVote(cs.evsw, types.EventDataVote{vote})

			switch vote.Type {
			case types.VoteTypePrevote:
				prevotes := cs.Votes.Prevotes(vote.Round)
				logger.Info("Added to prevote", " vote:", vote, " prevotes:", prevotes.StringShort())
				// First, unlock if prevotes is a valid POL.
				// >> lockRound < POLRound <= unlockOrChangeLockRound (see spec)
				// NOTE: If (lockRound < POLRound) but !(POLRound <= unlockOrChangeLockRound),
				// we'll still enterNewRound(H,vote.R) and enterPrecommit(H,vote.R) to process it
				// there.
				if (cs.LockedBlock != nil) && (cs.LockedRound < vote.Round) && (vote.Round <= cs.Round) {
					blockID, ok := prevotes.TwoThirdsMajority()
					if ok && !cs.LockedBlock.HashesTo(blockID.Hash) {
						logger.Info("Unlocking because of POL.", " lockedRound:", cs.LockedRound, " POLRound:", vote.Round)
						cs.LockedRound = 0
						cs.LockedBlock = nil
						cs.LockedBlockParts = nil
						types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
					}
				}
				if cs.Round <= vote.Round && prevotes.HasTwoThirdsAny() {
					// Round-skip over to PrevoteWait or goto Precommit.
					cs.enterNewRound(height, vote.Round) // if the vote is ahead of us
					if prevotes.HasTwoThirdsMajority() {
						cs.enterPrecommit(height, vote.Round)
					} else {
						cs.enterPrevote(height, vote.Round) // if the vote is ahead of us
						cs.enterPrevoteWait(height, vote.Round)
					}
				} else if cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round {
					// If the proposal is now complete, enter prevote of cs.Round.
					if cs.isProposalComplete() {
						cs.enterPrevote(height, cs.Round)
					}
				}
			case types.VoteTypePrecommit:
				precommits := cs.Votes.Precommits(vote.Round)
				logger.Info("Added to precommit", " vote:", vote, " precommits:", precommits.StringShort())
				blockID, ok := precommits.TwoThirdsMajority()
				if ok {
					if len(blockID.Hash) == 0 {
						cs.enterNewRound(height, vote.Round+1)
					} else {
						cs.enterNewRound(height, vote.Round)
						cs.enterPrecommit(height, vote.Round)
						cs.enterCommit(height, vote.Round)

						if cs.timeoutParams.SkipTimeoutCommit && precommits.HasAll() {
							// if we have all the votes now,
							// go straight to new round (skip timeout commit)
							// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, RoundStepNewHeight)
							cs.enterNewRound(cs.Height, 0)
						}

					}
				} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
					cs.enterNewRound(height, vote.Round)
					cs.enterPrecommit(height, vote.Round)
					cs.enterPrecommitWait(height, vote.Round)
				}
			default:
				PanicSanity(Fmt("Unexpected vote type %X", vote.Type)) // Should not happen.
			}
*/
		}

		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	} else {
		err = ErrVoteHeightMismatch
	}

	// Height mismatch, bad peer?
	logger.Info("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height, "err", err)
	return
}

func (cs *ConsensusState) signVote(type_ byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIndex,
		Height:           cs.Height,
		Round:            cs.Round,
		Type:             type_,
		BlockID:          types.BlockID{hash, header},
	}
	err := cs.privValidator.SignVote(cs.state.ChainID, vote)
	return vote, err
}

// sign the vote and publish on internalMsgQueue
func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	// if we don't have a key or we're not in the validator set, do nothing
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		return nil
	}
	vote, err := cs.signVote(type_, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		logger.Info("Signed and pushed vote", " height:", cs.Height, " round:", cs.Round, " vote:", vote, " error:", err)
		fmt.Printf("The vote is %s\n", vote.String())
		return vote
	} else {
		//if !cs.replayMode {
		logger.Warn("Error signing vote", " height:", cs.Height, " round:", cs.Round, " vote:", vote, " error:", err)
		//}
		return nil
	}
}

// Build the 2/3+ vote set parts and send them to other validators
func (cs *ConsensusState) sendMaj23Vote(votetype byte) {
	var votes []*types.Vote 

	if votetype == types.VoteTypePrevote {
		votes = cs.Votes.Prevotes(cs.Round).Votes()
	} else if votetype == types.VoteTypePrecommit {
		votes = cs.Votes.Precommits(cs.Round).Votes()
	}

	//logger.Debug(Fmt("votes included in aggregation %+v\n", votes))

	Maj23VoteSet, voteSetParts := types.MakeMaj23VoteSet(votes, 100)

	logger.Debug(Fmt("Generate Maj23VoteSet %#v\n", Maj23VoteSet))
	//logger.Debug(Fmt("Generate Maj23VoteSetParts %+v\n", voteSetParts))

	votesAggr := types.MakeVotesAggr(cs.Height, cs.Round, votetype, voteSetParts.Header(), cs.Validators.Size())

	logger.Debug(Fmt("Generate vote Aggre %#v\n", votesAggr))

	if votetype == types.VoteTypePrevote {
		cs.PrevoteMaj23Parts = voteSetParts
	} else if votetype == types.VoteTypePrecommit {
		cs.PrecommitMaj23Parts = voteSetParts
	}

	mi := msgInfo{&Maj23VotesAggrMessage{votesAggr}, ""}

	logger.Debug(Fmt("sendMaj23Vote: aggr header in Maj23VotesAggrMessage to send is %p\n", (mi.Msg)))
	//logger.Debug(Fmt("sendMaj23Vote: aggr value in Maj23VotesAggrMessage to send is %#v\n", (mi.Msg.(*Maj23VotesAggrMessage).Maj23VotesAggr)))

	//logger.Debug(Fmt("sendMaj23Vote: aggr header in Maj23VotesAggrMessage to send is %+v\n", struct{ Maj23VotesAggrMessage } &{mi.Msg}))

	// send votes aggregate header on internal msg queue
	cs.sendInternalMessage(msgInfo{&Maj23VotesAggrMessage{votesAggr}, ""})

	// send block parts on internal msg queue
	for i := 0; i < voteSetParts.Total(); i++ {
		part := voteSetParts.GetPart(i)
		cs.sendInternalMessage(msgInfo{&VotesAggrPartMessage{cs.Height, cs.Round, votetype, part}, ""})
		//logger.Debug(Fmt("Send %d vote set part(height %d round %d type %d)\n", i+1, cs.Height, cs.Round, votetype))
		//logger.Debug(Fmt("Send vote set part %+v)\n", part))
	}

	//logger.Debug(Fmt("Build and send Maj 2/3+ for (height %d round %d type %d)\n", cs.Height, cs.Round, votetype))

}

//---------------------------------------------------------

func CompareHRS(h1, r1 int, s1 RoundStepType, h2, r2 int, s2 RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}
