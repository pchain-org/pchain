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
	"github.com/tendermint/go-crypto"
)

const MaxBlockSize = 22020096 // 21MB TODO make it configurable

type Block struct {
	*Header    `json:"header"`
	*Data      `json:"data"`
	*ExData    `json:"exdata"`
	LastCommit *Commit `json:"last_commit"`
}

// TODO: version
func MakeBlock(height int, chainID string, txs []Tx, commit *Commit,
	prevBlockID BlockID, valHash, appHash []byte, blkExData []byte, partSize int) (*Block, *PartSet) {

	data := &Data{
		Txs: txs,
	}

	exData := &ExData{
		BlockExData: blkExData,
	}

	block := &Block{
		Header: &Header{
			ChainID:        chainID,
			Height:         height,
			Time:           time.Now(),
			NumTxs:         len(txs),
			LastBlockID:    prevBlockID,
			ValidatorsHash: BytesToHash160(valHash),
			AppHash:        BytesToHash256(appHash), // state merkle root of txs from the previous block.
		},
		LastCommit: commit,
		Data:       data,
		ExData:     exData,
	}
	block.FillHeader()

	fmt.Printf("MakeBlock(), block is %v\n", block.String())
	fmt.Printf("block.LastCommit is %v\n", block.LastCommit)

	return block, block.MakePartSet(partSize)
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockID BlockID,
	lastBlockTime time.Time, appHash []byte) error {
	if b.ChainID != chainID {
		return errors.New(Fmt("Wrong Block.Header.ChainID. Expected %v, got %v", chainID, b.ChainID))
	}
	if b.Height != lastBlockHeight+1 {
		return errors.New(Fmt("Wrong Block.Header.Height. Expected %v, got %v", lastBlockHeight+1, b.Height))
	}
	/*	TODO: Determine bounds for Time
		See blockchain/reactor "stopSyncingDurationMinutes"

		if !b.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/
	if b.NumTxs != len(b.Data.Txs) {
		return errors.New(Fmt("Wrong Block.Header.NumTxs. Expected %v, got %v", len(b.Data.Txs), b.NumTxs))
	}
	if !b.LastBlockID.Equals(lastBlockID) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockID.  Expected %v, got %v", lastBlockID, b.LastBlockID))
	}
	if !bytes.Equal(b.LastCommitHash.Bytes(), b.LastCommit.Hash().Bytes()) {
		return errors.New(Fmt("Wrong Block.Header.LastCommitHash.  Expected %X, got %X", b.LastCommitHash, b.LastCommit.Hash()))
	}

	logger.Debug(Fmt("ValidateBasic: Temporarily skip b.LastCommit check for block %d", b.Header.Height))


/*
	if b.Header.Height != 1 {
		if err := b.LastCommit.ValidateBasic(); err != nil {
			return err
		}
	}
*/

	if !bytes.Equal(b.DataHash.Bytes(), b.Data.Hash().Bytes()) {
		return errors.New(Fmt("Wrong Block.Header.DataHash.  Expected %X, got %X", b.DataHash, b.Data.Hash()))
	}
	/*
		if !bytes.Equal(b.AppHash, appHash) {
			return errors.New(Fmt("Wrong Block.Header.AppHash.  Expected %X, got %X", appHash, b.AppHash))
		}
	*/
	// NOTE: the AppHash and ValidatorsHash are validated later.
	return nil
}

type IntegratedBlock struct {
	Block         *Block
	Commit        *Commit
	BlockPartSize int
}

func MakeIntegratedBlock(block *Block, commit *Commit, blockPartSize int) *IntegratedBlock {

	if block == nil || commit == nil {
		return nil
	}

	return &IntegratedBlock{
		Block:         block,
		Commit:        commit,
		BlockPartSize: blockPartSize,
	}
}

func (b *Block) FillHeader() {
	if bytes.Equal(b.LastCommitHash.Bytes(), EMPTY_HASH160.Bytes()) {
		b.LastCommitHash = b.LastCommit.Hash()
	}
	if bytes.Equal(b.DataHash.Bytes(), EMPTY_HASH160.Bytes()) {
		b.DataHash = b.Data.Hash()
	}
}

// Computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() Hash160 {
	if b == nil || b.Header == nil || b.Data == nil || b.LastCommit == nil {
		return EMPTY_HASH160
	}
	b.FillHeader()
	return b.Header.Hash()
}

