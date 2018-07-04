package types

// canonical json is go-wire's json for structs with fields in alphabetical order
import (
	crypto "github.com/tendermint/go-crypto"
)

type CanonicalJSONBlockID struct {
	Hash        []byte                     `json:"hash,omitempty"`
	PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}

type CanonicalJSONPartSetHeader struct {
	Hash  []byte `json:"hash"`
	Total int    `json:"total"`
}

type CanonicalJSONProposal struct {
	BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
	Height           int                        `json:"height"`
	POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
	POLRound         int                        `json:"pol_round"`
	Round            int                        `json:"round"`
}

type CanonicalJSONVote struct {
	BlockID CanonicalJSONBlockID `json:"block_id"`
	Height  int                  `json:"height"`
	Round   int                  `json:"round"`
	Type    byte                 `json:"type"`
}

//------------------------------------
// Messages including a "chain id" can only be applied to one chain, hence "Once"

type CanonicalJSONOnceProposal struct {
	ChainID  string                `json:"chain_id"`
	Proposal CanonicalJSONProposal `json:"proposal"`
}

type CanonicalJSONOnceVote struct {
	ChainID string            `json:"chain_id"`
	Vote    CanonicalJSONVote `json:"vote"`
}

//-----------------------------
//author@liaoyd
type CanonicalJSONOnceValidatorMsg struct {
	ChainID string             `json:"chain_id"`
	ValidatorMsg   CanonicalJSONValidatorMsg `json:"validator_msg"`
}

type CanonicalJSONValidatorMsg struct {
	From           string        `json:"from"`
	Epoch          int           `json:"epoch"`
	ValidatorIndex int           `json:"validator_index"`
	Key            string        `json:"key"`
	PubKey         crypto.PubKey `json:"pub_key"`
	Power          uint64        `json:"power"`
	Action         string        `json:"action"`
        Target         string        `json:"target"`
}

//-----------------------------------
// Canonicalize the structs

func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID {
	return CanonicalJSONBlockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalPartSetHeader(blockID.PartsHeader),
	}
}

func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader {
	return CanonicalJSONPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalProposal(proposal *Proposal) CanonicalJSONProposal {
	return CanonicalJSONProposal{
		BlockPartsHeader: CanonicalPartSetHeader(proposal.BlockPartsHeader),
		Height:           proposal.Height,
		POLBlockID:       CanonicalBlockID(proposal.POLBlockID),
		POLRound:         proposal.POLRound,
		Round:            proposal.Round,
	}
}

func CanonicalVote(vote *Vote) CanonicalJSONVote {
	return CanonicalJSONVote{
		CanonicalBlockID(vote.BlockID),
		vote.Height,
		vote.Round,
		vote.Type,
	}
}

