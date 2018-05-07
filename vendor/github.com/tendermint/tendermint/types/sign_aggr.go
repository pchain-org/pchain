package types

import (
	//"bytes"
	//"errors"
	"fmt"
	//"io"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	//"github.com/tendermint/go-data"
	"io"
	"github.com/tendermint/go-wire"
)

//------------------------ signature aggregation -------------------
const MaxSignAggrSize = 22020096 // 21MB TODO make it configurable

type SignAggr struct {
	ChainID          string
	Height           int              `json:"height"`
	Round            int              `json:"round"`
	Type             byte             `json:"type"`
	NumValidators	 int              `json:"numValidators"`
	BlockID          BlockID          `json:"block_id"` // zero if vote is nil.
        BitArray         *BitArray         // valIndex -> hasVote?

	// BLS signature aggregation to be added here
	SignatureAggr	crypto.BLSSignature

	sum		int64             // Sum of voting power for seen votes, discounting conflicts
	maj23		BlockID		// First 2/3 majority seen
}

func (vote *SignAggr) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceVote{
		chainID,
		CanonicalJSONVote{
			CanonicalBlockID(vote.BlockID),
			vote.Height,
			vote.Round,
			vote.Type,
		},
	}, w, n, err)
}

func MakeSignAggr(height int, round int, mtype byte, numValidators int, blockID BlockID, chainID string, signAggr crypto.BLSSignature) *SignAggr {
        return &SignAggr{
		Height	: height,
		Round	: round,
		Type	: mtype,
		NumValidators: numValidators,
		BlockID	: blockID,
		ChainID: chainID,
                BitArray: NewBitArray(numValidators),
		SignatureAggr : signAggr,
		sum	: 0,
        }
}

func (sa *SignAggr) SignAggr() crypto.BLSSignature {
	return sa.SignatureAggr
}

func (sa *SignAggr) HasTwoThirdsMajority() bool {
	if sa == nil {
		return false
	}
	return sa.maj23.IsZero()
}

func (sa *SignAggr) SetMaj23(blockID BlockID) {
	sa.maj23 = blockID
}

func (sa *SignAggr) SetBitArray(newBitArray *BitArray) {
	sa.BitArray.Update(newBitArray)
}

func (sa *SignAggr) IsCommit() bool {
	if sa == nil {
		return false
	}
	if sa.Type != VoteTypePrecommit {
		return false
	}
	return sa.maj23.IsZero() != false
}

/*
func (sa *SignAggr) HasTwoThirdsAny() bool {
	if sa == nil {
		return false
	}
	return sa.sum > voteSet.valSet.TotalVotingPower()*2/3
}

func (sa *SignAggr) HasAll() bool {
	return sa.sum == sa.valSet.TotalVotingPower()
}
*/

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, PartSetHeader{}, false).
func (sa *SignAggr) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if sa == nil {
		return BlockID{}, false
	}
	if sa.maj23.IsZero() == true {
		return BlockID{}, false
	} else {
		return sa.maj23, true
	}
}

func (va *SignAggr) String() string {
	return va.StringIndented("")
}

func (va *SignAggr) StringIndented(indent string) string {
	if va == nil {
		return "nil-SignAggr"
	}
	return fmt.Sprintf(`SignAggr{
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
