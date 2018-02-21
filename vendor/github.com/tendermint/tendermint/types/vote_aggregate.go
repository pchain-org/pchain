package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	//"github.com/tendermint/go-data"
)

const MaxVoteSetSize = 22020096 // 21MB TODO make it configurable

type VotesAggr struct {
	Height           int64            `json:"height"`
	Round            int              `json:"round"`
	Type             byte             `json:"type"`
	VotePartsHeader  PartSetHeader	  `json:"votePartsHeader"`
        BitArray         *BitArray         // valIndex -> hasVote?
	NumValidators	 int              `json:"numValidators"`
	Sum              int64            `json:"sum"`     // vote sum
}

func MakeVotesAggr(height int64, round int, type byte, votePartsHeader PartSetHeader, num_validators int) *blockVotes {
        return &VotesAggr{
		Height : height,
		Round : round,
		Type : type,
		VotePartsHeader: votePartsHeader,
                BitArray:  NewBitArray(numValidators),
		NumValidators: num_validators,
		Sum : 0,
        }
}

func (va *VotesAggr) String() string {
	return va.StringIndented("")
}

func (va *VotesAggr) StringIndented(indent string) string {
	if va == nil {
		return "nil-VotesAggr"
	}
	return fmt.Sprintf(`VotesAggr{
%s  %v
%s  %v
%s  %v
%s  %v
}`,
		indent, va.Height,
		indent, va.Round,
		indent, va.Type,
		indent, va.NumValidators)
}

// ------------------------------------------------------------------------------
type Maj23VoteSet struct {
        Votes            []*Vote   // valIndex -> *Vote
}

func MakeMaj23VoteSet(votes []*Vote, partSize int) (*Maj23VoteSet, *PartSet) {
	numVotes := len(votes)

	voteset := &Maj23VoteSet{
		Votes:	make([]*Vote, numVotes),
	}

	voteset.AddVotes(votes)
	
	return voteset, voteset.MakePartSet(PartSize)
}

func (va *Maj23VoteSet) addVerifiedVote(vote *Vote, votingPower int64) {
        valIndex := vote.ValidatorIndex
        if existing := va.Votes[valIndex]; existing == nil {
                va.BitArray.SetIndex(valIndex, true)
                va.Votes[valIndex] = vote
                va.Sum += votingPower
        }
}

// Split it into parts
func (va *Maj23VoteSet) MakePartSet(partSize int) *PartSet {
        return NewPartSetFromData(wire.BinaryBytes(va), partSize)
}

// Fill votes from passed in parameter
func (va *Maj23VoteSet) AddVotes(votes []*Vote) {
        for i := 0; i < numValidators; i++ {
		if votes[i] != ni {
			// Use voting power 0 temproralily
			va.addVerifiedVote(votes[i], 0)
		}
        }    
}

func (va *Maj23VoteSet) Size() int {
	return len(va.Votes)
}

func (va *Maj23VoteSet) StringIndented(indent string) string {
	if va == nil {
		return "nil-Maj23VoteSet"
	}
	voteStrings := make([]string, len(va.Votes))
	for i, vote := range va.Votes {
		voteStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Maj23VoteSet{ %s  Precommits: %v }`,
		indent, strings.Join(voteStrings, "\n"+indent+"  "))
}

func (va *Maj23VoteSet) String() string {
	va.StringIndented(" ")
}

