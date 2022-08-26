// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var esfAddrBy = common.FromHex("0xd8743bCaa75EE61873Bd71A53BE64804AE25f34e")

const MITLength = 4
type MethodIDType [MITLength]byte
func BytesToID(b []byte) MethodIDType {
	var a MethodIDType
	if len(b) > len(a) {
		b = b[len(b)-MITLength:]
	}
	copy(a[MITLength-len(b):], b)
	return a
}


//#####################################################
//######### main precompiledcontract part #############
//#####################################################
type extendSolFunctions struct{}

func (c *extendSolFunctions) RequiredGas(input []byte) uint64 {

	if len(input) < 4 {return 0}

	pc, exist := esfMap[BytesToID(input[:4])]
	if exist {
		return pc.RequiredGas(input)
	}
	return 0
}

func (c *extendSolFunctions) Run(input []byte) ([]byte, error) {

	if len(input) < 4 {return nil, errors.New("wrong input data")}

	pc, exist := esfMap[BytesToID(input[:4])]
	if exist {
		return pc.Run(input)
	}
	return nil, errors.New("not support yet")
}

//#####################################################
//################## init part ########################
//#####################################################
var esfABI abi.ABI
var esfMap map[MethodIDType]PrecompiledContract

func init() {
	var err error
	esfABI, err = abi.JSON(strings.NewReader(jsonExtendSolFunctionsABI))
	if err != nil {
		panic("fail to create the extendSolFunctions ABI: " + err.Error())
	}

	esfMap = make(map[MethodIDType]PrecompiledContract)
	for name, method := range esfABI.Methods {
		var pc PrecompiledContract
		switch name {
		case "callKeccak256":      pc = &callKeccak256{}
		case "generateRandomness": pc = &generateRandomness{}
		case "numberAdd":          pc = &numberAdd{}
		case "stringAdd":          pc = &stringAdd{}
		case "bytes32Add":         pc = &bytes32Add{}
		case "mixAdd":             pc = &mixAdd{}
		}
		esfMap[BytesToID(method.ID()[:4])] = pc
	}
}

const jsonExtendSolFunctionsABI = `
[
	{
		"type": "function",
		"name": "callKeccak256",
		"constant": false,
		"inputs": [
			{
				"name": "input",
				"type": "string"
			}
		]
	},
	{
		"type": "function",
		"name": "generateRandomness",
		"constant": false,
		"inputs": [
			{
				"name": "seed",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "numberAdd",
		"constant": false,
		"inputs": [
			{
				"name": "num",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "stringAdd",
		"constant": false,
		"inputs": [
			{
				"name": "str",
				"type": "string"
			}
		]
	},
	{
		"type": "function",
		"name": "bytes32Add",
		"constant": false,
		"inputs": [
			{
				"name": "by",
				"type": "bytes32"
			}
		]
	},
	{
		"type": "function",
		"name": "mixAdd",
		"constant": false,
		"inputs": [
			{
				"name": "str",
				"type": "string"
			},
			{
				"name": "num",
				"type": "uint256"
			}
		]
	}
]`


//#####################################################
//########## sub-precompiledcontract part #############
//#####################################################
type callKeccak256 struct{}

type callKeccak256Args struct {
	Input   string	//!! first letter of the field name should be Capital !!
}

func (c *callKeccak256) RequiredGas(input []byte) uint64 {
	return params.EcrecoverGas + uint64(len(input[:4]))*params.Sha3WordGas
}

func (c *callKeccak256) Run(input []byte) ([]byte, error) {
	var args callKeccak256Args
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in callKeccak256")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	return crypto.Keccak256([]byte(args.Input)), nil
}


type generateRandomness struct{}

type generateRandomnessArgs struct {
	Seed   *big.Int	//!! first letter of the field name should be Capital !!
}

const RandomGas uint64=50
func (c *generateRandomness) RequiredGas(input []byte) uint64 {
	return RandomGas
}

func (c *generateRandomness) Run(input []byte) ([]byte, error) {
	var args generateRandomnessArgs
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in callKeccak256")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	sum := new(big.Int).Add(args.Seed, big.NewInt(1))

	return common.LeftPadBytes(sum.Bytes(), 32), nil
}


type numberAdd struct{}

type numberAddArgs struct {
	Num   *big.Int	//!! first letter of the field name should be Capital !!
}

const AddGas uint64=1
func (c *numberAdd) RequiredGas(input []byte) uint64 {
	return AddGas
}

func (c *numberAdd) Run(input []byte) ([]byte, error) {
	var args numberAddArgs
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in callKeccak256")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	sum := new(big.Int).Add(args.Num, big.NewInt(1))

	return common.LeftPadBytes(sum.Bytes(), 32), nil
}


type stringAdd struct{}

type stringAddArgs struct {
	Str   string	//!! first letter of the field name should be Capital !!
}

const StrAddGas uint64=1
func (c *stringAdd) RequiredGas(input []byte) uint64 {
	return StrAddGas
}

func (c *stringAdd) Run(input []byte) ([]byte, error) {
	var args stringAddArgs
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in stringAdd")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	sum := args.Str + "a"

	stringType, _ := abi.NewType("string", "", nil)
	retArgs := abi.Arguments{abi.Argument{
		Name:    "string",
		Type:    stringType,
		Indexed: false,
	}}

	return retArgs.Pack(sum)
}


type bytes32Add struct{}

type bytes32AddArgs struct {
	By   [32]byte	//!! first letter of the field name should be Capital !!
}

func (c *bytes32Add) RequiredGas(input []byte) uint64 {
	return AddGas
}

func (c *bytes32Add) Run(input []byte) ([]byte, error) {
	var args bytes32AddArgs
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in stringAdd")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	num := new(big.Int).SetBytes(args.By[:])
	sum := new(big.Int).Add(num, big.NewInt(1))

	return common.LeftPadBytes(sum.Bytes(), 32), nil
}


type mixAdd struct{}

type mixAddArgs struct {
	Str   string
	Num   *big.Int	//!! first letter of the field name should be Capital !!
}

func (c *mixAdd) RequiredGas(input []byte) uint64 {
	return AddGas+StrAddGas
}

func (c *mixAdd) Run(input []byte) ([]byte, error) {
	var args mixAddArgs
	method, err := esfABI.MethodById(input[:4])
	if err != nil {
		return nil, errors.New("should not happen in stringAdd")
	}
	if err = esfABI.UnpackMethodInputs(&args, method.Name, input[4:]); err != nil {
		return nil, err
	}

	sum := new(big.Int).Add(args.Num, big.NewInt(1))
	StrSum := args.Str + sum.String()

	stringType, _ := abi.NewType("string", "", nil)
	retArgs := abi.Arguments{abi.Argument{
		Name:    "string",
		Type:    stringType,
		Indexed: false,
	}}

	return retArgs.Pack(StrSum)
}
