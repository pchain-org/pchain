// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/params"
)

var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)

	Big8  = big8
	Big32 = big32
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig
	bc     *BlockChain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, *big.Int, error) {
	var (
		receipts     types.Receipts
		totalUsedGas = big.NewInt(0)
		err          error
		header       = block.Header()
		allLogs      []*types.Log
		gp           = new(GasPool).AddGas(block.GasLimit())
	)
	glog.Infof("Process() 0, totalUsedGas is %v\n", totalUsedGas)
	// Mutate the the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		ApplyDAOHardFork(statedb)
	}
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		//fmt.Println("tx:", i)
		statedb.StartRecord(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.config, p.bc, gp, statedb, header, tx, totalUsedGas, cfg)
		glog.Infof("Process() 1, totalUsedGas is %v\n", totalUsedGas)
		if err != nil {
			return nil, nil, nil, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	AccumulateRewards(statedb, header, block.Uncles())

	glog.Infof("Process() 2, totalUsedGas is %v\n", totalUsedGas)
	return receipts, allLogs, totalUsedGas, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc *BlockChain, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *big.Int, cfg vm.Config) (*types.Receipt, *big.Int, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, nil, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	// Apply the transaction to the current state (included in the env)
	coinbase := header.Coinbase
	glog.Infof("ApplyTransaction(), coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	glog.Infof("ApplyTransaction() 0, usedGas is %v\n", usedGas)
	_, gas, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, nil, err
	}
	glog.Infof("ApplyTransaction() 1, gas is %v\n", gas)
	// Update the state with pending changes
	usedGas.Add(usedGas, gas)
	glog.Infof("ApplyTransaction() 2, usedGas is %v\n", usedGas)
	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes(), usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = new(big.Int).Set(gas)
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}

	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	glog.V(logger.Debug).Infoln(receipt)
	glog.Infof("ApplyTransaction() 3, usedGas is %v\n", usedGas)
	glog.Infof("ApplyTransaction(), coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	return receipt, gas, err
}

// ApplyTransactionEx attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransactionEx(config *params.ChainConfig, bc *BlockChain, gp *GasPool, statedb *state.StateDB, header *types.Header,
			tx *types.Transaction, usedGas *big.Int, totalUsedMoney *big.Int, cfg vm.Config) (*types.Receipt, *big.Int, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, nil, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	// Apply the transaction to the current state (included in the env)
	coinbase := header.Coinbase
	glog.Infof("ApplyTransactionEx(), coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	glog.Infof("ApplyTransactionEx() 0, totalUsedMoney is %v\n", totalUsedMoney)
	_, gas, money, err := ApplyMessageEx(vmenv, msg, gp)
	if err != nil {
		return nil, nil, err
	}
	glog.Infof("ApplyTransactionEx() 1, money is %v\n", money)
	// Update the state with pending changes
	usedGas.Add(usedGas, gas)
	totalUsedMoney.Add(totalUsedMoney, money)
	glog.Infof("ApplyTransactionEx() 2, totalUsedMoney is %v\n", totalUsedMoney)
	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes(), usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = new(big.Int).Set(gas)
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}

	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	glog.V(logger.Debug).Infoln(receipt)
	glog.Infof("ApplyTransactionEx() 3, totalUsedMoney is %v\n", totalUsedMoney)
	glog.Infof("ApplyTransactionEx(), coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	return receipt, gas, err
}

// AccumulateRewards credits the coinbase of the given block with the
// mining reward. The total reward consists of the static block reward
// and rewards for included uncles. The coinbase of each uncle block is
// also rewarded.
func AccumulateRewards(statedb *state.StateDB, header *types.Header, uncles []*types.Header) {

	reward := new(big.Int).Set(BlockReward)
	coinbase := header.Coinbase
	glog.Infof("AccumulateRewards() 0, coinbase is %x, reward is %v, statedb is %p\n",
			coinbase, reward, statedb)

	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, BlockReward)
		r.Div(r, big8)
		statedb.AddBalance(uncle.Coinbase, r)

		r.Div(BlockReward, big32)
		reward.Add(reward, r)
	}

	glog.Infof("AccumulateRewards() 1, coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
	statedb.AddBalance(header.Coinbase, reward)
	glog.Infof("AccumulateRewards() 2, coinbase is %x, balance is %v\n", coinbase, statedb.GetBalance(coinbase))
}
