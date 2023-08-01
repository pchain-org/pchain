package pdbft

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

func needPatch(chainId string, blockNumber uint64) bool {
	return chainId == "child_0" && (blockNumber == 26536499 || blockNumber == 26536500)
}

func (sb *backend) accumulateRewardsPatch(state *state.StateDB, ep *epoch.Epoch, totalGasFee *big.Int) *big.Int {
	// Total Reward = Block Reward + Total Gas Fee
	var coinbaseReward *big.Int
	config := sb.chainConfig
	if config.PChainId == params.MainnetChainConfig.PChainId || config.PChainId == params.TestnetChainConfig.PChainId {
		// Main Chain

		// Coinbase Reward   = 80% of Total Reward
		// Foundation Reward = 20% of Total Reward
		rewardPerBlock := ep.RewardPerBlock
		if rewardPerBlock != nil && rewardPerBlock.Sign() == 1 {
			// 80% Coinbase Reward
			coinbaseReward = new(big.Int).Mul(rewardPerBlock, big.NewInt(8))
			coinbaseReward.Quo(coinbaseReward, big.NewInt(10))
			// 20% go to PChain Foundation (For official Child Chain running cost)
			foundationReward := new(big.Int).Sub(rewardPerBlock, coinbaseReward)
			state.AddBalance(foundationAddress, foundationReward)

			coinbaseReward.Add(coinbaseReward, totalGasFee)
		} else {
			coinbaseReward = totalGasFee
		}
	} else {
		// Child Chain
		rewardPerBlock := state.GetChildChainRewardPerBlock()
		if rewardPerBlock != nil && rewardPerBlock.Sign() == 1 {
			childChainRewardBalance := state.GetBalance(childChainRewardAddress)
			if childChainRewardBalance.Cmp(rewardPerBlock) == -1 {
				rewardPerBlock = childChainRewardBalance
			}
			// sub balance from childChainRewardAddress, reward per blocks
			state.SubBalance(childChainRewardAddress, rewardPerBlock)

			coinbaseReward = new(big.Int).Add(rewardPerBlock, totalGasFee)
		} else {
			coinbaseReward = totalGasFee
		}
	}
	return coinbaseReward
}
// AccumulateRewards credits the coinbase of the given block with the mining reward.
// Main Chain:
// The total reward consists of the 80% of static block reward of the Epoch and total tx gas fee.
// Child Chain:
// The total reward consists of the static block reward of Owner setup and total tx gas fee.
//
// If the coinbase is Candidate, divide the rewards by weight
func (sb *backend) accumulateRewardsPatch1(state *state.StateDB, header *types.Header, ep *epoch.Epoch,
	coinbase common.Address, coinbaseReward, totalGasFee *big.Int, selfRetrieveReward bool) {

	// Coinbase Reward   = Self Reward + Delegate Reward (if Deposit Proxied Balance > 0)
	//
	// IF commission > 0
	// Self Reward       = Self Reward + Commission Reward
	// Commission Reward = Delegate Reward * Commission / 100

	// Deposit Part
	selfDeposit := state.GetDepositBalance(coinbase)
	totalProxiedDeposit := state.GetTotalDepositProxiedBalance(coinbase)
	totalDeposit := new(big.Int).Add(selfDeposit, totalProxiedDeposit)

	var selfReward, delegateReward *big.Int
	if totalProxiedDeposit.Sign() == 0 {
		selfReward = coinbaseReward
	} else {
		selfReward = new(big.Int)
		selfPercent := new(big.Float).Quo(new(big.Float).SetInt(selfDeposit), new(big.Float).SetInt(totalDeposit))
		new(big.Float).Mul(new(big.Float).SetInt(coinbaseReward), selfPercent).Int(selfReward)

		delegateReward = new(big.Int).Sub(coinbaseReward, selfReward)
		commission := state.GetCommission(coinbase)
		if commission > 0 {
			commissionReward := new(big.Int).Mul(delegateReward, big.NewInt(int64(commission)))
			commissionReward.Quo(commissionReward, big.NewInt(100))
			// Add the commission to self reward
			selfReward.Add(selfReward, commissionReward)
			// Sub the commission from delegate reward
			delegateReward.Sub(delegateReward, commissionReward)
		}
	}

	config := sb.chainConfig
	outsideReward := config.IsOutOfStorage(header.Number, header.MainChainNumber)

	// Move the self reward to Reward Trie
	height := header.Number.Uint64()
	divideRewardByEpochPatch1(state, coinbase, ep.Number, height, selfReward, outsideReward, selfRetrieveReward/*, rollbackCatchup*/)

	// Calculate the Delegate Reward
	if delegateReward != nil && delegateReward.Sign() > 0 {
		totalIndividualReward := big.NewInt(0)
		// Split the reward based on Weight stack
		state.ForEachProxied(coinbase, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
			if depositProxiedBalance.Sign() == 1 {
				// deposit * delegateReward / total deposit
				individualReward := new(big.Int).Quo(new(big.Int).Mul(depositProxiedBalance, delegateReward), totalProxiedDeposit)
				divideRewardByEpochPatch1(state, key, ep.Number, height,individualReward, outsideReward, selfRetrieveReward/*, rollbackCatchup*/)
				totalIndividualReward.Add(totalIndividualReward, individualReward)
			}
			return true
		})
		// Recheck the Total Individual Reward, Float the difference
		cmp := delegateReward.Cmp(totalIndividualReward)
		if cmp == 1 {
			// if delegate reward > actual given reward, give remaining reward to Candidate
			diff := new(big.Int).Sub(delegateReward, totalIndividualReward)
			if outsideReward {
				state.AddRewardBalance(coinbase, diff)
			} else {
				log.Infof("should not be here")
			}
		} else if cmp == -1 {
			// if delegate reward < actual given reward, subtract the diff from Candidate
			diff := new(big.Int).Sub(totalIndividualReward, delegateReward)
			if outsideReward {
				state.SubRewardBalance(coinbase, diff)
			} else {
				log.Infof("should not be here")
			}
		}
	}
}


