package core

import (
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessageEx(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, *big.Int, bool, error) {
	return NewStateTransition(evm, msg, gp).TransitionDbEx()
}

// TransitionDbEx will move the state by applying the message against the given environment.
func (st *StateTransition) TransitionDbEx() (ret []byte, usedGas uint64, usedMoney *big.Int, failed bool, err error) {

	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := st.from() // err checked in preCheck

	homestead := st.evm.ChainConfig().IsHomestead(st.evm.BlockNumber)
	contractCreation := msg.To() == nil

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation, homestead)
	if err != nil {
		return nil, 0, nil, false, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, nil, false, err
	}

	var (
		evm = st.evm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)

	//log.Debugf("TransitionDbEx 0\n")

	if contractCreation {
		ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value)
	} else {
		// Increment the nonce for the next transaction
		//log.Debugf("TransitionDbEx 1, sender is %x, nonce is \n", sender.Address(), st.state.GetNonce(sender.Address())+1)

		st.state.SetNonce(sender.Address(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = evm.Call(sender, st.to().Address(), st.data, st.gas, st.value)

		//log.Debugf("TransitionDbEx 2\n")

	}

	//log.Debugf("TransitionDbEx 3\n")

	if vmerr != nil {
		log.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, nil, false, vmerr
		}
	}

	st.refundGas()

	//log.Debugf("TransitionDbEx 4, coinbase is %x, balance is %v\n",
	//	st.evm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	//st.state.AddBalance(st.evm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))
	usedMoney = new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)

	//log.Debugf("TransitionDbEx 5\n")

	//log.Debugf("TransitionDbEx, send.balance is %v\n", st.state.GetBalance(sender.Address()))
	//log.Debugf("TransitionDbEx, return ret-%v, st.gasUsed()-%v, usedMoney-%v, vmerr-%v, err-%v\n",
	//	ret, st.gasUsed(), usedMoney, vmerr, err)

	return ret, st.gasUsed(), usedMoney, vmerr != nil, err
}
