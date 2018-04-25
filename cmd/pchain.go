package main

import (
	"fmt"
	"gopkg.in/urfave/cli.v1"
	etm "github.com/pchain/ethermint/cmd/ethermint"
)

func pchainCmd(ctx *cli.Context) error {

	quit := make(chan int)

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")

	err := startMainChain(ctx, quit)
	if err != nil {
		fmt.Printf("start main chain failed with %v\n", err)
		return err
	}

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

	<- quit

	return nil
}


func startMainChain(ctx *cli.Context, quit chan int) error {

	fmt.Printf("start main chain\n")
	go etm.EthermintCmd(mainChain, ctx, quit)
	return nil
}

func startSideChain(ctx *cli.Context, chainId string, quit chan int) error {

	fmt.Printf("start side chain with %v\n", chainId)
	go etm.EthermintCmd(chainId, ctx, quit)
	return nil
}
