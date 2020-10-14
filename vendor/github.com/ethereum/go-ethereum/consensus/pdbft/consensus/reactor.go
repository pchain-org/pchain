package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/log"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	//sm "github.com/ethereum/go-ethereum/consensus/pdbft/state"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
)

const (
	StateChannel       = 0x20
	DataChannel        = 0x21
	VoteChannel        = 0x22
	VoteSetBitsChannel = 0x23

	waitReactorInterval         = 100 * time.Millisecond
	waitReactorTimes            = 50
	peerGossipSleepDuration     = 100 * time.Millisecond // Time to sleep if there's nothing to send.
	peerQueryMaj23SleepDuration = 2 * time.Second        // Time to sleep after each VoteSetMaj23Message sent
	maxConsensusMessageSize     = 1048576                // 1MB; NOTE: keep in sync with types.PartSet sizes.
)

//-----------------------------------------------------------------------------
var NodeID = ""

type ConsensusReactor struct {
	BaseService

	ChainId    string //make access easier
	conS       *ConsensusState
	evsw       types.EventSwitch
	peerStates sync.Map // map[string]*PeerState
	logger     log.Logger
	wg	       sync.WaitGroup
}

func NewConsensusReactor(consensusState *ConsensusState) *ConsensusReactor {
	conR := &ConsensusReactor{
		conS:    consensusState,
		ChainId: consensusState.chainConfig.PChainId,
		logger:  consensusState.backend.GetLogger(),
	}

	consensusState.conR = conR

	conR.BaseService = *NewBaseService(consensusState.backend.GetLogger(), "ConsensusReactor", conR)
	//conR.BaseReactor = *p2p.NewBaseReactor(consensusState.backend.GetLogger(), "ConsensusReactor", conR)
	return conR
}

func (conR *ConsensusReactor) OnStart() error {
	//log.Notice("ConsensusReactor ", "fastSync", conR.fastSync)
	conR.BaseService.OnStart()

	// callbacks for broadcasting new steps and votes to peers
	// upon their respective events (ie. uses evsw)
	conR.registerEventCallbacks()

	//if !conR.fastSync {
	// conR.catchupValidator(val, ok)
	_, err := conR.conS.Start()
	if err != nil {
		conR.logger.Errorf("ConsensusReactor) Start with error %v", err)
		return err
	}

	conR.startPeerRoutine()

	return nil
}

func (conR *ConsensusReactor) AfterStart() {

	// if there were peers added before start, start routines for them
	//wait at most 5 seconds, then start peer routings anyway
	for i:=0; i<waitReactorTimes && !conR.IsRunning(); i++ {
		log.Infof("(conR *ConsensusReactor) AfterStart(), wait %v times for conR running", i)
		time.Sleep(100 * time.Millisecond)
	}
	conR.startPeerRoutine()
}

func (conR *ConsensusReactor) OnStop() {
	conR.BaseService.OnStop()
	conR.conS.Stop()

	conR.logger.Infof("ConsensusReactor wait")
	conR.wg.Wait()
	conR.logger.Infof("ConsensusReactor wait over")
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer consensus.Peer) {

	conR.logger.Debug("add peer ============================================================")

	if _, ok := conR.peerStates.Load(peer.GetKey()); ok {
		conR.logger.Infof("peer %v has been added, return", peer.GetKey())
		return
	}

	peerKey := peer.GetKey()
	if peerKey == "" {
		conR.logger.Infof("peer key is empty")
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer, conR.logger)
	peer.SetPeerState(peerState)

	conR.peerStates.Store(peerKey, peerState)

	conR.logger.Debugf("peer is:%+v", peer)
	conR.logger.Debugf("peer key is:%+v", peer.GetKey())
	conR.logger.Infof("peer %v added", peer.GetKey())

	if conR.IsRunning() {
		// Begin routines for this peer.
		go conR.gossipDataRoutine(peer, peerState)
		go conR.gossipVotesRoutine(peer, peerState)

		// Send our state to peer.
		conR.sendNewRoundStepMessages(peer)
	}
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer consensus.Peer, reason interface{}) {
	/*
	if !conR.IsRunning() {
		return
	}
	*/
	ps, ok := peer.GetPeerState().(*PeerState)
	if !ok {
		conR.logger.Debug("Peer has no state", "peer", peer)
	}
	if ps != nil {
		ps.Disconnect()
	}
	conR.peerStates.Delete(peer.GetKey())
}

