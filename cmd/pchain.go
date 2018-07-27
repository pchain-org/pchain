package main

import (
	"gopkg.in/urfave/cli.v1"
	"github.com/pchain/chain"
	"fmt"
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
		return err
	}

	// Load PChain Node
	err = chainMgr.LoadChains()
	if err != nil {
		fmt.Printf("load chains failed\n")
		return err
	}

	err = chainMgr.StartEthP2P()
	if err != nil {
		fmt.Printf("start eth p2p failed\n")
		return err
	}

	err = chainMgr.StartChains()
	if err != nil {
		fmt.Printf("start chains failed\n")
		return err
	}

	err = chainMgr.StartRPC()
	if err != nil {
		fmt.Printf("start rpc failed\n")
		return err
	}

	chainMgr.StartInspectEvent()

	chainMgr.WaitChainsStop()

	chainMgr.Stop()

	return nil
}

