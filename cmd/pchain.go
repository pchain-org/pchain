package main

import (
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"github.com/pchain/rpc"
	"github.com/pchain/chain"
)

func pchainCmd(ctx *cli.Context) error {

	quit := make(chan int)

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")

	var chains []*chain.Chain = make([]*chain.Chain, 0)
	mainChain := chain.LoadMainChain(ctx, mainChain)
	childChain := chain.LoadChildChain(ctx, "abcd")
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

