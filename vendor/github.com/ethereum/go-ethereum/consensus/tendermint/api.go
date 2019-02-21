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
func (api *API) GetCurrentEpochNumber() (uint64, error) {
	return api.tendermint.core.consensusState.Epoch.Number, nil
}

// GetEpoch retrieves the Epoch Detail by Number
func (api *API) GetEpoch(number uint64) (*tdmTypes.EpochApi, error) {

	var resultEpoch *epoch.Epoch
	curEpoch := api.tendermint.core.consensusState.Epoch
	if number < 0 || number > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}

	if number == curEpoch.Number {
		resultEpoch = curEpoch
	} else {
		resultEpoch = epoch.LoadOneEpoch(curEpoch.GetDB(), number, nil)
	}

	validators := make([]*tdmTypes.EpochValidator, len(resultEpoch.Validators.Validators))
	for i, val := range resultEpoch.Validators.Validators {
		validators[i] = &tdmTypes.EpochValidator{
			Address:        common.BytesToAddress(val.Address),
			PubKey:         val.PubKey,
			Amount:         val.VotingPower,
			RemainingEpoch: val.RemainingEpoch,
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
func (api *API) GetNextEpochVote() (*tdmTypes.EpochVotesApi, error) {

	ep := api.tendermint.core.consensusState.Epoch
	if ep.GetNextEpoch() != nil {

		votes := ep.GetNextEpoch().GetEpochValidatorVoteSet().Votes
		votesApi := make([]*tdmTypes.EpochValidatorVoteApi, 0, len(votes))
		for _, v := range votes {
			votesApi = append(votesApi, &tdmTypes.EpochValidatorVoteApi{
				EpochValidator: tdmTypes.EpochValidator{
					Address: v.Address,
					PubKey:  v.PubKey,
					Amount:  v.Amount,
				},
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

func (api *API) GetNextEpochValidators() ([]*tdmTypes.EpochValidator, error) {

	height := api.chain.CurrentBlock().NumberU64()

	ep := api.tendermint.core.consensusState.Epoch
	nextEp := ep.GetNextEpoch()
	if nextEp == nil {
		return nil, errors.New("voting for next epoch has not started yet")
	} else if height <= ep.GetVoteEndHeight() {
		return nil, errors.New("hash vote stage now, please wait for reveal stage")
	} else {
		state, err := api.chain.State()
		if err != nil {
			return nil, err
		}

		nextValidators := nextEp.Validators.Copy()
		err = epoch.DryRunUpdateEpochValidatorSet(state, nextValidators, nextEp.GetEpochValidatorVoteSet())
		if err != nil {
			return nil, err
		}

		validators := make([]*tdmTypes.EpochValidator, 0, len(nextValidators.Validators))
		for _, val := range nextValidators.Validators {
			validators = append(validators, &tdmTypes.EpochValidator{
				Address:        common.BytesToAddress(val.Address),
				PubKey:         val.PubKey,
				Amount:         val.VotingPower,
				RemainingEpoch: val.RemainingEpoch,
			})
		}

		return validators, nil
	}
}

// GeneratePrivateValidator
func (api *API) GeneratePrivateValidator(from common.Address) (*tdmTypes.PrivValidator, error) {
	validator := tdmTypes.GenPrivValidatorKey(from)
	return validator, nil
}
