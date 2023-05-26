package core

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	pabi "github.com/pchain/abi"
)


type chainEnhPrecompileInputPacket struct {
	tx *types.Transaction
	bc *BlockChain
	statedb *state.StateDB
	ops *types.PendingOps
	header *types.Header
	cch CrossChainHelper
	mining bool
}

type chainEnhPrecompile struct{}
func (*chainEnhPrecompile) RequiredGas(input interface{}) uint64 {

	inputPacket, ok := input.(chainEnhPrecompileInputPacket)
	if !ok {
		return 0
	}

	data := inputPacket.tx.Data()
	function, err := pabi.FunctionTypeFromId(data[:4])
	if err != nil {
		return 0
	}

	return function.RequiredGas()
}

func (*chainEnhPrecompile) Run(input interface{}) ([]byte, error) {

	inputPacket, ok := input.(chainEnhPrecompileInputPacket)
	if !ok {
		return nil, errors.New("invalid input format")
	}

	tx := inputPacket.tx
	bc := inputPacket.bc
	statedb := inputPacket.statedb
	ops := inputPacket.ops
	header := inputPacket.header
	cch := inputPacket.cch
	mining := inputPacket.mining

	data := tx.Data()
	function, err := pabi.FunctionTypeFromId(data[:4])
	if err != nil {
		return nil, err
	}

	if applyCb := GetApplyCb(function); applyCb != nil {
		if function.IsCrossChainType() {
			if fn, ok := applyCb.(CrossChainApplyCb); ok {
				cch.GetMutex().Lock()
				err := fn(tx, statedb, bc, header, ops, cch, mining)
				cch.GetMutex().Unlock()

				if err != nil {
					return nil, err
				}
			} else {
				panic("callback func is wrong, this should not happened, please check the code")
			}
		} else {
			if fn, ok := applyCb.(NonCrossChainApplyCb); ok {
				if err := fn(tx, statedb, bc, header, ops); err != nil {
					return nil, err
				}
			} else {
				panic("callback func is wrong, this should not happened, please check the code")
			}
		}
		return nil, nil
	}

	return nil, errors.New("apply_callback not found")
}

func init() {
	vm.SetEndhancedPrecompile(pabi.ChainContractMagicAddr, &chainEnhPrecompile{})
}
