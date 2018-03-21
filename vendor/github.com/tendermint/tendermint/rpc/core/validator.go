package core

import (
	cm "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/tendermint/epoch"
)

func CurrentEpochNumber() (*ctypes.ResultUint64, error) {
	return &ctypes.ResultUint64{Value: uint64(consensusState.GetRoundState().Epoch.Number)}, nil
}

func Epoch(number int)  (*ctypes.ResultEpoch, error) {

	curEpoch := consensusState.GetRoundState().Epoch
	if number < 0 || number > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}

	if number == curEpoch.Number {
		return &ctypes.ResultEpoch{Epoch: curEpoch.MakeOneEpochDoc()}, nil
	}

	spEpoch := epoch.LoadOneEpoch(curEpoch.GetDB(), number)

	return &ctypes.ResultEpoch{Epoch: spEpoch.MakeOneEpochDoc()}, nil
}

func Validators() (*ctypes.ResultValidators, error) {
	blockHeight, validators := consensusState.GetValidators()
	return &ctypes.ResultValidators{blockHeight, validators}, nil
}

func ValidatorOperation(from string, epoch int, power uint64, action string, target string, sig []byte) (*ctypes.ResultValidatorOperation, error) {

	fmt.Println("in func ValidatorOperation(s string) (*ctypes.ResultValidatorOperation, error)")

	data := fmt.Sprintf("%s-%X-%X-%s-%s", from, epoch, power, action, target)
	fmt.Printf("in func ValidatorOperation(), data to verify is: %v, sig is: %X\n", data, sig)

	if len(sig) != 65 {
		return &ctypes.ResultValidatorOperation{}, fmt.Errorf("signature must be 65 bytes long")
	}
	if sig[64] != 27 && sig[64] != 28 {
		return &ctypes.ResultValidatorOperation{}, fmt.Errorf("invalid Ethereum signature (V is not 27 or 28)")
	}
	sig[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	rpk, err := crypto.Ecrecover(signHash([]byte(data)), sig)
	if err != nil {
		return &ctypes.ResultValidatorOperation{}, err
	}

	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	fmt.Printf("in func ValidatorOperation(), recovered address is %s, address is : %s\n", recoveredAddr.Hex(), from)

	if recoveredAddr != common.HexToAddress(from) {
		return &ctypes.ResultValidatorOperation{}, fmt.Errorf("recovered address is not the address send the message")
	}

	key := fmt.Sprintf("%X", (crypto.FromECDSAPub(pubKey)))
	fmt.Printf("in func ValidatorOperation(), recovered address is %s, key is : %s\n", recoveredAddr.Hex(), key)

	//check epoch
	if epoch <= consensusState.GetRoundState().Epoch.Number {
		return &ctypes.ResultValidatorOperation{}, errors.New("epoch should be bigger than current epoch number")
	}

	/*
	if EthApi.getBalance(from) < power {
		return &ctypes.ResultValidatorOperation{}, errors.New("no enough balance")
		EthApi.SubBalance(from, power)
		EthApi.AddDeposit(from, power)
	}
	*/

	cm.SendValidatorMsgToCons(from, key, epoch, power, action, target)
	return &ctypes.ResultValidatorOperation{
		From: from,
		Epoch: epoch,
		Power: power,
		Action: action,
		Target: target,
	}, nil
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

func signHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}
