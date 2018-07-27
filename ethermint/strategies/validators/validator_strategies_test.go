package validatorStrategies

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

var validators = []*validatorsReward{
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(100), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(50), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(30), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(10), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(20), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(90), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(40), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(80), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(70), nil, nil},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE350F"), big.NewInt(60), nil, nil},
}

func TestValidatorsStrategy_SmoothReward(t *testing.T) {

	median := findMedian(validators)
	m, accuracy := median.Float64()
	if accuracy == big.Exact && m == 55 {
		t.Logf("Median Result is correct %v", m)
	} else {
		t.Errorf("Median Reuslt is wrong %v", m)
	}

	totalSmooth := calculateSmooth(validators, median)
	calculateRewardPercent(validators, totalSmooth)

	for _, v := range validators {
		//t.Log(v.Deposit, v.rewardPercent)
		fv := new(big.Float).Mul(big.NewFloat(550e+18), v.rewardPercent)
		//t.Log(fv)
		t.Log(fv.Int(nil))

	}
}
