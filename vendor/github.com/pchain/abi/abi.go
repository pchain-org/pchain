package abi

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type FunctionType int

const (
	CreateChildChain FunctionType = iota
	JoinChildChain
	DepositInMainChain
	DepositInChildChain
	WithdrawFromChildChain
	WithdrawFromMainChain
	SaveDataToMainChain
	VoteNextEpoch
	RevealVote
	// Unknown
	Unknown
)

func (t FunctionType) RequiredGas() uint64 {
	switch t {
	case CreateChildChain:
		return 42000
	case JoinChildChain:
		return 21000
	case DepositInMainChain:
		return 42000
	case DepositInChildChain:
		return 0
	case WithdrawFromChildChain:
		return 42000
	case WithdrawFromMainChain:
		return 0
	case SaveDataToMainChain:
		return 0
	case VoteNextEpoch:
		return 21000
	case RevealVote:
		return 21000
	default:
		return 0
	}
}

func (t FunctionType) String() string {
	switch t {
	case CreateChildChain:
		return "CreateChildChain"
	case JoinChildChain:
		return "JoinChildChain"
	case DepositInMainChain:
		return "DepositInMainChain"
	case DepositInChildChain:
		return "DepositInChildChain"
	case WithdrawFromChildChain:
		return "WithdrawFromChildChain"
	case WithdrawFromMainChain:
		return "WithdrawFromMainChain"
	case SaveDataToMainChain:
		return "SaveDataToMainChain"
	case VoteNextEpoch:
		return "VoteNextEpoch"
	case RevealVote:
		return "RevealVote"
	default:
		return "UnKnown"
	}
}

func StringToFunctionType(s string) FunctionType {
	switch s {
	case "CreateChildChain":
		return CreateChildChain
	case "JoinChildChain":
		return JoinChildChain
	case "DepositInMainChain":
		return DepositInMainChain
	case "DepositInChildChain":
		return DepositInChildChain
	case "WithdrawFromChildChain":
		return WithdrawFromChildChain
	case "WithdrawFromMainChain":
		return WithdrawFromMainChain
	case "SaveDataToMainChain":
		return SaveDataToMainChain
	case "VoteNextEpoch":
		return VoteNextEpoch
	case "RevealVote":
		return RevealVote
	default:
		return Unknown
	}
}

type CreateChildChainArgs struct {
	ChainId          string
	MinValidators    uint16
	MinDepositAmount *big.Int
	StartBlock       *big.Int
	EndBlock         *big.Int
}

type JoinChildChainArgs struct {
	PubKey        string
	ChainId       string
	DepositAmount *big.Int
}

type DepositInMainChainArgs struct {
	ChainId string
	Amount  *big.Int
}

type DepositInChildChainArgs struct {
	ChainId string
	TxHash  common.Hash
}

type WithdrawFromChildChainArgs struct {
	ChainId string
	Amount  *big.Int
}

type WithdrawFromMainChainArgs struct {
	ChainId string
	TxHash  common.Hash
}

type VoteNextEpochArgs struct {
	ChainId  string
	VoteHash common.Hash
}

type RevealVoteArgs struct {
	ChainId string
	PubKey  string
	Amount  *big.Int
	Salt    string
}

const jsonChainABI = `
[
	{
		"type": "function",
		"name": "CreateChildChain",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "minValidators",
				"type": "uint16"
			},
			{
				"name": "minDepositAmount",
				"type": "uint256"
			},
			{
				"name": "startBlock",
				"type": "uint256"
			},
			{
				"name": "endBlock",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "JoinChildChain",
		"constant": false,
		"inputs": [
			{
				"name": "pubKey",
				"type": "string"
			},
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "depositAmount",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "DepositInMainChain",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "amount",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "DepositInChildChain",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "txHash",
				"type": "hash"
			}
		]
	},
	{
		"type": "function",
		"name": "WithdrawFromChildChain",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "amount",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "WithdrawFromMainChain",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "txHash",
				"type": "hash"
			}
		]
	},
	{
		"type": "function",
		"name": "SaveDataToMainChain",
		"constant": false,
		"inputs": [
			{
				"name": "data",
				"type": "bytes"
			}
		]
	},
	{
		"type": "function",
		"name": "VoteNextEpoch",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "voteHash",
				"type": "hash"
			}
		]
	},
	{
		"type": "function",
		"name": "RevealVote",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "pubKey",
				"type": "string"
			},
			{
				"name": "amount",
				"type": "uint256"
			},
			{
				"name": "salt",
				"type": "string"
			}
		]
	}
]`

var ChainContractMagicAddr = common.BytesToAddress([]byte{101}) // don't conflict with go-ethereum/core/vm/contracts.go

var ChainABI abi.ABI

func init() {
	var err error
	ChainABI, err = abi.JSON(strings.NewReader(jsonChainABI))
	if err != nil {
		panic("fail to create the chain ABI: " + err.Error())
	}
}

func IsPChainContractAddr(addr *common.Address) bool {
	return *addr == ChainContractMagicAddr
}

func FunctionTypeFromId(sigdata []byte) (FunctionType, error) {
	m, err := ChainABI.MethodById(sigdata)
	if err != nil {
		return Unknown, err
	}

	return StringToFunctionType(m.Name), nil
}
