package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/bridge"
	"github.com/ethereum/go-ethereum/consensus/pdbft/consensus"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	consensus.NodeID = chainMgr.GetNodeID()[0:16]

	// Start Main Chain
	err = chainMgr.StartMainChain()

	// Load Child Chain
	err = chainMgr.LoadChildChains(requestChildChain)
	if err != nil {
		log.Errorf("Load Child Chains failed. %v", err)
		return err
	}

	// Start Child Chain
	err = chainMgr.StartChildChains()
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

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")

		chainMgr.StopChain()
		chainMgr.WaitChainsStop()
		chainMgr.Stop()

		for i := 3; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.Info(fmt.Sprintf("Already shutting down, interrupt %d more times for panic.", i-1))
			}
		}
		bridge.Debug_Exit() // ensure trace and CPU profile data is flushed.
		bridge.Debug_LoadPanic("boom")
	}()

	chainMgr.Wait()

	return nil
}
