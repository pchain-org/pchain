package trial_backend

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	//"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	//"bytes"
	//"time"
	"github.com/ethereum/go-ethereum/core/state"
	"fmt"
)

// APIs returns the RPC APIs this consensus engine provides.
func (sb *backend) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "tendermint",
		Version:   "1.0",
		Service:   &API{chain: chain, tendermint: sb},
		Public:    true,
	}}
}

// Start implements consensus.Istanbul.Start
func (sb *backend) Start(chain consensus.ChainReader, currentBlock func() *types.Block, hasBadBlock func(hash common.Hash) bool) error {

	fmt.Printf("Tendermint: (sb *backend) Start, add logic here\n")

	/*
	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()
	if sb.coreStarted {
		return istanbul.ErrStartedEngine
	}

	// clear previous data
	sb.proposedBlockHash = common.Hash{}
	if sb.commitCh != nil {
		close(sb.commitCh)
	}
	sb.commitCh = make(chan *types.Block, 1)

	sb.chain = chain
	sb.currentBlock = currentBlock
	sb.hasBadBlock = hasBadBlock

	if err := sb.core.Start(); err != nil {
		return err
	}

	sb.coreStarted = true
	*/
	return nil
}


// Stop implements consensus.Istanbul.Stop
func (sb *backend) Stop() error {

	fmt.Printf("Tendermint: (sb *backend) Stop, add logic here\n")

	/*
	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()
	if !sb.coreStarted {
		return istanbul.ErrStoppedEngine
	}
	if err := sb.core.Stop(); err != nil {
		return err
	}
	sb.coreStarted = false
	*/
	return nil
}


// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (sb *backend) Author(header *types.Header) (common.Address, error) {

	fmt.Printf("Tendermint: (sb *backend) Author, add logic here\n")

	return common.Address{}, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (sb *backend) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {

	fmt.Printf("Tendermint: (sb *backend) VerifyHeader, add logic here\n")

	return nil
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (sb *backend) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {

	fmt.Printf("Tendermint: (sb *backend) verifyHeader, add logic here\n")

	return nil
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (sb *backend) verifyCascadingFields(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	// The genesis block is the always valid dead-end

	fmt.Printf("Tendermint: (sb *backend) verifyCascadingFields, add logic here\n")

	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (sb *backend) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	fmt.Printf("Tendermint: (sb *backend) VerifyHeaders, add logic here\n")

	return abort, results
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (sb *backend) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {

	fmt.Printf("Tendermint: (sb *backend) VerifyUncles, add logic here\n")

	return nil
}

// verifySigner checks whether the signer is in parent's validator set
func (sb *backend) verifySigner(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {

	fmt.Printf("Tendermint: (sb *backend) verifySigner, add logic here\n")

	return nil
}

// verifyCommittedSeals checks whether every committed seal is signed by one of the parent's validators
func (sb *backend) verifyCommittedSeals(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {

	fmt.Printf("Tendermint: (sb *backend) verifyCommittedSeals, add logic here\n")

	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (sb *backend) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	// get parent header and ensure the signer is in parent's validator set

	fmt.Printf("Tendermint: (sb *backend) VerifySeal, add logic here\n")

	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (sb *backend) Prepare(chain consensus.ChainReader, header *types.Header) error {

	fmt.Printf("Tendermint: (sb *backend) Prepare, add logic here\n")

	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
//
// Note, the block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (sb *backend) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {

	fmt.Printf("Tendermint: (sb *backend) Finalize, add logic here\n")

	return nil, nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (sb *backend) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {

	fmt.Printf("Tendermint: (sb *backend) Seal, add logic here\n")

	return nil, nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have based on the previous blocks in the chain and the
// current signer.
func (sb *backend) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {

	fmt.Printf("Tendermint: (sb *backend) CalcDifficulty, add logic here\n")

	return new(big.Int).SetUint64(0)
}

// update timestamp and signature of the block based on its number of transactions
func (sb *backend) updateBlock(parent *types.Header, block *types.Block) (*types.Block, error) {

	fmt.Printf("Tendermint: (sb *backend) updateBlock, add logic here\n")

	return nil, nil
}

// FIXME: Need to update this for Istanbul
// sigHash returns the hash which is used as input for the Istanbul
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func sigHash(header *types.Header) (hash common.Hash) {

	fmt.Printf("Tendermint: (sb *backend) sigHash, add logic here\n")

	return common.Hash{}
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header) (common.Address, error) {

	fmt.Printf("Tendermint: (sb *backend) ecrecover, add logic here\n")

	return common.Address{}, nil
}

// prepareExtra returns a extra-data of the given header and validators
func prepareExtra(header *types.Header, vals []common.Address) ([]byte, error) {

	fmt.Printf("Tendermint: (sb *backend) prepareExtra, add logic here\n")

	return nil, nil
}

// writeSeal writes the extra-data field of the given header with the given seals.
// suggest to rename to writeSeal.
func writeSeal(h *types.Header, seal []byte) error {

	fmt.Printf("Tendermint: (sb *backend) writeSeal, add logic here\n")

	return nil
}

// writeCommittedSeals writes the extra-data field of a block header with given committed seals.
func writeCommittedSeals(h *types.Header, committedSeals [][]byte) error {

	fmt.Printf("Tendermint: (sb *backend) writeCommittedSeals, add logic here\n")

	return nil
}
