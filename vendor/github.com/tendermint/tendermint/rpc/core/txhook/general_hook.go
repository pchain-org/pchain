package core

import (
	"fmt"
)

var (
	GHFuncName string = "GeneralHook"
)

func init() {
	RegisterCommitCb(GHFuncName, ghCommitCb)
}

func ghCommitCb(brCommit BrCommit) error {

	fmt.Println("ghCommitCb")
	chainID := brCommit.GetChainId()
	if chainID != "pchain" {
		block := brCommit.GetCurrentBlock()
		//means new epoch has been voted, should save the block to main chain
		if block.BlockExData != nil && len(block.BlockExData) != 0 {
			brCommit.SaveCurrentBlock2MainChain()
		}
	}
	return nil
}