func (conR *ConsensusReactor) startPeerRoutine() {

	conR.peerStates.Range(func(_, val interface{}) bool{

		peerState := val.(*PeerState)
		peer := peerState.Peer
		go conR.gossipDataRoutine(peer, peerState)
		go conR.gossipVotesRoutine(peer, peerState)

		// Send our state to peer.
		conR.sendNewRoundStepMessages(peer)
		return true
	})
}

// Implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
func (conR *ConsensusReactor) Receive(chID uint64, src consensus.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		conR.logger.Debug("Receive", "src", src, "chId", chID, "bytes", msgBytes)
		return
	}

	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		conR.logger.Warn("Error decoding message", "src", src, "chId", chID, "msg", msg, "error", err, "bytes", msgBytes)
		// TODO punish peer?
		return
	}
	conR.logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps, exist := src.GetPeerState().(*PeerState)
	if !exist || ps == nil{
		// in case of nil peer state, due to consensus reactor start in the middle of the running, re-add it into the reactor
		conR.logger.Debug("Receive, ps not exist, add peer", "src", src)
		conR.AddPeer(src)
		ps = src.GetPeerState().(*PeerState)
	}
	//ps := src.Data.Get(conR.ChainId + "." + types.PeerStateKey).(*PeerState)

	switch chID {
	case StateChannel:
		// fmt.Println(chID, src, msg)
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg)
		case *CommitStepMessage:
			ps.ApplyCommitStepMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		default:
			conR.logger.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		//if conR.fastSync {
		//	log.Warn("Ignoring message received during fastSync", "msg", msg)
		//	return
		//}
		switch msg := msg.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.GetKey()}
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.GetKey()}
		case *Maj23SignAggrMessage:
			ps.SetHasMaj23SignAggr(msg.Maj23SignAggr)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.GetKey()}
		default:
			conR.logger.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		//if conR.fastSync {
		//	log.Warn("Ignoring message received during fastSync", "msg", msg)
		//	return
		//}
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, valSize := cs.Height, cs.Validators.Size()
			cs.mtx.Unlock()
			ps.EnsureVoteBitArrays(height, uint64(valSize))
			ps.SetHasVote(msg.Vote)

			conR.conS.peerMsgQueue <- msgInfo{msg, src.GetKey()}

		default:
			// don't punish (leave room for soft upgrades)
			conR.logger.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		conR.logger.Warn(Fmt("Unknown chId %X", chID))
	}

	if err != nil {
		conR.logger.Warn("Error in Receive()", "error", err)
	}
}

// implements events.Eventable
func (conR *ConsensusReactor) SetEventSwitch(evsw types.EventSwitch) {
	conR.evsw = evsw
	conR.conS.SetEventSwitch(evsw)
}

//--------------------------------------

