package validatorStrategies

import (
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	tmTypes "github.com/tendermint/tendermint/types"
	"math/big"
)

type ValidatorsStrategy struct {
	currentValidators []*tmTypes.GenesisValidator
}

func (strategy *ValidatorsStrategy) Receiver() common.Address {
	return common.Address{}
}

func (strategy *ValidatorsStrategy) SetValidators(validators []*tmTypes.GenesisValidator) {
	strategy.currentValidators = validators
	glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) SetValidators(): %v", strategy.currentValidators)
}

func (strategy *ValidatorsStrategy) GetValidators()([]*tmTypes.GenesisValidator) {
	return strategy.currentValidators
}

func (strategy *ValidatorsStrategy) CollectTx(tx *ethTypes.Transaction) {
	glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), tx.To: %x, validator: %x", tx.To(), tx.Data())
	if reflect.DeepEqual(tx.To(), common.HexToAddress("0000000000000000000000000000000000000001")) {
		glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), Adding validator: %v", tx.Data())
		glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) CollectTx(), do nothing now")
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
	}
}

func (strategy *ValidatorsStrategy) GetUpdatedValidators() []*tmTypes.GenesisValidator {
	glog.V(logger.Debug).Infof("(strategy *TxBasedValidatorsStrategy) GetUpdatedValidators():%v", strategy.currentValidators)
	return []*tmTypes.GenesisValidator{}
}

func (strategy *ValidatorsStrategy)AccumulateRewards(statedb *state.StateDB, header *ethTypes.Header,
							uncles []*ethTypes.Header, totalUsedMoney *big.Int, rewardPerBlock *big.Int) {

	glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() start, with %v validators\n", len(strategy.currentValidators))

	reward := new(big.Int).Set(rewardPerBlock)
	glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 0, reward is: %v, gasUsed is: %v, totalUsedMoney is: %v\n",
			reward, header.GasUsed, totalUsedMoney)

	reward.Add(reward, totalUsedMoney)

	totalAmount := int64(0)
	for _, v := range strategy.currentValidators {
		totalAmount += abs(v.Amount)
	}

	if totalAmount == 0 {
		glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 1, totalAmount is 0, just return\n")
		return
	}

	glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 2, total reward is: %v, total amount is: %v\n", reward, totalAmount)

	for _, v := range strategy.currentValidators {
		balance := statedb.GetBalance(v.EthAccount)
		glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 3, before the reward, individual (%x) balance is: %v\n", v.EthAccount, balance)
		indReward := big.NewInt(1).Mul(reward, big.NewInt(v.Amount))
		indReward.Div(indReward, big.NewInt(totalAmount))
		glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 4, individual reward is: %v\n", indReward)
		statedb.AddBalance(v.EthAccount, indReward)
		balance = statedb.GetBalance(v.EthAccount)
		glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() 5, individual reward is: %v, balance is: %v\n", indReward, balance)
	}

	glog.Infof("(strategy *ValidatorsStrategy)AccumulateRewards() end\n")
}

func abs(x int64) int64 {
	switch {
	case x < 0:
		return -x
	case x == 0:
		return 0 // return correctly abs(-0)
	}
	return x
}

