package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/go-crypto"
	"math/big"
	"time"
)

type EpochApi struct {
	Number           uint64            `json:"number"`
	RewardPerBlock   *big.Int          `json:"reward_per_block"`
	StartBlock       uint64            `json:"start_block"`
	EndBlock         uint64            `json:"end_block"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	VoteStartBlock   uint64            `json:"vote_start_block"`
	VoteEndBlock     uint64            `json:"vote_end_block"`
	RevealStartBlock uint64            `json:"reveal_start_block"`
	RevealEndBlock   uint64            `json:"reveal_end_block"`
	Status           int               `json:"status"`
	Validators       []*EpochValidator `json:"validators"`
}

type EpochVotesApi struct {
	EpochNumber uint64                   `json:"vote_for_epoch"`
	StartBlock  uint64                   `json:"start_block"`
	EndBlock    uint64                   `json:"end_block"`
	Votes       []*EpochValidatorVoteApi `json:"votes"`
}

type EpochValidatorVoteApi struct {
	EpochValidator
	Salt     string      `json:"salt"`
	VoteHash common.Hash `json:"vote_hash"` // VoteHash = Keccak256(Epoch Number + PubKey + Amount + Salt)
	TxHash   common.Hash `json:"tx_hash"`
}

type EpochValidator struct {
	Address common.Address `json:"address"`
	PubKey  crypto.PubKey  `json:"public_key"`
	Amount  *big.Int       `json:"voting_power"`
}
