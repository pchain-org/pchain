package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	//"github.com/tendermint/go-data"
)

const MaxBlockSize = 22020096 // 21MB TODO make it configurable

type Block struct {
	ExData   *ExData            `json:"exdata"`
	TdmExtra *TendermintExtra   `json:"tdmexdata"`
}


func MakeBlock(height int, chainID string, commit *Commit,
	prevBlockID BlockID, valHash, appHash []byte, blkExData []byte, partSize int) (*Block, *PartSet) {

	exData := &ExData{
		BlockExData: blkExData,
	}

	TdmExtra := &TendermintExtra{
		Header: &Header{
			ChainID:        chainID,
			Height:         height,
			Time:           time.Now(),
			LastBlockID:    prevBlockID,
			ValidatorsHash: valHash,
		},
		LastCommit: commit,
	}

	block := &Block{
		ExData: exData,
		TdmExtra: TdmExtra,
	}

	block.FillHeader()

	return block, block.MakePartSet(partSize)
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockID BlockID,
	lastBlockTime time.Time, appHash []byte) error {

	if b.TdmExtra.Header.ChainID != chainID {
		return errors.New(Fmt("Wrong Block.Header.ChainID. Expected %v, got %v", chainID, b.TdmExtra.Header.ChainID))
	}
	if b.TdmExtra.Header.Height != lastBlockHeight+1 {
		return errors.New(Fmt("Wrong Block.Header.Height. Expected %v, got %v", lastBlockHeight+1, b.TdmExtra.Header.Height))
	}
	if !b.TdmExtra.Header.LastBlockID.Equals(lastBlockID) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockID.  Expected %v, got %v", lastBlockID, b.TdmExtra.Header.LastBlockID))
	}
	if !bytes.Equal(b.TdmExtra.Header.LastCommitHash, b.TdmExtra.LastCommit.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.LastCommitHash.  Expected %X, got %X", b.TdmExtra.Header.LastCommitHash, b.TdmExtra.LastCommit.Hash()))
	}
	if b.TdmExtra.Header.Height != 1 {
		if err := b.TdmExtra.LastCommit.ValidateBasic(); err != nil {
			return err
		}
	}
	return nil
}

type IntegratedBlock struct {
	Block *Block
	Commit *Commit
	BlockPartSize int
}

func MakeIntegratedBlock(block *Block, commit *Commit, blockPartSize int) (*IntegratedBlock) {

	if block == nil || commit == nil {
		return nil
	}

	return &IntegratedBlock {
		Block: block,
		Commit: commit,
		BlockPartSize: blockPartSize,
	}
}

func (b *Block) FillHeader() {
	if b.TdmExtra.Header.LastCommitHash == nil {
		b.TdmExtra.Header.LastCommitHash = b.TdmExtra.LastCommit.Hash()
	}
}


// Computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() []byte {
	// fmt.Println(">>", b.Data)
	if b == nil || b.TdmExtra.Header == nil || b.TdmExtra.LastCommit == nil {
		return nil
	}
	b.FillHeader()
	return b.TdmExtra.Header.Hash()
}


func (b *Block) MakePartSet(partSize int) *PartSet {

	return NewPartSetFromData(wire.BinaryBytes(b), partSize)
}

// Convenience.
// A nil block never hashes to anything.
// Nothing hashes to a nil hash.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

func (b *Block) String() string {
	return b.StringIndented("")
}

