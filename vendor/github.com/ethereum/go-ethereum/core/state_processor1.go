package core

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	pabi "github.com/pchain/abi"
	"math/big"
)

// ApplyTransactionEx attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func applyTransactionEx(msg types.Message, config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, ops *types.PendingOps, header *types.Header,
	tx *types.Transaction, usedGas *uint64, totalUsedMoney *big.Int, cfg vm.Config, cch CrossChainHelper, mining bool, evm *vm.EVM) (*types.Receipt, uint64, error) {

	/*
	// Not allow contract creation on PChain Main Chain
	if config.IsMainChain() && tx.To() == nil {
		return nil, 0, ErrNoContractOnMainChain
	}
	*/
	blockNumber := header.Number
	blockHash := header.Hash()

	if !pabi.IsPChainContractAddr(tx.To()) {

		//log.Debugf("ApplyTransactionEx 1\n")

		// Create a new context to be used in the EVM environment.
		txContext := NewEVMTxContext(msg)
		evm.Reset(txContext, statedb)

		//log.Debugf("ApplyTransactionEx 2\n")

		// Apply the transaction to the current state (included in the env).
		_, gas, money, failed, err := ApplyMessageEx(evm, msg, gp)
		if err != nil {
			return nil, 0, err
		}

		// Update the state with pending changes.
		var root []byte
		if config.IsByzantium(blockNumber) {
			//log.Debugf("ApplyTransactionEx(), is byzantium\n")
			statedb.Finalise(true)
		} else {
			//log.Debugf("ApplyTransactionEx(), is not byzantium\n")
			root = statedb.IntermediateRoot(false).Bytes()
		}
		*usedGas += gas
		totalUsedMoney.Add(totalUsedMoney, money)

		// Create a new receipt for the transaction, storing the intermediate root and gas used
		// by the tx.
		receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
		if failed {
			receipt.Status = types.ReceiptStatusFailed
		} else {
			receipt.Status = types.ReceiptStatusSuccessful
		}
		//log.Debugf("ApplyTransactionEx，new receipt with (root,failed,*usedGas) = (%v,%v,%v)\n", root, failed, *usedGas)
		receipt.TxHash = tx.Hash()
		//log.Debugf("ApplyTransactionEx，new receipt with txhash %v\n", receipt.TxHash)
		receipt.GasUsed = gas
		//log.Debugf("ApplyTransactionEx，new receipt with gas %v\n", receipt.GasUsed)
		// If the transaction created a contract, store the creation address in the receipt.
		if msg.To() == nil {
			receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
		}
		// Set the receipt logs and create the bloom filter.
		receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
		//log.Debugf("ApplyTransactionEx，new receipt with receipt.Logs %v\n", receipt.Logs)
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		receipt.BlockHash = blockHash
		receipt.BlockNumber = blockNumber
		receipt.TransactionIndex = uint(statedb.TxIndex())
		//log.Debugf("ApplyTransactionEx，new receipt with receipt.Bloom %v\n", receipt.Bloom)
		//log.Debugf("ApplyTransactionEx 4\n")
		return receipt, gas, err

	} else {

		// the first 4 bytes is the function identifier
		data := tx.Data()
		function, err := pabi.FunctionTypeFromId(data[:4])
		if err != nil {
			return nil, 0, err
		}
		log.Infof("ApplyTransactionEx() 0, Chain Function is %v\n", function.String())

		// check Function main/child flag
		if config.IsMainChain() && !function.AllowInMainChain() {
			return nil, 0, ErrNotAllowedInMainChain
		} else if !config.IsMainChain() && !function.AllowInChildChain() {
			return nil, 0, ErrNotAllowedInChildChain
		}

		from := msg.From()
		// Make sure this transaction's nonce is correct
		if msg.CheckNonce() {
			nonce := statedb.GetNonce(from)
			if nonce < msg.Nonce() {
				log.Info("ApplyTransactionEx() abort due to nonce too high")
				return nil, 0, ErrNonceTooHigh
			} else if nonce > msg.Nonce() {
				log.Info("ApplyTransactionEx() abort due to nonce too low")
				return nil, 0, ErrNonceTooLow
			}
		}

		// pre-buy gas according to the gas limit
		gasLimit := tx.Gas()
		gasValue := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), tx.GasPrice())
		if statedb.GetBalance(from).Cmp(gasValue) < 0 {
			return nil, 0, fmt.Errorf("insufficient PI for gas (%x). Req %v, has %v", from.Bytes()[:4], gasValue, statedb.GetBalance(from))
		}
		if err := gp.SubGas(gasLimit); err != nil {
			return nil, 0, err
		}
		statedb.SubBalance(from, gasValue)
		log.Infof("ApplyTransactionEx() 1, gas is %v, gasPrice is %v, gasValue is %v\n", gasLimit, tx.GasPrice(), gasValue)

		// use gas
		gas := function.RequiredGas()
		if gasLimit < gas {
			return nil, 0, vm.ErrOutOfGas
		}

		// Check Tx Amount
		if statedb.GetBalance(from).Cmp(tx.Value()) == -1 {
			return nil, 0, fmt.Errorf("insufficient PI for tx amount (%x). Req %v, has %v", from.Bytes()[:4], tx.Value(), statedb.GetBalance(from))
		}

		if applyCb := GetApplyCb(function); applyCb != nil {
			if function.IsCrossChainType() {
				if fn, ok := applyCb.(CrossChainApplyCb); ok {
					cch.GetMutex().Lock()
					err := fn(tx, statedb, ops, cch, mining)
					cch.GetMutex().Unlock()

					if err != nil {
						return nil, 0, err
					}
				} else {
					panic("callback func is wrong, this should not happened, please check the code")
				}
			} else {
				if fn, ok := applyCb.(NonCrossChainApplyCb); ok {
					if err := fn(tx, statedb, bc, ops); err != nil {
						return nil, 0, err
					}
				} else {
					panic("callback func is wrong, this should not happened, please check the code")
				}
			}
		}

		// refund gas
		remainingGas := gasLimit - gas
		remaining := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), tx.GasPrice())
		statedb.AddBalance(from, remaining)
		gp.AddGas(remainingGas)

		*usedGas += gas
		totalUsedMoney.Add(totalUsedMoney, new(big.Int).Mul(new(big.Int).SetUint64(gas), tx.GasPrice()))
		log.Infof("ApplyTransactionEx() 2, totalUsedMoney is %v\n", totalUsedMoney)

		// Update the state with pending changes
		var root []byte
		if config.IsByzantium(blockNumber) {
			statedb.Finalise(true)
		} else {
			root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
		}

		// Create a new receipt for the transaction, storing the intermediate root and gas used
		// by the tx.
		receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
		receipt.Status = types.ReceiptStatusFailed

		//fix receipt status value
		mainBlock := blockNumber
		if !bc.chainConfig.IsMainChain() {
			mainBlock = header.MainChainNumber
		}
		if bc.Config().IsSelfRetrieveReward(mainBlock) {
			receipt.Status = types.ReceiptStatusSuccessful
		}

		receipt.TxHash = tx.Hash()
		receipt.GasUsed = gas

		// Set the receipt logs and create a bloom for filtering
		receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		receipt.BlockHash = blockHash
		receipt.BlockNumber = blockNumber
		receipt.TransactionIndex = uint(statedb.TxIndex())

		statedb.SetNonce(msg.From(), statedb.GetNonce(msg.From())+1)
		log.Infof("ApplyTransactionEx() 3, totalUsedMoney is %v\n", totalUsedMoney)

		return receipt, 0, nil
	}
}

// ApplyTransactionEx attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransactionEx(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB, ops *types.PendingOps, header *types.Header,
	tx *types.Transaction, usedGas *uint64, totalUsedMoney *big.Int, cfg vm.Config, cch CrossChainHelper, mining bool) (*types.Receipt, uint64, error) {

	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number), header.BaseFee)
	if err != nil {
		return nil, 0, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, cfg)
	return applyTransactionEx(msg, config, bc, author, gp, statedb, ops, header, tx, usedGas, totalUsedMoney, 
		cfg, cch, mining, vmenv)
}