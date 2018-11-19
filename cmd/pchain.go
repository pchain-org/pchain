package main

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
	"strings"
	"github.com/ethereum/go-ethereum/consensus/tendermint/consensus"
)

func pchainCmd(ctx *cli.Context) error {

	if ctx == nil {
		log.Error("oh, ctx is null, how pchain works?")
		return nil
	}

	log.Info("Starting PChain...")
	log.Info("PChain supports large scale block-chain applications with multi-chain")

	chainMgr := chain.GetCMInstance(ctx)

	// ChildChainFlag flag
	requestChildChain := strings.Split(ctx.GlobalString(ChildChainFlag.Name), ",")

	// Initial P2P Server
	chainMgr.InitP2P()

	// Load Main Chain
	err := chainMgr.LoadMainChain(ctx)
	if err != nil {
		log.Errorf("Load Main Chain failed. %v", err)
		return nil
	}

	//set the event.TypeMutex to cch
	chainMgr.InitCrossChainHelper()

	// Start P2P Server
	err = chainMgr.StartP2PServer()
	if err != nil {
		log.Errorf("Start P2P Server failed. %v", err)
		return err
	}
	consensus.NodeID = chainMgr.GetNodeID()

	// Start Main Chain
	err = chainMgr.StartMainChain()

	// Load Child Chain
	err = chainMgr.LoadChains(requestChildChain)
	if err != nil {
		log.Errorf("Load Child Chains failed. %v", err)
		return err
	}

	// Start Child Chain
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
