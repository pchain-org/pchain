package core

import (
	"fmt"

	. "github.com/tendermint/go-common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// TODO: limit/permission on (max - min)
func BlockchainInfo(context *RPCDataContext, minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	if maxHeight == 0 {
		maxHeight = context.blockStore.Height()
	} else {
		maxHeight = MinInt(context.blockStore.Height(), maxHeight)
	}
	if minHeight == 0 {
		minHeight = MaxInt(1, maxHeight-20)
	}
	logger.Debug("BlockchainInfoHandler", " maxHeight:", maxHeight, " minHeight:", minHeight)

	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := context.blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	return &ctypes.ResultBlockchainInfo{context.blockStore.Height(), blockMetas}, nil
}

//-----------------------------------------------------------------------------

func Block(context *RPCDataContext, height int) (*ctypes.ResultBlock, error) {
	if height == 0 {
		return nil, fmt.Errorf("Height must be greater than 0")
	}
	if height > context.blockStore.Height() {
		return nil, fmt.Errorf("Height must be less than the current blockchain height")
	}

	blockMeta := context.blockStore.LoadBlockMeta(height)
	block := context.blockStore.LoadBlock(height)
	return &ctypes.ResultBlock{blockMeta, block}, nil
}

//-----------------------------------------------------------------------------

func Commit(context *RPCDataContext, height int) (*ctypes.ResultCommit, error) {
	if height == 0 {
		return nil, fmt.Errorf("Height must be greater than 0")
	}
	storeHeight := context.blockStore.Height()
	if height > storeHeight {
		return nil, fmt.Errorf("Height must be less than or equal to the current blockchain height")
	}

	header := context.blockStore.LoadBlockMeta(height).Header

	// If the next block has not been committed yet,
	// use a non-canonical commit
	if height == storeHeight {
		commit := context.blockStore.LoadSeenCommit(height)
		return &ctypes.ResultCommit{header, commit, false}, nil
	}

	// Return the canonical commit (comes from the block at height+1)
	commit := context.blockStore.LoadBlockCommit(height)
	return &ctypes.ResultCommit{header, commit, true}, nil
}
