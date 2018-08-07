package main

import (
	"fmt"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
)

func pchainCmd(ctx *cli.Context) error {

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?\n")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")

	chainMgr := chain.GetCMInstance(ctx)

	err := chainMgr.StartP2P()
	if err != nil {
		fmt.Printf("start p2p failed\n")
		return nil
	}

	err = chainMgr.LoadAndStartMainChain()
	if err != nil {
		fmt.Printf("Load and start main chain failed. %v", err)
		return nil
	}

	// Load PChain Child Node
	err = chainMgr.LoadChains()
	if err != nil {
		fmt.Printf("load chains failed\n")
		return nil
	}

	err = chainMgr.StartChains()
	if err != nil {
		fmt.Printf("start chains failed\n")
		return nil
	}

	err = chainMgr.StartRPC()
	if err != nil {
		fmt.Printf("start rpc failed\n")
		return nil
	}

	chainMgr.StartInspectEvent()

	chainMgr.WaitChainsStop()

	chainMgr.Stop()

	return nil
}
