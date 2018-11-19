package types

import (
	"errors"
	"fmt"
	"io"

	//. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	// crypto "github.com/tendermint/go-crypto"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

type Proposal struct {
	NodeID			string			`json:"node_id"`
	Height           uint64           `json:"height"`
	Round            int              `json:"round"`
	Hash 		 []byte         `json:"hash"`
	BlockPartsHeader PartSetHeader    `json:"block_parts_header"`
	POLRound         int              `json:"pol_round"`    // -1 if null.
	POLBlockID       BlockID          `json:"pol_block_id"` // zero if null.
	ProposerNetAddr	 string           `json:"proposer_net_addr"`
	ProposerPeerKey  string           `json:"proposer_peer_key"`
	Signature        crypto.Signature `json:"signature"`
}

// polRound: -1 if no polRound.
func NewProposal(height uint64, round int, hash []byte, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID, peerKey string) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		Hash:		  hash,
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
		POLBlockID:       polBlockID,
		ProposerPeerKey:  peerKey,
	}
}

func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %s %s %v}", p.Height, p.Round,
		p.BlockPartsHeader, p.POLRound, p.POLBlockID, p.ProposerNetAddr, p.ProposerPeerKey, p.Signature)
}

func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceProposal{
		ChainID:  chainID,
		Proposal: CanonicalProposal(p),
	}, w, n, err)
}

func (p *Proposal) BlockHash() []byte {
	if p == nil {
		return []byte{}
	} else {
		return p.BlockPartsHeader.Hash
	}
}

func (p *Proposal) BlockHeaderHash() []byte{
	if p == nil {
		return []byte{}
	} else {
		return p.Hash
	}
}
