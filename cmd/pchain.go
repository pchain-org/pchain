package main

import (
	"gopkg.in/urfave/cli.v1"
	"github.com/pchain/chain"
	"github.com/pchain/rpc"
	"fmt"
	"github.com/pchain/p2p"
	etm "github.com/pchain/ethermint/cmd/ethermint"
)

func pchainCmd(ctx *cli.Context) error {

	quit := make(chan int)

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?\n")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")

	// Start PChain P2P Node
	p2pConfig := etm.GetTendermintConfig(chain.MainChain, ctx)
	pchainP2P, p2pErr := p2p.StartP2P(p2pConfig)
	if p2pErr != nil {
		fmt.Printf("Failed to start PChain P2P node: %v", p2pErr)
		return nil
	}

	// Load PChain Node
	chains := make([]*chain.Chain, 0)
	mainChain := chain.LoadMainChain(ctx, chain.MainChain, pchainP2P)
	if mainChain == nil {
		fmt.Printf("main chain load failed\n")
		return nil
	}

	childId := "child0"
	childChain := chain.LoadChildChain(ctx, childId, pchainP2P)
	if childChain == nil {
		fmt.Printf("child chain load failed, try to create one\n")
		err := chain.CreateChildChain(ctx, childId, "{10000000000000000000000000000000000, 100}, {10000000000000000000000000000000000, 100}, { 10000000000000000000000000000000000, 100}")
		if err != nil {
			fmt.Printf("child chain creation failed\n")
		}

		childChain = chain.LoadChildChain(ctx, childId, pchainP2P)
		if childChain == nil {
			fmt.Printf("child chain load failed\n")
			return nil
		}
	}

	chains = append(chains, mainChain)
	chains = append(chains, childChain)



	// Start the PChain
	for _, c := range chains {
		chain.StartChain(c, quit)
	}




	rpc.StartRPC(ctx, chains)
	<- quit

	rpc.StopRPC()
	pchainP2P.StopP2P()

	return nil
}

