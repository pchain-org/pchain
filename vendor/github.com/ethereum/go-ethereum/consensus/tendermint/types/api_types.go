package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"time"
)

type EpochApi struct {
	Number           hexutil.Uint64    `json:"number"`
	RewardPerBlock   *hexutil.Big      `json:"reward_per_block"`
	StartBlock       hexutil.Uint64    `json:"start_block"`
	EndBlock         hexutil.Uint64    `json:"end_block"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	VoteStartBlock   hexutil.Uint64    `json:"vote_start_block"`
	VoteEndBlock     hexutil.Uint64    `json:"vote_end_block"`
	RevealStartBlock hexutil.Uint64    `json:"reveal_start_block"`
	RevealEndBlock   hexutil.Uint64    `json:"reveal_end_block"`
	Validators       []*EpochValidator `json:"validators"`
}

type EpochVotesApi struct {
	EpochNumber hexutil.Uint64           `json:"vote_for_epoch"`
	StartBlock  hexutil.Uint64           `json:"start_block"`
	EndBlock    hexutil.Uint64           `json:"end_block"`
	Votes       []*EpochValidatorVoteApi `json:"votes"`
}

type EpochValidatorVoteApi struct {
	EpochValidator
	Salt     string      `json:"salt"`
	VoteHash common.Hash `json:"vote_hash"` // VoteHash = Keccak256(Epoch Number + PubKey + Amount + Salt)
	TxHash   common.Hash `json:"tx_hash"`
}

type EpochValidator struct {
	Address        common.Address `json:"address"`
	PubKey         string         `json:"public_key"`
	Amount         *hexutil.Big   `json:"voting_power"`
	RemainingEpoch hexutil.Uint64 `json:"remain_epoch"`
}
