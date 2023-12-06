package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
)

var (
	dumpCommand = cli.Command{
		Name:   "dump",
		Usage:  "Dump accounts info",
		Action: utils.MigrateFlags(dump),
		Flags: []cli.Flag{
			utils.DataDirFlag,
			utils.ChainIdFlag,
		},
		Category: "CHAIN/DATA COMMANDS",
		Description: `
Dump all accounts infomation to json file.`,
	}
)

func dump(ctx *cli.Context) error {
	//make the log only output the error
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlError, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
	ctx.GlobalSet("verbosity", "1") //0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail

	if ctx == nil {
		log.Errorf("ctx is null, pchain does not work")
		return nil
	}

	chainId := ctx.GlobalString(utils.ChainIdFlag.Name)
	fmt.Printf("loading chain: %v\n", chainId)
	pchain := (*chain.Chain)(nil)
	if params.IsMainChain(chainId) {
		pchain = chain.LoadMainChain(ctx, chainId)
	} else {
		pchain = chain.LoadChildChain(ctx, chainId)
	}
	if pchain == nil {
		log.Errorf("Load Chain '%s' failed.", chainId)
		return nil
	}
	fmt.Printf("done.\n")

	var ethereum *eth.Ethereum
	pchain.EthNode.ServiceNoRunning(&ethereum)
	if ethereum == nil {
		log.Errorf("dump(), ethereum is nil")
		return errors.New("ethereum is nil")
	}

	chain := ethereum.BlockChain()
	curBlock := chain.CurrentBlock().NumberU64()
	if curBlock == uint64(0) {
		fmt.Printf("only one block, no need do snapshot\n")
		os.Exit(0)
	}

	if !continueWork(curBlock) {
		os.Exit(0)
	}

	stateDb, err := state.New(chain.CurrentBlock().Root(), chain.StateCache())
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("dump-%v-%v.csv", chain.Config().PChainId, curBlock)
	stateDb.RawDumpBalanceToFile(chain.CurrentBlock().NumberU64(), fileName)
	fmt.Printf("Done. Please find %v under current folder. \n", fileName)
	return nil
}
