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

import "errors"

var (
	// ErrKnownBlock is returned when a block to import is already known locally.
	ErrKnownBlock = errors.New("block already known")

	// ErrGasLimitReached is returned by the gas pool if the amount of gas required
	// by a transaction is higher than what's left in the block.
	ErrGasLimitReached = errors.New("gas limit reached")

	// ErrBlacklistedHash is returned if a block to import is on the blacklist.
	ErrBlacklistedHash = errors.New("blacklisted hash")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")

	// ErrNoContractOnMainChain is returned if the contract creation tx has been submit to PChain main chain
	ErrNoContractOnMainChain = errors.New("no contract creation on main chain")

	// ErrInvalidTx4 is returned if the tx4 has been checked during execution
	ErrInvalidTx4 = errors.New("invalid Tx4")

	// Delegation Error
	// ErrCancelSelfDelegate is returned if the cancel delegate apply to the self address
	ErrCancelSelfDelegate = errors.New("can not cancel self delegation")

	// ErrDelegateAmount is returned if the delegate amount less than 0
	ErrDelegateAmount = errors.New("delegation amount must be greater or equal to 1000 PI")

	// ErrInsufficientProxiedBalance is returned if the cancellation amount of executing a transaction
	// is higher than the proxied balance of the user's account.
	ErrInsufficientProxiedBalance = errors.New("cancel amount greater than your Proxied Balance")

	// ErrAlreadyCandidate is returned if the request address has become candidate already
	ErrAlreadyCandidate = errors.New("address become candidate already")

	// ErrNotCandidate is returned if the request address is not a candidate
	ErrNotCandidate = errors.New("address not candidate")

	// ErrMinimumSecurityDeposit is returned if the request security deposit less than the minimum value
	ErrMinimumSecurityDeposit = errors.New("security deposit not meet the minimum value")

	// ErrCommission is returned if the request Commission value not between 0 and 100
	ErrCommission = errors.New("commission percentage (between 0 and 100) out of range")

	// Vote Error
	// ErrVoteAmountTooLow is returned if the vote amount less than proxied delegation amount
	ErrVoteAmountTooLow = errors.New("vote amount too low")

	// ErrVoteAmountTooHight is returned if the vote amount greater than proxied amount + self amount
	ErrVoteAmountTooHight = errors.New("vote amount too high")
)
