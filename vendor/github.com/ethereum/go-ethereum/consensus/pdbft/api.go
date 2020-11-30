package pdbft

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"math/big"
)

// API is a user facing RPC API of Tendermint
type API struct {
	chain      consensus.ChainReader
	tendermint *backend
}

// GetCurrentEpochNumber retrieves the current epoch number.
func (api *API) GetCurrentEpochNumber() (hexutil.Uint64, error) {
	return hexutil.Uint64(api.tendermint.core.consensusState.Epoch.Number), nil
}

// GetEpoch retrieves the Epoch Detail by Number
func (api *API) GetEpoch(num hexutil.Uint64) (*tdmTypes.EpochApi, error) {

	number := uint64(num)
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
			PubKey:         val.PubKey.KeyString(),
			Amount:         (*hexutil.Big)(val.VotingPower),
			RemainingEpoch: hexutil.Uint64(val.RemainingEpoch),
		}
	}

	// Epoch Reward per block on main chain is 80% of total reward
	// Child chain do not use this value as reward
	eightyPercent := new(big.Int).Mul(resultEpoch.RewardPerBlock, big.NewInt(8))
	eightyPercent.Div(eightyPercent, big.NewInt(10))

	return &tdmTypes.EpochApi{
		Number:           hexutil.Uint64(resultEpoch.Number),
		RewardPerBlock:   (*hexutil.Big)(eightyPercent),
		StartBlock:       hexutil.Uint64(resultEpoch.StartBlock),
		EndBlock:         hexutil.Uint64(resultEpoch.EndBlock),
		StartTime:        resultEpoch.StartTime,
		EndTime:          resultEpoch.EndTime,
		VoteStartBlock:   hexutil.Uint64(resultEpoch.GetVoteStartHeight()),
		VoteEndBlock:     hexutil.Uint64(resultEpoch.GetVoteEndHeight()),
		RevealStartBlock: hexutil.Uint64(resultEpoch.GetRevealVoteStartHeight()),
		RevealEndBlock:   hexutil.Uint64(resultEpoch.GetRevealVoteEndHeight()),
		Validators:       validators,
	}, nil
}

// GetEpochVote
func (api *API) GetNextEpochVote() (*tdmTypes.EpochVotesApi, error) {

	ep := api.tendermint.core.consensusState.Epoch
	if ep.GetNextEpoch() != nil {

		var votes []*epoch.EpochValidatorVote
		if ep.GetNextEpoch().GetEpochValidatorVoteSet() != nil {
			votes = ep.GetNextEpoch().GetEpochValidatorVoteSet().Votes
		}
		votesApi := make([]*tdmTypes.EpochValidatorVoteApi, 0, len(votes))
		for _, v := range votes {
			var pkstring string
			if v.PubKey != nil {
				pkstring = v.PubKey.KeyString()
			}

			votesApi = append(votesApi, &tdmTypes.EpochValidatorVoteApi{
				EpochValidator: tdmTypes.EpochValidator{
					Address: v.Address,
					PubKey:  pkstring,
					Amount:  (*hexutil.Big)(v.Amount),
				},
				Salt:     v.Salt,
				VoteHash: v.VoteHash,
				TxHash:   v.TxHash,
			})
		}

		return &tdmTypes.EpochVotesApi{
			EpochNumber: hexutil.Uint64(ep.GetNextEpoch().Number),
			StartBlock:  hexutil.Uint64(ep.GetNextEpoch().StartBlock),
			EndBlock:    hexutil.Uint64(ep.GetNextEpoch().EndBlock),
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

		mainBlock := api.chain.CurrentBlock().Number()
		if !api.chain.Config().IsMainChain() {
			mainBlock = api.chain.CurrentBlock().Header().MainChainNumber
		}

		markProposedInEpoch := api.chain.Config().IsMarkProposedInEpoch(mainBlock)

		nextValidators := ep.Validators.Copy()
		err = epoch.DryRunUpdateEpochValidatorSet(state, ep.Number, nextValidators,
			nextEp.GetEpochValidatorVoteSet(), markProposedInEpoch)
		if err != nil {
			return nil, err
		}

		validators := make([]*tdmTypes.EpochValidator, 0, len(nextValidators.Validators))
		for _, val := range nextValidators.Validators {
			var pkstring string
			if val.PubKey != nil {
				pkstring = val.PubKey.KeyString()
			}
			validators = append(validators, &tdmTypes.EpochValidator{
				Address:        common.BytesToAddress(val.Address),
				PubKey:         pkstring,
				Amount:         (*hexutil.Big)(val.VotingPower),
				RemainingEpoch: hexutil.Uint64(val.RemainingEpoch),
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
