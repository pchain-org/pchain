package core

import (
	"fmt"
	"errors"
	ep "github.com/tendermint/tendermint/epoch"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	tmTypes "github.com/tendermint/tendermint/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	etdFuncName string = "SendValidatorMessage"
)

const (
	SVM_JOIN = "join"
	SVM_WITHDRAW = "withdraw"
	SVM_ACCEPT = "accept"
)

type svmEntity struct {
	voMap map[int]*ep.ValidatorOperationSet
	loaded bool
}

var localEntity = svmEntity{voMap:nil, loaded:false}

func init() {

	RegisterReceiveTxCb(etdFuncName, svmReceiveTxCb)
	RegisterCheckTxCb(etdFuncName, svmCheckTxCb)
	RegisterDeliverTxCb(etdFuncName, svmDeliverTxCb)
	RegisterCommitCb(etdFuncName, svmCommitCb)
	RegisterRefreshABCIResponseCbMap(etdFuncName, svmRefreshABCIResponseCb)
}

func svmReceiveTxCb() error {
	fmt.Println("svm_ReceiveTxCb")
	return nil
}

func svmCheckTxCb(tx *ethTypes.Transaction) error {

	fmt.Println("svm_DeliverTxCb")

	params := tx.ExtendTxData().Params
	hashInt, _ := params.Get("hash")
	hash := tx_b2s(hashInt.([]uint8))
	actionInt, _ := params.Get("action")
	action := tx_b2s(actionInt.([]uint8))

	if action == SVM_ACCEPT {
		vo := ep.LoadOperationWithHash(hash)
		if vo == nil {
			return errors.New("hash " + hash + " not found in ValidatorOperation, should exist")
		}
	}

	return nil
}

func svmDeliverTxCb(tx *ethTypes.Transaction) error {

	fmt.Println("svm_DeliverTxCb")
	if !localEntity.loaded {
		localEntity.voMap = make(map[int]*ep.ValidatorOperationSet)
		localEntity.loaded = true
	}

	var err error = nil
	txhash := tx.Hash().Hex()

	params := tx.ExtendTxData().Params
	fromStrInt, _ := params.Get("from")
	fromStr := tx_b2s(fromStrInt.([]uint8))
	epochInt, _ := params.Get("epoch")
	epoch := tx_b2i(epochInt.([]uint8))
	powerInt, _ := params.Get("power")
	power := tx_b2i(powerInt.([]uint8))
	actionInt, _ := params.Get("action")
	action := tx_b2s(actionInt.([]uint8))
	hashInt, _ := params.Get("hash")
	hash := tx_b2s(hashInt.([]uint8))

	vo := ep.LoadOperationWithHash(hash)

	epochKey := ep.VA_UNCONFIRMED_EPOCH

	if action == SVM_JOIN || action == SVM_WITHDRAW {

		if vo != nil {
			return errors.New("hash " + hash + " should not exist")
		}

		vo = &ep.ValidatorOperation{
			Validator: fromStr,
			TxHash: txhash,
			Action: action,
			Amount: int(power),
			Epoch: int(epoch),
			Confirmed: false,
		}
		voSet, _ := localEntity.voMap[epochKey]
		voSet, err = ep.AddValidatorOperation(voSet, epochKey, vo)
		if err != nil {
			return  err
		}
		localEntity.voMap[epochKey] = voSet

	} else if action == SVM_ACCEPT {

		if vo == nil {
			return errors.New("hash " + hash + " should not exist")
		}

		if  vo.Confirmed {
			epochKey = vo.Epoch
		}

		voSet, ok := localEntity.voMap[epochKey]
		if !ok {
			voSet = ep.LoadValidatorOperationSet(epochKey)
			if voSet == nil {
				return errors.New("voSet should be empty")
			}
		}

		var found bool = false
		validatorVO := voSet.Operations[vo.Validator]
		for i:=0; i<len(validatorVO); i++ {
			if validatorVO[i].TxHash == vo.TxHash {

				for j:=0; j<len(validatorVO[i].VoteSet); j++ {
					if validatorVO[i].VoteSet[j] == fromStr {
						return errors.New("should not re-vote")
					}
				}

				validatorVO[i].VoteSet = append(validatorVO[i].VoteSet, fromStr)
				found = true
				break
			}
		}

		if !found {
			return errors.New("validator not found in epoch")
		}

		localEntity.voMap[epochKey] = voSet
	}

	return nil
}

func svmCommitCb(state BrState, block int) error {

	fmt.Println("svm_CommitCb")

	var err error = nil
	valSet, _, err := state.GetValidators()
	if err != nil {
		return err
	}

	vals := valSet.Validators
	totalPower := int64(0)
	for i:=0; i<len(vals); i++ {
		totalPower += vals[i].VotingPower
	}
	valsMap := toValidatorMap(vals)

	toConfirmedVOList := make([]*ep.ValidatorOperation, 0)
	unConfirmedVOSet, ok := localEntity.voMap[ep.VA_UNCONFIRMED_EPOCH]
	if ok {
		for _, v1 := range unConfirmedVOSet.Operations {
			for i:=0; i<len(v1); i++ {
				vo := v1[i]
				total := int64(0)
				for j:=0; j<len(vo.VoteSet); j++ {
					amount, ok1 := valsMap[vo.VoteSet[j]]
					if ok1 {
						total += amount
					}
				}
				if total > totalPower*2/3 {
					vo.Confirmed = true
					vo.ConfirmedBlock = block
					toConfirmedVOList = append(toConfirmedVOList, vo)
				}
			}
		}
	}

	for i:=0; i<len(toConfirmedVOList); i++ {
		vo := toConfirmedVOList[i]
		ep.RemoveValidatorOperation(unConfirmedVOSet, vo)
		voSet := localEntity.voMap[vo.Epoch]
		voSet, err = ep.AddValidatorOperation(voSet, vo.Epoch, vo)
		if err != nil {
			return err
		}
		localEntity.voMap[vo.Epoch] = voSet
	}

	for _, v := range localEntity.voMap {
		v.Save()
	}

	return nil
}

func svmRefreshABCIResponseCb() error {
	fmt.Println("svm_RefreshABCIResponseCb")
	localEntity.loaded = false
	localEntity.voMap = nil
	return nil
}

func toValidatorMap(vals []*tmTypes.Validator) map[string]int64 {

	if len(vals) == 0 {
		return nil
	}

	result := make(map[string]int64)
	for i:=0; i<len(vals); i++ {
		val := vals[i]
		result[common.ToHex(val.Address)] = val.VotingPower
	}

	return result
}
