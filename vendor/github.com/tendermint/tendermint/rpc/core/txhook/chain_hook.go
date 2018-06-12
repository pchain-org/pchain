package core

import (
	"fmt"
)

var (
	WFCCFuncName string = "WithdrawFromChildChain"
)


func init() {
	RegisterCommitCb(WFCCFuncName, wfccCommitCb)
}

func wfccCommitCb(brCommit BrCommit) error {

	fmt.Println("wfccCommitCb")

	cch := brCommit.GetCrossChainHelper()
	block := brCommit.GetCurrentBlock()
	chainId := brCommit.GetChainId()
	return cch.SaveBlock(block, chainId)

}