// AccumulateRewards credits the coinbase of the given block with the mining reward.
// Main Chain:
// The total reward consists of the 80% of static block reward of the Epoch and total tx gas fee.
// Child Chain:
// The total reward consists of the static block reward of Owner setup and total tx gas fee.
//
// If the coinbase is Candidate, divide the rewards by weight
func (sb *backend) accumulateRewardsPatch2(state *state.StateDB, header *types.Header, ep *epoch.Epoch,
	coinbase common.Address, coinbaseReward, totalGasFee *big.Int, selfRetrieveReward bool) {

	// Coinbase Reward   = Self Reward + Delegate Reward (if Deposit Proxied Balance > 0)
	//
	// IF commission > 0
	// Self Reward       = Self Reward + Commission Reward
	// Commission Reward = Delegate Reward * Commission / 100

	// Deposit Part
	selfDeposit := state.GetDepositBalance(coinbase)
	totalProxiedDeposit := state.GetTotalDepositProxiedBalance(coinbase)
	totalDeposit := new(big.Int).Add(selfDeposit, totalProxiedDeposit)

	var selfReward, delegateReward *big.Int
	if totalProxiedDeposit.Sign() == 0 {
		selfReward = coinbaseReward
	} else {
		selfReward = new(big.Int)
		selfPercent := new(big.Float).Quo(new(big.Float).SetInt(selfDeposit), new(big.Float).SetInt(totalDeposit))
		new(big.Float).Mul(new(big.Float).SetInt(coinbaseReward), selfPercent).Int(selfReward)

		delegateReward = new(big.Int).Sub(coinbaseReward, selfReward)
		commission := state.GetCommission(coinbase)
		if commission > 0 {
			commissionReward := new(big.Int).Mul(delegateReward, big.NewInt(int64(commission)))
			commissionReward.Quo(commissionReward, big.NewInt(100))
			// Add the commission to self reward
			selfReward.Add(selfReward, commissionReward)
			// Sub the commission from delegate reward
			delegateReward.Sub(delegateReward, commissionReward)
		}
	}

	// Move the self reward to Reward Trie
	config := sb.chainConfig
	outsideReward := config.IsOutOfStorage(header.Number, header.MainChainNumber)

	height := header.Number.Uint64()
	divideRewardByEpochPatch2(state, coinbase, ep.Number, height, selfReward, outsideReward, selfRetrieveReward)

	// Calculate the Delegate Reward
	if delegateReward != nil && delegateReward.Sign() > 0 {
		totalIndividualReward := big.NewInt(0)
		// Split the reward based on Weight stack
		state.ForEachProxied(coinbase, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
			if depositProxiedBalance.Sign() == 1 {
				// deposit * delegateReward / total deposit
				individualReward := new(big.Int).Quo(new(big.Int).Mul(depositProxiedBalance, delegateReward), totalProxiedDeposit)
				divideRewardByEpochPatch2(state, key, ep.Number, height,individualReward, outsideReward, selfRetrieveReward)
				totalIndividualReward.Add(totalIndividualReward, individualReward)
			}
			return true
		})
		// Recheck the Total Individual Reward, Float the difference
		cmp := delegateReward.Cmp(totalIndividualReward)
		if cmp == 1 {
			// if delegate reward > actual given reward, give remaining reward to Candidate
			diff := new(big.Int).Sub(delegateReward, totalIndividualReward)
			if outsideReward {
				state.GetState1DB().AddOutsideRewardBalanceByEpochNumber(coinbase, ep.Number,height, diff)
			} else {
				log.Infof("should not be here")
			}
		} else if cmp == -1 {
			// if delegate reward < actual given reward, subtract the diff from Candidate
			diff := new(big.Int).Sub(totalIndividualReward, delegateReward)
			if outsideReward {
				state.GetState1DB().SubOutsideRewardBalanceByEpochNumber(coinbase, ep.Number,height, diff)
			} else {
				log.Infof("should not be here")
			}
		}
	}
}