// Listens for new steps and votes,
// broadcasting the result to peers
func (conR *ConsensusReactor) registerEventCallbacks() {

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringNewRoundStep(), func(data types.TMEventData) {
		rs := data.(types.EventDataRoundState).RoundState.(*RoundState)
		conR.broadcastNewRoundStep(rs)
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringVote(), func(data types.TMEventData) {
		edv := data.(types.EventDataVote)
		conR.broadcastHasVoteMessage(edv.Vote)
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringRequest(), func(data types.TMEventData) {
		//if conR.conS.Step < RoundStepPropose {
		re := data.(types.EventDataRequest)
		block := re.Proposal
		conR.logger.Infof("registerEventCallbacks received block height: %d, conR.conS.Height: %d, conR.conS.Step: %v", block.NumberU64(), conR.conS.Height, conR.conS.Step)
		//wait block in new height or new block has been inserted to start a new height
		if block.NumberU64() >= conR.conS.Height {

			//meas a block has been inserted into blockchain, let's start a new height
			if block.NumberU64() >= conR.conS.Height+1 {
				conR.conS.StartNewHeight()
			}

			//set block here
			conR.conS.blockFromMiner = re.Proposal
			conR.logger.Infof("registerEventCallbacks received Request Event conR.conS.blockFromMiner has been set with height: %v", conR.conS.blockFromMiner.NumberU64())
		} else {
			conR.logger.Info("registerEventCallbacks received Request Event", "conR.conS.Height", conR.conS.Height, "conR.conS.Step", conR.conS.Step)
		}
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringSignAggr(), func(data types.TMEventData) {
		edv := data.(types.EventDataSignAggr)
		conR.broadcastSignAggr(edv.SignAggr)
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringVote2Proposer(), func(data types.TMEventData) {
		edv := data.(types.EventDataVote2Proposer)
		conR.sendVote2Proposer(edv.Vote, edv.ProposerKey)
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringFinalCommitted(), func(data types.TMEventData) {
		conR.logger.Info("registerEventCallbacks received Final Committed Event", "conR.conS.Height", conR.conS.Height, "conR.conS.Step", conR.conS.Step)
		
		edfc := data.(types.EventDataFinalCommitted)
		
		if edfc.BlockNumber == conR.conS.Height {
			conR.logger.Info("start new height to apply this commit", "new height", edfc.BlockNumber + 1)
			conR.conS.StartNewHeight()
		}
	})
}

func (conR *ConsensusReactor) broadcastNewRoundStep(rs *RoundState) {

	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.conS.backend.GetBroadcaster().BroadcastMessage(StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		conR.conS.backend.GetBroadcaster().BroadcastMessage(StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

func (conR *ConsensusReactor) broadcastSignAggr(sign *types.SignAggr) {
	if sign != nil {
		msg := &Maj23SignAggrMessage{Maj23SignAggr: sign}
		conR.peerStates.Range(func(_, val interface{}) bool {
			peerState := val.(*PeerState)
			go func(peer consensus.Peer, peerState *PeerState) {
				if peer.Send(DataChannel, struct{ ConsensusMessage }{msg}) == nil {
					peerState.SetHasMaj23SignAggr(sign)
				}
			}(peerState.Peer, peerState)
			return true
		})
	}
}

func (conR *ConsensusReactor) sendVote2Proposer(vote *types.Vote, proposerKey string) {
	if vote != nil {
		peerState, ok := conR.peerStates.Load(proposerKey)
		if ok {
			msg := &VoteMessage{vote}
			peerState.(*PeerState).Peer.Send(VoteChannel, struct{ ConsensusMessage }{msg})
		} else {
			conR.logger.Infof("proposerKey is :%+v, proposer could be offline\n", proposerKey)
		}
	} else {
		panic("vote is nil")
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote) {
	// only the proposer needs to broadcast HasVoteMessage
	if conR.conS.IsProposer() {
		msg := &HasVoteMessage{
			Height: vote.Height,
			Round:  (int)(vote.Round),
			Type:   vote.Type,
			Index:  (int)(vote.ValidatorIndex),
		}
		conR.conS.backend.GetBroadcaster().BroadcastMessage(StateChannel, struct{ ConsensusMessage }{msg})
	}
}

func makeRoundStepMessages(rs *RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  rs.Step,
		SecondsSinceStartTime: int(time.Now().Sub(rs.StartTime).Seconds()),
	}
	if rs.Step == RoundStepCommit {
		csMsg = &CommitStepMessage{
			Height:           rs.Height,
			BlockPartsHeader: rs.ProposalBlockParts.Header(),
			BlockParts:       rs.ProposalBlockParts.BitArray(),
		}
	}
	return
}

func (conR *ConsensusReactor) sendNewRoundStepMessages(peer consensus.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		peer.Send(StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		peer.Send(StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer consensus.Peer, ps *PeerState) {

	conR.wg.Add(1)
	defer func() {
		conR.wg.Done()
		conR.logger.Infof("ConsensusReactor done one routine ")
	}()

	id := peer.GetKey()
	conR.logger.Infof("gossipDataRoutine start for peer %v", id)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer == nil {
			conR.logger.Infof("Peer is nil, Stopping gossipDataRoutine for peer %v", id)
			return
		}

		ps := peer.GetPeerState().(*PeerState)
		if !ps.Connected {
			conR.logger.Infof("Peer disconnected, stopping gossipDataRoutine for peer %v", peer)
			return
		}

		if !conR.IsRunning() {
			conR.logger.Infof("Consensus reactor is not running, stopping gossipDataRoutine for peer %v", peer)
			return
		}

		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		//only send proposal and blockpart when this node is proposer and round is less/equal to other peer
		if !rs.isProposer || (rs.Height != prs.Height) || (rs.Round > prs.Round) {
			//log.Info("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		/*
		log.Info("gossipDataRoutine ",
				"rs.Height", rs.Height, "rs.Round", rs.Round, "rs.Step", rs.Step,
				"ps.Height", prs.Height, "ps.Round", prs.Round, "ps.Step", prs.Step)
		*/

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartsHeader) {
			//log.Info("ProposalBlockParts matched", "blockParts", prs.ProposalBlockParts)
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(int(index))
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				if err := peer.Send(DataChannel, struct{ ConsensusMessage }{msg}); err == nil {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, int(index))
				}
				continue OUTER_LOOP
			}
		}

		//log.Info("gossipDataRoutine", "rs.Proposal", rs.Proposal, "prs.Proposal", prs.Proposal)

		if rs.Proposal != nil && !prs.Proposal {

			// Proposal: share the proposal metadata with peer.
			log.Info("send proposal to peer", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			msg := &ProposalMessage{Proposal: rs.Proposal}
			if err := peer.Send(DataChannel, struct{ ConsensusMessage }{msg}); err == nil {
				ps.SetHasProposal(rs.Proposal)
			}

			// ProposalPOL: lets peer know which POL votes we have so far.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if 0 <= rs.Proposal.POLRound {
				msg := &ProposalPOLMessage{
					Height:           rs.Height,
					ProposalPOLRound: rs.Proposal.POLRound,
					ProposalPOL:      rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray(),
				}
				peer.Send(DataChannel, struct{ ConsensusMessage }{msg})
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer consensus.Peer, ps *PeerState) {

	conR.wg.Add(1)
	defer func() {
		conR.wg.Done()
		conR.logger.Infof("ConsensusReactor done one routine ")
	}()

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0
	id := peer.GetKey()
OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if peer == nil {
			conR.logger.Infof("Peer is nil, Stopping gossipVotesRoutine for peer %v", id)
			return
		}

		ps1 := peer.GetPeerState().(*PeerState)
		if !ps1.Connected {
			conR.logger.Infof("Peer disconnected, stopping gossipVotesRoutine for peer %v", peer)
			return
		}

		if !conR.IsRunning() {
			conR.logger.Infof("Consensus reactor is not running, stopping gossipVotesRoutine for peer %v", peer)
			return
		}

		rs := conR.conS.GetRoundState()
		prs := ps1.GetRoundState()
		//prs := ps.GetRoundState()

		switch sleeping {
		case 1: // First sleep
			sleeping = 2
		case 2: // No more sleep
			sleeping = 0
		}

		if conR.conS.privValidator == nil {
			panic("conR.conS.privValidator is nil")
		}

		if peer.GetKey() != conR.conS.ProposerPeerKey {
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		//logger.Debug("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
		//	"prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send Prevotes, Precommits.
		if rs.Height == prs.Height /*&& prs.Round == rs.Round */{

			if prs.Step <= RoundStepPrevoteWait {
				var prevoteSA *types.SignAggr = nil
				if rs.VoteSignAggr != nil &&
					rs.VoteSignAggr.Prevotes(prs.Round) != nil &&
					rs.VoteSignAggr.Prevotes(prs.Round).HasTwoThirdsMajority(conR.conS.Validators) {
					prevoteSA = rs.VoteSignAggr.Prevotes(prs.Round)
				}
				if prevoteSA != nil && !prs.PrevoteMaj23SignAggr {
					if ps.PickSendSignAggr(prevoteSA) {
						conR.logger.Debug("Picked rs.VoteSignAggr.Prevotes to send")
						continue OUTER_LOOP
					}
				}
			}

			if rs.Step <= RoundStepPrecommitWait {
				var precommitSA *types.SignAggr = nil
				if rs.VoteSignAggr != nil &&
					rs.VoteSignAggr.Precommits(prs.Round) != nil &&
					rs.VoteSignAggr.Precommits(prs.Round).HasTwoThirdsMajority(conR.conS.Validators) {
					precommitSA = rs.VoteSignAggr.Precommits(prs.Round)
				}
				if precommitSA != nil && !prs.PrecommitMaj23SignAggr {
					if ps.PickSendSignAggr(precommitSA) {
						conR.logger.Debug("Picked rs.VoteSignAggr.Precommits to send")
						continue OUTER_LOOP
					}
				}
			}

		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			conR.logger.Debug("No votes to send, sleeping", "peer", peer)
		} else if sleeping == 2 {
			// Continued sleep...
			sleeping = 1
		}

		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor"
}

//-----------------------------------------------------------------------------

// Read only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                   uint64              // Height peer is at
	Round                    int                 // Round peer is at, -1 if unknown.
	Step                     RoundStepType       // Step peer is at
	StartTime                time.Time           // Estimated start of round 0 at this height
	Proposal                 bool                // True if peer has proposal for this round
	ProposalBlockPartsHeader types.PartSetHeader //
	ProposalBlockParts       *BitArray           //
	ProposalPOLRound         int                 // Proposal's POL round. -1 if none.
	ProposalPOL              *BitArray           // nil until ProposalPOLMessage received.
	Prevotes                 *BitArray           // All votes peer has for this round
	Precommits               *BitArray           // All precommits peer has for this round

	// Fields used for BLS signature aggregation
	PrevoteMaj23SignAggr   bool
	PrecommitMaj23SignAggr bool
}

func (prs PeerRoundState) String() string {
	return prs.StringIndented("")
}

func (prs PeerRoundState) StringIndented(indent string) string {
	return fmt.Sprintf(`PeerRoundState{
%s  %v/%v/%v @%v
%s  Proposal %v -> %v
%s  POL      %v (round %v)
%s  Prevotes   %v
%s  Precommits %v
%s}`,
		indent, prs.Height, prs.Round, prs.Step, prs.StartTime,
		indent, prs.ProposalBlockPartsHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent)
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

type PeerState struct {
	Peer consensus.Peer

	mtx sync.Mutex
	PeerRoundState

	Connected bool
	logger    log.Logger
}

func NewPeerState(peer consensus.Peer, logger log.Logger) *PeerState {
	return &PeerState{
		Peer: peer,
		PeerRoundState: PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
		},
		Connected: true,
		logger:    logger,
	}
}

// Returns an atomic snapshot of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PeerRoundState // copy
	return &prs
}

// Returns an atomic snapshot of the PeerRoundState's height
// used by the mempool to ensure peers are caught up before broadcasting new txs
func (ps *PeerState) GetHeight() uint64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.PeerRoundState.Height
}

func (ps *PeerState) Disconnect() {
	ps.Connected = false
}

func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	if ps == nil {
		panic("ps.mtx is nil")
	}
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != proposal.Height || ps.Round < proposal.Round {
		return
	}

	if ps.Round == proposal.Round && ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockPartsHeader = proposal.BlockPartsHeader
	ps.ProposalBlockParts = NewBitArray(proposal.BlockPartsHeader.Total)
	ps.ProposalPOLRound = proposal.POLRound
	ps.ProposalPOL = nil // Nil until ProposalPOLMessage received.
	ps.PrevoteMaj23SignAggr = false
	ps.PrecommitMaj23SignAggr = false
}

func (ps *PeerState) SetHasProposalBlockPart(height uint64, round int, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockParts.SetIndex((uint64)(index), true)
}

// Received msg saying proposer has 2/3+ votes including the signature aggregation
func (ps *PeerState) SetHasMaj23SignAggr(signAggr *types.SignAggr) {
	ps.logger.Debug("enter SetHasMaj23SignAggr()\n")

	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != signAggr.Height || ps.Round != signAggr.Round {
		return
	}

	if signAggr.Type == types.VoteTypePrevote {
		ps.PrevoteMaj23SignAggr = true
	} else if signAggr.Type == types.VoteTypePrecommit {
		ps.PrecommitMaj23SignAggr = true
	} else {
		panic(Fmt("Invalid signAggr type %d", signAggr.Type))
	}
}

// PickSendSignAggr sends signature aggregation to peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendSignAggr(signAggr *types.SignAggr) (ok bool) {
	msg := &Maj23SignAggrMessage{signAggr}
	if ps.Peer.Send(DataChannel, struct{ ConsensusMessage }{msg}) == nil {
		ps.SetHasMaj23SignAggr(msg.Maj23SignAggr)
		return true
	}
	return false
}

// PickVoteToSend sends vote to peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) (ok bool) {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		if ps.Peer.Send(VoteChannel, struct{ ConsensusMessage }{msg}) == nil {
			ps.SetHasVote(vote)
			return true
		}
	}
	return false
}

// votes: Must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	height, round, type_, size := votes.Height(), votes.Round(), votes.Type(), votes.Size()

	ps.ensureVoteBitArrays(height, uint64(size))

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		return votes.GetByIndex(int(index)), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height uint64, round int, type_ byte) *BitArray {
	if !types.IsVoteTypeValid(type_) {
		PanicSanity("Invalid vote type")
	}

	if ps.Height == height {
		if ps.Round == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.Prevotes
			case types.VoteTypePrecommit:
				return ps.Precommits
			}
		}
		if ps.ProposalPOLRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.ProposalPOL
			case types.VoteTypePrecommit:
				return nil
			}
		}
		return nil
	}

	return nil
}

// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height uint64, numValidators uint64) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height uint64, numValidators uint64) {
	if ps.Height == height {
		if ps.Prevotes == nil {
			ps.Prevotes = NewBitArray(numValidators)
		}
		if ps.Precommits == nil {
			ps.Precommits = NewBitArray(numValidators)
		}
		if ps.ProposalPOL == nil {
			ps.ProposalPOL = NewBitArray(numValidators)
		}
	}
}

func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, int(vote.Round), vote.Type, int(vote.ValidatorIndex))
}

func (ps *PeerState) setHasVote(height uint64, round int, type_ byte, index int) {
	ps.logger.Debug("setHasVote()", "height", height, "round", round, "index", index)
	// NOTE: some may be nil BitArrays -> no side effects.
	ps.getVoteBitArray(height, round, type_).SetIndex(uint64(index), true)
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.logger.Debug("ApplyNewRoundStepMessage()",
		"msg.Height", msg.Height, "msg.Round", msg.Round, "msg.Step", msg.Step,
		"ps.Height", ps.Height, "ps.Round", ps.Round, "ps.Step", ps.Step)

	// Ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.Height, ps.Round, ps.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = msg.Step
	ps.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.Proposal = false
		ps.ProposalBlockPartsHeader = types.PartSetHeader{}
		ps.ProposalBlockParts = nil
		ps.ProposalPOLRound = -1
		ps.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.Prevotes = nil
		ps.Precommits = nil
	}

	ps.logger.Debug("After ApplyNewRoundStepMessage()",
		"msg.Height", msg.Height, "msg.Round", msg.Round, "msg.Step", msg.Step,
		"ps.Height", ps.Height, "ps.Round", ps.Round, "ps.Step", ps.Step)
}

