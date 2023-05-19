// Copyright 2014 The go-ethereum Authors
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
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")

	// ErrKnownBlock is returned when a block to import is already known locally.
	ErrKnownBlock = errors.New("block already known")

	// ErrGasLimitReached is returned by the gas pool if the amount of gas required
	// by a transaction is higher than what's left in the block.
	ErrGasLimitReached = errors.New("gas limit reached")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrBlacklistedHash is returned if a block to import is on the blacklist.
	ErrBlacklistedHash = errors.New("blacklisted hash")

	// ErrNoContractOnMainChain is returned if the contract creation tx has been submit to PChain main chain
	ErrNoContractOnMainChain = errors.New("no contract creation on main chain")

	// ErrInvalidTx4 is returned if the tx4 has been checked during execution
	ErrInvalidTx4 = errors.New("invalid Tx4")

	// Delegation Error
	// ErrCancelSelfDelegate is returned if the cancel delegate apply to the self address
	ErrCancelSelfDelegate = errors.New("can not cancel self delegation")

	// ErrCannotDelegate is returned if the request address does not have deposit balance in Annual/SemiAnnual Supernode
	ErrCannotDelegate = errors.New("Annual/SemiAnnual Supernode candidate not accept new delegator")

	// ErrCannotCancelDelegate is returned if the request address belongs to Annual/SemiAnnual Supernode
	ErrCannotCancelDelegate = errors.New("Annual/SemiAnnual Supernode candidate can not cancel delegation")

	// ErrDelegateAmount is returned if the delegate amount less than 0
	ErrDelegateAmount = errors.New("delegation amount must be greater or equal to 1000 PI")

	// ErrInsufficientProxiedBalance is returned if the cancellation amount of executing a transaction
	// is higher than the proxied balance of the user's account.
	ErrInsufficientProxiedBalance = errors.New("cancel amount greater than your Proxied Balance")

	// ErrAlreadyCandidate is returned if the request address has become candidate already
	ErrAlreadyCandidate = errors.New("address become candidate already")

	// ErrCannotCandidate is returned if the request address belongs to Annual/SemiAnnual Supernode
	ErrCannotCandidate = errors.New("Annual/SemiAnnual Supernode can not become candidate")

	// ErrCannotCancelCandidate is returned if the request address belongs to Annual/SemiAnnual Supernode
	ErrCannotCancelCandidate = errors.New("Annual/SemiAnnual Supernode can not cancel candidate")

	// ErrNotCandidate is returned if the request address is not a candidate
	ErrNotCandidate = errors.New("address not candidate")

	//ErrExceedDelegationAddressLimit is returned if delegated address number exceed the limit
	ErrExceedDelegationAddressLimit = errors.New("exceed the delegation address limit")

	// ErrMinimumSecurityDeposit is returned if the request security deposit less than the minimum value
	ErrMinimumSecurityDeposit = errors.New("security deposit not meet the minimum value")

	// ErrCommission is returned if the request Commission value not between 0 and 100
	ErrCommission = errors.New("commission percentage (between 0 and 100) out of range")

	// ErrInsufficientFundsForTransfer is returned if the transaction sender doesn't
	// have enough funds for transfer(topmost call only).
	ErrInsufficientFundsForTransfer = errors.New("insufficient funds for transfer")

	// Vote Error
	// ErrVoteAmountTooLow is returned if the vote amount less than proxied delegation amount
	ErrVoteAmountTooLow = errors.New("vote amount too low")

	// ErrVoteAmountTooHight is returned if the vote amount greater than proxied amount + self amount
	ErrVoteAmountTooHight = errors.New("vote amount too high")

	// ErrNotOwner is returned if the Address not owner
	ErrNotOwner = errors.New("address not owner")

	// ErrNotAllowedInMainChain is returned if the transaction with main flag = false be sent to main chain
	ErrNotAllowedInMainChain = errors.New("transaction not allowed in main chain")

	// ErrNotAllowedInChildChain is returned if the transaction with child flag = false be sent to child chain
	ErrNotAllowedInChildChain = errors.New("transaction not allowed in child chain")

	// ErrTxTypeNotSupported is returned if a transaction is not supported in the
	// current network configuration.
	ErrTxTypeNotSupported = types.ErrTxTypeNotSupported

	// ErrTipAboveFeeCap is a sanity error to ensure no one is able to specify a
	// transaction with a tip higher than the total fee cap.
	ErrTipAboveFeeCap = errors.New("max priority fee per gas higher than max fee per gas")

	// ErrTipVeryHigh is a sanity error to avoid extremely big numbers specified
	// in the tip field.
	ErrTipVeryHigh = errors.New("max priority fee per gas higher than 2^256-1")

	// ErrFeeCapVeryHigh is a sanity error to avoid extremely big numbers specified
	// in the fee cap field.
	ErrFeeCapVeryHigh = errors.New("max fee per gas higher than 2^256-1")

	// ErrFeeCapTooLow is returned if the transaction fee cap is less than the
	// the base fee of the block.
	ErrFeeCapTooLow = errors.New("max fee per gas less than block base fee")
	
	// ErrSenderNoEOA is returned if the sender of a transaction is a contract.
	ErrSenderNoEOA = errors.New("sender not an eoa")

	// ErrInvalidTx4 is returned if the tx4 has been checked during execution
	ErrTryCCTTxExec = errors.New("failed try run CCTTxExec")
)
