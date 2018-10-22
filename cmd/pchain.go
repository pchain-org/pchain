package main

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
)

func pchainCmd(ctx *cli.Context) error {

	if ctx == nil {
		log.Error("oh, ctx is null, how pchain works?")
		return nil
	}

	log.Info("Starting PChain...")
	log.Info("PChain supports large scale block-chain applications with multi-chain")

	chainMgr := chain.GetCMInstance(ctx)

	err := chainMgr.StartP2P()
	if err != nil {
		log.Error("start p2p failed")
		return err
	}

	err = chainMgr.LoadAndStartMainChain(ctx)
	if err != nil {
		log.Errorf("Load and start main chain failed. %v", err)
		return nil
	}

	// Load PChain Child Node
	err = chainMgr.LoadChains()
	if err != nil {
		log.Error("load chains failed")
		return err
	}

	err = chainMgr.StartChains()
	if err != nil {
		log.Error("start chains failed")
		return err
	}

	err = chainMgr.StartRPC()
	if err != nil {
		log.Error("start rpc failed")
		return err
	}

	chainMgr.StartInspectEvent()

	chainMgr.WaitChainsStop()

	chainMgr.Stop()

	return nil
}
