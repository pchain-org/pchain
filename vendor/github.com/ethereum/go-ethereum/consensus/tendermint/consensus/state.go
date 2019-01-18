package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"math"
	"reflect"
	"sync"
	"time"

	"context"

	//	"github.com/ethereum/go-ethereum/common"
	consss "github.com/ethereum/go-ethereum/consensus"
	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	sm "github.com/ethereum/go-ethereum/consensus/tendermint/state"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	pabi "github.com/pchain/abi"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	//	"github.com/ethereum/go-ethereum/crypto"
	"crypto/ecdsa"
	"crypto/sha256"
	//"encoding/binary"
	"github.com/ethereum/go-ethereum/crypto"
	tmdcrypto "github.com/tendermint/go-crypto"
	//	"golang.org/x/net/context"
	"math/big"
	//"github.com/pchain/chain"
)

type Backend interface {
	Commit(proposal *types.TdmBlock, seals [][]byte) error
	ChainReader() consss.ChainReader
	GetBroadcaster() consss.Broadcaster
	GetLogger() log.Logger
}

//-----------------------------------------------------------------------------
// Timeout Parameters

// TimeoutParams holds timeouts and deltas for each round step.
// All timeouts and deltas in milliseconds.
type TimeoutParams struct {
	WaitForMinerBlock0 int
	Propose0           int
	ProposeDelta       int
	Prevote0           int
	PrevoteDelta       int
	Precommit0         int
	PrecommitDelta     int
	Commit0            int
	SkipTimeoutCommit  bool
}

// Wait this long for a proposal
func (tp *TimeoutParams) WaitForMinerBlock() time.Duration {
	return time.Duration(tp.WaitForMinerBlock0) * time.Millisecond
}

//In PDBFT, wait for this long for Proposer to send proposal
//the more round, the more time to wait for proposer's proposal
func (tp *TimeoutParams) Propose(round int) time.Duration {
	return time.Duration(tp.Propose0 /*+tp.ProposeDelta*round*/) * time.Millisecond
}

//In PDBFT, wait for this long for Non-Proposer validator to vote prevote
//the more round, the more time to wait for validator's prevote
func (tp *TimeoutParams) Prevote(round int) time.Duration {
	return time.Duration(tp.Prevote0+tp.PrevoteDelta*int(math.Pow(1.5, float64(round)))) * time.Millisecond
}

//In PDBFT, wait for this long for validator to vote precommit
func (tp *TimeoutParams) Precommit(round int) time.Duration {
	return time.Duration(tp.Precommit0 /*+tp.PrecommitDelta*round*/) * time.Millisecond
}

// After receiving +2/3 precommits for a single block (a commit), wait this long for stragglers in the next height's RoundStepNewHeight
func (tp *TimeoutParams) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(tp.Commit0) * time.Millisecond)
}

// InitTimeoutParamsFromConfig initializes parameters from config
func InitTimeoutParamsFromConfig(config cfg.Config) *TimeoutParams {
	return &TimeoutParams{
		WaitForMinerBlock0: config.GetInt("timeout_wait_for_miner_block"),
		Propose0:           config.GetInt("timeout_propose"),
		ProposeDelta:       config.GetInt("timeout_propose_delta"),
		Prevote0:           config.GetInt("timeout_prevote"),
		PrevoteDelta:       config.GetInt("timeout_prevote_delta"),
		Precommit0:         config.GetInt("timeout_precommit"),
		PrecommitDelta:     config.GetInt("timeout_precommit_delta"),
		Commit0:            config.GetInt("timeout_commit"),
		SkipTimeoutCommit:  config.GetBool("skip_timeout_commit"),
	}
}

//-------------------------------------
type VRFProposer struct {
	Height   uint64
	Round    int

	valIndex int
	Proposer *types.Validator
}

func (propser *VRFProposer) Validate(height uint64, round int) bool {
	if propser.Height == height && propser.Round == round {
		return true
	} else {
		return false
	}
}

//-----------------------------------------------------------------------------
// Errors

var (
	ErrMinerBlock               = errors.New("Miner block is nil")
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
	ErrInvalidSignatureAggr     = errors.New("Invalid signature aggregation")
	ErrDuplicateSignatureAggr   = errors.New("Duplicate signature aggregation")
	ErrNotMaj23SignatureAggr    = errors.New("Signature aggregation has no +2/3 power")
)

//-----------------------------------------------------------------------------
// RoundStepType enum type

type RoundStepType uint8 // These must be numeric, ordered.

const (
	RoundStepNewHeight         = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit
	RoundStepNewRound          = RoundStepType(0x02) // Setup new round and go to RoundStepPropose
	RoundStepWaitForMinerBlock = RoundStepType(0x03) // wait proposal block from miner
	RoundStepPropose           = RoundStepType(0x04) // Did propose, gossip proposal
	RoundStepPrevote           = RoundStepType(0x05) // Did prevote, gossip prevotes
	RoundStepPrevoteWait       = RoundStepType(0x06) // Did receive any +2/3 prevotes, start timeout
	RoundStepPrecommit         = RoundStepType(0x07) // Did precommit, gossip precommits
	RoundStepPrecommitWait     = RoundStepType(0x08) // Did receive any +2/3 precommits, start timeout
	RoundStepCommit            = RoundStepType(0x09) // Entered commit state machine
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
	case RoundStepWaitForMinerBlock:
		return "RoundStepWaitForMinerBlock"
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
	default:
		return "RoundStepUnknown" // Cannot panic.
	}
}

//-----------------------------------------------------------------------------

// Immutable when returned from ConsensusState.GetRoundState()
// TODO: Actually, only the top pointer is copied,
// so access to field pointers is still racey
type RoundState struct {
	Height             uint64 // Height we are working on
	Round              int
	Step               RoundStepType
	StartTime          time.Time
	CommitTime         time.Time // Subjective time when +2/3 precommits for Block at Round were found
	Validators         *types.ValidatorSet
	Proposal           *types.Proposal
	ProposalBlock      *types.TdmBlock
	ProposalBlockParts *types.PartSet
	ProposerPeerKey    string // Proposer's peer key
	LockedRound        int
	LockedBlock        *types.TdmBlock
	LockedBlockParts   *types.PartSet
	Votes              *HeightVoteSet
	VoteSignAggr       *HeightVoteSignAggr
	CommitRound        int //

	// Following fields are used for PDBFT/BLS signature aggregation
	PrevoteMaj23SignAggr   *types.SignAggr
	PrecommitMaj23SignAggr *types.SignAggr

	proposer *VRFProposer //proposer for current height||round
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
	Height   uint64        `json:"height"`
	Round    int           `json:"round"`
	Step     RoundStepType `json:"step"`
}

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v", ti.Duration, ti.Height, ti.Round, ti.Step)
}

type PrivValidator interface {
	GetAddress() []byte
	GetPubKey() tmdcrypto.PubKey
	SignVote(chainID string, vote *types.Vote) error
	SignProposal(chainID string, proposal *types.Proposal) error
}

