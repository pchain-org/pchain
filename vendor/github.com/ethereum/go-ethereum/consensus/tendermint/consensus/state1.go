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
func (bs *ConsensusState) LoadBlock(height uint64) *types.Block {

	cr := bs.GetChainReader()

	ethBlock := cr.GetBlock(common.Hash{}, height)
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

func (bs *ConsensusState) LoadLastTendermintExtra() (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit, current block height is %v\n", curHeight)
	if curHeight == 0 {
		return nil, 0
	}

	return bs.LoadTendermintExtra(curHeight)
}

func (bs *ConsensusState) LoadTendermintExtra(height uint64) (*types.TendermintExtra, uint64) {

	cr := bs.backend.ChainReader()

	fmt.Printf("(cs *ConsensusState) LoadSeenCommit height is %v\n", height)
	ethBlock := cr.GetBlockByNumber(height)
	if ethBlock == nil {
		fmt.Printf("(cs *ConsensusState) LoadSeenCommit nil block\n")
		return nil, 0
	}

	header := cr.GetHeader(ethBlock.Hash(), ethBlock.NumberU64())
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit header is %v\n", header)
	tdmExtra, err := types.ExtractTendermintExtra(header)
	if err != nil {
		fmt.Printf("(cs *ConsensusState) LoadSeenCommit got error: %v\n", err)
		return nil, 0
	}
	fmt.Printf("(cs *ConsensusState) LoadSeenCommit got error: %v\n", err)

	return tdmExtra, tdmExtra.Height
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