func (b *Block) MakePartSet(partSize int) *PartSet {
	return NewPartSetFromData(wire.BinaryBytes(b), partSize)
}

// Convenience.
// A nil block never hashes to anything.
// Nothing hashes to a nil hash.
func (b *Block) HashesTo(hash Hash160) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash().Bytes(), hash.Bytes())
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
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.ExData.StringIndented(indent+"  "),
		indent, b.LastCommit.StringIndented(indent+"  "),
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
	NumTxs         int       `json:"num_txs"` // XXX: Can we get rid of this?
	LastBlockID    BlockID   `json:"last_block_id"`
	LastCommitHash Hash160   `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       Hash160   `json:"data_hash"`        // transactions
	ValidatorsHash Hash160   `json:"validators_hash"`  // validators for the current block
	AppHash        Hash256   `json:"app_hash"`         // state after txs from the previous block
}

// NOTE: hash is nil if required fields are missing.
func (h *Header) Hash() Hash160 {
	if len(h.ValidatorsHash) == 0 {
		return EMPTY_HASH160
	}
	return BytesToHash160(merkle.SimpleHashFromMap(map[string]interface{}{
		"ChainID":     h.ChainID,
		"Height":      h.Height,
		"Time":        h.Time,
		"NumTxs":      h.NumTxs,
		"LastBlockID": h.LastBlockID,
		"LastCommit":  h.LastCommitHash,
		"Data":        h.DataHash,
		"Validators":  h.ValidatorsHash,
		"App":         h.AppHash,
	}))
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
		indent, h.NumTxs,
		indent, h.LastBlockID,
		indent, h.LastCommitHash,
		indent, h.DataHash,
		indent, h.ValidatorsHash,
		indent, h.AppHash,
		indent, h.Hash())
}

//-------------------------------------

// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"blockID"`
	Height		int	`json:"height"`
	Round		int	`json:"round"`

	// BLS signature aggregation to be added here
	SignAggr	crypto.BLSSignature     `json:"SignAggr"`
	BitArray        *BitArray

	// Volatile
	hash            []byte
}



func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

func (commit *Commit) Size() int {
	return commit.BitArray.Size()
}

func (commit *Commit) NumCommits() int {
	return commit.BitArray.NumBitsSet()
}


func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
/*
	if commit.Type() != VoteTypePrecommit {
		return fmt.Errorf("Invalid commit type. Expected VoteTypePrecommit, got %v",
			precommit.Type)
	}

	// shall we validate the signature aggregation?
*/

	return nil
}

func (commit *Commit) Hash() Hash160 {
	if commit.hash == nil {
/*
		bs := make([]interface{}, len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			bs[i] = precommit
		}
*/
		hash := merkle.SimpleHashFromBinary(*commit)
		commit.hash = hash
	}
	return BytesToHash160(commit.hash)
}

func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	return fmt.Sprintf(`Commit{
%s  BlockID:    %v
%s  Height:     %v
%s  Round:      %v
%s  Type:       %v
%s  BitArray:   %v
%s}#%X`,
		indent, commit.BlockID,
		indent, commit.Height,
		indent, commit.Round,
		indent, commit.Type,
		indent, commit.BitArray.String(),
		indent, commit.hash)
}

//-----------------------------------------------------------------------------

type Data struct {

	// Txs that will be applied by state @ block.Height+1.
	// NOTE: not all txs here are valid.  We're just agreeing on the order first.
	// This means that block.AppHash does not include these txs.
	Txs Txs `json:"txs"`

	// Volatile
	hash Hash160
}

type ExData struct {
	BlockExData []byte `json:"ex_data"`

	// Volatile
	hash Hash160
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

func (data *Data) Hash() Hash160 {
	if bytes.Equal(data.hash.Bytes(), EMPTY_HASH160.Bytes()) {
		data.hash = data.Txs.Hash() // NOTE: leaves of merkle tree are TxIDs
	}
	return data.hash
}

func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("Tx:%v", tx)
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%X`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}

//--------------------------------------------------------------------------------

type BlockID struct {
	Hash        Hash160       `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 && blockID.PartsHeader.IsZero()
}

func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash.Bytes(), other.Hash.Bytes()) &&
		blockID.PartsHeader.Equals(other.PartsHeader)
}

func (blockID BlockID) Key() string {
	return string(blockID.Hash.Bytes()) + string(wire.BinaryBytes(blockID.PartsHeader))
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
