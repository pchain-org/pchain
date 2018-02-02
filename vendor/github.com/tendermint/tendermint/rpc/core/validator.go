package core

import (
	cm "github.com/tendermint/tendermint/consensus"
	types "github.com/tendermint/tendermint/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb/errors"
)


func Validators() (*ctypes.ResultValidators, error) {
	blockHeight, validators := consensusState.GetValidators()
	return &ctypes.ResultValidators{blockHeight, validators}, nil
}

func ValidatorOperation(epoch int, key string, power uint64, flag string) (*ctypes.ResultValidatorOperation, error) {
	//fmt.Println("in func ValidatorOperation(s string) (*ctypes.ResultValidatorOperation, error)")
	cm.SendValidatorMsgToCons(epoch, key, power, flag)
	return &ctypes.ResultValidatorOperation{}, nil
}

func ValidatorEpoch(address string) (*ctypes.ResultValidatorEpoch, error) {
	//fmt.Println("in func ValidatorEpoch(address string) (*ctypes.ResultValidatorEpoch, error)")

	blockHeight, validators := consensusState.GetValidators()
	//fmt.Printf("in func GetValidators returns: %v\n", validators)
	//fmt.Printf("in func GetValidators address is : %v\n", address)

	var result *ctypes.ResultValidatorEpoch = nil
	var err error = nil

	if address == "all" {

		//fmt.Println("in func GetValidators 0")
		valWithOperations := make([]*ctypes.ResultValidatorEpochValidator, len(validators))

		for i, v := range validators {

			valWithOperations[i] = &ctypes.ResultValidatorEpochValidator {
				Validator: v,
				Operation: nil,
			}

			acceptVotes := types.AcceptVoteSet[v.PubKey.KeyString()]
			if acceptVotes != nil {
				valWithOperations[i].Operation = &ctypes.ResultValidatorOperationSimple {
					Epoch: acceptVotes.Epoch,
					Operation: acceptVotes.Action,
					Amount: acceptVotes.Power,
				}
			}
		}

		result = &ctypes.ResultValidatorEpoch{
			BlockHeight: blockHeight,
			Validators: valWithOperations,
			Epoch: consensusState.GetRoundState().Epoch,
		}

	} else {
		//fmt.Printf("in func GetValidators 1\n")
		found := false
		for i, v := range validators {

			//fmt.Printf("in func GetValidators 2, address is:%v, v.Address is %X\n", address, string(v.Address))
			if address == fmt.Sprintf("%X", v.Address) {
				valWithOperations := make([]*ctypes.ResultValidatorEpochValidator, 1)
				valWithOperations[0] = &ctypes.ResultValidatorEpochValidator {
					Validator: v,
					Operation: nil,
				}

				acceptVotes := types.AcceptVoteSet[v.PubKey.KeyString()]
				if acceptVotes != nil {
					valWithOperations[i].Operation = &ctypes.ResultValidatorOperationSimple {
						Epoch: acceptVotes.Epoch,
						Operation: acceptVotes.Action,
						Amount: acceptVotes.Power,
					}
				}

				result = &ctypes.ResultValidatorEpoch{
					BlockHeight: blockHeight,
					Validators: valWithOperations,
					Epoch: consensusState.GetRoundState().Epoch,
				}
				found = true
				break
			}
		}

		if !found {
			err = errors.New("no validator found")
		}
	}

	return result, err
}
