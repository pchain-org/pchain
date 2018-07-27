package validatorStrategies

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pchain/common/plogger"
	"github.com/tendermint/abci/types"
	"math/big"
	"sort"
)

var plog = plogger.GetLogger("ValidatorStrategy")

type ValidatorsStrategy struct {
	currentValidators []*types.Validator
}

func (strategy *ValidatorsStrategy) Receiver() common.Address {
	return common.Address{}
}

func (strategy *ValidatorsStrategy) SetValidators(validators []*types.Validator) {
	strategy.currentValidators = validators
}

func (strategy *ValidatorsStrategy) GetValidators() []*types.Validator {
	return strategy.currentValidators
}

func (strategy *ValidatorsStrategy) CollectTx(tx *ethTypes.Transaction) {
	// This function do nothing
	//glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), tx.To: %x, validator: %x", tx.To(), tx.Data())
	//if reflect.DeepEqual(tx.To(), common.HexToAddress("0000000000000000000000000000000000000001")) {
	//	glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), Adding validator: %v", tx.Data())
	//	glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), do nothing now")
	/*
		pubKey, err := crypto.PubKeyFromBytes(tx.Data())
		if err != nil {
			strategy.currentValidators = append(
				strategy.currentValidators,
				&tmTypes.GenesisValidator{
					PubKey : pubKey,
					Amount:  tx.Value().Int64(),
				},
			)
		} else {
			glog.V(logger.Info).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), pubkey err: %v", err)
		}
	*/
	//}
}

func (strategy *ValidatorsStrategy) GetUpdatedValidators() []*types.Validator {
	return []*types.Validator{}
}

func (strategy *ValidatorsStrategy) AccumulateRewards(state *state.StateDB, header *ethTypes.Header,
	uncles []*ethTypes.Header, totalUsedMoney *big.Int, rewardPerBlock *big.Int) {
	plog.Debugf("Validators Strategy - Block %v - Start", header.Number)

	totalReward := new(big.Int).Add(totalUsedMoney, rewardPerBlock)
	plog.Debugf("Validators Strategy - Total Reward is %v", totalReward)

	// Build Reward Data
	validators := make([]*validatorsReward, 0, len(strategy.currentValidators))
	for _, v := range strategy.currentValidators {
		validators = append(validators, &validatorsReward{v.Address, v.Power, nil, nil})
	}

	median := findMedian(validators)
	totalSmooth := calculateSmooth(validators, median)
	calculateRewardPercent(validators, totalSmooth)

	// Add Balance per validator
	for _, v := range validators {
		plog.Debugf("Validators Strategy - Validator (%x)", v.Address)
		plog.Debugf("Validators Strategy - Before the Reward, balance is: %v", state.GetBalance(v.Address))

		reward, _ := new(big.Float).Mul(new(big.Float).SetInt(totalReward), v.rewardPercent).Int(nil)
		plog.Debugf("Validators Strategy - Reward is: %v", reward)

		state.AddBalance(v.Address, reward)
		plog.Debugf("Validators Strategy - After the Reward, balance is: %v", state.GetBalance(v.Address))
	}

	plog.Debugf("Validators Strategy Calculate the Reward, Block %v - End", header.Number)
}

// --------------------------------------------
// Calculation of the Validator Reward Strategy
/*
	Step 1
	Find the Median Deposit among Validators Deposit

	Step 2
	Calculate the Smooth Percent based on Median
	x = deposit / median
	x = x / (x+1)

	Step 3
	Calculate the final Reward

*/
func findMedian(validators []*validatorsReward) *big.Float {
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Deposit.Cmp(validators[j].Deposit) == -1
	})

	size := len(validators)
	if size == 1 {
		return new(big.Float).SetInt(validators[0].Deposit)
	} else if size%2 == 0 {
		// Odd Validators
		middleTotal := new(big.Int).Add(validators[size/2-1].Deposit, validators[size/2].Deposit)
		return new(big.Float).Quo(new(big.Float).SetInt(middleTotal), big.NewFloat(2))
	} else {
		// Even Validators, return the middle
		return new(big.Float).SetInt(validators[size/2-1].Deposit)
	}
}

func calculateSmooth(validators []*validatorsReward, median *big.Float) *big.Float {
	totalSmooth := big.NewFloat(0)
	for _, v := range validators {
		p := new(big.Float).Quo(new(big.Float).SetInt(v.Deposit), median)
		v.smoothPercent = new(big.Float).Quo(p, new(big.Float).Add(p, big.NewFloat(1)))
		// Sum the smooth
		totalSmooth.Add(totalSmooth, v.smoothPercent)
	}
	return totalSmooth
}

func calculateRewardPercent(validators []*validatorsReward, totalSmooth *big.Float) {
	for _, v := range validators {
		v.rewardPercent = new(big.Float).Quo(v.smoothPercent, totalSmooth)
	}
}

// --------------------------------
// Validator Reward Type definition
type validatorsReward struct {
	Address       common.Address
	Deposit       *big.Int
	smoothPercent *big.Float
	rewardPercent *big.Float
}
