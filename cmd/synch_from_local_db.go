package main

import (
	"errors"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/consensus/pdbft"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

var (
	ErrNoChildChain = errors.New("no child chain")
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
			pdbft.RoughCheckSyncFlag,
			SourceDataDirFlag,
			utils.DataDirFlag,
		},
		Description: `
synchronize one chain from one local chain's directory, an alternative of network synchronization.`,
	}
)

var sldVars struct {
	chainId    string
	synChild   bool
	dataDir    string
	srcDataDir string

	srcCM, localCM               *chain.ChainManager
	srcMainChain, localMainChain *chain.Chain
	srcChain, localChain         *chain.Chain
	wg                           sync.WaitGroup
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

	sldVars.wg.Add(2)

	go localSync(sldVars.srcMainChain, sldVars.localMainChain, &sldVars.wg)

	if sldVars.synChild {
		go localSync(sldVars.srcChain, sldVars.localChain, &sldVars.wg)
	}

	sldVars.wg.Wait()

	return nil
}

func localSync(srcChain, dstChain *chain.Chain, wg *sync.WaitGroup) (err error) {

	srcEthChain := getEthBlockChain(srcChain)
	dstEthChain := mustHaveDstEthChain(srcChain, dstChain)

	defer func() {
		sldVars.wg.Done()
		if err != nil {
			log.Infof("localSync() for chain %v returned with error(%v)", dstEthChain.Config().PChainId, err)
		} else {
			log.Infof("localSync() for chain %v returned, synchronization finished", dstEthChain.Config().PChainId)
		}
	}()

	startWith := dstEthChain.CurrentBlock().NumberU64()
	startWith++

	for {
		blocks := fetch(srcEthChain, startWith, 500)
		count := len(blocks)
		if count == 0 {
			return nil
		}

		err = importBlockResult(dstEthChain, blocks)
		for err != nil && err == core.ErrKnownBlock {
			curBlock := dstEthChain.CurrentBlock().NumberU64()
			leftBocks := make(types.Blocks, 0)
			for _, block := range blocks {
				if block.NumberU64() > curBlock {
					leftBocks = append(leftBocks, block)
				}
			}
			blocks = leftBocks
			err = importBlockResult(dstEthChain, blocks)
		}

		if err != nil {
			return err
		}

		startWith += uint64(count)
	}

	return nil
}

func mustHaveDstEthChain(srcChain, dstChain *chain.Chain) *core.BlockChain {

	srcEthChain := getEthBlockChain(srcChain)
	dstEthChain := getEthBlockChain(dstChain)

	chainId := srcEthChain.Config().PChainId
	if params.IsMainChain(chainId) || dstEthChain != nil {
		return dstEthChain
	}

	for dstEthChain == nil {
		err := sldVars.localCM.LoadChains([]string{chainId /* = sldVars.chainId*/})
		if err != nil {
			log.Info("local chain may not created, wait for 10 seconds")
			time.Sleep(10 * time.Second)
			continue
		}

		childChain := sldVars.localCM.GetChildChain(chainId /* = sldVars.chainId*/)
		if childChain == nil {
			log.Info("local chain may not created, wait for 10 seconds")
			time.Sleep(10 * time.Second)
			continue
		}

		dstEthChain = getEthBlockChain(childChain)
		if dstEthChain != nil {
			childChain.EthNode.StartIPC()
		}
	}

	return dstEthChain
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

	sldVars.chainId = ctx.GlobalString(utils.ChainIdFlag.Name)
	sldVars.synChild = false
	sldVars.dataDir = ctx.GlobalString(utils.DataDirFlag.Name)
	sldVars.srcDataDir = ctx.GlobalString(SourceDataDirFlag.Name)

	log.Infof("chainId is %v", sldVars.chainId)
	log.Infof("data dir is %v", sldVars.dataDir)
	log.Infof("source data dir is %v", sldVars.srcDataDir)

	srcMainChain, srcChain, err := loadBlockChain(ctx, sldVars.chainId, sldVars.srcDataDir, false)
	if err != nil {
		return err
	}

	localMainChain, localChain, err := loadBlockChain(ctx, sldVars.chainId, sldVars.dataDir, true)
	if err != nil && err != ErrNoChildChain {
		return err
	}

	if srcMainChain == nil || srcChain == nil || localMainChain == nil /*|| localChain == nil*/ {
		return errors.New("some chain load failed")
	}

	ethMainChain := getEthBlockChain(localMainChain)
	sldVars.synChild = ethMainChain.Config().PChainId != sldVars.chainId

	sldVars.srcMainChain = srcMainChain
	sldVars.srcChain = srcChain
	sldVars.localMainChain = localMainChain
	sldVars.localChain = localChain

	return nil
}

func loadBlockChain(ctx *cli.Context, chainId, datadir string, isLocal bool) (*chain.Chain, *chain.Chain, error) {

	ctx.GlobalSet(utils.DataDirFlag.Name, datadir)

	mainChain := (*chain.Chain)(nil)
	childChain := (*chain.Chain)(nil)
	chainMgr := chain.NewCMInstance(ctx)
	err := chainMgr.LoadMainChain(ctx)

	defer func() {
		if isLocal && mainChain != nil {

			mainChain.EthNode.StartIPC()

			if childChain != nil {
				childChain.EthNode.StartIPC()
			}
		}
	}()

	if err != nil {
		return nil, nil, err
	}

	if !isLocal {
		sldVars.srcCM = chainMgr
	} else {
		sldVars.localCM = chainMgr
	}

	//set the event.TypeMutex to cch
	chainMgr.InitCrossChainHelper()
	chainMgr.StartInspectEvent()

	mainChain = chainMgr.GetMainChain()
	ethMainChain := getEthBlockChain(mainChain)

	if ethMainChain.Config().PChainId == chainId {
		return mainChain, mainChain, nil
	}

	err = chainMgr.LoadChains([]string{chainId})
	if err != nil {
		return mainChain, nil, err
	}

	childChain = chainMgr.GetChildChain(chainId)
	if childChain == nil {
		return mainChain, nil, ErrNoChildChain
	}

	return mainChain, childChain, err
}

func getEthBlockChain(chain *chain.Chain) *core.BlockChain {
	if chain == nil {
		return nil
	}

	var ethereum *eth.Ethereum
	chain.EthNode.ServiceNoRunning(&ethereum)
	return ethereum.BlockChain()
}