func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.ProposalBlockParts = msg.BlockParts
}

func (ps *PeerState) ApplyProposalPOLMessage(msg *ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}
	if ps.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// TODO: Merge onto existing ps.ProposalPOL?
	// We might have sent some prevotes in the meantime.
	ps.ProposalPOL = msg.ProposalPOL
}

func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

// The peer has responded with a bitarray of votes that it has
// of the corresponding BlockID.
// ourVotes: BitArray of votes we have for msg.BlockID
// NOTE: if ourVotes is nil (e.g. msg.Height < rs.Height),
// we conservatively overwrite ps's votes w/ msg.Votes.
func (ps *PeerState) ApplyVoteSetBitsMessage(msg *VoteSetBitsMessage, ourVotes *BitArray) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	votes := ps.getVoteBitArray(msg.Height, msg.Round, msg.Type)
	if votes != nil {
		if ourVotes == nil {
			votes.Update(msg.Votes)
		} else {
			otherVotes := votes.Sub(ourVotes)
			hasVotes := otherVotes.Or(msg.Votes)
			votes.Update(hasVotes)
		}
	}
}

func (ps *PeerState) String() string {
	return ps.StringIndented("")
}

func (ps *PeerState) StringIndented(indent string) string {
	return fmt.Sprintf(`PeerState{
%s  Key %v
%s  PRS %v
%s}`,
		indent, ps.Peer.GetKey(),
		indent, ps.PeerRoundState.StringIndented(indent+"  "),
		indent)
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeNewRoundStep  = byte(0x01)
	msgTypeCommitStep    = byte(0x02)
	msgTypeProposal      = byte(0x11)
	msgTypeProposalPOL   = byte(0x12)
	msgTypeBlockPart     = byte(0x13) // both block & POL
	msgTypeVote          = byte(0x14)
	msgTypeHasVote       = byte(0x15)
	msgTypeVoteSetMaj23  = byte(0x16)
	msgTypeVoteSetBits   = byte(0x17)
	msgTypeMaj23SignAggr = byte(0x18)
)

