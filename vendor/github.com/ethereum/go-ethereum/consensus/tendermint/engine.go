package tendermint

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/hashicorp/golang-lru"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/tendermint/go-wire"
	"math/big"
	"time"
)

const (
	// fetcherID is the ID indicates the block is from Tendermint engine
	fetcherID = "tendermint"
)

var (
	// errInvalidProposal is returned when a prposal is malformed.
	errInvalidProposal = errors.New("invalid proposal")
	// errInvalidSignature is returned when given signature is not signed by given
	// address.
	errInvalidSignature = errors.New("invalid signature")
	// errUnknownBlock is returned when the list of validators is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")
	// errUnauthorized is returned if a header is signed by a non authorized entity.
	errUnauthorized = errors.New("unauthorized")
	// errInvalidDifficulty is returned if the difficulty of a block is not 1
	errInvalidDifficulty = errors.New("invalid difficulty")
	// errInvalidExtraDataFormat is returned when the extra data format is incorrect
	errInvalidExtraDataFormat = errors.New("invalid extra data format")
	// errInvalidMixDigest is returned if a block's mix digest is not Istanbul digest.
	errInvalidMixDigest = errors.New("invalid Tendermint mix digest")
	// errInvalidNonce is returned if a block's nonce is invalid
	errInvalidNonce = errors.New("invalid nonce")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")
	// errInconsistentValidatorSet is returned if the validator set is inconsistent
	errInconsistentValidatorSet = errors.New("inconsistent validator set")
	// errInvalidTimestamp is returned if the timestamp of a block is lower than the previous block's timestamp + the minimum block period.
	errInvalidTimestamp = errors.New("invalid timestamp")
	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidVotingChain = errors.New("invalid voting chain")
	// errInvalidVote is returned if a nonce value is something else that the two
	// allowed constants of 0x00..0 or 0xff..f.
	errInvalidVote = errors.New("vote nonce not 0x00..0 or 0xff..f")
	// errInvalidCommittedSeals is returned if the committed seal is not signed by any of parent validators.
	errInvalidCommittedSeals = errors.New("invalid committed seals")
	// errEmptyCommittedSeals is returned if the field of committed seals is zero.
	errEmptyCommittedSeals = errors.New("zero committed seals")
	// errMismatchTxhashes is returned if the TxHash in header is mismatch.
	errMismatchTxhashes = errors.New("mismatch transactions hashes")

	// errInvalidMainChainNumber is returned when child chain block doesn't contain the valid main chain height
	errInvalidMainChainNumber = errors.New("invalid Main Chain Height")
	// errMainChainNotCatchup is returned if child chain wait more than 300 seconds for main chain to catch up
	errMainChainNotCatchup = errors.New("unable proceed the block due to main chain not catch up by waiting for more than 300 seconds, please catch up the main chain first")
)

var (
	now = time.Now

	inmemoryAddresses  = 20 // Number of recent addresses from ecrecover
	recentAddresses, _ = lru.NewARC(inmemoryAddresses)

	_ consensus.Engine = (*backend)(nil)
)

// APIs returns the RPC APIs this consensus engine provides.
func (sb *backend) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "tdm",
		Version:   "1.0",
		Service:   &API{chain: chain, tendermint: sb},
		Public:    true,
	}}
}

