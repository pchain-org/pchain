package utils

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/node"
	"gopkg.in/urfave/cli.v1"
	//"github.com/ethereum/go-ethereum/accounts/keystore"
	//"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/eth"
	//"github.com/ethereum/go-ethereum/ethclient"
	//"github.com/ethereum/go-ethereum/console"
)

func StartNodeEx(ctx *cli.Context, stack *node.Node) error {

	log.Debug("StartNode->stack.Start()")
	if err := stack.StartEx(); err != nil {
		Fatalf("Error starting protocol stack: %v", err)
	}

	// Mine or not?
	mining := false
	var ethereum *eth.Ethereum
	if err := stack.Service(&ethereum); err == nil {
		if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
			mining = tdm.ShouldStart()
			if mining {
				stack.GetLogger().Info("PDBFT Consensus Engine will be start shortly")
			}
		}
	}

	// Start auxiliary services if enabled
	if mining || ctx.GlobalBool(DeveloperFlag.Name) {
		// Mining only makes sense if a full Ethereum node is running
		if ctx.GlobalBool(LightModeFlag.Name) || ctx.GlobalString(SyncModeFlag.Name) == "light" {
			Fatalf("Light clients do not support mining")
		}
		var ethereum *eth.Ethereum
		if err := stack.Service(&ethereum); err != nil {
			Fatalf("Ethereum service not running: %v", err)
		}

		// Use a reduced number of threads if requested
		if threads := ctx.GlobalInt(MinerThreadsFlag.Name); threads > 0 {
			type threaded interface {
				SetThreads(threads int)
			}
			if th, ok := ethereum.Engine().(threaded); ok {
				th.SetThreads(threads)
			}
		}
		// Set the gas price to the limits from the CLI and start mining
		ethereum.TxPool().SetGasPrice(GlobalBig(ctx, MinerGasPriceFlag.Name))
		if err := ethereum.StartMining(true); err != nil {
			Fatalf("Failed to start mining: %v", err)
		}
	}

	return nil
}