// Tracks consensus state across block heights and rounds.
type ConsensusState struct {
	BaseService

	chainConfig   *params.ChainConfig
	privValidator PrivValidator // for signing votes
	cch           core.CrossChainHelper

	mtx sync.Mutex
	RoundState
	Epoch *ep.Epoch
	state *sm.State // State until height-1.

	peerMsgQueue     chan msgInfo   // serializes msgs affecting state (proposals, block parts, votes)
	internalMsgQueue chan msgInfo   // like peerMsgQueue but for our own proposals, parts, votes
	timeoutTicker    TimeoutTicker  // ticker for timeouts
	timeoutParams    *TimeoutParams // parameters and functions for timeout intervals

	evsw types.EventSwitch

	nSteps int // used for testing to limit the number of transitions the state makes

	// allow certain function to be overwritten for testing
	decideProposal func(height uint64, round int)
	doPrevote      func(height uint64, round int)
	setProposal    func(proposal *types.Proposal) error

	done chan struct{}

	blockFromMiner *ethTypes.Block
	backend        Backend

	conR *ConsensusReactor

	logger log.Logger
}

func NewConsensusState(backend Backend, config cfg.Config, chainConfig *params.ChainConfig, cch core.CrossChainHelper) *ConsensusState {
	cs := &ConsensusState{
		chainConfig:      chainConfig,
		cch:              cch,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(backend.GetLogger()),
		timeoutParams:    InitTimeoutParamsFromConfig(config),
		done:             make(chan struct{}),
		blockFromMiner:   nil,
		backend:          backend,
		logger:           backend.GetLogger(),
	}

	// set function defaults (may be overwritten before calling Start)
	cs.decideProposal = cs.defaultDecideProposal
	cs.doPrevote = cs.defaultDoPrevote
	//cs.setProposal = cs.defaultSetProposal
	cs.setProposal = cs.newSetProposal

	// Don't call scheduleRound0 yet.
	// We do that upon Start().

	cs.BaseService = *NewBaseService(backend.GetLogger(), "ConsensusState", cs)
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
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.getRoundState()
}

func (cs *ConsensusState) getRoundState() *RoundState {
	rs := cs.RoundState // copy
	return &rs
}

func (cs *ConsensusState) GetValidators() (uint64, []*types.Validator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	_, val, _ := cs.state.GetValidators()
	return cs.state.TdmExtra.Height, val.Copy().Validators
}

// Sets our private validator account for signing votes.
func (cs *ConsensusState) SetPrivValidator(priv PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

func BytesToBig(data []byte) *big.Int {
	n := new(big.Int)
	n.SetBytes(data)
	return n
}

//PDBFT VRF proposer selection
func (cs *ConsensusState) updateProposer() {

	//if need to re-initialize proposer, we use VRF
	//if select proposer for different round in one height, we use round-robin
	byVRF := false
	if cs.proposer == nil || cs.proposer.Proposer == nil || cs.Height != cs.proposer.Height {
		cs.proposer = &VRFProposer{}
		byVRF = true
	} else if cs.Round != cs.proposer.Round {
		log.Debug("update proposer for changing round",
			"cs.proposer.Round", cs.proposer.Round, "cs.Round", cs.Round)
	}

	cs.proposer.Height = cs.Height
	cs.proposer.Round = cs.Round

	idx := -1
	if byVRF {
		var roundBytes= make([]byte, 8)
		//binary.BigEndian.PutUint64(roundBytes, uint64(cs.proposer.Round))
		chainReader := cs.backend.ChainReader()
		head := chainReader.CurrentHeader().Hash()
		vrfBytes := append(roundBytes, head[:]...)
		hs := sha256.New()
		hs.Write(vrfBytes)
		hv := hs.Sum(nil)
		hash := new(big.Int)
		hash.SetBytes(hv[:])
		//n := big.NewInt(int64(cs.Validators.Size()))
		n := big.NewInt(0)
		validators := cs.Validators.Validators
		for _, validator := range validators {
			n.Add(n, validator.VotingPower)
		}
		n.Mod(hash, n)

		for i, validator := range validators {
			n.Sub(n, validator.VotingPower)
			if n.Sign() == -1 {
				idx = i
				break
			}
		}
	} else {
		idx = (cs.proposer.valIndex+1) % cs.Validators.Size()
	}

	//idx := int(n.Int64())
	if idx >= cs.Validators.Size() || idx < 0 {
		cs.proposer.Proposer = nil
		PanicConsensus(Fmt("The index of proposer out of range", "index:", idx, "range:", cs.Validators.Size()))
	} else {
		cs.proposer.valIndex = idx
		cs.proposer.Proposer = cs.Validators.Validators[idx]
	}
	log.Debug("update proposer", "height", cs.Height, "round", cs.Round, "idx", idx)
}

// Sets our private validator account for signing votes.
func (cs *ConsensusState) GetProposer() *types.Validator {

	cs.logger.Infof("cs.proposer, cs.Height, cs.Round are (%v, %v, %v)\n", cs.proposer, cs.Height, cs.Round)

	if cs.proposer == nil || cs.proposer.Proposer == nil || cs.Height != cs.proposer.Height || cs.Round != cs.proposer.Round {
		cs.updateProposer()
	}
	return cs.proposer.Proposer
}

// Returns true if this validator is the proposer.
func (cs *ConsensusState) IsProposer() bool {

	proposer := cs.GetProposer()
	privalidator := cs.privValidator
	cs.logger.Debugf("proposer, privalidator are (%v, %v)\n", proposer, privalidator)
	if bytes.Equal(proposer.Address, privalidator.GetAddress()) {
		cs.logger.Debugf("IsProposer() return true\n")
		return true
	} else {
		cs.logger.Debugf("IsProposer() return false\n")
		return false
	}
}

// Set the local timer
func (cs *ConsensusState) SetTimeoutTicker(timeoutTicker TimeoutTicker) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.timeoutTicker = timeoutTicker
}

func (cs *ConsensusState) LoadCommit(height uint64) *types.Commit {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	tdmExtra, height := cs.LoadTendermintExtra(height)
	return tdmExtra.SeenCommit
}

func (cs *ConsensusState) OnStart() error {

	// NOTE: we will get a build up of garbage go routines
	//  firing on the tockChan until the receiveRoutine is started
	//  to deal with them (by that point, at most one will be valid)
	cs.timeoutTicker.Start()

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	cs.StartNewHeight()

	//cs.id = chain.GetNodeID()

	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
/*
func (cs *ConsensusState) startRoutines(maxSteps int) {
	cs.timeoutTicker.Start()
	go cs.receiveRoutine(maxSteps)
}
*/
func (cs *ConsensusState) OnStop() {

	cs.BaseService.OnStop()
	cs.timeoutTicker.Stop()
}

// NOTE: be sure to Stop() the event switch and drain
// any event channels or this may deadlock
func (cs *ConsensusState) Wait() {
	<-cs.done
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
func (cs *ConsensusState) AddProposalBlockPart(height uint64, round int, part *types.Part, peerKey string) error {

	if peerKey == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerKey}
	}

	// TODO: wait for event?!
	return nil
}

