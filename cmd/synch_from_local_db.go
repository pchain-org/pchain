package main

import (
	"errors"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
	"net/http"
	_ "net/http/pprof"
)

var (
	SourceDataDirFlag = cli.StringFlag{
		Name:  "sourceDataDir",
		Usage: "source chain's data directory\n",
	}

	synchFromLocalDBCommand = cli.Command{
		Name:     "synchfromlocaldb",
		Usage:    "synchronize one chain from one local chain's directory",
		Category: "TOOLkIT COMMANDS",
		Action:   utils.MigrateFlags(synchFromLocalDB),
		Flags: []cli.Flag{
			utils.ChainIdFlag,
			utils.GCModeFlag,
			utils.RoughCheckSyncFlag,
			SourceDataDirFlag,
			utils.DataDirFlag,
		},
		Description: `
synchronize one chain from one local chain's directory, an alternative of network synchronization.`,
	}
)

var sldVars struct {
	chainId    string
	dataDir    string
	srcDataDir string

	srcMainChain, localMainChain *core.BlockChain
	srcChain, localChain         *core.BlockChain
	mainStop, stop               chan struct{}
}

func synchFromLocalDB(ctx *cli.Context) error {

	err := sfldInitEnv(ctx)
	if err != nil {
		return err
	}

	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Error("Failure in running pprof server", "err", err)
		}
	}()

	sldVars.mainStop = make(chan struct{})
	go localSync(sldVars.srcMainChain, sldVars.localMainChain, &sldVars.mainStop)

	if sldVars.localMainChain.Config().PChainId != sldVars.localChain.Config().PChainId {
		go localSync(sldVars.srcChain, sldVars.localChain, &sldVars.stop)
		<-sldVars.stop
	}

	<-sldVars.mainStop

	return nil
}

func localSync(srcChain, dstChain *core.BlockChain, stop *chan struct{}) error {

	done := false
	startWith := dstChain.CurrentBlock().NumberU64()
	startWith++

	for !done {
		blocks := fetch(srcChain, startWith, 256)
		count := len(blocks)
		if count == 0 {
			return nil
		}
		err := importBlockResult(dstChain, blocks)
		if err != nil {
			return err
		}
		startWith += uint64(count)
	}

	close(*stop)
	return nil
}

func fetch(srcChain *core.BlockChain, startWith uint64, count uint64) types.Blocks {

	srcCurrentBlock := srcChain.CurrentBlock()
	if srcCurrentBlock.NumberU64() < startWith {
		return nil
	}

	blocks := make([]*types.Block, 0)
	blockNum := startWith
	readCount := uint64(0)
	for ; readCount < count; readCount++ {
		block := srcChain.GetBlockByNumber(blockNum)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		blockNum++
	}

	return blocks
}

func importBlockResult(dstChain *core.BlockChain, blocks types.Blocks) error {

	if index, err := dstChain.InsertChain(blocks); err != nil {
		log.Info("Downloaded item processing failed", "number", blocks[index].NumberU64(), "hash", blocks[index].Hash(), "err", err)
		return err
	}
	return nil
}

func sfldInitEnv(ctx *cli.Context) error {

	common.RoughCheckSync = ctx.GlobalBool(utils.RoughCheckSyncFlag.Name)

	sldVars.chainId = ctx.GlobalString(utils.ChainIdFlag.Name)
	sldVars.dataDir = ctx.GlobalString(utils.DataDirFlag.Name)
	sldVars.srcDataDir = ctx.GlobalString(SourceDataDirFlag.Name)

	log.Infof("chainId is %v", sldVars.chainId)
	log.Infof("data dir is %v", sldVars.dataDir)
	log.Infof("source data dir is %v", sldVars.srcDataDir)

	srcMainChain, srcChain, err := loadBlockChain(ctx, sldVars.chainId, sldVars.srcDataDir)
	if err != nil {
		return err
	}

	localMainChain, localChain, err := loadBlockChain(ctx, sldVars.chainId, sldVars.dataDir)
	if err != nil {
		return err
	}

	if srcMainChain == nil || srcChain == nil || localMainChain == nil || localChain == nil {
		return errors.New("some chain load failed")
	}

	sldVars.srcMainChain = srcMainChain
	sldVars.srcChain = srcChain
	sldVars.localMainChain = localMainChain
	sldVars.localChain = localChain

	return nil
}

func loadBlockChain(ctx *cli.Context, chainId, datadir string) (*core.BlockChain, *core.BlockChain, error) {

	ctx.GlobalSet(utils.DataDirFlag.Name, datadir)

	chainMgr := chain.NewCMInstance(ctx)
	err := chainMgr.LoadMainChain(ctx)
	if err != nil {
		return nil, nil, err
	}

	//set the event.TypeMutex to cch
	chainMgr.InitCrossChainHelper()
	chainMgr.StartInspectEvent()

	var ethereum *eth.Ethereum
	chainMgr.GetMainChain().EthNode.ServiceNoRunning(&ethereum)
	mainChain := ethereum.BlockChain()

	if mainChain.Config().PChainId == chainId {
		return mainChain, mainChain, nil
	}

	err = chainMgr.LoadChains([]string{chainId})
	if err != nil {
		return nil, nil, err
	}

	cmChildChain := chainMgr.GetChildChain(chainId)
	if cmChildChain == nil {
		return nil, nil, errors.New("no child chain")
	}
	cmChildChain.EthNode.ServiceNoRunning(&ethereum)
	childChain := ethereum.BlockChain()

	return mainChain, childChain, err
}
