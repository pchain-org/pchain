package types

import (
	"time"
)

type BlockMeta struct {
	BlockID BlockID `json:"block_id"` // the block hash and partsethash
	ChainID        string    `json:"chain_id"`
	Height         uint64       `json:"height"`
	Time           time.Time `json:"time"`
	LastBlockID    BlockID   `json:"last_block_id"`
	SeenCommitHash []byte    `json:"seen_commit_hash"` // commit from validators from the last block
	ValidatorsHash []byte    `json:"validators_hash"`  // validators for the current block

}

func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		BlockID: BlockID{block.Hash(), blockParts.Header()},
		ChainID: block.TdmExtra.ChainID,
		Height: block.TdmExtra.Height,
		Time: block.TdmExtra.Time,
		LastBlockID: block.TdmExtra.LastBlockID,
		SeenCommitHash: block.TdmExtra.SeenCommitHash,
		ValidatorsHash: block.TdmExtra.ValidatorsHash,
	}
}
