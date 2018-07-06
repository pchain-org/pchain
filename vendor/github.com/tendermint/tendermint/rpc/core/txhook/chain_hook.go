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
	brCommit.SaveCurrentBlock2MainChain()
	return nil
}