// Start implements consensus.Tendermint.Start
func (sb *backend) Start(chain consensus.ChainReader, currentBlock func() *types.Block, hasBadBlock func(hash common.Hash) bool) error {

	sb.logger.Info("Tendermint (backend) Start, add logic here")

	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()
	if sb.coreStarted {
		return ErrStartedEngine
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

	if _, err := sb.core.Start(); err != nil {
		return err
	}

	sb.coreStarted = true

	return nil
}

// Stop implements consensus.Tendermint.Stop
func (sb *backend) Stop() error {

	sb.logger.Info("Tendermint (backend) Stop, add logic here")

	//debug.PrintStack()

	sb.coreMu.Lock()
	defer sb.coreMu.Unlock()
	if !sb.coreStarted {
		return ErrStoppedEngine
	}
	if !sb.core.Stop() {
		return errors.New("tendermint stop error")
	}
	sb.coreStarted = false

	return nil
}

// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (sb *backend) Author(header *types.Header) (common.Address, error) {

	sb.logger.Info("Tendermint (backend) Author, add logic here")

	return common.HexToAddress("0x136c0e42f4e1b4efd930d2d88a3f3aa4996b6e2e"), nil
	//return common.Address{}, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (sb *backend) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {

	sb.logger.Info("Tendermint (backend) VerifyHeader, add logic here")

	return sb.verifyHeader(chain, header, nil)
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (sb *backend) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {

	if header.Number == nil {
		return errUnknownBlock
	}

	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}

	// Ensure that the extra data format is satisfied
	if _, err := tdmTypes.ExtractTendermintExtra(header); err != nil {
		return errInvalidExtraDataFormat
	}

	// Ensure that the coinbase is valid
	if header.Nonce != (types.TendermintEmptyNonce) && !bytes.Equal(header.Nonce[:], types.TendermintNonce) {
		return errInvalidNonce
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != types.TendermintDigest {
		return errInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in Istanbul
	if header.UncleHash != types.TendermintNilUncleHash {
		return errInvalidUncleHash
	}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if header.Difficulty == nil || header.Difficulty.Cmp(types.TendermintDefaultDifficulty) != 0 {
		return errInvalidDifficulty
	}

	if fieldError := sb.verifyCascadingFields(chain, header, parents); fieldError != nil {
		return fieldError
	}

	// Check the MainChainNumber if on Child Chain
	if sb.chainConfig.PChainId != params.MainnetChainConfig.PChainId {
		if header.MainChainNumber == nil {
			return errInvalidMainChainNumber
		}

		tried := 0
		for {
			// Check our main chain has already run ahead
			ourMainChainHeight := sb.core.cch.GetHeightFromMainChain()
			if ourMainChainHeight.Cmp(header.MainChainNumber) >= 0 {
				break
			}

			if tried == 10 {
				sb.logger.Warnf("Tendermint (backend) VerifyHeader, Main Chain Number mismatch, after retried %d times", tried)
				return errMainChainNotCatchup
			}

			// Sleep for a while and check again
			duration := 30 * time.Second
			tried++
			sb.logger.Infof("Tendermint (backend) VerifyHeader, Main Chain Number mismatch, wait for %v then try again (count %d)", duration, tried)
			time.Sleep(duration)
		}
	}

	return nil
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (sb *backend) verifyCascadingFields(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	// The genesis block is the always valid dead-end

	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	// Ensure that the block's timestamp isn't too close to it's parent
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	/*
		if parent.Time.Uint64()+sb.config.BlockPeriod > header.Time.Uint64() {
			return errInvalidTimestamp
		}
		// Verify validators in extraData. Validators in snapshot and extraData should be the same.
		snap, err := sb.snapshot(chain, number-1, header.ParentHash, parents)
		if err != nil {
			return err
		}

		validators := make([]byte, len(snap.validators())*common.AddressLength)
		for i, validator := range snap.validators() {
			copy(validators[i*common.AddressLength:], validator[:])
		}
	*/

	err := sb.verifyCommittedSeals(chain, header, parents)
	return err
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (sb *backend) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	sb.logger.Info("Tendermint (backend) VerifyHeaders, add logic here")

	go func() {
		for i, header := range headers {
			err := sb.verifyHeader(chain, header, headers[:i])
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()

	return abort, results
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (sb *backend) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {

	if len(block.Uncles()) > 0 {
		return errInvalidUncleHash
	}
	return nil
}

// verifyCommittedSeals checks whether every committed seal is signed by one of the parent's validators
func (sb *backend) verifyCommittedSeals(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {

	sb.logger.Info("Tendermint (backend) verifyCommittedSeals, add logic here")

	tdmExtra, err := tdmTypes.ExtractTendermintExtra(header)
	if err != nil {
		return errInvalidExtraDataFormat
	}

	epoch := sb.core.consensusState.Epoch
	if epoch == nil || epoch.Validators == nil {
		sb.logger.Errorf("verifyCommittedSeals error. Epoch %v", epoch)
		return errInconsistentValidatorSet
	}

	valSet := epoch.Validators
	if !bytes.Equal(valSet.Hash(), tdmExtra.ValidatorsHash) {
		sb.logger.Errorf("verifyCommittedSeals error. Our Validator Set %x, tdmExtra Valdiator %x", valSet.Hash(), tdmExtra.ValidatorsHash)
		return errInconsistentValidatorSet
	}

	seenCommit := tdmExtra.SeenCommit
	if !bytes.Equal(tdmExtra.SeenCommitHash, seenCommit.Hash()) {
		sb.logger.Errorf("verifyCommittedSeals SeenCommit is %#+v", seenCommit)
		sb.logger.Errorf("verifyCommittedSeals error. Our SeenCommitHash %x, tdmExtra SeenCommitHash %x", seenCommit.Hash(), tdmExtra.SeenCommitHash)
		return errInvalidCommittedSeals
	}

	if err = valSet.VerifyCommit(tdmExtra.ChainID, tdmExtra.Height, seenCommit); err != nil {
		return errInvalidSignature
	}

	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (sb *backend) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	// get parent header and ensure the signer is in parent's validator set
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}

	// ensure that the difficulty equals to defaultDifficulty
	if header.Difficulty.Cmp(types.TendermintDefaultDifficulty) != 0 {
		return errInvalidDifficulty
	}

	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (sb *backend) Prepare(chain consensus.ChainReader, header *types.Header) error {

	header.Nonce = types.TendermintEmptyNonce
	header.MixDigest = types.TendermintDigest

	// copy the parent extra data as the header extra data
	number := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// use the same difficulty for all blocks
	header.Difficulty = types.TendermintDefaultDifficulty

	/*
		// Assemble the voting snapshot
		snap, err := sb.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return err
		}

		// get valid candidate list
		sb.candidatesLock.RLock()
		var addresses []common.Address
		var authorizes []bool
		for address, authorize := range sb.candidates {
			if snap.checkVote(address, authorize) {
				addresses = append(addresses, address)
				authorizes = append(authorizes, authorize)
			}
		}
		sb.candidatesLock.RUnlock()


		// pick one of the candidates randomly
		if len(addresses) > 0 {
			index := rand.Intn(len(addresses))
			// add validator voting in coinbase
			header.Coinbase = addresses[index]
			if authorizes[index] {
				copy(header.Nonce[:], nonceAuthVote)
			} else {
				copy(header.Nonce[:], nonceDropVote)
			}
		}
	*/
	// add validators in snapshot to extraData's validators section
	extra, err := prepareExtra(header, nil)
	if err != nil {
		return err
	}
	header.Extra = extra

	// set header's timestamp
	//header.Time = new(big.Int).Add(parent.Time, new(big.Int).SetUint64(sb.config.BlockPeriod))
	//if header.Time.Int64() < time.Now().Unix() {
	header.Time = big.NewInt(time.Now().Unix())
	//}

	// Add Main Chain Height if running on Child Chain
	if sb.chainConfig.PChainId != params.MainnetChainConfig.PChainId {
		header.MainChainNumber = sb.core.cch.GetHeightFromMainChain()
	}

	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
//
// Note, the block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (sb *backend) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt, ops *types.PendingOps) (*types.Block, error) {

	sb.logger.Infof("Tendermint (backend) Finalize, receipts are: %v", receipts)

	// Check if any Child Chain need to be launch and Update their account balance accordingly
	if sb.chainConfig.PChainId == params.MainnetChainConfig.PChainId {
		// Check the Child Chain Start
		readyId, updateBytes, removedId := sb.core.cch.ReadyForLaunchChildChain(header.Number, state)
		if len(readyId) > 0 || updateBytes != nil || len(removedId) > 0 {
			if ok := ops.Append(&types.LaunchChildChainsOp{
				ChildChainIds:       readyId,
				NewPendingIdx:       updateBytes,
				DeleteChildChainIds: removedId,
			}); !ok {
				// This should not happened
				sb.logger.Error("Tendermint (backend) Finalize, Fail to append LaunchChildChainsOp, only one LaunchChildChainsOp is allowed in each block")
			}
		}
	}

	// Check the Epoch switch and update their account balance accordingly (Refund the Locked Balance)
	if ok, newValidators, _ := sb.core.consensusState.Epoch.ShouldEnterNewEpoch(header.Number.Uint64(), state); ok {
		ops.Append(&tdmTypes.SwitchEpochOp{
			NewValidators: newValidators,
		})

	}

	// Calculate the rewards, and drop the uncles
	// TODO: we need consider reward here

	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.TendermintNilUncleHash

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts), nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (sb *backend) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {

	sb.logger.Info("Tendermint (backend) Seal, add logic here")

	// update the block header timestamp and signature and propose the block to core engine
	header := block.Header()
	number := header.Number.Uint64()
	/*
		// Bail out if we're unauthorized to sign a block
		snap, err := sb.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return nil, err
		}
		if _, v := snap.ValSet.GetByAddress(sb.address); v == nil {
			return nil, errUnauthorized
		}
	*/
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return nil, consensus.ErrUnknownAncestor
	}
	block, err := sb.updateBlock(parent, block)
	if err != nil {
		return nil, err
	}
	// wait for the timestamp of header, use this to adjust the block period
	delay := time.Unix(block.Header().Time.Int64(), 0).Sub(now())
	select {
	case <-time.After(delay):
	case <-stop:
		return nil, nil
	}
	// get the proposed block hash and clear it if the seal() is completed.
	sb.sealMu.Lock()
	sb.proposedBlockHash = block.Hash()
	clear := func() {
		sb.proposedBlockHash = common.Hash{}
		sb.sealMu.Unlock()
	}
	defer clear()

	// post block into Istanbul engine
	sb.logger.Infof("Tendermint (backend) Seal, before fire event with block height: %d", block.NumberU64())
	go tdmTypes.FireEventRequest(sb.core.EventSwitch(), tdmTypes.EventDataRequest{Proposal: block})
	//go sb.EventMux().Post(tdmTypes.RequestEvent{
	//	Proposal: block,
	//})

	for {
		select {
		case result, ok := <-sb.commitCh:

			if ok {
				sb.logger.Infof("Tendermint (backend) Seal, got result with block.Hash: %x, result.Hash: %x", block.Hash(), result.Hash())
				// if the block hash and the hash from channel are the same,
				// return the result. Otherwise, keep waiting the next hash.
				if block.Hash() == result.Hash() {
					return result, nil
				}
				sb.logger.Info("Tendermint (backend) Seal, hash are different")
			} else {
				sb.logger.Info("Tendermint (backend) Seal, has been restart, just return")
				return nil, nil
			}

		case <-stop:
			return nil, nil
		}
	}

	return nil, nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have based on the previous blocks in the chain and the
// current signer.
func (sb *backend) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {

	return types.TendermintDefaultDifficulty
}

// Commit implements istanbul.Backend.Commit
func (sb *backend) Commit(proposal *tdmTypes.TdmBlock, seals [][]byte) error {
	// Check if the proposal is a valid block
	block := proposal.Block

	h := block.Header()
	// Append seals into extra-data
	err := writeCommittedSeals(h, proposal.TdmExtra)
	if err != nil {
		return err
	}
	// update block's header
	block = block.WithSeal(h)

	sb.logger.Infof("Tendermint (backend) Commit, hash: %x, number: %v", block.Hash(), block.Number().Int64())
	sb.logger.Infof("Tendermint (backend) Commit, block: %s", block.String())

	// - if the proposed and committed blocks are the same, send the proposed hash
	//   to commit channel, which is being watched inside the engine.Seal() function.
	// - otherwise, we try to insert the block.
	// -- if success, the ChainHeadEvent event will be broadcasted, try to build
	//    the next block and the previous Seal() will be stopped.
	// -- otherwise, a error will be returned and a round change event will be fired.
	if sb.proposedBlockHash == block.Hash() {
		// feed block hash to Seal() and wait the Seal() result
		sb.commitCh <- block
		return nil
	}
	sb.logger.Infof("Tendermint (backend) Commit, sb.broadcaster is %v", sb.broadcaster)
	if sb.broadcaster != nil {
		sb.broadcaster.Enqueue(fetcherID, block)
	}
	return nil
}

// Stop implements consensus.Istanbul.Stop
func (sb *backend) ChainReader() consensus.ChainReader {

	return sb.chain
}

// GetEpoch Get Epoch from Tendermint Engine
func (sb *backend) GetEpoch() *epoch.Epoch {
	return sb.core.consensusState.Epoch
}

// SetEpoch Set Epoch to Tendermint Engine
func (sb *backend) SetEpoch(ep *epoch.Epoch) {
	sb.core.consensusState.Epoch = ep
}

// update timestamp and signature of the block based on its number of transactions
func (sb *backend) updateBlock(parent *types.Header, block *types.Block) (*types.Block, error) {

	sb.logger.Info("Tendermint (backend) updateBlock, add logic here")

	header := block.Header()
	/*
		//sign the hash
		seal, err := sb.Sign(sigHash(header).Bytes())
		if err != nil {
		    return nil, err
		}
	*/
	//err := writeSeal(header, seal)
	err := writeSeal(header, []byte{})
	if err != nil {
		return nil, err
	}

	return block.WithSeal(header), nil
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

	//logger.Info("Tendermint (backend) sigHash, add logic here")

	return common.Hash{}
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header) (common.Address, error) {

	//logger.Info("Tendermint (backend) ecrecover, add logic here")

	return common.Address{}, nil
}

// prepareExtra returns a extra-data of the given header and validators
func prepareExtra(header *types.Header, vals []common.Address) ([]byte, error) {

	//logger.Info("Tendermint (backend) prepareExtra, add logic here")

	header.Extra = types.MagicExtra
	return nil, nil
}

// writeSeal writes the extra-data field of the given header with the given seals.
// suggest to rename to writeSeal.
func writeSeal(h *types.Header, seal []byte) error {

	//logger.Info("Tendermint (backend) writeSeal, add logic here")

	/*
		if len(seal)%types.IstanbulExtraSeal != 0 {
			return errInvalidSignature
		}

		tdmExtra, err := tdmTypes.ExtractTendermintExtra(h)
		if err != nil {
			fmt.Printf("Tendermint: (sb *backend) writeSeal, 0\n")
			return err
		}

		//tdmExtra.Seal = seal
		payload, err := rlp.EncodeToBytes(&tdmExtra)
		if err != nil {
			fmt.Printf("Tendermint: (sb *backend) writeSeal, 1 with err %v\n", err)
			return err
		}
	*/
	payload := types.MagicExtra
	h.Extra = payload
	return nil
}

// writeCommittedSeals writes the extra-data field of a block header with given committed seals.
func writeCommittedSeals(h *types.Header, tdmExtra *tdmTypes.TendermintExtra) error {

	//logger.Info("Tendermint (backend) writeCommittedSeals, add logic here")

	/*
		if len(committedSeals) == 0 {
			return errInvalidCommittedSeals
		}

		for _, seal := range committedSeals {
			if len(seal) != types.IstanbulExtraSeal {
				return errInvalidCommittedSeals
			}
		}

		tdmExtra, err := types.ExtractTendermintExtra(h)
		if err != nil {
			return err
		}
	*/
	payload := wire.BinaryBytes(*tdmExtra)
	//payload, err := rlp.EncodeToBytes(tdmExtra)
	//if err != nil {
	//	return err
	//}

	h.Extra = payload
	return nil
}
