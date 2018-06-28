package core

import (
	"math/big"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)


// ApplyTransactionEx attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransactionEx(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB,
			header *types.Header, tx *types.Transaction, usedGas *uint64, totalUsedMoney *big.Int, cfg vm.Config, cch CrossChainHelper) (*types.Receipt, uint64, error) {


	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, err
	}

	etd := tx.ExtendTxData()
	if  etd == nil || etd.FuncName == "" {

		msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
		if err != nil {
			return nil, 0, err
		}
		// Create a new context to be used in the EVM environment
		context := NewEVMContext(msg, header, bc, author)
		// Create a new environment which holds all relevant information
		// about the transaction and calling mechanisms.
		vmenv := vm.NewEVM(context, statedb, config, cfg)
		// Apply the transaction to the current state (included in the env)
		_, gas, money, failed, err := ApplyMessageEx(vmenv, msg, gp)
		if err != nil {
			return nil, 0, err
		}
		// Update the state with pending changes
		var root []byte
		if config.IsByzantium(header.Number) {
			statedb.Finalise(true)
		} else {
			root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
		}
		*usedGas += gas
		totalUsedMoney.Add(totalUsedMoney, money)

		// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
		// based on the eip phase, we're passing wether the root touch-delete accounts.
		receipt := types.NewReceipt(root, failed, *usedGas)
		receipt.TxHash = tx.Hash()
		receipt.GasUsed = gas
		// if the transaction created a contract, store the creation address in the receipt.
		if msg.To() == nil {
			receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
		}
		// Set the receipt logs and create a bloom for filtering
		receipt.Logs = statedb.GetLogs(tx.Hash())
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

		return receipt, gas, err

	} else {

		glog.Infof("ApplyTransactionEx() 0, etd.FuncName is %v\n", etd.FuncName)

		if applyCb := GetApplyCb(etd.FuncName); applyCb != nil {
			cch.GetMutex().Lock()
			defer cch.GetMutex().Unlock()
			if err := applyCb(tx, statedb, cch); err != nil {
				return nil, 0, err
			}
		}

		// Update the state with pending changes
		var root []byte
		if config.IsByzantium(header.Number) {
			statedb.Finalise(true)
		} else {
			root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
		}
		*usedGas += 0
		receipt := types.NewReceipt(root, true, *usedGas)
		receipt.TxHash = tx.Hash()
		receipt.GasUsed = 0

		// Set the receipt logs and create a bloom for filtering
		receipt.Logs = statedb.GetLogs(tx.Hash())
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

		statedb.SetNonce(msg.From(), statedb.GetNonce(msg.From())+1)
		return receipt, 0, nil
	}
}

