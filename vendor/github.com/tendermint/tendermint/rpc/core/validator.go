package core

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/syndtr/goleveldb/leveldb/errors"
	cm "github.com/tendermint/tendermint/consensus"
	ep "github.com/tendermint/tendermint/epoch"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"math/big"
)

func CurrentEpochNumber(context *RPCDataContext) (*ctypes.ResultUint64, error) {
	return &ctypes.ResultUint64{Value: uint64(context.consensusState.GetRoundState().Epoch.Number)}, nil
}

func Epoch(context *RPCDataContext, number int) (*ctypes.ResultEpoch, error) {

	var resultEpoch *ep.Epoch
	curEpoch := context.consensusState.GetRoundState().Epoch
	if number < 0 || number > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}

	if number == curEpoch.Number {
		resultEpoch = curEpoch
		//return &ctypes.ResultEpoch{Epoch: curEpoch.MakeOneEpochDoc()}, nil
	} else {
		resultEpoch = ep.LoadOneEpoch(curEpoch.GetDB(), number)
	}

	validators := make([]types.GenesisValidator, len(resultEpoch.Validators.Validators))
	for i, val := range resultEpoch.Validators.Validators {
		validators[i] = types.GenesisValidator{
			EthAccount: common.BytesToAddress(val.Address),
			PubKey:     val.PubKey,
			Amount:     val.VotingPower,
			Name:       "",
		}
	}

	return &ctypes.ResultEpoch{
		resultEpoch.Number,
		resultEpoch.RewardPerBlock,
		resultEpoch.StartBlock,
		resultEpoch.EndBlock,
		resultEpoch.StartTime,
		resultEpoch.EndTime,
		resultEpoch.GetVoteStartHeight(),
		resultEpoch.GetVoteEndHeight(),
		resultEpoch.GetRevealVoteStartHeight(),
		resultEpoch.GetRevealVoteEndHeight(),
		resultEpoch.BlockGenerated,
		resultEpoch.Status,
		validators,
	}, nil
}

func Validators(context *RPCDataContext) (*ctypes.ResultValidators, error) {
	blockHeight, validators := context.consensusState.GetValidators()
	return &ctypes.ResultValidators{blockHeight, validators}, nil
}

func EpochVotes(context *RPCDataContext) (*ctypes.ResultEpochVotes, error) {
	ep := context.consensusState.GetRoundState().Epoch
	if ep.GetNextEpoch() != nil {
		return &ctypes.ResultEpochVotes{
			ep.GetNextEpoch().Number,
			ep.GetNextEpoch().StartBlock,
			ep.GetNextEpoch().EndBlock,
			ep.GetNextEpoch().GetEpochValidatorVoteSet().Votes,
		}, nil
	}
	return nil, errors.New("next epoch has not been proposed")
}

// Deprecated
func ValidatorOperation(context *RPCDataContext, from string, epoch int, power uint64, action string, target string, sig []byte) (*ctypes.ResultValidatorOperation, error) {

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
	if epoch <= context.consensusState.GetRoundState().Epoch.Number {
		return &ctypes.ResultValidatorOperation{}, errors.New("epoch should be bigger than current epoch number")
	}

	/*
		if EthApi.getBalance(from) < power {
			return &ctypes.ResultValidatorOperation{}, errors.New("no enough balance")
			EthApi.SubBalance(from, power)
			EthApi.AddDeposit(from, power)
		}
	*/
	cm.SendValidatorMsgToCons(from, key, epoch, new(big.Int).SetUint64(power), action, target)
	return &ctypes.ResultValidatorOperation{
		From:   from,
		Epoch:  epoch,
		Power:  power,
		Action: action,
		Target: target,
	}, nil
}

func ValidatorEpoch(context *RPCDataContext, address string, epoch int) (*ctypes.ResultValidatorEpoch, error) {

	//fmt.Println("in func ValidatorEpoch(address string) (*ctypes.ResultValidatorEpoch, error)")
	curEpoch := context.consensusState.GetRoundState().Epoch
	if epoch < 0 || epoch > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}

	vals := []*types.Validator{}

	if epoch == curEpoch.Number {
		vals = curEpoch.Validators.Validators
	}

	spEpoch := ep.LoadOneEpoch(curEpoch.GetDB(), epoch)
	vals = spEpoch.Validators.Validators

	var validator *types.Validator = nil
	for i := 0; i < len(vals); i++ {
		fmt.Println("in func ValidatorEpoch(), string is %s, address is %s",
			address, fmt.Sprintf("%x", vals[i].Address))
		if address == fmt.Sprintf("%x", vals[i].Address) {
			validator = vals[i]
			break
		}
	}

	return &ctypes.ResultValidatorEpoch{
		EpochNumber: epoch,
		Validator: &types.GenesisValidator{
			EthAccount: common.BytesToAddress(validator.Address),
			PubKey:     validator.PubKey,
			Amount:     validator.VotingPower,
			Name:       "",
		},
	}, nil
	/*
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
	*/
}

// Deprecated
func UnconfirmedValidatorsOperation(context *RPCDataContext) (*ctypes.ResultValidatorsOperation, error) {
	return getValidatorsOperation(context, ep.VA_UNCONFIRMED_EPOCH)
}

// Deprecated
func ConfirmedValidatorsOperation(context *RPCDataContext, epoch int) (*ctypes.ResultValidatorsOperation, error) {
	return getValidatorsOperation(context, epoch)
}

// Deprecated
func getValidatorsOperation(context *RPCDataContext, epoch int) (*ctypes.ResultValidatorsOperation, error) {

	curEpoch := context.consensusState.GetRoundState().Epoch
	if epoch < ep.VA_UNCONFIRMED_EPOCH || epoch > curEpoch.Number {
		return nil, errors.New("epoch number out of range")
	}
	voSet := ep.LoadValidatorOperationSet(epoch)

	resultVoArr := make([]*ep.ValidatorOperation, 0)
	if voSet != nil {
		for _, voArr := range voSet.Operations {
			resultVoArr = append(resultVoArr, voArr...)
		}
	}

	return &ctypes.ResultValidatorsOperation{VOArray: resultVoArr}, nil
}

// Deprecated
func signHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}
