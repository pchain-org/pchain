package types

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// PendingOps tracks the operations(except balance related stuff since it's tracked in statedb) that need to be applied after consensus achieved.
type PendingOps struct {
	ops []PendingOp
}

func (pending *PendingOps) Append(op1 PendingOp) bool {
	for _, op := range pending.ops {
		if op.Conflict(op1) {
			return false
		}
	}
	pending.ops = append(pending.ops, op1)
	return true
}

func (pending *PendingOps) Ops() []PendingOp {
	ret := make([]PendingOp, len(pending.ops))
	copy(ret, pending.ops)
	return ret
}

type PendingOp interface {
	Conflict(op PendingOp) bool
	String() string
}

// CreateChildChain op
type CreateChildChainOp struct {
	From             common.Address
	ChainId          string
	MinValidators    uint16
	MinDepositAmount *big.Int
	StartBlock       *big.Int
	EndBlock         *big.Int
}

func (op *CreateChildChainOp) Conflict(op1 PendingOp) bool {
	if op1, ok := op1.(*CreateChildChainOp); ok {
		return op.ChainId == op1.ChainId
	}
	return false
}

func (op *CreateChildChainOp) String() string {
	return fmt.Sprintf("CreateChildChainOp - From: %x, ChainId: %s, MinValidators: %d, MinDepositAmount: %x, StartBlock: %x, EndBlock: %x",
		op.From, op.ChainId, op.MinValidators, op.MinDepositAmount, op.StartBlock, op.EndBlock)
}

// JoinChildChain op
type JoinChildChainOp struct {
	From          common.Address
	PubKey        string
	ChainId       string
	DepositAmount *big.Int
}

func (op *JoinChildChainOp) Conflict(op1 PendingOp) bool {
	if op1, ok := op1.(*JoinChildChainOp); ok {
		return op.ChainId == op1.ChainId && op.From == op1.From
	}
	return false
}

func (op *JoinChildChainOp) String() string {
	return fmt.Sprintf("JoinChildChainOp - From: %x, PubKey: %s, ChainId: %s, DepositAmount: %x",
		op.From, op.PubKey, op.ChainId, op.DepositAmount)
}

// LaunchChildChain op
type LaunchChildChainsOp struct {
	ChildChainIds       []string
	NewPendingIdx       []byte
	DeleteChildChainIds []string
}

func (op *LaunchChildChainsOp) Conflict(op1 PendingOp) bool {
	if _, ok := op1.(*LaunchChildChainsOp); ok {
		// Only one LaunchChildChainsOp is allowed in each block
		return true
	}
	return false
}

func (op *LaunchChildChainsOp) String() string {
	return fmt.Sprintf("LaunchChildChainsOp - Launch Child Chain: %v, New Pending Child Chain Length: %v, To be deleted Child Chain: %v",
		op.ChildChainIds, len(op.NewPendingIdx), op.DeleteChildChainIds)
}

// CrossChainTx
type CrossChainTx struct {
	From    common.Address
	ChainId string
	TxHash  common.Hash
}

func (cct *CrossChainTx) Equal(cct1 *CrossChainTx) bool {
	if cct == nil && cct1 == nil {
		return true
	}

	if cct == nil || cct1 == nil {
		return false
	}

	return cct.From == cct1.From && cct.ChainId == cct1.ChainId && cct.TxHash == cct1.TxHash
}

// MarkChildChainToMainChainTxUsed op
type MarkChildChainToMainChainTxUsedOp struct {
	CrossChainTx
}

func (op *MarkChildChainToMainChainTxUsedOp) Conflict(op1 PendingOp) bool {
	if op1, ok := op1.(*MarkChildChainToMainChainTxUsedOp); ok {
		return op.CrossChainTx.Equal(&op1.CrossChainTx)
	}
	return false
}

func (op *MarkChildChainToMainChainTxUsedOp) String() string {
	return fmt.Sprintf("MarkChildChainToMainChainTxUsedOp - From: %x, ChainId: %s, TxHash: %x",
		op.From, op.ChainId, op.TxHash)
}

// SaveBlockToMainChain op
type SaveDataToMainChainOp struct {
	Data []byte
}

func (op *SaveDataToMainChainOp) Conflict(op1 PendingOp) bool {
	return false
}

func (op *SaveDataToMainChainOp) String() string {
	return fmt.Sprintf("SaveDataToMainChainOp")
}

// VoteNextEpoch op
type VoteNextEpochOp struct {
	From     common.Address
	VoteHash common.Hash
	TxHash   common.Hash
}

func (op *VoteNextEpochOp) Conflict(op1 PendingOp) bool {
	return false
}

func (op *VoteNextEpochOp) String() string {
	return fmt.Sprintf("VoteNextEpoch")
}

// RevealVote op
type RevealVoteOp struct {
	From   common.Address
	Pubkey string
	Amount *big.Int
	Salt   string
	TxHash common.Hash
}

func (op *RevealVoteOp) Conflict(op1 PendingOp) bool {
	return false
}

func (op *RevealVoteOp) String() string {
	return fmt.Sprintf("RevealVote")
}
