package core

import (
	"fmt"
	"github.com/ethereum/go-ethereum/consensus"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core/types"
)

// Consider moving the apply logic to each op (how to avoid import circular reference?)
func ApplyOp(op types.PendingOp, bc *BlockChain, cch CrossChainHelper) error {
	switch op := op.(type) {
	case *types.CreateChildChainOp:
		return cch.CreateChildChain(op.From, op.ChainId, op.MinValidators, op.MinDepositAmount, op.StartBlock, op.EndBlock)
	case *types.JoinChildChainOp:
		return cch.JoinChildChain(op.From, op.PubKey, op.ChainId, op.DepositAmount)
	case *types.LaunchChildChainsOp:
		if len(op.ChildChainIds) > 0 {
			var events []interface{}
			for _, childChainId := range op.ChildChainIds {
				events = append(events, CreateChildChainEvent{ChainId: childChainId})
			}
			bc.PostChainEvents(events, nil)
		}
		if op.NewPendingIdx != nil || len(op.DeleteChildChainIds) > 0 {
			cch.ProcessPostPendingData(op.NewPendingIdx, op.DeleteChildChainIds)
		}
		return nil
	case *types.MarkMainChainToChildChainTxOp:
		return cch.MarkToChildChainTx(op.From, op.ChainId, op.TxHash, false)
	case *types.MarkTxUsedOnChildChainOp:
		return cch.MarkTxUsedOnChildChain(op.From, op.ChainId, op.TxHash)
	case *types.MarkChildChainToMainChainTxUsedOp:
		return cch.MarkFromChildChainTx(op.From, op.ChainId, op.TxHash, true)
	case *types.SaveDataToMainChainOp:
		return cch.SaveChildChainProofDataToMainChain(op.Data)
	case *tmTypes.SwitchEpochOp:
		eng := bc.engine.(consensus.Tendermint)
		nextEp, err := eng.GetEpoch().EnterNewEpoch(op.NewValidators)
		if err == nil {
			eng.SetEpoch(nextEp)
		}
		return err
	default:
		return fmt.Errorf("unknown op: %v", op)
	}
}