// May block on send if queue is full.
func (cs *ConsensusState) SetProposalAndBlock(proposal *types.Proposal, block *types.TdmBlock, parts *types.PartSet, peerKey string) error {
	cs.SetProposal(proposal, peerKey)
	for i := 0; i < parts.Total(); i++ {
		part := parts.GetPart(i)
		cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerKey)
	}
	return nil // TODO errors
}

//------------------------------------------------------------
// internal functions for managing the state

func (cs *ConsensusState) updateRoundStep(round int, step RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *RoundState) {
	//log.Info("scheduleRound0", "now", time.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(time.Now())
	cs.scheduleTimeout(sleepDuration, rs.Height, 0, RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height uint64, round int, step RoundStepType) {
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
		cs.logger.Warn("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) ReconstructLastCommit(state *sm.State) {

	state.TdmExtra, _ = cs.LoadLastTendermintExtra()
	if state.TdmExtra == nil {
		return
	}
}

func (cs *ConsensusState) newStep() {
	rs := cs.RoundStateEvent()
	//cs.wal.Save(rs)
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
				cs.logger.Warn("receiveRoutine. reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		//rs := cs.RoundState
		var mi msgInfo

		select {
		case mi = <-cs.peerMsgQueue:
			//cs.wal.Save(mi)
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			rs := cs.RoundState
			cs.handleMsg(mi, rs)
		case mi = <-cs.internalMsgQueue:
			//cs.wal.Save(mi)
			// handles proposals, block parts, votes
			rs := cs.RoundState
			cs.handleMsg(mi, rs)
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			//cs.wal.Save(ti)
			// if the timeout is relevant to the rs
			// go to the next step
			rs := cs.RoundState
			cs.handleTimeout(ti, rs)
		case <-cs.Quit:

			// NOTE: the internalMsgQueue may have signed messages from our
			// priv_val that haven't hit the WAL, but its ok because
			// priv_val tracks LastSig

			/*
				// close wal now that we're done writing to it
				if cs.wal != nil {
					cs.wal.Stop()
				}
			*/

			close(cs.done)
			return
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo, rs RoundState) {
	//	cs.mtx.Lock()
	//	defer cs.mtx.Unlock()

	var err error
	msg, peerKey := mi.Msg, mi.PeerKey
	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		cs.logger.Debugf("handleMsg: Received proposal message %v", msg.Proposal)
		cs.mtx.Lock()
		err = cs.setProposal(msg.Proposal)
		cs.mtx.Unlock()
	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		cs.logger.Infof("handleMsg. BlockPartMessage: %v", msg)
		cs.mtx.Lock()
		_, err = cs.addProposalBlockPart(msg.Height, msg.Round, msg.Part, peerKey != "")
		if err != nil && msg.Round != cs.Round {
			err = nil
		}
		cs.mtx.Unlock()
	case *Maj23SignAggrMessage:
		// Msg saying a set of 2/3+ signatures had been received
		cs.mtx.Lock()
		err = cs.handleSignAggr(msg.Maj23SignAggr)
		cs.mtx.Unlock()
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		cs.logger.Infof("handleMsg. VoteMessage: %v", msg)
		cs.mtx.Lock()
		err := cs.tryAddVote(msg.Vote, peerKey)
		cs.mtx.Unlock()
		if err == ErrAddingVote {
			// TODO: punish peer
		}

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.
		// We could make note of this and help filter in broadcastHasVoteMessage().
	default:
		cs.logger.Warnf("handleMsg. Unknown msg type %v", reflect.TypeOf(msg))
	}

	if err != nil {
		cs.logger.Errorf("handleMsg. msg: %v, error: %v", msg, err)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs RoundState) {
	cs.logger.Infof("Received tock. timeout: %v, (%v/%v/%v), Current: (%v/%v/%v)", ti.Duration, ti.Height, ti.Round, ti.Step, rs.Height, rs.Round, rs.Step)

	// timeouts must be for current height, round, step
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) {
		cs.logger.Warn("Ignoring tock because we're ahead")
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	cs.logger.Debugf("step is :%+v", ti.Step)
	switch ti.Step {
	case RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		// XXX: should we fire timeout here (for timeout commit)?
		cs.enterNewRound(ti.Height, 0)
	case RoundStepWaitForMinerBlock:
		types.FireEventTimeoutPropose(cs.evsw, cs.RoundStateEvent())
		if cs.blockFromMiner != nil {
			cs.logger.Warn("another round of RoundStepWaitForMinerBlock, something wrong!!!")
		}
		//only wait miner block for time of tp.WaitForMinerBlock, or we try to get another round
		cs.enterPropose(ti.Height, ti.Round)
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
func (cs *ConsensusState) enterNewRound(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != RoundStepNewHeight) {
		cs.logger.Warnf("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}

	if now := time.Now(); cs.StartTime.After(now) {
		cs.logger.Warn("Need to set a buffer and log.Warn() here for sanity.", "startTime", cs.StartTime, "now", now)
	}

	cs.logger.Infof("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
	cs.logger.Infof("Validators: %v", cs.Validators)

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, RoundStepNewRound)
	if round == 0 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
		cs.PrevoteMaj23SignAggr = nil
		cs.PrecommitMaj23SignAggr = nil
	}
	cs.VoteSignAggr.SetRound(round + 1) // also track next round (round+1) to allow round-skipping
	cs.Votes.SetRound(round + 1)
	types.FireEventNewRound(cs.evsw, cs.RoundStateEvent())

	// Immediately go to enterPropose.
	if cs.IsProposer() && cs.blockFromMiner == nil {
		cs.logger.Info("we are proposer, but blockFromMiner is nil, let's wait a second!!!")
		cs.scheduleTimeout(cs.timeoutParams.WaitForMinerBlock(), height, round, RoundStepWaitForMinerBlock)
		return
	}

	cs.enterPropose(height, round)
}

// Enter: from NewRound(height,round).
func (cs *ConsensusState) enterPropose(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPropose <= cs.Step) {
		cs.logger.Warnf("enterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}
	cs.logger.Infof("enterPropose(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)

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

	// Save block to main chain (this happens only on validator node).
	// Note!!! This will BLOCK the WHOLE consensus stack since it blocks receiveRoutine.
	// TODO: what if there're more than one round for a height? 'saveBlockToMainChain' would be called more than once
	if cs.state.TdmExtra.NeedToSave && cs.state.TdmExtra.ChainID != "pchain" {
		if cs.privValidator != nil && cs.IsProposer() {
			cs.logger.Infof("enterPropose: saveBlockToMainChain height: %v", cs.state.TdmExtra.Height)
			lastBlock := cs.GetChainReader().GetBlockByNumber(cs.state.TdmExtra.Height)
			cs.saveBlockToMainChain(lastBlock)
			cs.state.TdmExtra.NeedToSave = false
		}
	}

	if cs.state.TdmExtra.NeedToBroadcast && cs.state.TdmExtra.ChainID != "pchain" {
		if cs.privValidator != nil && cs.IsProposer() {
			cs.logger.Infof("enterPropose: broadcastTX3ProofDataToMainChain height: %v", cs.state.TdmExtra.Height)
			lastBlock := cs.GetChainReader().GetBlockByNumber(cs.state.TdmExtra.Height)
			cs.broadcastTX3ProofDataToMainChain(lastBlock)
			cs.state.TdmExtra.NeedToBroadcast = false
		}
	}

	// If we don't get the proposal and all block parts quick enough, enterPrevote
	cs.scheduleTimeout(cs.timeoutParams.Propose(round), height, round, RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		cs.logger.Info("we are not validator yet!!!!!!!!saaaaaaad")
		return
	}

	if !cs.IsProposer() {
		cs.logger.Info("enterPropose: Not our turn to propose", "proposer", cs.GetProposer(), "privValidator", cs.privValidator)
	} else {
		cs.logger.Info("enterPropose: Our turn to propose", "proposer", cs.GetProposer(), "privValidator", cs.privValidator)
		cs.decideProposal(height, round)
	}
}

func (cs *ConsensusState) defaultDecideProposal(height uint64, round int) {
	var block *types.TdmBlock
	var blockParts *types.PartSet
	var proposerPeerKey string

	// Decide on block
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block, blockParts = cs.LockedBlock, cs.LockedBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
		if block == nil { // on error
			return
		}
	}

	// fmt.Println("defaultDecideProposal: cs nodeInfo %#v\n", cs.nodeInfo)
	cs.logger.Debugf("defaultDecideProposal: Proposer (peer key %s)", proposerPeerKey)

	// Make proposal
	polRound, polBlockID := cs.VoteSignAggr.POLInfo()
	cs.logger.Debugf("proposal hash: %X", block.Hash())
	if NodeID == "" {
		panic("Node id is nil")
	}
	proposerPeerKey = NodeID
	proposal := types.NewProposal(height, round, block.Hash(), blockParts.Header(), polRound, polBlockID, proposerPeerKey)
	err := cs.privValidator.SignProposal(cs.state.TdmExtra.ChainID, proposal)
	if err == nil {

		cs.logger.Infof("Signed proposal block, height: %v", block.TdmExtra.Height)
		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		for i := 0; i < blockParts.Total(); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
	} else {
		log.Warn("enterPropose: Error signing proposal", "height", height, "round", round, "error", err)
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
		sa := cs.VoteSignAggr.Prevotes(cs.Proposal.POLRound)
		if sa != nil {
			return sa.HasTwoThirdsMajority(cs.Validators)
		} else {
			return false
		}
	}
}

// Create the next block to propose and return it.
// Returns nil block upon error.
// NOTE: keep it side-effect free for clarity.
func (cs *ConsensusState) createProposalBlock() (*types.TdmBlock, *types.PartSet) {

	//here we wait for ethereum block to propose
	if cs.blockFromMiner != nil {

		ethBlock := cs.blockFromMiner
		var commit = &types.Commit{}
		var epochBytes []byte

		// New Height equal to the first block in new Epoch
		if cs.Height == cs.Epoch.StartBlock {
			epochBytes = cs.Epoch.Bytes()
		} else if cs.Height == 1 {
			// We're creating a proposal for the first block.
			// always setup the epoch so that it'll be sent to the main chain.
			epochBytes = cs.state.Epoch.Bytes()
		} else {
			shouldProposeEpoch := cs.Epoch.ShouldProposeNextEpoch(cs.Height)
			if shouldProposeEpoch {
				lastHeight := cs.backend.ChainReader().CurrentBlock().Number().Uint64()
				lastBlockTime := time.Unix(cs.backend.ChainReader().CurrentBlock().Time().Int64(), 0)
				epochBytes = cs.Epoch.ProposeNextEpoch(lastHeight, lastBlockTime).Bytes()
			}
		}

		_, val, _ := cs.state.GetValidators()

		//This block could be used for later round
		//cs.blockFromMiner = nil

		// retrieve TX3ProofData for TX4
		var tx3ProofData []*ethTypes.TX3ProofData
		txs := ethBlock.Transactions()
		for _, tx := range txs {
			if pabi.IsPChainContractAddr(tx.To()) {
				data := tx.Data()
				function, err := pabi.FunctionTypeFromId(data[:4])
				if err != nil {
					continue
				}

				if function == pabi.WithdrawFromMainChain {
					var args pabi.WithdrawFromMainChainArgs
					data := tx.Data()
					if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromMainChain.String(), data[4:]); err != nil {
						continue
					}

					proof := cs.cch.GetTX3ProofData(args.ChainId, args.TxHash)
					if proof != nil {
						tx3ProofData = append(tx3ProofData, proof)
					}
				}
			}
		}

		return types.MakeBlock(cs.Height, cs.state.TdmExtra.ChainID, commit, ethBlock,
			val.Hash(), cs.Epoch.Number, epochBytes,
			tx3ProofData, 65536)
	} else {
		cs.logger.Warn("block from miner should not be nil, let's start another round")
		return nil, nil
	}
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevoteWait < cs.Step) {
		cs.logger.Warnf("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}

	defer func() {
		// Done enterPrevote:
		if cs.Step == RoundStepPropose {
			cs.updateRoundStep(round, RoundStepPrevote)
			cs.newStep()
		}

		//trigger the timer in bls-vote mode to make the steps go ahead
		cs.enterPrevoteWait(height, round)
	}()

	cs.logger.Infof("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)

	// Sign and broadcast vote as necessary
	if cs.isProposalComplete() {
		cs.doPrevote(height, round)
	}

}

func (cs *ConsensusState) defaultDoPrevote(height uint64, round int) {
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		cs.logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		cs.logger.Warn("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	err := cs.ProposalBlock.ValidateBasic(cs.state.TdmExtra)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		cs.logger.Warnf("enterPrevote: ProposalBlock is invalid, error: %v", err)
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Validate TX4
	err = cs.ValidateTX4(cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		cs.logger.Warnf("enterPrevote: ProposalBlock is invalid, error: %v", err)
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Valdiate proposal block
	proposedNextEpoch := ep.FromBytes(cs.ProposalBlock.TdmExtra.EpochBytes)
	if proposedNextEpoch != nil && proposedNextEpoch.Number == cs.Epoch.Number+1 {
		lastHeight := cs.backend.ChainReader().CurrentBlock().Number().Uint64()
		lastBlockTime := time.Unix(cs.backend.ChainReader().CurrentBlock().Time().Int64(), 0)
		err = cs.Epoch.ValidateNextEpoch(proposedNextEpoch, lastHeight, lastBlockTime)
		if err != nil {
			// ProposalBlock is invalid, prevote nil.
			cs.logger.Warnf("enterPrevote: Proposal Next Epoch is invalid, error: %v", err)
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

// In PDBFT, wait for 2/3 votes for prevote
func (cs *ConsensusState) enterPrevoteWait(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrevoteWait <= cs.Step) {
		cs.logger.Warnf("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}

	cs.logger.Infof("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.timeoutParams.Prevote(round), height, round, RoundStepPrevoteWait)
}

// In PBDFT, when prevote round ends, enter to vote for precommit
func (cs *ConsensusState) enterPrecommit(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommit <= cs.Step) {
		cs.logger.Warnf("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}

	cs.logger.Infof("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, RoundStepPrecommit)
		cs.newStep()

		//trigger the timer in bls-vote mode to make the steps go ahead
		cs.enterPrecommitWait(height, round)
	}()

	blockID, ok := cs.VoteSignAggr.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil
	if !ok {
		if cs.LockedBlock != nil {
			cs.logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			cs.logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil
	types.FireEventPolka(cs.evsw, cs.RoundStateEvent())

	// the latest POLRound should be this round
	polRound, _ := cs.VoteSignAggr.POLInfo()
	if polRound < round {
		PanicSanity(Fmt("This POLRound should be %v but got %", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			cs.logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			cs.logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		cs.logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		types.FireEventRelock(cs.evsw, cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID.Hash, blockID.PartsHeader)
		return
	}

	cs.logger.Debugf("cs proposal hash:%+v", cs.ProposalBlock.Hash())
	cs.logger.Debugf("block id:%+v", blockID.Hash)

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		cs.logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID.Hash)
		// Validate the block.
		if err := cs.ProposalBlock.ValidateBasic(cs.state.TdmExtra); err != nil {
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
	cs.LockedRound = -1
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

// In PDBFT, wait for 2/3 votes for precommit
func (cs *ConsensusState) enterPrecommitWait(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && RoundStepPrecommitWait <= cs.Step) {
		cs.logger.Warnf("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)
		return
	}
	/*
		// Temp use here, need to change it to use cs.VoteSignAggr finally
		if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
			PanicSanity(Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
		}
	*/
	cs.logger.Infof("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step)

	defer func() {
		// Done enterPrecommitWait:
		cs.updateRoundStep(round, RoundStepPrecommitWait)
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.timeoutParams.Precommit(round), height, round, RoundStepPrecommitWait)

}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height uint64, commitRound int) {
	if cs.Height != height || RoundStepCommit <= cs.Step {
		cs.logger.Warnf("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step)
		return
	}
	cs.logger.Infof("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step)

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

	blockID, ok := cs.VoteSignAggr.Precommits(commitRound).TwoThirdsMajority()
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
func (cs *ConsensusState) tryFinalizeCommit(height uint64) {

	if cs.Height != height {
		PanicSanity(Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	//blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	blockID, ok := cs.VoteSignAggr.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || len(blockID.Hash) == 0 {
		cs.logger.Warn("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.", "height", height)
		return
	}
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		// TODO: this happens every time if we're not a validator (ugly logs)
		// TODO: ^^ wait, why does it matter that we're a validator?
		cs.logger.Warn("Attempt to finalize failed. We don't have the commit block.", "height", height, "proposal-block", cs.ProposalBlock.Hash(), "commit-block", blockID.Hash)
		return
	}
	//	go
	cs.logger.Infof("do finalizeCommit at height: %v", height)
	cs.finalizeCommit(height)
}

// Increment height and goto RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height uint64) {
	if cs.Height != height || cs.Step != RoundStepCommit {
		cs.logger.Warnf("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step)
		return
	}

	// fmt.Println("precommits:", cs.VoteSignAggr.Precommits(cs.CommitRound))
	blockID, ok := cs.VoteSignAggr.Precommits(cs.CommitRound).TwoThirdsMajority()
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
	if err := block.ValidateBasic(cs.state.TdmExtra); err != nil {
		PanicConsensus(Fmt("+2/3 committed an invalid block: %v", err))
	}

	// Save to blockStore.
	//if cs.blockStore.Height() < block.TdmExtra.Height {
	if cs.state.TdmExtra.Height < block.TdmExtra.Height {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.VoteSignAggr.Precommits(cs.CommitRound)
		seenCommit := precommits.MakeCommit()

		block.TdmExtra.SeenCommit = seenCommit
		block.TdmExtra.SeenCommitHash = seenCommit.Hash()

		// update 'NeedToSave' field here
		if block.TdmExtra.ChainID != "pchain" {
			// check epoch
			if len(block.TdmExtra.EpochBytes) > 0 {
				block.TdmExtra.NeedToSave = true
				cs.logger.Infof("NeedToSave set to true due to epoch. Chain: %s, Height: %v", block.TdmExtra.ChainID, block.TdmExtra.Height)
			}
			// check special cross-chain tx
			txs := block.Block.Transactions()
			for _, tx := range txs {
				if pabi.IsPChainContractAddr(tx.To()) {
					data := tx.Data()
					function, err := pabi.FunctionTypeFromId(data[:4])
					if err != nil {
						continue
					}

					if function == pabi.WithdrawFromChildChain {
						block.TdmExtra.NeedToBroadcast = true
						cs.logger.Infof("NeedToBroadcast set to true due to tx. Tx: %s, Chain: %s, Height: %v", function.String(), block.TdmExtra.ChainID, block.TdmExtra.Height)
						break
					}
				}
			}
		}

		// Fire event for new block.
		types.FireEventNewBlock(cs.evsw, types.EventDataNewBlock{block})
		types.FireEventNewBlockHeader(cs.evsw, types.EventDataNewBlockHeader{int(block.TdmExtra.Height)})

		//the second parameter as signature has been set above
		err := cs.backend.Commit(block, [][]byte{})
		if err != nil {
			cs.logger.Errorf("Commit fail. error: %v", err)
		}
	} else {
		cs.logger.Warn("Calling finalizeCommit on already stored block", "height", block.TdmExtra.Height)
	}

	return
}

//-----------------------------------------------------------------------------
func (cs *ConsensusState) newSetProposal(proposal *types.Proposal) error {
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
	if !cs.GetProposer().PubKey.VerifyBytes(types.SignBytes(cs.chainConfig.PChainId, proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.logger.Debugf("proposal is: %X", proposal.Hash)
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	cs.ProposerPeerKey = proposal.ProposerPeerKey
	return nil
}

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
	if !cs.GetProposer().PubKey.VerifyBytes(types.SignBytes(cs.state.TdmExtra.ChainID, proposal), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit, once we have the full block.
func (cs *ConsensusState) addProposalBlockPart(height uint64, round int, part *types.Part, verify bool) (added bool, err error) {

	if cs.Height != height || cs.Round != round {
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
		tdmBlock := &types.TdmBlock{}
		cs.ProposalBlock, err = tdmBlock.FromBytes(cs.ProposalBlockParts.GetReader())

		cs.logger.Info("Received complete proposal block", "block", cs.ProposalBlock.String(), "err", err)

		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		//log.Info("Received complete proposal block", "height", cs.ProposalBlock.Height, "hash", cs.ProposalBlock.Hash())
		if RoundStepPropose <= cs.Step && cs.Step <= RoundStepPrevoteWait && cs.isProposalComplete() {
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
func (cs *ConsensusState) setMaj23SignAggr(signAggr *types.SignAggr) (error, bool) {
	cs.logger.Debug("enter setMaj23SignAggr()")
	cs.logger.Debugf("Received SignAggr %#v", signAggr)

	// Does not apply
	if signAggr.Height != cs.Height || signAggr.Round != cs.Round {
		cs.logger.Debug("does not apply")
		return nil, false
	}

	if signAggr.SignAggr() == nil {
		cs.logger.Debug("SignAggr() is nil ")
	}
	maj23, err := cs.blsVerifySignAggr(signAggr)

	if err != nil || maj23 == false {
		cs.logger.Warnf("verifyMaj23SignAggr: Invalid signature aggregation, error:%+v, maj23:%+v", err, maj23)
		cs.logger.Warnf("SignAggr:%+v", signAggr)
		return ErrInvalidSignatureAggr, false
	}

	if signAggr.Type == types.VoteTypePrevote {
		// How if the signagure aggregation is for another block
		if cs.PrevoteMaj23SignAggr != nil {
			return ErrDuplicateSignatureAggr, false
		}

		cs.VoteSignAggr.AddSignAggr(signAggr)
		cs.PrevoteMaj23SignAggr = signAggr

		if (cs.LockedBlock != nil) && (cs.LockedRound < signAggr.Round) {
			blockID := cs.PrevoteMaj23SignAggr.Maj23
			if !cs.LockedBlock.HashesTo(blockID.Hash) {
				cs.logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", signAggr.Round)
				cs.LockedRound = -1
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil
				types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
			}
		}

		cs.logger.Debugf("setMaj23SignAggr:prevote aggr %#v", cs.PrevoteMaj23SignAggr)
	} else if signAggr.Type == types.VoteTypePrecommit {
		if cs.PrecommitMaj23SignAggr != nil {
			return ErrDuplicateSignatureAggr, false
		}

		cs.logger.Debugf("signAggr:%+v", signAggr)
		_, err := cs.VoteSignAggr.AddSignAggr(signAggr)
		if err != nil {
			panic(err)
		}
		cs.PrecommitMaj23SignAggr = signAggr
		cs.logger.Debugf("setMaj23SignAggr:precommit aggr %#v", cs.PrecommitMaj23SignAggr)
	} else {
		cs.logger.Warn(Fmt("setMaj23SignAggr: invalid type %d for signAggr %#v\n", signAggr.Type, signAggr))
		return ErrInvalidSignatureAggr, false
	}

	if signAggr.Type == types.VoteTypePrevote {
		cs.logger.Infof("setMaj23SignAggr: Received 2/3+ prevotes for block %d, enter precommit", cs.Height)
		if cs.isProposalComplete() {
			cs.logger.Debugf("receive block:%+v", cs.ProposalBlock)
			cs.enterPrecommit(cs.Height, cs.Round)
			return nil, true

		} else {
			cs.logger.Debug("block is not completed")
			return nil, false
		}
	} else if signAggr.Type == types.VoteTypePrecommit {
		cs.logger.Info(Fmt("setMaj23SignAggr: Received 2/3+ precommits for block %d, enter commit\n", cs.Height))

		// TODO : Shall go to this state?
		// cs.tryFinalizeCommit(height)
		if cs.isProposalComplete() {
			cs.logger.Debug("block is completed")

			cs.enterCommit(cs.Height, cs.Round)
			return nil, true
		} else {
			cs.logger.Debug("block is not completed")
			return nil, false
		}

	} else {
		panic("Invalid signAggr type")
		return nil, false
	}
	return nil, false
}

func (cs *ConsensusState) handleSignAggr(signAggr *types.SignAggr) error {
	if signAggr == nil {
		return fmt.Errorf("SignAggr is nil")
	}
	if signAggr.Height == cs.Height && signAggr.Round == cs.Round {
		err, _ := cs.setMaj23SignAggr(signAggr)
		return err
	}
	return nil
}

func (cs *ConsensusState) BLSVerifySignAggr(signAggr *types.SignAggr) (bool, error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.blsVerifySignAggr(signAggr)
}

func (cs *ConsensusState) blsVerifySignAggr(signAggr *types.SignAggr) (bool, error) {
	cs.logger.Debug("enter BLSVerifySignAggr()")
	cs.logger.Infof("sign aggr bitmap:%+v", signAggr.BitArray)
	if signAggr == nil {
		cs.logger.Info("Invalid Sign(nil)")
		return false, fmt.Errorf("Invalid SignAggr(nil)")
	}

	if signAggr.SignAggr() == nil {
		cs.logger.Info("Invalid BLSSignature(nil)")
		return false, fmt.Errorf("Invalid BLSSignature(nil)")
	}

	bitMap := signAggr.BitArray
	validators := cs.Validators

	/*
		quorum := big.NewInt(0)
		quorum.Mul(cs.Validators.TotalVotingPower(), big.NewInt(2))
		quorum.Div(quorum, big.NewInt(3))
		quorum.Add(quorum, big.NewInt(1))
		if validators.Size() != (int)(bitMap.Size()) {
			cs.logger.Info("validators are not matched")
			return false, fmt.Errorf(Fmt("validators are not matched, consensus validators:%v, signAggr validators:%v"), validators.Validators, signAggr.BitArray)
		}
	*/
	powerSum, err := validators.TalliedVotingPower(bitMap)
	if err != nil {
		cs.logger.Info("tallied voting power")
		return false, err
	}

	quorum := types.Loose23MajorThreshold(validators.TotalVotingPower(), signAggr.Round)

	var maj23 bool
	if powerSum.Cmp(quorum) >= 0 {
		maj23 = true
	} else {
		maj23 = false
	}

	aggrPubKey := validators.AggrPubKey(bitMap)
	if aggrPubKey == nil {
		cs.logger.Info("can not aggregate pubkeys")
		return false, fmt.Errorf("can not aggregate pubkeys")
	}

	vote := &types.Vote{
		BlockID: signAggr.BlockID,
		Height:  signAggr.Height,
		Round:   (uint64)(signAggr.Round),
		Type:    signAggr.Type,
	}

	if !aggrPubKey.VerifyBytes(types.SignBytes(signAggr.ChainID, vote), (signAggr.SignAggr())) {
		cs.logger.Info("Invalid aggregate signature")
		return false, errors.New("Invalid aggregate signature")
	}

	return maj23, nil
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
				cs.logger.Warn("Found conflicting vote from ourselves. Did you unsafe_reset a validator?", "height", vote.Height, "round", vote.Round, "type", vote.Type)
				return err
			}
			return err
		} else {
			// Probably an invalid signature. Bad peer.
			cs.logger.Warn("Error attempting to add vote", "error", err)
			return ErrAddingVote
		}
	}
	return nil
}

//-----------------------------------------------------------------------------
//only proposer would invoke this function
func (cs *ConsensusState) addVote(vote *types.Vote, peerKey string) (added bool, err error) {
	cs.logger.Info("addVote", "voteHeight", vote.Height, "voteType", vote.Type, "csHeight", cs.Height)

	if !cs.IsProposer() {
		cs.logger.Warn("addVode should only happen if this node is proposer")
		return
	}

	// A precommit for the previous height or previous round, just ignore
	if vote.Height != cs.Height || int(vote.Round) != cs.Round {
		cs.logger.Warn("addVote, vote is for previous blocks or previous round, just ignore\n")
		return
	}

	if vote.Type == types.VoteTypePrevote {
		if cs.Votes.Prevotes(cs.Round).HasTwoThirdsMajority() {
			return
		}
	} else {
		if cs.Votes.Precommits(cs.Round).HasTwoThirdsMajority() {
			return
		}
	}

	added, err = cs.Votes.AddVote(vote, peerKey)
	if added {
		if vote.Type == types.VoteTypePrevote {
			// If 2/3+ votes received, send them to other validators
			if cs.Votes.Prevotes(cs.Round).HasTwoThirdsMajority() {
				cs.logger.Debug(Fmt("addVote: Got 2/3+ prevotes %+v\n", cs.Votes.Prevotes(cs.Round)))
				// Send signature aggregation
				cs.sendMaj23SignAggr(vote.Type)
			}
		} else if vote.Type == types.VoteTypePrecommit {
			if cs.Votes.Precommits(cs.Round).HasTwoThirdsMajority() {
				cs.logger.Debugf("addVote: Got 2/3+ precommits %+v", cs.Votes.Precommits(cs.Round))
				// Send signature aggregation
				cs.sendMaj23SignAggr(vote.Type)
			}
		}
	}

	return
	/*
		types.FireEventVote(cs.evsw, types.EventDataVote{vote})

		height := cs.Height
		switch vote.Type {
		case types.VoteTypePrevote:
			//prevotes := cs.Votes.Prevotes(int(vote.Round))
			prevotes := cs.VoteSignAggr.Prevotes(int(vote.Round))
			cs.logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())
			// First, unlock if prevotes is a valid POL.
			// >> lockRound < POLRound <= unlockOrChangeLockRound (see spec)
			// NOTE: If (lockRound < POLRound) but !(POLRound <= unlockOrChangeLockRound),
			// we'll still enterNewRound(H,vote.R) and enterPrecommit(H,vote.R) to process it
			// there.
			if (cs.LockedBlock != nil) && (cs.LockedRound < int(vote.Round)) && (int(vote.Round) <= cs.Round) {
				blockID, ok := prevotes.TwoThirdsMajority()
				cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 0")
				if ok && !cs.LockedBlock.HashesTo(blockID.Hash) {
					cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 1")
					cs.logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
					cs.LockedRound = -1
					cs.LockedBlock = nil
					cs.LockedBlockParts = nil
					types.FireEventUnlock(cs.evsw, cs.RoundStateEvent())
				}
			}

			if cs.Round <= int(vote.Round) && prevotes.HasTwoThirdsAny(cs.Validators) {
				// Round-skip over to PrevoteWait or goto Precommit.
				cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 2")
				cs.enterNewRound(height, int(vote.Round)) // if the vote is ahead of us
				if prevotes.HasTwoThirdsMajority(cs.Validators) {
					cs.enterPrecommit(height, int(vote.Round))
				} else {
					cs.enterPrevote(height, int(vote.Round)) // if the vote is ahead of us
					cs.enterPrevoteWait(height, int(vote.Round))
				}
			} else if cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == int(vote.Round) {
				// If the proposal is now complete, enter prevote of cs.Round.
				cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 3")
				if cs.isProposalComplete() {
					cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 4")
					cs.enterPrevote(height, cs.Round)
				}
			} else {
				//in other conditions, trigger prevote wait to make this round go ahead
				cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 5")
				cs.enterPrevoteWait(height, int(vote.Round))
			}
		case types.VoteTypePrecommit:
			//precommits := cs.Votes.Precommits(int(vote.Round))
			precommits := cs.VoteSignAggr.Precommits(int(vote.Round))
			cs.logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())

			blockID, ok := precommits.TwoThirdsMajority()
			if ok {
				cs.logger.Info("(cs *ConsensusState) VoteTypePrecommit 0")
				if len(blockID.Hash) == 0 {
					cs.logger.Info("(cs *ConsensusState) VoteTypePrecommit 1")
					cs.enterNewRound(height, int(vote.Round+1))
				} else {
					cs.logger.Info("(cs *ConsensusState) VoteTypePrecommit 2")
					cs.enterNewRound(height, int(vote.Round))
					cs.enterPrecommit(height, int(vote.Round))
					cs.enterCommit(height, int(vote.Round))

					if cs.timeoutParams.SkipTimeoutCommit && precommits.HasAll(cs.Validators) {
						cs.logger.Info("(cs *ConsensusState) VoteTypePrecommit 3")
						// if we have all the votes now,
						// go straight to new round (skip timeout commit)
						// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, RoundStepNewHeight)
						cs.enterNewRound(cs.Height, 0)
					}
				}
			} else if cs.Round <= int(vote.Round) && precommits.HasTwoThirdsAny(cs.Validators) {
				cs.logger.Info("(cs *ConsensusState) VoteTypePrecommit 4")
				cs.enterNewRound(height, int(vote.Round))
				cs.enterPrecommit(height, int(vote.Round))
				cs.enterPrecommitWait(height, int(vote.Round))
			} else {
				//in other conditions, trigger precommit wait to make this round go ahead
				cs.logger.Info("(cs *ConsensusState) VoteTypePrevote 5")
				cs.enterPrecommitWait(height, int(vote.Round))
			}
		default:
			PanicSanity(Fmt("Unexpected vote type %X", vote.Type)) // Should not happen.
		}
	*/
}

func (cs *ConsensusState) signVote(type_ byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   uint64(valIndex),
		Height:           uint64(cs.Height),
		Round:            uint64(cs.Round),
		Type:             type_,
		BlockID:          types.BlockID{hash, header},
	}
	err := cs.privValidator.SignVote(cs.state.TdmExtra.ChainID, vote)
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
		if !cs.IsProposer() {
			if cs.ProposerPeerKey != "" {
				v2pMsg := types.EventDataVote2Proposer{vote, cs.ProposerPeerKey}
				types.FireEventVote2Proposer(cs.evsw, v2pMsg)
			} else {
				cs.logger.Warn("sign and vote, Proposer key is nil")
			}
		} else {
			cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		}
		cs.logger.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		cs.logger.Debugf("block is:%+v", vote.BlockID)
		return vote
	} else {
		cs.logger.Warn("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "error", err)
		return nil
	}
}

// Build the 2/3+ signature aggregation based on vote set and send it to other validators
func (cs *ConsensusState) sendMaj23SignAggr(voteType byte) {
	cs.logger.Info("Enter sendMaj23SignAggr()")

	var votes []*types.Vote
	var blockID, maj23 types.BlockID
	var ok bool

	if voteType == types.VoteTypePrevote {
		votes = cs.Votes.Prevotes(cs.Round).Votes()
		maj23, ok = cs.Votes.Prevotes(cs.Round).TwoThirdsMajority()
	} else if voteType == types.VoteTypePrecommit {
		votes = cs.Votes.Precommits(cs.Round).Votes()
		maj23, ok = cs.Votes.Precommits(cs.Round).TwoThirdsMajority()
	}

	if ok == false {
		cs.logger.Error("Votset does not have +2/3 voting")
	}

	cs.logger.Debugf("vote len is: %v", len(votes))
	numValidators := cs.Validators.Size()
	signBitArray := NewBitArray((uint64)(numValidators))
	var sigs []*tmdcrypto.Signature
	var ss []byte
	for index, vote := range votes {
		if vote != nil {
			blockID = vote.BlockID
			ss = vote.SignBytes
			signBitArray.SetIndex((uint64)(index), true)
			sigs = append(sigs, &(vote.Signature))
		}
	}
	cs.logger.Debugf("send maj block ID: %X", blockID.Hash)

	// step 1: build BLS signature aggregation based on signatures in votes
	// bitarray, signAggr := BuildSignAggr(votes)
	signature := tmdcrypto.BLSSignatureAggregate(sigs)
	if signature == nil {
		cs.logger.Error("Can not aggregate signature")
		return
	}

	signAggr := types.MakeSignAggr(cs.Height, cs.Round, voteType, numValidators, blockID, cs.Votes.chainID, signBitArray, signature)
	signAggr.SignBytes = ss

	// Set sign bitmap
	//signAggr.SetBitArray(signBitArray)

	if maj23.IsZero() == true {
		cs.logger.Debugf("The maj23 blockID is zero %+v", maj23)
		return
	}

	// Set ma23 block ID
	signAggr.SetMaj23(maj23)
	cs.logger.Debugf("Generate Maj23SignAggr %#v", signAggr)

	signEvent := types.EventDataSignAggr{SignAggr: signAggr}
	types.FireEventSignAggr(cs.evsw, signEvent)

	// send sign aggregate msg on internal msg queue
	cs.sendInternalMessage(msgInfo{&Maj23SignAggrMessage{signAggr}, ""})
}

//---------------------------------------------------------

func CompareHRS(h1 uint64, r1 int, s1 RoundStepType, h2 uint64, r2 int, s2 RoundStepType) int {
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

func (cs *ConsensusState) ValidateTX4(b *types.TdmBlock) error {
	var index int

	txs := b.Block.Transactions()
	for _, tx := range txs {
		if pabi.IsPChainContractAddr(tx.To()) {
			data := tx.Data()
			function, err := pabi.FunctionTypeFromId(data[:4])
			if err != nil {
				continue
			}

			if function == pabi.WithdrawFromMainChain {
				// index of tx4 and tx3ProofData should exactly match one by one.
				if index >= len(b.TX3ProofData) {
					return errors.New("tx3 proof data missing")
				}
				tx3ProofData := b.TX3ProofData[index]
				index++

				if err := cs.cch.ValidateTX3ProofData(tx3ProofData); err != nil {
					return err
				}

				if err := cs.cch.ValidateTX4WithInMemTX3ProofData(tx, tx3ProofData); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (cs *ConsensusState) saveBlockToMainChain(block *ethTypes.Block) {

	client := cs.cch.GetClient()
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	//ctx := context.Background() // testing only!

	proofData, err := ethTypes.NewChildChainProofData(block)
	if err != nil {
		cs.logger.Error("saveDataToMainChain: failed to create proof data", "block", block, "err", err)
		return
	}

	bs, err := rlp.EncodeToBytes(proofData)
	if err != nil {
		cs.logger.Error("saveDataToMainChain: failed to encode proof data", "proof data", proofData, "err", err)
		return
	}
	cs.logger.Infof("saveDataToMainChain proof data length: %d", len(bs))

	number, err := client.BlockNumber(ctx)
	if err != nil {
		cs.logger.Error("saveDataToMainChain: failed to get BlockNumber at the beginning.", "err", err)
		return
	}

	// We use BLS Consensus PrivateKey to sign the digest data
	var prv *ecdsa.PrivateKey
	if prvValidator, ok := cs.privValidator.(*types.PrivValidator); ok {
		prv, err = crypto.ToECDSA(prvValidator.PrivKey.(tmdcrypto.BLSPrivKey).Bytes())
		if err != nil {
			cs.logger.Error("saveDataToMainChain: failed to get PrivateKey", "err", err)
			return
		}
	} else {
		panic("saveDataToMainChain: unexpected privValidator type")
	}
	hash, err := client.SendDataToMainChain(ctx, bs, prv)
	if err != nil {
		cs.logger.Error("saveDataToMainChain(rpc) failed", "err", err)
		return
	} else {
		cs.logger.Infof("saveDataToMainChain(rpc) success, hash: %x", hash)
	}

	//we wait for 3 blocks, if not write to main chain, just return
	curNumber := number
	for new(big.Int).Sub(curNumber, number).Int64() < 3 {

		tmpNumber, err := client.BlockNumber(ctx)
		if err != nil {
			cs.logger.Error("saveDataToMainChain: failed to get BlockNumber, abort to wait for 3 blocks", "err", err)
			return
		}

		if tmpNumber.Cmp(curNumber) > 0 {
			_, isPending, err := client.TransactionByHash(ctx, hash)
			if !isPending && err == nil {
				cs.logger.Info("saveDataToMainChain: tx packaged in block in main chain")
				return
			}

			curNumber = tmpNumber
		} else {
			// we don't want to make too many rpc calls
			// TODO: estimate the right interval
			time.Sleep(1 * time.Second)
		}
	}

	cs.logger.Error("saveDataToMainChain: tx not packaged in any block after 3 blocks in main chain")
}

func (cs *ConsensusState) broadcastTX3ProofDataToMainChain(block *ethTypes.Block) {
	client := cs.cch.GetClient()
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	//ctx := context.Background() // testing only!

	proofData, err := ethTypes.NewTX3ProofData(block)
	if err != nil {
		cs.logger.Error("broadcastTX3ProofDataToMainChain: failed to create proof data", "block", block, "err", err)
		return
	}

	bs, err := rlp.EncodeToBytes(proofData)
	if err != nil {
		cs.logger.Error("broadcastTX3ProofDataToMainChain: failed to encode proof data", "proof data", proofData, "err", err)
		return
	}
	cs.logger.Infof("broadcastTX3ProofDataToMainChain proof data length: %d", len(bs))

	err = client.BroadcastDataToMainChain(ctx, cs.state.TdmExtra.ChainID, bs)
	if err != nil {
		cs.logger.Error("broadcastTX3ProofDataToMainChain(rpc) failed", "err", err)
		return
	}
}
