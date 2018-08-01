package validatorStrategies

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

var validators = []*validatorsReward{
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3500"), big.NewInt(100), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3501"), big.NewInt(50), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3502"), big.NewInt(30), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3503"), big.NewInt(10), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3504"), big.NewInt(20), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3505"), big.NewInt(90), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3506"), big.NewInt(40), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3507"), big.NewInt(80), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3508"), big.NewInt(70), nil, nil, big.NewInt(0)},
	{common.HexToAddress("43890699D5D4A7A6D5E5A172F95E67DEE8EE3509"), big.NewInt(60), nil, nil, big.NewInt(0)},
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

	total, _ := big.NewFloat(550e+18).Int(nil)

	calculateReward(validators, totalSmooth, total)

	sumTotal := big.NewInt(0)

	for _, v := range validators {
		t.Log(v.rewardPercent)
		t.Log(v.Address, v.rewardAmount)
		sumTotal.Add(sumTotal, v.rewardAmount)
	}

	t.Log(sumTotal)
}
