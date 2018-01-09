package minerRewardStrategies

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	//	abciEthTypes "github.com/tendermint/ethermint/types"
	"github.com/tendermint/ethermint/ethereum"
	"math/big"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/core"
	emTypes "github.com/tendermint/ethermint/types"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	tmTypes "github.com/tendermint/tendermint/types"
)

type MinerRewardStrategy struct {
}


func (strategy *MinerRewardStrategy) Receiver() common.Address {
	//return common.HexToAddress("7ef5a6135f1fd6a02593eedc869c6d41d934aef8")
	return ethereum.Coinbase()
}

func (strategy *MinerRewardStrategy) SetValidators(validators []*tmTypes.Validator) {
}

func (r *MinerRewardStrategy) GetValidators()([]*tmTypes.Validator) {
	return nil
}

func (r *MinerRewardStrategy) CollectTx(tx *ethTypes.Transaction) {
}

func (r *MinerRewardStrategy) GetUpdatedValidators() []*tmTypes.Validator {
	return nil
}

func (strategy *MinerRewardStrategy)AccumulateRewards(statedb *state.StateDB, header *ethTypes.Header,
							uncles []*ethTypes.Header, totalUsedMoney *big.Int) {
	reward := new(big.Int).Set(emTypes.RewardPerBlock)
	reward.Add(reward, totalUsedMoney)

	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, core.Big8)
		r.Sub(r, header.Number)
		r.Mul(r, emTypes.RewardPerBlock)
		r.Div(r, core.Big8)
		statedb.AddBalance(uncle.Coinbase, r)

		r.Div(emTypes.RewardPerBlock, core.Big32)
		reward.Add(reward, r)
	}

	coinbase := header.Coinbase
	glog.Infof("(strategy *MinerRewardStrategy)AccumulateRewards() 0, coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	statedb.AddBalance(header.Coinbase, reward)
	glog.Infof("(strategy *MinerRewardStrategy)AccumulateRewards() 1, coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
}