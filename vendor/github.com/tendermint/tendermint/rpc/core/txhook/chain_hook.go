package core

import (
	"fmt"
)

var (
	WFCCFuncName                      string = "WithdrawFromChildChain"
	CheckAndLaunchChildChain_FuncName        = "CheckAndLaunchChildChain"
)

func init() {
	RegisterCommitCb(WFCCFuncName, wfccCommitCb)
	RegisterCommitCb(CheckAndLaunchChildChain_FuncName, checkAndLaunchChildChain)
}

func wfccCommitCb(brCommit BrCommit) error {

	fmt.Println("wfccCommitCb")
	brCommit.SaveCurrentBlock2MainChain()
	return nil
}

func checkAndLaunchChildChain(br BrCommit) error {
	if br.GetChainId() != "pchain" {
		return nil
	}

	plog.Debugln("CheckAndLaunchChildChain - start")

	cch := br.GetCrossChainHelper()

	// Invoke async start chain event
	cch.ReadyForLaunchChildChain(uint64(br.GetCurrentBlock().Height))

	plog.Debugln("CheckAndLaunchChildChain - end")
	return nil
}