func divideRewardByEpochPatch1(state *state.StateDB, addr common.Address, epochNumber uint64,height uint64, reward *big.Int,
	outsideReward, selfRetrieveReward/*, rollbackCatchup*/ bool) {
	epochReward := new(big.Int).Quo(reward, big.NewInt(12))
	lastEpochReward := new(big.Int).Set(reward)
	for i := epochNumber; i < epochNumber+12; i++ {
		if i == epochNumber+11 {
			if outsideReward {
				state.AddRewardBalance(addr, lastEpochReward)
			} else {
				log.Infof("should not be here")
			}
		} else {
			if outsideReward {
				state.AddRewardBalance(addr, epochReward)
			} else {
				log.Infof("should not be here")
			}
			lastEpochReward.Sub(lastEpochReward, epochReward)
		}
	}
	if !selfRetrieveReward {
		log.Infof("should not be here")
	}
}


func divideRewardByEpochPatch2(state *state.StateDB, addr common.Address, epochNumber uint64,height uint64, reward *big.Int,
	outsideReward, selfRetrieveReward bool) {
	epochReward := new(big.Int).Quo(reward, big.NewInt(12))
	lastEpochReward := new(big.Int).Set(reward)
	for i := epochNumber; i < epochNumber+12; i++ {
		if i == epochNumber+11 {
			if outsideReward {
				state.GetState1DB().AddOutsideRewardBalanceByEpochNumber(addr, i, height,lastEpochReward)
			} else {
				log.Infof("should not be here")
			}
		} else {
			if outsideReward {
				//if !rollbackCatchup {
				state.GetState1DB().AddOutsideRewardBalanceByEpochNumber(addr, i, height,epochReward)
			} else {
				log.Infof("should not be here")
			}
			lastEpochReward.Sub(lastEpochReward, epochReward)
		}
	}
	if !selfRetrieveReward {
		log.Infof("should not be here")
	}
}

