package main

import (
	"github.com/pchain/chain"
	"github.com/pchain/common/plogger"
	"gopkg.in/urfave/cli.v1"
)

var logger = plogger.GetLogger("main")

func pchainCmd(ctx *cli.Context) error {

	if ctx == nil {
		logger.Errorln("oh, ctx is null, how PChain works?")
		return nil
	}

	logger.Infoln("Starting PChain...")
	logger.Infoln("PChain supports large scale block-chain applications with multi-chain")

	chainMgr := chain.GetCMInstance(ctx)

	err := chainMgr.StartP2P()
	if err != nil {
		logger.Errorln("start p2p failed")
		return nil
	}

	err = chainMgr.LoadAndStartMainChain()
	if err != nil {
		logger.Errorf("Load and start main chain failed. %v", err)
		return nil
	}

	// Load PChain Child Node
	err = chainMgr.LoadChains()
	if err != nil {
		logger.Errorln("load chains failed")
		return nil
	}

	err = chainMgr.StartChains()
	if err != nil {
		logger.Errorln("start chains failed")
		return nil
	}

	err = chainMgr.StartRPC()
	if err != nil {
		logger.Errorln("start rpc failed")
		return nil
	}

	chainMgr.StartInspectEvent()

	chainMgr.WaitChainsStop()

	chainMgr.Stop()

	return nil
}
