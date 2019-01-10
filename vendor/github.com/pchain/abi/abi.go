package abi

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type FunctionType struct {
	id    int
	cross bool
}

var (
	// Cross Chain Function
	CreateChildChain       = FunctionType{0, true}
	JoinChildChain         = FunctionType{1, true}
	DepositInMainChain     = FunctionType{2, true}
	DepositInChildChain    = FunctionType{3, true}
	WithdrawFromChildChain = FunctionType{4, true}
	WithdrawFromMainChain  = FunctionType{5, true}
	SaveDataToMainChain    = FunctionType{6, true}
	// Non-Cross Chain Function
	VoteNextEpoch   = FunctionType{10, false}
	RevealVote      = FunctionType{11, false}
	Delegate        = FunctionType{12, false}
	CancelDelegate  = FunctionType{13, false}
	Candidate       = FunctionType{14, false}
	CancelCandidate = FunctionType{15, false}
	// Unknown
	Unknown = FunctionType{-1, false}
)

func (t FunctionType) IsCrossChainType() bool {
	return t.cross
}

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
	case Delegate, CancelDelegate, Candidate:
		return 21000
	case CancelCandidate:
		return 100000
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
	case Delegate:
		return "Delegate"
	case CancelDelegate:
		return "CancelDelegate"
	case Candidate:
		return "Candidate"
	case CancelCandidate:
		return "CancelCandidate"
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
	case "Delegate":
		return Delegate
	case "CancelDelegate":
		return CancelDelegate
	case "Candidate":
		return Candidate
	case "CancelCandidate":
		return CancelCandidate
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
	PubKey    []byte
	ChainId   string
	Signature []byte
}

type DepositInMainChainArgs struct {
	ChainId string
}

type DepositInChildChainArgs struct {
	ChainId string
	TxHash  common.Hash
}

type WithdrawFromChildChainArgs struct {
	ChainId string
}

type WithdrawFromMainChainArgs struct {
	ChainId string
	Amount  *big.Int
	TxHash  common.Hash
}

type VoteNextEpochArgs struct {
	VoteHash common.Hash
}

type RevealVoteArgs struct {
	PubKey    []byte
	Amount    *big.Int
	Salt      string
	Signature []byte
}

type DelegateArgs struct {
	Candidate common.Address
}

type CancelDelegateArgs struct {
	Candidate common.Address
	Amount    *big.Int
}

type CandidateArgs struct {
	Commission uint8
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
				"type": "bytes"
			},
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "signature",
				"type": "bytes"
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
				"type": "bytes32"
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
				"name": "amount",
				"type": "uint256"
			},
			{
				"name": "txHash",
				"type": "bytes32"
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
				"name": "voteHash",
				"type": "bytes32"
			}
		]
	},
	{
		"type": "function",
		"name": "RevealVote",
		"constant": false,
		"inputs": [
			{
				"name": "pubKey",
				"type": "bytes"
			},
			{
				"name": "amount",
				"type": "uint256"
			},
			{
				"name": "salt",
				"type": "string"
			},
			{
				"name": "signature",
				"type": "bytes"
			}
		]
	},
	{
		"type": "function",
		"name": "Delegate",
		"constant": false,
		"inputs": [
			{
				"name": "candidate",
				"type": "address"
			}
		]
	},
	{
		"type": "function",
		"name": "CancelDelegate",
		"constant": false,
		"inputs": [
			{
				"name": "candidate",
				"type": "address"
			},
			{
				"name": "amount",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "Candidate",
		"constant": false,
		"inputs": [
			{
				"name": "commission",
				"type": "uint8"
			}
		]
	},
	{
		"type": "function",
		"name": "CancelCandidate",
		"constant": false,
		"inputs": []
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
	return addr != nil && *addr == ChainContractMagicAddr
}

func FunctionTypeFromId(sigdata []byte) (FunctionType, error) {
	m, err := ChainABI.MethodById(sigdata)
	if err != nil {
		return Unknown, err
	}

	return StringToFunctionType(m.Name), nil
}