func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}

	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%X`,
		indent, b.ExData.StringIndented(indent+"  "),
		indent, b.TdmExtra.Header.StringIndented(indent+"  "),
		indent, b.TdmExtra.LastCommit.StringIndented(indent+"  "),
		indent, b.Hash())
}

func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	} else {
		return fmt.Sprintf("Block#%X", b.Hash())
	}
}

//-----------------------------------------------------------------------------
type Header struct {
	ChainID        string    `json:"chain_id"`
	Height         int       `json:"height"`
	Time           time.Time `json:"time"`
	LastBlockID    BlockID   `json:"last_block_id"`
	LastCommitHash []byte    `json:"last_commit_hash"` // commit from validators from the last block
	ValidatorsHash []byte    `json:"validators_hash"`  // validators for the current block
}

// NOTE: hash is nil if required fields are missing.
func (h *Header) Hash() []byte {
	if len(h.ValidatorsHash) == 0 {
		return nil
	}
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"ChainID":     h.ChainID,
		"Height":      h.Height,
		"Time":        h.Time,
		"LastBlockID": h.LastBlockID,
		"LastCommit":  h.LastCommitHash,
		"Validators":  h.ValidatorsHash,
	})
}

func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  ChainID:        %v
%s  Height:         %v
%s  Time:           %v
%s  NumTxs:         %v
%s  LastBlockID:    %v
%s  LastCommit:     %X
%s  Data:           %X
%s  Validators:     %X
%s  App:            %X
%s}#%X`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Time,
		indent, h.LastBlockID,
		indent, h.LastCommitHash,
		indent, h.ValidatorsHash,
		indent, h.Hash())
}

//-------------------------------------

// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"blockID"`
	Precommits []*Vote `json:"precommits"`

	// Volatile
	firstPrecommit *Vote
	hash           []byte
	bitArray       *BitArray
}

func (commit *Commit) FirstPrecommit() *Vote {
	if len(commit.Precommits) == 0 {
		return nil
	}
	if commit.firstPrecommit != nil {
		return commit.firstPrecommit
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.firstPrecommit = precommit
			return precommit
		}
	}
	return nil
}

func (commit *Commit) Height() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Height
}

func (commit *Commit) Round() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Round
}

func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Precommits)
}

func (commit *Commit) BitArray() *BitArray {
	if commit.bitArray == nil {
		commit.bitArray = NewBitArray(len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			commit.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return commit.bitArray
}

func (commit *Commit) GetByIndex(index int) *Vote {
	return commit.Precommits[index]
}

func (commit *Commit) IsCommit() bool {
	if len(commit.Precommits) == 0 {
		return false
	}
	return true
}

func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
	if len(commit.Precommits) == 0 {
		return errors.New("No precommits in commit")
	}
	height, round := commit.Height(), commit.Round()

	// validate the precommits
	for _, precommit := range commit.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

func (commit *Commit) Hash() []byte {
	if commit.hash == nil {
		bs := make([]interface{}, len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			bs[i] = precommit
		}
		commit.hash = merkle.SimpleHashFromBinaries(bs)
	}
	return commit.hash
}

func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Commit{
%s  BlockID:    %v
%s  Precommits: %v
%s}#%X`,
		indent, commit.BlockID,
		indent, strings.Join(precommitStrings, "\n"+indent+"  "),
		indent, commit.hash)
}

type ExData struct {

	BlockExData []byte `json:"ex_data"`

	// Volatile
	hash []byte
}

func (exData *ExData) StringIndented(indent string) string {
	if exData == nil {
		return "nil-ExData"
	}

	return fmt.Sprintf(`ExData{
%s  %v
%s}#%X`,
		indent, string(exData.BlockExData),
		indent, exData.hash)
}

//--------------------------------------------------------------------------------

type BlockID struct {
	Hash        []byte        `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 && blockID.PartsHeader.IsZero()
}

func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash, other.Hash) &&
		blockID.PartsHeader.Equals(other.PartsHeader)
}

func (blockID BlockID) Key() string {
	return string(blockID.Hash) + string(wire.BinaryBytes(blockID.PartsHeader))
}

func (blockID BlockID) WriteSignBytes(w io.Writer, n *int, err *error) {
	if blockID.IsZero() {
		wire.WriteTo([]byte("null"), w, n, err)
	} else {
		wire.WriteJSON(CanonicalBlockID(blockID), w, n, err)
	}

}

func (blockID BlockID) String() string {
	return fmt.Sprintf(`%X:%v`, blockID.Hash, blockID.PartsHeader)
}
