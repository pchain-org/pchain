package abi

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type FunctionType struct {
	id    int
	cross bool // Tx type, cross chain / non cross chain
	main  bool // allow to be execute on main chain or not
	child bool // allow to be execute on child chain or not
}

var (
	// Cross Chain Function
	CreateChildChain             = FunctionType{0, true, true, false}
	JoinChildChain               = FunctionType{1, true, true, false}
	DepositInMainChain           = FunctionType{2, true, true, false}
	DepositInChildChain          = FunctionType{3, true, false, true}
	WithdrawFromChildChain       = FunctionType{4, true, false, true}
	WithdrawFromMainChain        = FunctionType{5, true, true, false}
	SaveDataToMainChain          = FunctionType{6, true, true, false}
	SetBlockReward               = FunctionType{7, true, false, true}
	CrossChainTransferRequest    = FunctionType{8, true, true, false}
	CrossChainTransferExec       = FunctionType{9, true, false, true}
	// Non-Cross Chain Function
	VoteNextEpoch                = FunctionType{10, false, true, true}
	RevealVote                   = FunctionType{11, false, true, true}
	Delegate                     = FunctionType{12, false, true, true}
	CancelDelegate               = FunctionType{13, false, true, true}
	Candidate                    = FunctionType{14, false, true, true}
	CancelCandidate              = FunctionType{15, false, true, true}
	ExtractReward                = FunctionType{16, false, true, true}
	// Unknown
	Unknown = FunctionType{-1, false, false, false}
)

func (t FunctionType) IsCrossChainType() bool {
	return t.cross
}

func (t FunctionType) AllowInMainChain() bool {
	return t.main
}

func (t FunctionType) AllowInChildChain() bool {
	return t.child
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
	case CrossChainTransferRequest:
		return 42000
	case CrossChainTransferExec:
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
	case SetBlockReward:
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
	case CrossChainTransferRequest:
		return "CrossChainTransferRequest"
	case CrossChainTransferExec:
		return "CrossChainTransferExec"
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
	case SetBlockReward:
		return "SetBlockReward"
	case ExtractReward:
		return "ExtractReward"
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
	case "CrossChainTransferRequest":
		return CrossChainTransferRequest
	case "CrossChainTransferExec":
		return CrossChainTransferExec
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
	case "SetBlockReward":
		return SetBlockReward
	case "ExtractReward":
		return ExtractReward
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

type CrossChainTransferRequestArgs struct {
	FromChainId string
	ToChainId   string
	Amount  *big.Int
}

type CrossChainTransferExecArgs struct {
	MainBlockNumber *big.Int
	TxHash      common.Hash
	Owner       common.Address
	FromChainId string
	ToChainId   string
	Amount      *big.Int
	Status      uint64
	LocalStatus uint64
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

type SetBlockRewardArgs struct {
	ChainId string
	Reward  *big.Int
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
		"name": "CrossChainTransferRequest",
		"constant": false,
		"inputs": [
			{
				"name": "fromChainId",
				"type": "string"
			},
			{
				"name": "toChainId",
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
		"name": "CrossChainTransferExec",
		"constant": false,
		"inputs": [
			{
				"name": "mainBlockNumber",
				"type": "uint256"
			},
			{
				"name": "txHash",
				"type": "bytes32"
			},
			{
				"name": "owner",
				"type": "address"
			},
			{
				"name": "fromChainId",
				"type": "string"
			},
			{
				"name": "toChainId",
				"type": "string"
			},
			{
				"name": "amount",
				"type": "uint256"
			},
			{
				"name": "status",
				"type": "uint64"
			},
			{
				"name": "localStatus",
				"type": "uint64"
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
	},
	{
		"type": "function",
		"name": "SetBlockReward",
		"constant": false,
		"inputs": [
			{
				"name": "chainId",
				"type": "string"
			},
			{
				"name": "reward",
				"type": "uint256"
			}
		]
	},
	{
		"type": "function",
		"name": "ExtractReward",
		"constant": false,
		"inputs": [
			{
				"name": "address",
				"type": "address"
			}
		]
	}
]`

// PChain Child Chain Token Incentive Address
var ChildChainTokenIncentiveAddr = common.BytesToAddress([]byte{100})

// PChain Internal Contract Address
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
