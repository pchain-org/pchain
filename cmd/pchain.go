package main

import (
	"gopkg.in/urfave/cli.v1"
	"github.com/pchain/chain"
	"github.com/pchain/rpc"
	"fmt"
)

func pchainCmd(ctx *cli.Context) error {

	quit := make(chan int)

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?\n")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")

	var chains []*chain.Chain = make([]*chain.Chain, 0)
	mainChain := chain.LoadMainChain(ctx, chain.MainChain)
	if mainChain == nil {
		fmt.Printf("main chain load failed\n")
		return nil
	}

	childId := "child0"
	childChain := chain.LoadChildChain(ctx, childId)
	if childChain == nil {
		fmt.Printf("child chain load failed, try to create one\n")
		err := chain.CreateChildChain(ctx, childId, "{10000000000000000000000000000000000, 100}, {10000000000000000000000000000000000, 100}, { 10000000000000000000000000000000000, 100}")
		if err != nil {
			fmt.Printf("child chain creation failed\n")
		}

		childChain = chain.LoadChildChain(ctx, childId)
		if childChain == nil {
			fmt.Printf("child chain load failed\n")
			return nil
		}
	}

	chains = append(chains, mainChain)
	chains = append(chains, childChain)

	chain.StartMainChain(ctx, mainChain, quit)
	chain.StartChildChain(ctx, childChain, quit)

	/*
	err := startMainChain(ctx, quit)
	if err != nil {
		fmt.Printf("start main chain failed with %v\n", err)
		return err
	}
	*/
	/*
	sideChains := []string{"a", "b"}
	for i:=0; i<len(sideChains); i++ {
		err = startSideChain(ctx, sideChains[i], quit)
		if err != nil {
			fmt.Printf("start main chain failed with %v\n", err)
			return err
		}
	}
	*/

	rpc.StartRPC(ctx, chains)

	<- quit

	rpc.StopRPC()

	return nil
}

