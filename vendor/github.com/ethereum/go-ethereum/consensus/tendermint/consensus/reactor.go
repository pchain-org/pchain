package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"
	//sm "github.com/ethereum/go-ethereum/consensus/tendermint/state"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/tendermint/go-clist"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	peerGossipSleepDuration     = 100 * time.Millisecond // Time to sleep if there's nothing to send.
	peerQueryMaj23SleepDuration = 2 * time.Second        // Time to sleep after each VoteSetMaj23Message sent
	maxConsensusMessageSize     = 1048576                // 1MB; NOTE: keep in sync with types.PartSet sizes.
)

//-----------------------------------------------------------------------------

type ConsensusReactor struct {
	p2p.BaseReactor // BaseService + p2p.Switch

	conS     *ConsensusState
	evsw     types.EventSwitch
}

func NewConsensusReactor(consensusState *ConsensusState) *ConsensusReactor {
	conR := &ConsensusReactor{
		conS:     consensusState,
	}
	conR.BaseReactor = *p2p.NewBaseReactor(log, "ConsensusReactor", conR)
	return conR
}

func (conR *ConsensusReactor) OnStart() error {
	//log.Notice("ConsensusReactor ", "fastSync", conR.fastSync)
	conR.BaseReactor.OnStart()

	// callbacks for broadcasting new steps and votes to peers
	// upon their respective events (ie. uses evsw)
	conR.registerEventCallbacks()

	//if !conR.fastSync {
		// conR.catchupValidator(val, ok)
		_, err := conR.conS.Start()
		if err != nil {
			return err
		}
	//}
	return nil
}

func (conR *ConsensusReactor) OnStop() {
	conR.BaseReactor.OnStop()
	conR.conS.Stop()
}

/*
// Switch from the fast_sync to the consensus:
// reset the state, turn off fast_sync, start the consensus-state-machine
func (conR *ConsensusReactor) SwitchToConsensus(state *sm.State) {
	log.Notice("SwitchToConsensus")
	conR.conS.ReconstructLastCommit(state)
	// NOTE: The line below causes broadcastNewRoundStepRoutine() to
	// broadcast a NewRoundStepMessage.
	conR.conS.UpdateToStateAndEpoch(state, conR.conS.epoch)
	//conR.fastSync = false
	conR.conS.Start()
}
*/

// Implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			ID:                StateChannel,
			Priority:          5,
			SendQueueCapacity: 100,
		},
		&p2p.ChannelDescriptor{
			ID:                 DataChannel, // maybe split between gossiping current block and catchup stuff
			Priority:           10,          // once we gossip the whole block there's nothing left to send until next height or round
			SendQueueCapacity:  100,
			RecvBufferCapacity: 50 * 4096,
		},
		&p2p.ChannelDescriptor{
			ID:                 VoteChannel,
			Priority:           5,
			SendQueueCapacity:  100,
			RecvBufferCapacity: 100 * 100,
		},
		&p2p.ChannelDescriptor{
			ID:                 VoteSetBitsChannel,
			Priority:           1,
			SendQueueCapacity:  2,
			RecvBufferCapacity: 1024,
		},
	}
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer *p2p.Peer) {
	if !conR.IsRunning() {
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer)
	peer.Data.Set(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey, peerState)

	// Begin routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
	go conR.queryMaj23Routine(peer, peerState)

	go conR.validatorExMsgRoutine(peer, peerState)
	// go conR.GetDiffValidator()

	// Send our state to peer.
	conR.sendNewRoundStepMessages(peer)
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}
	// TODO
	//peer.Data.Get(PeerStateKey).(*PeerState).Disconnect()
}

// Implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
func (conR *ConsensusReactor) Receive(chID byte, src *p2p.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		log.Debug("Receive", "src", src, "chId", chID, "bytes", msgBytes)
		return
	}

	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "src", src, "chId", chID, "msg", msg, "error", err, "bytes", msgBytes)
		// TODO punish peer?
		return
	}
	log.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps := src.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)

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
		case *VoteSetMaj23Message:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()
			if height != msg.Height {
				return
			}
			// Peer claims to have a maj23 for some BlockID at H,R,S,
			votes.SetPeerMaj23(msg.Round, msg.Type, ps.Peer.Key, msg.BlockID)
			// Respond with a VoteSetBitsMessage showing which votes we have.
			// (and consequently shows which we don't have)
			var ourVotes *BitArray
			switch msg.Type {
			case types.VoteTypePrevote:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
			case types.VoteTypePrecommit:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				log.Warn("Bad VoteSetBitsMessage field Type")
				return
			}
			src.TrySend(conR.conS.state.TdmExtra.ChainID, VoteSetBitsChannel, struct{ ConsensusMessage }{&VoteSetBitsMessage{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: msg.BlockID,
				Votes:   ourVotes,
			}})
		case *TestMessage:
			fmt.Println(chID, src, msg)
			fmt.Println("get test message!!!!!!!!!")
			// fmt.Println(src.PubKey())
			switch msg.ValidatorMsg.Action {
			case "JOIN":
				// msg.ValidatorMsg.PubKey = src.PubKey()     //get PubKey
				validatorMsg := msg.ValidatorMsg
				from := validatorMsg.From
				ValidatorMsgMap[from] = validatorMsg //store request
				if _, ok := types.AcceptVoteSet[from]; !ok {
					types.AcceptVoteSet[from] = conR.NewAcceptVotes(validatorMsg, from) //new vote list to add vote
				}
			case "ACCEPT":
				conR.tryAddAcceptVote(msg.ValidatorMsg) //TODO
			}
		default:
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		//if conR.fastSync {
		//	log.Warn("Ignoring message received during fastSync", "msg", msg)
		//	return
		//}
		switch msg := msg.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.Key}
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.Key}
		default:
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
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
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
			cs.mtx.Unlock()
			ps.EnsureVoteBitArrays(height, uint64(valSize))
			ps.EnsureVoteBitArrays(height-1, uint64(lastCommitSize))
			ps.SetHasVote(msg.Vote)

			conR.conS.peerMsgQueue <- msgInfo{msg, src.Key}

		default:
			// don't punish (leave room for soft upgrades)
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteSetBitsChannel:
		//if conR.fastSync {
		//	log.Warn("Ignoring message received during fastSync", "msg", msg)
		//	return
		//}
		switch msg := msg.(type) {
		case *VoteSetBitsMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()

			if height == msg.Height {
				var ourVotes *BitArray
				switch msg.Type {
				case types.VoteTypePrevote:
					ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
				case types.VoteTypePrecommit:
					ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
				default:
					log.Warn("Bad VoteSetBitsMessage field Type")
					return
				}
				ps.ApplyVoteSetBitsMessage(msg, ourVotes)
			} else {
				ps.ApplyVoteSetBitsMessage(msg, nil)
			}
		default:
			// don't punish (leave room for soft upgrades)
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		log.Warn(Fmt("Unknown chId %X", chID))
	}

	if err != nil {
		log.Warn("Error in Receive()", "error", err)
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
		log.Info("registerEventCallbacks received Request Event ", "request", data.(types.EventDataRequest))
		//if conR.conS.Step < RoundStepPropose {
		re := data.(types.EventDataRequest)
		block := re.Proposal
		log.Info("with height","block.NumberU64()", block.NumberU64(), "conR.conS.Height", conR.conS.Height, "conR.conS.Step", conR.conS.Step )
		//wait block in new height or new block has been inserted to start a new height
		if block.NumberU64() == conR.conS.Height || block.NumberU64() == conR.conS.Height + 1 {

			//meas a block has been inserted into blockchain, let's start a new height
			if block.NumberU64() == conR.conS.Height + 1 {
				conR.conS.StartNewHeight()
			}

			//set block here
			conR.conS.blockFromMiner = re.Proposal
			log.Info("registerEventCallbacks received Request Event conR.conS.blockFromMiner has been set")
		} else {
			log.Info("registerEventCallbacks received Request Event", "conR.conS.Height", conR.conS.Height, "conR.conS.Step", conR.conS.Step )
		}
	})

	types.AddListenerForEvent(conR.evsw, "conR", types.EventStringFinalCommitted(), func(data types.TMEventData) {
		log.Info("registerEventCallbacks received Final Committed Event", "conR.conS.Step", conR.conS.Step)
	})
}

