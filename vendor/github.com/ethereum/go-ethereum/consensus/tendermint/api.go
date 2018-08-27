package tendermint

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

// API is a user facing RPC API of Tendermint
type API struct {
	chain      consensus.ChainReader
	tendermint *backend
}

// GetCurrentEpochNumber retrieves the current epoch number.
func (api *API) GetCurrentEpochNumber() (int, error) {
	return api.tendermint.core.consensusState.Epoch.Number, nil
}

// GetEpoch retrieves the Epoch Detail by Number
func (api *API) GetEpoch(number int) (*tdmTypes.EpochApi, error) {

	var resultEpoch *epoch.Epoch
	curEpoch := api.tendermint.core.consensusState.Epoch
	if number < 0 || number > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}

	if number == curEpoch.Number {
		resultEpoch = curEpoch
	} else {
		resultEpoch = epoch.LoadOneEpoch(curEpoch.GetDB(), number)
	}

	validators := make([]tdmTypes.GenesisValidator, len(resultEpoch.Validators.Validators))
	for i, val := range resultEpoch.Validators.Validators {
		validators[i] = tdmTypes.GenesisValidator{
			EthAccount: common.BytesToAddress(val.Address),
			PubKey:     val.PubKey,
			Amount:     val.VotingPower,
			Name:       "",
		}
	}

	return &tdmTypes.EpochApi{
		Number:           resultEpoch.Number,
		RewardPerBlock:   resultEpoch.RewardPerBlock,
		StartBlock:       resultEpoch.StartBlock,
		EndBlock:         resultEpoch.EndBlock,
		StartTime:        resultEpoch.StartTime,
		EndTime:          resultEpoch.EndTime,
		VoteStartBlock:   resultEpoch.GetVoteStartHeight(),
		VoteEndBlock:     resultEpoch.GetVoteEndHeight(),
		RevealStartBlock: resultEpoch.GetRevealVoteStartHeight(),
		RevealEndBlock:   resultEpoch.GetRevealVoteEndHeight(),
		Status:           resultEpoch.Status,
		Validators:       validators,
	}, nil
}

// GetEpochVote
func (api *API) GetEpochVote() (*tdmTypes.EpochVotesApi, error) {

	ep := api.tendermint.core.consensusState.GetRoundState().Epoch
	if ep.GetNextEpoch() != nil {

		votes := ep.GetNextEpoch().GetEpochValidatorVoteSet().Votes
		votesApi := make([]tdmTypes.EpochValidatorVoteApi, 0, len(votes))
		for _, v := range votes {
			votesApi = append(votesApi, tdmTypes.EpochValidatorVoteApi{
				Address:  v.Address,
				PubKey:   v.PubKey,
				Amount:   v.Amount,
				Salt:     v.Salt,
				VoteHash: v.VoteHash,
				TxHash:   v.TxHash,
			})
		}

		return &tdmTypes.EpochVotesApi{
			EpochNumber: ep.GetNextEpoch().Number,
			StartBlock:  ep.GetNextEpoch().StartBlock,
			EndBlock:    ep.GetNextEpoch().EndBlock,
			Votes:       votesApi,
		}, nil
	}
	return nil, errors.New("next epoch has not been proposed")
}
