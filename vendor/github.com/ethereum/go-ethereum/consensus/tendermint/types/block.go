package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/tendermint/go-common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	//"github.com/tendermint/go-data"
)

const MaxBlockSize = 22020096 // 21MB TODO make it configurable


type TdmBlock struct {
	Block   *ethTypes.Block            `json:"block"`
	TdmExtra *TendermintExtra   `json:"tdmexdata"`
}

func MakeBlock(height uint64, chainID string, commit *Commit,
	block *ethTypes.Block, valHash []byte, epochBytes []byte, partSize int) (*TdmBlock, *PartSet) {

	TdmExtra := &TendermintExtra{
		ChainID:        chainID,
		Height:         uint64(height),
		Time:           time.Now(),
		ValidatorsHash: valHash,
		SeenCommit: commit,
		EpochBytes: epochBytes,
	}

	tdmBlock := &TdmBlock{
		Block: block,
		TdmExtra: TdmExtra,
	}

	return tdmBlock, tdmBlock.MakePartSet(partSize)
}

// Basic validation that doesn't involve state data.
func (b *TdmBlock) ValidateBasic(tdmExtra *TendermintExtra) error {

	fmt.Printf("b.TdmExtra is %v, tdmExtra is %v", b.TdmExtra, tdmExtra)
	if b.TdmExtra.ChainID != tdmExtra.ChainID {
		return errors.New(Fmt("Wrong Block.Header.ChainID. Expected %v, got %v", tdmExtra.ChainID, b.TdmExtra.ChainID))
	}
	if b.TdmExtra.Height != tdmExtra.Height+1 {
		return errors.New(Fmt("Wrong Block.Header.Height. Expected %v, got %v", tdmExtra.Height+1, b.TdmExtra.Height))
	}

	/*
	if !b.TdmExtra.BlockID.Equals(blockID) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockID.  Expected %v, got %v", blockID, b.TdmExtra.BlockID))
	}
	if !bytes.Equal(b.TdmExtra.SeenCommitHash, b.TdmExtra.SeenCommit.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.LastCommitHash.  Expected %X, got %X", b.TdmExtra.SeenCommitHash, b.TdmExtra.SeenCommit.Hash()))
	}
	if b.TdmExtra.Height != 1 {
		if err := b.TdmExtra.SeenCommit.ValidateBasic(); err != nil {
			return err
		}
	}
	*/
	return nil
}

func (b *TdmBlock) FillSeenCommitHash() {
	if b.TdmExtra.SeenCommitHash == nil {
		b.TdmExtra.SeenCommitHash = b.TdmExtra.SeenCommit.Hash()
	}
}


// Computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *TdmBlock) Hash() []byte {
	// fmt.Println(">>", b.Data)
	if b == nil || b.TdmExtra.SeenCommit == nil {
		return nil
	}
	b.FillSeenCommitHash()
	return b.TdmExtra.Hash()
}


func (b *TdmBlock) MakePartSet(partSize int) *PartSet {

	return NewPartSetFromData(b.ToBytes(), partSize)
}

func (b *TdmBlock) ToBytes() []byte {

	type TmpBlock struct {
		BlockData []byte
		TdmExtra *TendermintExtra
	}
	//fmt.Printf("TdmBlock.toBytes 0 with block: %v\n", b)

	blockByte, err := b.Block.EncodeRLP1()
	if err != nil {
		fmt.Printf("TdmBlock.toBytes error\n")
	}
	//fmt.Printf("TdmBlock.toBytes 1 with blockbyte: %v\n", blockByte)
	bb := &TmpBlock{
		BlockData:   blockByte,
		TdmExtra:    b.TdmExtra,
	}
	//fmt.Printf("TdmBlock.toBytes 1 with tdmblock: %v\n", bb)

	ret :=  wire.BinaryBytes(bb)
	//fmt.Printf("TdmBlock.toBytes 1 with ret:%v\n", ret)
	return ret
}

func (b *TdmBlock) FromBytes(reader io.Reader) (*TdmBlock, error) {

	type TmpBlock struct {
		BlockData []byte
		TdmExtra *TendermintExtra
	}

	//fmt.Printf("TdmBlock.FromBytes \n")

	var n int
	var err error
	bb := wire.ReadBinary(&TmpBlock{}, reader, MaxBlockSize, &n, &err).(*TmpBlock)
	if err != nil {
		fmt.Printf("TdmBlock.FromBytes 0 error: %v\n", err)
		return nil, err
	}

	block := &ethTypes.Block{}
	block, err = block.DecodeRLP1(bb.BlockData)
	if err != nil {
		fmt.Printf("TdmBlock.FromBytes 1 error: %v\n", err)
		return nil, err
	}

	tdmBlock := &TdmBlock{
		Block: block,
		TdmExtra: bb.TdmExtra,
	}

	//fmt.Printf("TdmBlock.FromBytes 2 with: %v\n", tdmBlock)
	return tdmBlock, nil
}

// Convenience.
// A nil block never hashes to anything.
// Nothing hashes to a nil hash.
func (b *TdmBlock) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

func (b *TdmBlock) String() string {
	return b.StringIndented("")
}

func (b *TdmBlock) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}

	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%X`,
		indent, b.Block.String(),
		indent, b.TdmExtra,
		indent, b.TdmExtra.SeenCommit.StringIndented(indent+"  "),
		indent, b.Hash())
}

func (b *TdmBlock) StringShort() string {
	if b == nil {
		return "nil-Block"
	} else {
		return fmt.Sprintf("Block#%X", b.Hash())
	}
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

func (commit *Commit) Height() uint64 {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Height
}

func (commit *Commit) Round() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return int(commit.FirstPrecommit().Round)
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
		commit.bitArray = NewBitArray(uint64(len(commit.Precommits)))
		for i, precommit := range commit.Precommits {
			commit.bitArray.SetIndex(uint64(i), precommit != nil)
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
		if int(precommit.Round) != round {
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
