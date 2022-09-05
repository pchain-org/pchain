package vm

import (
	"github.com/ethereum/go-ethereum/common"
)

type EnhancedPrecompiledContract interface {
	RequiredGas(input interface{}) uint64  // RequiredPrice calculates the contract gas use
	Run(input interface{}) ([]byte, error)           // Run runs the precompiled contract
}

var enhancedPrecompilesContracts map[common.Address]EnhancedPrecompiledContract

func SetEndhancedPrecompile(addr common.Address, pEnh EnhancedPrecompiledContract) {
	if enhancedPrecompilesContracts == nil {
		enhancedPrecompilesContracts = make(map[common.Address]EnhancedPrecompiledContract)
	}
	enhancedPrecompilesContracts[addr] = pEnh
}

func RunEnhancedPrecompiledContract(p EnhancedPrecompiledContract, input interface{}, suppliedGas uint64) (ret []byte, remainingGas uint64, err error) {
	gasCost := p.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, ErrOutOfGas
	}
	suppliedGas -= gasCost
	output, err := p.Run(input)
	return output, suppliedGas, err
}