func (conR *ConsensusReactor) broadcastNewRoundStep(rs *RoundState) {

	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.Switch.Broadcast(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		conR.Switch.Broadcast(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  int(vote.Round),
		Type:   vote.Type,
		Index:  int(vote.ValidatorIndex),
	}
	conR.Switch.Broadcast(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{msg})
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range conR.Switch.Peers().List() {
			ps := peer.Data.Get(PeerStateKey).(*PeerState)
			prs := ps.GetRoundState()
			if prs.Height == vote.Height {
				// TODO: Also filter on round?
				peer.TrySend(StateChannel, struct{ ConsensusMessage }{msg})
			} else {
				// Height doesn't match
				// TODO: check a field, maybe CatchupCommitRound?
				// TODO: But that requires changing the struct field comment.
			}
		}
	*/
}

func makeRoundStepMessages(rs *RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: int(time.Now().Sub(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
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

func (conR *ConsensusReactor) sendNewRoundStepMessages(peer *p2p.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		peer.Send(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		peer.Send(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {
	log := log.New("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Notice(Fmt("Stopping gossipDataRoutine for %v.", peer))
			return
		}
		rs := conR.conS.GetRoundState()

		ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
		prs := ps1.GetRoundState()
		//prs := ps.GetRoundState()

		/*
		log.Info("gossipDataRoutine ",
			"prs1.Height", prs1.Height, "prs1.Round", prs1.Round, "prs1.Step", prs1.Step,
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
				if peer.Send(conR.conS.state.TdmExtra.ChainID, DataChannel, struct{ ConsensusMessage }{msg}) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, int(index))
				}
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height, help catch up. we catch up by using ethereum's p2p now
		if (0 < prs.Height) && (prs.Height < rs.Height) {

			log.Info("Data catchup", "height", rs.Height, "peerHeight", prs.Height, "peerProposalBlockParts", prs.ProposalBlockParts)
			//we send the last block to increase prs.Height
			block := conR.conS.GetChainReader().GetBlockByNumber(prs.Height)
			if block != nil {
				log.Info("Data catchup", "readed block height", block.Number().Uint64())
				conR.conS.backend.GetBroadcaster().BroadcastBlock(block, true)
			}/*
			//log.Info("Data catchup", "height", rs.Height, "peerHeight", prs.Height, "peerProposalBlockParts", prs.ProposalBlockParts)
			if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
				// Ensure that the peer's PartSetHeader is correct
				blockMeta := conR.conS.blockStore.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					log.Warn("Failed to load block meta", "peer height", prs.Height, "our height", rs.Height, "blockstore height", conR.conS.blockStore.Height(), "pv", conR.conS.privValidator)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				} else if !blockMeta.BlockID.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
					log.Info("Peer ProposalBlockPartsHeader mismatch, sleeping",
						"peerHeight", prs.Height, "blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Load the part
				part := conR.conS.blockStore.LoadBlockPart(prs.Height, index)
				if part == nil {
					log.Warn("Could not load part", "index", index,
						"peerHeight", prs.Height, "blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Send the part
				msg := &BlockPartMessage{
					Height: prs.Height, // Not our height, so it doesn't matter.
					Round:  prs.Round,  // Not our height, so it doesn't matter.
					Part:   part,
				}
				if peer.Send(conR.conS.state.ChainID, DataChannel, struct{ ConsensusMessage }{msg}) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				}

				continue OUTER_LOOP
			} else {
			*/
				//log.Info("No parts to send in catch-up, sleeping")
				time.Sleep(peerGossipSleepDuration)
				continue OUTER_LOOP
			/*}*/

		}

		// If height and round don't match, sleep.
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			//log.Info("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// By here, height and round match.
		// Proposal block parts were already matched and sent if any were wanted.
		// (These can match on hash so the round doesn't matter)
		// Now consider sending other things, like the Proposal itself.

		// Send Proposal && ProposalPOL BitArray?
		if rs.Proposal != nil && !prs.Proposal {
			// Proposal: share the proposal metadata with peer.
			{
				msg := &ProposalMessage{Proposal: rs.Proposal}
				if peer.Send(conR.conS.state.TdmExtra.ChainID, DataChannel, struct{ ConsensusMessage }{msg}) {
					ps.SetHasProposal(rs.Proposal)
				}
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
				peer.Send(conR.conS.state.TdmExtra.ChainID, DataChannel, struct{ ConsensusMessage }{msg})
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
	log := log.New("peer", peer)

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Notice(Fmt("Stopping gossipVotesRoutine for %v.", peer))
			return
		}
		rs := conR.conS.GetRoundState()

		ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
		prs := ps1.GetRoundState()
		//prs := ps.GetRoundState()

		switch sleeping {
		case 1: // First sleep
			sleeping = 2
		case 2: // No more sleep
			sleeping = 0
		}

		//log.Debug("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
		//	"prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send LastCommit, Prevotes, Precommits.
		if rs.Height == prs.Height {
			// If there are lastCommits to send...
			if prs.Step == RoundStepNewHeight {
				if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, rs.LastCommit) {
					log.Debug("Picked rs.LastCommit to send")
					continue OUTER_LOOP
				}
			}
			// If there are prevotes to send...
			if prs.Step <= RoundStepPrevote && prs.Round != -1 && prs.Round <= rs.Round {
				if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, rs.Votes.Prevotes(prs.Round)) {
					log.Debug("Picked rs.Prevotes(prs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are precommits to send...
			if prs.Step <= RoundStepPrecommit && prs.Round != -1 && prs.Round <= rs.Round {
				if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, rs.Votes.Precommits(prs.Round)) {
					log.Debug("Picked rs.Precommits(prs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are POLPrevotes to send...
			if prs.ProposalPOLRound != -1 {
				if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
					if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, polPrevotes) {
						log.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send")
						continue OUTER_LOOP
					}
				}
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommit.
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, rs.LastCommit) {
				log.Debug("Picked rs.LastCommit to send")
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Commit.
		if prs.Height != 0 && rs.Height >= prs.Height+2 {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			//commit := conR.conS.blockStore.LoadBlockCommit(prs.Height)
			tdmExtra, _ := conR.conS.LoadTendermintExtra(prs.Height)
			commit := tdmExtra.SeenCommit
			log.Info("Loaded BlockCommit for catch-up", "height", prs.Height, "commit", commit)
			if ps.PickSendVote(conR.conS.state.TdmExtra.ChainID, commit) {
				log.Debug("Picked Catchup commit to send")
				continue OUTER_LOOP
			}
		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			log.Debug("No votes to send, sleeping", "peer", peer,
				"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits)
		} else if sleeping == 2 {
			// Continued sleep...
			sleeping = 1
		}

		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

// NOTE: `queryMaj23Routine` has a simple crude design since it only comes
// into play for liveness when there's a signature DDoS attack happening.
func (conR *ConsensusReactor) queryMaj23Routine(peer *p2p.Peer, ps *PeerState) {
	log := log.New("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Notice(Fmt("Stopping queryMaj23Routine for %v.", peer))
			return
		}

		// Maybe send Height/Round/Prevotes
		{
			rs := conR.conS.GetRoundState()
			ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
			prs := ps1.GetRoundState()
			//prs := ps.GetRoundState()

			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					}})
					time.Sleep(peerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/Precommits
		{
			rs := conR.conS.GetRoundState()
			ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
			prs := ps1.GetRoundState()
			//prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrecommit,
						BlockID: maj23,
					}})
					time.Sleep(peerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/ProposalPOL
		{
			rs := conR.conS.GetRoundState()
			ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
			prs := ps1.GetRoundState()
			//prs := ps.GetRoundState()
			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					}})
					time.Sleep(peerQueryMaj23SleepDuration)
				}
			}
		}

		// Little point sending LastCommitRound/LastCommit,
		// These are fleeting and non-blocking.

		// Maybe send Height/CatchupCommitRound/CatchupCommit.
		{
			ps1 := peer.Data.Get(conR.conS.state.TdmExtra.ChainID + "." + types.PeerStateKey).(*PeerState)
			prs := ps1.GetRoundState()
			//prs := ps.GetRoundState()
			if prs.CatchupCommitRound != -1 && 0 < prs.Height && int64(prs.Height) <= conR.conS.GetChainReader().CurrentHeader().Number.Int64() {
				commit := conR.conS.LoadCommit(prs.Height)
				peer.TrySend(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
					Height:  prs.Height,
					Round:   commit.Round(),
					Type:    types.VoteTypePrecommit,
					BlockID: commit.BlockID,
				}})
				time.Sleep(peerQueryMaj23SleepDuration)
			}
		}

		time.Sleep(peerQueryMaj23SleepDuration)

		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // conR.StringIndented("")
}

func (conR *ConsensusReactor) StringIndented(indent string) string {
	s := "ConsensusReactor{\n"
	s += indent + "  " + conR.conS.StringIndented(indent+"  ") + "\n"
	for _, peer := range conR.Switch.Peers().List() {
		ps := peer.Data.Get(types.PeerStateKey).(*PeerState)
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}

//-----------------------------------------------------------------------------

// Read only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                   uint64                 // Height peer is at
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
	LastCommitRound          int                 // Round of commit for last height. -1 if none.
	LastCommit               *BitArray           // All commit precommits of commit for last height.
	CatchupCommitRound       int                 // Round that we have commit for. Not necessarily unique. -1 if none.
	CatchupCommit            *BitArray           // All commit precommits peer has for this height & CatchupCommitRound
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
%s  LastCommit %v (round %v)
%s  Catchup    %v (round %v)
%s}`,
		indent, prs.Height, prs.Round, prs.Step, prs.StartTime,
		indent, prs.ProposalBlockPartsHeader, prs.ProposalBlockParts,
		indent, prs.ProposalPOL, prs.ProposalPOLRound,
		indent, prs.Prevotes,
		indent, prs.Precommits,
		indent, prs.LastCommit, prs.LastCommitRound,
		indent, prs.CatchupCommit, prs.CatchupCommitRound,
		indent)
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

type PeerState struct {
	Peer *p2p.Peer

	mtx sync.Mutex
	PeerRoundState
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{
		Peer: peer,
		PeerRoundState: PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
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

func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != proposal.Height || ps.Round != proposal.Round {
		return
	}
	if ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockPartsHeader = proposal.BlockPartsHeader
	ps.ProposalBlockParts = NewBitArray(proposal.BlockPartsHeader.Total)
	ps.ProposalPOLRound = proposal.POLRound
	ps.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

func (ps *PeerState) SetHasProposalBlockPart(height uint64, round int, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockParts.SetIndex(uint64(index), true)
}

// PickVoteToSend sends vote to peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(chainID string, votes types.VoteSetReader) (ok bool) {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		return ps.Peer.Send(chainID, VoteChannel, struct{ ConsensusMessage }{msg})
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

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, uint64(size))

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, type_, int(index))
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
		if ps.CatchupCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.CatchupCommit
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
	if ps.Height == height+1 {
		if ps.LastCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height uint64, round int, numValidators int) {
	if ps.Height != height {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			PanicSanity(Fmt("Conflicting CatchupCommitRound. Height: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
		}
	*/
	if ps.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.CatchupCommitRound = round
	if round == ps.Round {
		ps.CatchupCommit = ps.Precommits
	} else {
		ps.CatchupCommit = NewBitArray(uint64(numValidators))
	}
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
		if ps.CatchupCommit == nil {
			ps.CatchupCommit = NewBitArray(numValidators)
		}
		if ps.ProposalPOL == nil {
			ps.ProposalPOL = NewBitArray(numValidators)
		}
	} else if ps.Height == height+1 {
		if ps.LastCommit == nil {
			ps.LastCommit = NewBitArray(numValidators)
		}
	}
}

func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, int(vote.Round), vote.Type, int(vote.ValidatorIndex))
}

func (ps *PeerState) setHasVote(height uint64, round int, type_ byte, index int) {
	log := log.New("peer", ps.Peer, "peerRound", ps.Round, "height", height, "round", round)
	log.Debug("setHasVote(LastCommit)", "lastCommit", ps.LastCommit, "index", index)

	// NOTE: some may be nil BitArrays -> no side effects.
	ps.getVoteBitArray(height, round, type_).SetIndex(uint64(index), true)
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.Height, ps.Round, ps.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step
	psCatchupCommitRound := ps.CatchupCommitRound
	psCatchupCommit := ps.CatchupCommit

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
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = ps.Precommits
		} else {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.CatchupCommitRound = -1
		ps.CatchupCommit = nil
	}
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
		indent, ps.Peer.Key,
		indent, ps.PeerRoundState.StringIndented(indent+"  "),
		indent)
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeNewRoundStep = byte(0x01)
	msgTypeCommitStep   = byte(0x02)
	msgTypeProposal     = byte(0x11)
	msgTypeProposalPOL  = byte(0x12)
	msgTypeBlockPart    = byte(0x13) // both block & POL
	msgTypeVote         = byte(0x14)
	msgTypeHasVote      = byte(0x15)
	msgTypeVoteSetMaj23 = byte(0x16)
	msgTypeVoteSetBits  = byte(0x17)
	//new liaoyd
	msgTypeTest = byte(0x03)
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
	//new liaoyd
	wire.ConcreteType{&TestMessage{}, msgTypeTest},
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

//new liaoyd
// type TestMessage struct {
// 	IP    string
// 	Value string
// }
type TestMessage struct {
	ValidatorMsg *types.ValidatorMsg
}

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//new liaoyd
func (m *TestMessage) String() string {
	return fmt.Sprintf("[TestMessage %v]", m.ValidatorMsg)
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

//----------------------------------------------
//author@liaoyd
// type AcceptVotes struct {
// 	Height int            `json:"height"`
// 	Key    string         `json:"key"`
// 	PubKey crypto.PubKey  `json:"pub_key"`
// 	Power  uint64         `"power"`
// 	Sum    int64          `"sum"`
// 	Votes  []*types.ExMsg `votes`
// 	Maj23  bool           `"maj23"`
// }

// type PreVal struct {
// 	ValidatorSet *types.ValidatorSet `json:"validator_set"`
// }

// var AcceptVoteSet map[string]*types.AcceptVotes //votes

func (conR *ConsensusReactor) NewAcceptVotes(msg *types.ValidatorMsg, key string) *types.AcceptVotes {
	return &types.AcceptVotes{
		Epoch: msg.Epoch,
		Key: key,
		PubKey: msg.PubKey,
		Power: msg.Power,
		Action: msg.Action,
		Sum:   0,
		Votes: make([]*types.ValidatorMsg, conR.conS.Validators.Size()),
		Maj23: false,
	}
}

//------------------------------------------
var ValidatorMsgList = clist.New() //transfer

var ValidatorMsgMap map[string]*types.ValidatorMsg //request

func SendValidatorMsgToCons(from string, key string, epoch int, power uint64, action string, target string) {
	// fmt.Println("in func SendExMsgToCons(s string)")
	validatorMsg := types.NewValidatorMsg(from, key, epoch, power, action, target)

	ValidatorMsgList.PushBack(validatorMsg)
}

func (conR *ConsensusReactor) validatorExMsgRoutine(peer *p2p.Peer, ps *PeerState) {
	fmt.Println("in func validatorExMsgRoutine(peer *p2p.Peer, ps *PeerState)")
	types.AcceptVoteSet = make(map[string]*types.AcceptVotes)
	// AcceptVoteSet := types.AcceptVoteSet
	ValidatorMsgMap = make(map[string]*types.ValidatorMsg)
	// AcceptVoteSet = make(map[string]*types.AcceptVotes)

	var next *clist.CElement

	for {
		if next == nil {
			next = ValidatorMsgList.FrontWait()
		}
		msg := next.Value.(*types.ValidatorMsg)
		fmt.Println("msg:", msg)

		switch msg.Action {
		case "JOIN": //joinValidator
			// privVal := types.LoadPrivValidator("/mnt/vdb/ethermint-validatortest/.ethermint/priv_validator.json")
			privVal := types.LoadPrivValidator(conR.conS.config.GetString("priv_validator_file"))
			msg.PubKey = privVal.PubKey
			// fmt.Println("priVal:", privVal)
			// fmt.Println("priValPubKey:", privVal.PubKey)
			tMsg := &TestMessage{ValidatorMsg: msg}
			fmt.Println("broadcast message!!!!!!")
			peer.Send(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{tMsg})

			from := msg.From
			ValidatorMsgMap[from] = msg
			if _, ok := types.AcceptVoteSet[from]; !ok {
				types.AcceptVoteSet[from] = conR.NewAcceptVotes(msg, from) //new vote list to add vote
			}
		case "ACCEPT": //acceptJoinReq
			if conR.conS.privValidator == nil || !conR.conS.Validators.HasAddress(conR.conS.privValidator.GetAddress()) {
				fmt.Println("we are not in validator set")
				break
			}
			if received, ok := ValidatorMsgMap[msg.Target]; ok {
				if received.Epoch <= conR.conS.Epoch.Number {
					fmt.Println("request height is lower than consensus height")
					break
				}
				if received.Power == msg.Power && received.Epoch == msg.Epoch {
					addr := conR.conS.privValidator.GetAddress()
					valIndex, _ := conR.conS.Validators.GetByAddress(addr)
					msg.ValidatorIndex = valIndex
					msg.PubKey = received.PubKey

					conR.conS.privValidator.SignValidatorMsg(conR.conS.state.TdmExtra.ChainID, msg)
					tMsg := &TestMessage{ValidatorMsg: msg}
					fmt.Println("sending tMsg!!!", tMsg)
					// conR.Switch.Broadcast(StateChannel, struct{ ConsensusMessage }{tMsg})
					peer.Send(conR.conS.state.TdmExtra.ChainID, StateChannel, struct{ ConsensusMessage }{tMsg})
					conR.tryAddAcceptVote(msg)

				} else {
					fmt.Println("different power or height")
				}
			} else {
				fmt.Println("didn't received JOIN request")
			}
		}
		next = next.NextWait()
		continue
	}
}

func (conR *ConsensusReactor) tryAddAcceptVote(validatorMsg *types.ValidatorMsg) (success bool, err error) {
	fmt.Println("in func (conR *ConsensusReactor) tryAddAcceptVote(validatorMsg *types.ValidatorMsg) (success bool, err error)")
	_, val := conR.conS.Validators.GetByIndex(validatorMsg.ValidatorIndex)
	if val == nil {
		fmt.Println("bad index!!!!!")
		return false, types.ErrVoteInvalidValidatorIndex
	}
	if !val.PubKey.VerifyBytes(types.SignBytes(conR.conS.state.TdmExtra.ChainID, validatorMsg), validatorMsg.Signature) {
		// Bad signature.
		fmt.Println("bad signature!!!!!")
		return false, types.ErrVoteInvalidSignature
	}
	if validatorMsg.Epoch <= conR.conS.Epoch.Number {
		fmt.Println("bad height!!!!!")
		return false, nil
	}
	return conR.addAcceptVotes(validatorMsg)
}

func (conR *ConsensusReactor) addAcceptVotes(validatorMsg *types.ValidatorMsg) (success bool, err error) {
	fmt.Println("in func (conR *ConsensusReactor) addAcceptVotes(validatorMsg *types.ValidatorMsg) (success bool, err error)")
	_, val := conR.conS.Validators.GetByIndex(validatorMsg.ValidatorIndex)
	target := validatorMsg.Target
	if types.AcceptVoteSet[target].Votes[validatorMsg.ValidatorIndex] != nil {
		fmt.Println("duplicate vote!!!")
		return false, nil
	}
	types.AcceptVoteSet[target].Sum += val.VotingPower
	types.AcceptVoteSet[target].Votes[validatorMsg.ValidatorIndex] = validatorMsg
	if types.AcceptVoteSet[target].Sum > conR.conS.Validators.TotalVotingPower()*2/3 && !types.AcceptVoteSet[target].Maj23 {
		fmt.Println("update validator set!!!!")
		types.AcceptVoteSet[target].Maj23 = true
		//these following values should be filled when join/withdraw, and not modified here
		/*
		types.AcceptVoteSet[target].Epoch = validatorMsg.Epoch
		types.AcceptVoteSet[target].PubKey = validatorMsg.PubKey
		types.AcceptVoteSet[target].Power = validatorMsg.Power
		types.AcceptVoteSet[target].Key = target
		types.AcceptVoteSet[target].Action = validatorMsg.Action
		*/
	}
	return true, nil
}
