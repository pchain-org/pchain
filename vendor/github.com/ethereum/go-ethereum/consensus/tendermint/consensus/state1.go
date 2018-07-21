package consensus

import (
	"fmt"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	consss "github.com/ethereum/go-ethereum/consensus"
	//"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	sm "github.com/ethereum/go-ethereum/consensus/tendermint/state"
	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
)

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) GetChainReader() consss.ChainReader {
	return bs.backend.ChainReader()
}


//this function is called when the system starts or a block has been inserted into
//the insert could be self/other triggered
//anyway, we start/restart a new height with the latest block update
func (cs *ConsensusState) StartNewHeight() {

	//cs.timeoutTicker.Stop()

	//reload the block
	cr := cs.backend.ChainReader()
	curEthBlock := cr.CurrentBlock()
	curHeight := curEthBlock.NumberU64()
	fmt.Printf("(cs *ConsensusState) StartNewHeight, current block height is %v\n", curHeight)
	if curHeight != 0 {
		//reload
		state, epoch := cs.node.InitStateAndEpoch()
		cs.Initialize()
		cs.ApplyBlockEx(curEthBlock, state, epoch)
		cs.UpdateToStateAndEpoch(state, epoch)
	}

	cs.newStep()

	//cs.timeoutTicker.Start()

	cs.scheduleRound0(cs.GetRoundState())
}

func (bs *ConsensusState) ApplyBlockEx(block *ethTypes.Block, state *sm.State, epoch *ep.Epoch) error {
	return nil
}


// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *ConsensusState) LoadBlock(height uint64) *types.Block {

	cr := bs.GetChainReader()

	ethBlock := cr.GetBlockByNumber(height)
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
