package core

import (
	"fmt"
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
		return nil
	case *types.MarkMainChainToChildChainTxOp:
		return cch.MarkToChildChainTx(op.From, op.ChainId, op.TxHash, false)
	case *types.MarkTxUsedOnChildChainOp:
		return cch.MarkTxUsedOnChildChain(op.From, op.ChainId, op.TxHash)
	case *types.MarkChildChainToMainChainTxUsedOp:
		return cch.MarkFromChildChainTx(op.From, op.ChainId, op.TxHash, true)
	case *types.SaveDataToMainChainOp:
		return cch.SaveChildChainDataToMainChain(op.Data)
	default:
		return fmt.Errorf("unknown op: %v", op)
	}
}
