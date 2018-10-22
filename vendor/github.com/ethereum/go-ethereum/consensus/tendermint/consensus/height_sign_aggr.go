package consensus

import (
	"strings"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/log"
)

type RoundVoteSignAggr struct {
	Prevotes   *types.SignAggr
	Precommits *types.SignAggr
}

/*
Keeps track of all signature aggregations from round 0 to round 'round'.

Also keeps track of up to one RoundVoteSignAggr greater than
'round' from each peer, to facilitate catchup syncing of commits.

A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
We let each peer provide us with up to 2 unexpected "catchup" rounds.
One for their LastCommit round, and another for the official commit round.
*/
type HeightVoteSignAggr struct {
	chainID			string
	height			uint64
	valSet			*types.ValidatorSet

	mtx			sync.Mutex
	round			int                       // max tracked round
	roundVoteSignAggrs	map[int]*RoundVoteSignAggr // keys: [0...round]
	logger log.Logger

	// peerCatchupRounds	map[string][]int          // keys: peer.Key; values: at most 2 rounds
}

func NewHeightVoteSignAggr(chainID string, height uint64, valSet *types.ValidatorSet, logger log.Logger) *HeightVoteSignAggr {
	hvs := &HeightVoteSignAggr{
		chainID: chainID,
		logger: logger,
	}
	hvs.Reset(height, valSet)
	return hvs
}

func (hvs *HeightVoteSignAggr) Reset(height uint64, valSet *types.ValidatorSet) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSignAggrs = make(map[int]*RoundVoteSignAggr)
//	hvs.peerCatchupRounds = make(map[string][]int)

	hvs.addRound(0)
	hvs.round = 0
}

func (hvs *HeightVoteSignAggr) Height() uint64 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.height
}

func (hvs *HeightVoteSignAggr) Round() int {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.round
}

// Create more RoundVoteSignAggr up to round.
func (hvs *HeightVoteSignAggr) SetRound(round int) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if hvs.round != 0 && (round < hvs.round+1) {
		PanicSanity("SetRound() must increment hvs.round")
	}
	for r := hvs.round + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSignAggrs[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

func (hvs *HeightVoteSignAggr) addRound(round int) {
	if _, ok := hvs.roundVoteSignAggrs[round]; ok {
		PanicSanity("addRound() for an existing round")
	}
	hvs.logger.Debug("addRound(round)", " round:", round)

	hvs.roundVoteSignAggrs[round] = &RoundVoteSignAggr{
		Prevotes:   nil,
		Precommits: nil,
	}
}

// Duplicate votes return added=false, err=nil.
func (hvs *HeightVoteSignAggr) AddSignAggr(signAggr *types.SignAggr) (added bool, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(signAggr.Type) {
		return
	}
	existing := hvs.getSignAggr(signAggr.Round, signAggr.Type)

	if existing != nil {
		hvs.logger.Warn("Found existing signature aggregation for (height %v round %v type %v)", signAggr.Height, signAggr.Round, signAggr.Type)
		return false, nil

	}

	rvs, ok := hvs.roundVoteSignAggrs[signAggr.Round]

	if !ok {
		return false, nil
	}

	if signAggr.Type == types.VoteTypePrevote {
		rvs.Prevotes = signAggr
	} else if signAggr.Type == types.VoteTypePrecommit {
		rvs.Precommits = signAggr
	} else {
		hvs.logger.Warn("Invalid signature aggregation for (height %v round %v type %v)", signAggr.Height, signAggr.Round, signAggr.Type)
		return false, nil
	}

	return true, nil
}

func (hvs *HeightVoteSignAggr) Prevotes(round int) *types.SignAggr {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getSignAggr(round, types.VoteTypePrevote)
}

func (hvs *HeightVoteSignAggr) Precommits(round int) *types.SignAggr {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getSignAggr(round, types.VoteTypePrecommit)
}

// Last round and blockID that has +2/3 prevotes for a particular block or nil.
// Returns -1 if no such round exists.
func (hvs *HeightVoteSignAggr) POLInfo() (polRound int, polBlockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- {
		rvs := hvs.getSignAggr(r, types.VoteTypePrevote)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, types.BlockID{}
}

func (hvs *HeightVoteSignAggr) getSignAggr(round int, type_ byte) *types.SignAggr {
	rvs, ok := hvs.roundVoteSignAggrs[round]
	if !ok {
		return nil
	}
	switch type_ {
	case types.VoteTypePrevote:
		return rvs.Prevotes
	case types.VoteTypePrecommit:
		return rvs.Precommits
	default:
		PanicSanity(Fmt("Unexpected vote type %X", type_))
		return nil
	}
}

/*

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
func (hvs *HeightVoteSignAggr) SetPeerMaj23(round int, type_ byte, peerID string, blockID types.BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !types.IsVoteTypeValid(type_) {
		return
	}
	signAggr := hvs.getSignAggr(round, type_)
	if signAggr == nil {
		return
	}
	signAggr.SetPeerMaj23(peerID, blockID)
}
*/

func (hvs *HeightVoteSignAggr) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSignAggr) StringIndented(indent string) string {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	vsStrings := make([]string, 0, (len(hvs.roundVoteSignAggrs)+1)*2)
	// rounds 0 ~ hvs.round inclusive
	for round := 0; round <= hvs.round; round++ {
		voteSetString := hvs.roundVoteSignAggrs[round].Prevotes.String()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSignAggrs[round].Precommits.String()
		vsStrings = append(vsStrings, voteSetString)
	}

	// all other peer catchup rounds
	for round, roundVoteSignAggr := range hvs.roundVoteSignAggrs {
		if round <= hvs.round {
			continue
		}
		voteSetString := roundVoteSignAggr.Prevotes.String()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = roundVoteSignAggr.Precommits.String()
		vsStrings = append(vsStrings, voteSetString)
	}

	return Fmt(`HeightVoteSignAggr{H:%v R:0~%v
%s  %v
%s}`,
		hvs.height, hvs.round,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}

