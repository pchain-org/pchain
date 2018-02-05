package types

import (
	"errors"
	"fmt"
	"io"

	//. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"

	abci "github.com/tendermint/abci/types"
	// crypto "github.com/tendermint/go-crypto"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	Height           int              `json:"height"`
	Round            int              `json:"round"`
	BlockPartsHeader PartSetHeader    `json:"block_parts_header"`
	POLRound         int              `json:"pol_round"`    // -1 if null.
	POLBlockID       BlockID          `json:"pol_block_id"` // zero if null.
	Signature        crypto.Signature `json:"signature"`
}

// polRound: -1 if no polRound.
func NewProposal(height int, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
		POLBlockID:       polBlockID,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %v}", p.Height, p.Round,
		p.BlockPartsHeader, p.POLRound, p.POLBlockID, p.Signature)
}

func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceProposal{
		ChainID:  chainID,
		Proposal: CanonicalProposal(p),
	}, w, n, err)
}

//-----------------
//author@liaoyd
type ValidatorMsg struct {
	Epoch          int              `json:"epoch"`
	ValidatorIndex int              `json:"validator_index"`
	Key            string           `json:"key"`
	PubKey         crypto.PubKey    `json:"pub_key"`
	Power          uint64           `json:"power"`
	Action         string           `json:"action"`
	Signature      crypto.Signature `json:"signature"`
}

func NewValidatorMsg(epoch int, key string, power uint64, action string) *ValidatorMsg {
	return &ValidatorMsg{
		Epoch:  epoch,
		Key:    key,
		Power:  power,
		Action: action,
	}
}

func (e *ValidatorMsg) String() string {
	return fmt.Sprintf("ValidatorMsg{Epoch:%v ValidatorIndex:%v Key:%s Power:%v Action:%s Signature:%v}", e.Epoch, e.ValidatorIndex, e.Key, e.Power, e.Action, e.Signature)
}

func (e *ValidatorMsg) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceValidatorMsg{
		ChainID: chainID,
		ValidatorMsg:   CanonicalValidatorMsg(e),
	}, w, n, err)
}

type AcceptVotes struct {
	Epoch  int           `json:"epoch"`
	Key    string        `json:"key"`
	PubKey crypto.PubKey `json:"pub_key"`
	Power  uint64        `"power"`
	Action string        `"action"`
	Sum    int64         `"sum"`
	Votes  []*ValidatorMsg      `votes`
	Maj23  bool          `"maj23"`
}

type PreVal struct {
	ValidatorSet *ValidatorSet `json:"validator_set"`
}

var AcceptVoteSet map[string]*AcceptVotes //votes

// var ValidatorChannel chan []*abci.Validator
var ValidatorChannel chan int
var EndChannel chan []*abci.Validator

var ValChangedEpoch map[int][]*AcceptVotes

//for updating validator during restart
// var DurStart chan []*abci.Validator
// var EndStart chan int