type ConsensusMessage interface{}

var _ = wire.RegisterInterface(
	struct{ ConsensusMessage }{},
	wire.ConcreteType{&NewRoundStepMessage{}, msgTypeNewRoundStep},
	wire.ConcreteType{&CommitStepMessage{}, msgTypeCommitStep},
	wire.ConcreteType{&ProposalMessage{}, msgTypeProposal},
	wire.ConcreteType{&ProposalPOLMessage{}, msgTypeProposalPOL},
	wire.ConcreteType{&BlockPartMessage{}, msgTypeBlockPart},
	wire.ConcreteType{&VoteMessage{}, msgTypeVote},
	wire.ConcreteType{&HasVoteMessage{}, msgTypeHasVote},
	wire.ConcreteType{&VoteSetMaj23Message{}, msgTypeVoteSetMaj23},
	wire.ConcreteType{&VoteSetBitsMessage{}, msgTypeVoteSetBits},
	wire.ConcreteType{&Maj23SignAggrMessage{}, msgTypeMaj23SignAggr},
)

// TODO: check for unnecessary extra bytes at the end.
func DecodeMessage(bz []byte) (msgType byte, msg ConsensusMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ ConsensusMessage }{}, r, maxConsensusMessageSize, n, &err).(struct{ ConsensusMessage }).ConsensusMessage
	return
}

