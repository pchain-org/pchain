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
	Maj23		 BlockID	  `json:"maj23"`
        BitArray         *BitArray        `json:"BitArray"`
	Sum		 int64            `json:"Sum"` 

	// BLS signature aggregation to be added here
	SignatureAggr	crypto.BLSSignature	`json:"SignatureAggr"`

}

func (sa *SignAggr) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceSignAggr{
		chainID,
		CanonicalJSONSignAggr{
			sa.Height,
			sa.Round,
			sa.Type,
			sa.NumValidators,
			CanonicalBlockID(sa.BlockID),
			CanonicalBlockID(sa.BlockID),
			sa.Sum,
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
		Maj23	: blockID,
		ChainID: chainID,
                BitArray: NewBitArray(numValidators),
		SignatureAggr : signAggr,
		Sum	: 0,
        }
}

func (sa *SignAggr) SignAggr() crypto.BLSSignature {
	return sa.SignatureAggr
}

func (sa *SignAggr) HasTwoThirdsMajority() bool {
	if sa == nil {
		return false
	}
	return sa.Maj23.IsZero()
}

func (sa *SignAggr) SetMaj23(blockID BlockID) {
	sa.Maj23 = blockID
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
	return sa.Maj23.IsZero() != false
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
	if sa.Maj23.IsZero() == true {
		return BlockID{}, false
	} else {
		return sa.Maj23, true
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
