package consensus

import (
	"fmt"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	consss "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/common"
)

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) GetChainReader() consss.ChainReader {
	return bs.backend.ChainReader()
}


// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) LoadBlock(height int) *types.Block {

	cr := bs.GetChainReader()

	ethBlock := cr.GetBlock(common.Hash{}, uint64(height))
	if ethBlock == nil {
		return nil
	}
	blkExData, err := ethBlock.EncodeRLP1()
	if err != nil {
		return nil
	}
	ExData := &types.ExData {
		BlockExData: blkExData,
	}


	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	if header == nil {
		return nil
	}
	TdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		return nil
	}

	return &types.Block{
		ExData: ExData,
		TdmExtra: TdmExtra,
	}
}

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) LoadBlockCommit(height int) *types.Commit {
	return bs.LoadSeenCommit(height)
}

// NOTE: the Precommit-vote heights are for the block at `height`
func (bs *ConsensusState) LoadSeenCommit(height int) *types.Commit {

	cr := bs.backend.ChainReader()
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit height is %v\n", height)
	ethBlock := cr.GetBlockByNumber(uint64(height))
	if ethBlock == nil {
		return nil
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit header is %v\n", header)
	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		fmt.Printf("(cs *ConsensusState) LoadSeenCommit got error: %v\n", err)
		return nil
	}

	return tdmExtra.SeenCommit
}
/*
func (cs *ConsensusState) LoadBlockMeta(height int) *types.BlockMeta {

	cr := cs.backend.ChainReader()

	header := cr.GetHeader(common.Hash{}, uint64(height))

	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		fmt.Printf("(cs *ConsensusState) LoadBlockMeta got error: %v", err)
		return nil
	}

	blockMeta :=  &types.BlockMeta {
		BlockID: "",
		ChainID: tdmExtra.ChainID,
		Height: tdmExtra.Height,
		Time: tdmExtra.Time,
		LastBlockID: tdmExtra.LastBlockID
		SeenCommitHash: tdmExtra.SeenCommit,
		ValidatorsHash: tdmExtra.ValidatorsHash,
	}

	return blockMeta
}
*/