//-------------------------------------

// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                uint64
	Round                 int
	Step                  RoundStepType
	SecondsSinceStartTime int
	LastCommitRound       int
}

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

type CommitStepMessage struct {
	Height           uint64
	BlockPartsHeader types.PartSetHeader
	BlockParts       *BitArray
}

func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep H:%v BP:%v BA:%v]", m.Height, m.BlockPartsHeader, m.BlockParts)
}

//-------------------------------------

type ProposalMessage struct {
	Proposal *types.Proposal
}

func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

//-------------------------------------

type ProposalPOLMessage struct {
	Height           uint64
	ProposalPOLRound int
	ProposalPOL      *BitArray
}

func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

//-------------------------------------

type BlockPartMessage struct {
	Height uint64
	Round  int
	Part   *types.Part
}

func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

//-------------------------------------

type VoteMessage struct {
	Vote *types.Vote
}

func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

//-------------------------------------

type Maj23SignAggrMessage struct {
	Maj23SignAggr *types.SignAggr
}

func (m *Maj23SignAggrMessage) String() string {
	return fmt.Sprintf("[SignAggr %v]", m.Maj23SignAggr)
}

//-------------------------------------

type HasVoteMessage struct {
	Height uint64
	Round  int
	Type   byte
	Index  int
}

func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v} VI:%v]", m.Index, m.Height, m.Round, m.Type, m.Index)
}

//-------------------------------------

type VoteSetMaj23Message struct {
	Height  uint64
	Round   int
	Type    byte
	BlockID types.BlockID
}

func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

//-------------------------------------

type VoteSetBitsMessage struct {
	Height  uint64
	Round   int
	Type    byte
	BlockID types.BlockID
	Votes   *BitArray
}

func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}
