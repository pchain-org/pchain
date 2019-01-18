package types

import (
	//"bytes"
	//"errors"
	"fmt"
	"math/big"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	//"github.com/tendermint/go-data"
	"github.com/tendermint/go-wire"
	"io"
)

//------------------------ signature aggregation -------------------
const MaxSignAggrSize = 22020096 // 21MB TODO make it configurable

type SignAggr struct {
	ChainID       string
	Height        uint64    `json:"height"`
	Round         int       `json:"round"`
	Type          byte      `json:"type"`
	NumValidators int       `json:"numValidators"`
	BlockID       BlockID   `json:"blockid"`
	Maj23         BlockID   `json:"maj23"`
	BitArray      *BitArray `json:"bitarray"`
	Sum           int64     `json:"sum"`

	// BLS signature aggregation to be added here
	SignatureAggr crypto.BLSSignature `json:"SignatureAggr"`
	SignBytes     []byte              `json:"sign_bytes"`
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

func MakeSignAggr(height uint64, round int, mtype byte, numValidators int, blockID BlockID, chainID string, bitArray *BitArray, signAggr crypto.BLSSignature) *SignAggr {
	return &SignAggr{
		Height:        height,
		Round:         round,
		Type:          mtype,
		NumValidators: numValidators,
		BlockID:       blockID,
		Maj23:         blockID,
		ChainID:       chainID,
		BitArray:      bitArray,
		SignatureAggr: signAggr,
		Sum:           0,
	}
}

func (sa *SignAggr) SignAggr() crypto.BLSSignature {
	if sa != nil {
		return sa.SignatureAggr
	} else {
		return nil
	}
}

func (sa *SignAggr) SignRound() int {
	if sa == nil {
		return -1
	} else {
		return sa.Round
	}
}

func (sa *SignAggr) HasTwoThirdsMajority(valSet *ValidatorSet) bool {
	if valSet == nil {
		return false
	}
	talliedVotingPower, err := valSet.TalliedVotingPower(sa.BitArray)
	if err != nil {
		return false
	}

	/*
		quorum := big.NewInt(0)
		quorum.Mul(valSet.totalVotingPower, big.NewInt(2))
		quorum.Div(quorum, big.NewInt(3))
		quorum.Add(quorum, big.NewInt(1))
	*/
	quorum := Loose23MajorThreshold(valSet.TotalVotingPower(), sa.Round)

	return talliedVotingPower.Cmp(quorum) >= 0
}

func (sa *SignAggr) SetMaj23(blockID BlockID) {
	if sa == nil {
		sa.Maj23 = blockID
	}
}

func (sa *SignAggr) Size() int {
	if sa == nil {
		return 0
	}
	return (int)(sa.NumValidators)
}

func (sa *SignAggr) SetBitArray(newBitArray *BitArray) {
	if sa != nil {
		sa.BitArray.Update(newBitArray)
	}
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

func (sa *SignAggr) MakeCommit() *Commit {
	//        if sa.Type != types.VoteTypePrecommit {
	//               PanicSanity("Cannot MakeCommit() unless SignAggr.Type is VoteTypePrecommit")
	//        }

	// Make sure we have a 2/3 majority
	/*        if sa.HasTwoThirdsMajority()== false {
	                  PanicSanity("Cannot MakeCommit() unless a blockhash has +2/3")
	          }
	*/
	return &Commit{
		BlockID:  sa.Maj23,
		Height:   sa.Height,
		Round:    sa.Round,
		BitArray: sa.BitArray.Copy(),
		SignAggr: sa.SignAggr(),
	}
}

func (sa *SignAggr) SignAggrVerify(msg []byte, valSet *ValidatorSet) bool {
	if msg == nil || valSet == nil {
		return false
	}
	if (int)(sa.BitArray.Size()) != len(valSet.Validators) {
		return false
	}
	pubKey := valSet.AggrPubKey(sa.BitArray)
	return pubKey.VerifyBytes(msg, sa.SignatureAggr) && sa.HasTwoThirdsMajority(valSet)
}

func (sa *SignAggr) HasTwoThirdsAny(valSet *ValidatorSet) bool {
	if sa == nil {
		return false
	}

	/*
		twoThird := new(big.Int).Mul(voteSet.valSet.TotalVotingPower(), big.NewInt(2))
		twoThird.Div(twoThird, big.NewInt(3))sa
	*/
	twoThirdPlus1 := Loose23MajorThreshold(valSet.TotalVotingPower(), sa.Round)
	twoThird := twoThirdPlus1.Sub(twoThirdPlus1, big.NewInt(1))

	return big.NewInt(sa.Sum).Cmp(twoThird) == 1
}

func (sa *SignAggr) HasAll(valSet *ValidatorSet) bool {
	return big.NewInt(sa.Sum).Cmp(valSet.TotalVotingPower()) == 0
}

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

func (va *SignAggr) StringShort() string {
	return va.StringIndented("")
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
%s  %v
}`,
		indent, va.Height,
		indent, va.Round,
		indent, va.Type,
		indent, va.NumValidators,
		indent, va.BlockID)
}
