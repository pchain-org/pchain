package main

import (
	"errors"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/consensus/pdbft"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"github.com/pchain/rpc"
	"gopkg.in/urfave/cli.v1"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
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
	dataDir    string
	srcDataDir string

	srcCM, localCM               *chain.ChainManager
	srcMainChain, localMainChain *core.BlockChain
	srcChain, localChain         *core.BlockChain
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

	if sldVars.localMainChain.Config().PChainId != sldVars.localChain.Config().PChainId {
		go localSync(sldVars.srcChain, sldVars.localChain, &sldVars.wg)
	}

	sldVars.wg.Wait()

	return nil
}

func localSync(srcChain, dstChain *core.BlockChain, wg *sync.WaitGroup) (err error) {

	defer func() {
		wg.Done()
		if err != nil {
			log.Infof("localSync() for chain %v returned with error(%v)", dstChain.Config().PChainId, err)
		} else {
			log.Infof("localSync() for chain %v returned, synchronization finished", dstChain.Config().PChainId)
		}
	}()

	dstChain = mustHaveDstChain(srcChain, dstChain)

	startWith := dstChain.CurrentBlock().NumberU64()
	startWith++

	for {
		blocks := fetch(srcChain, startWith, 256)
		count := len(blocks)
		if count == 0 {
			return nil
		}

		err = importBlockResult(dstChain, blocks)
		if err != nil {
			return err
		}
		startWith += uint64(count)
	}

	return nil
}

func mustHaveDstChain(srcChain, dstChain *core.BlockChain) *core.BlockChain {
	if srcChain.Config().PChainId == "pchain" || dstChain != nil {
		return dstChain
	}

	for dstChain == nil {
		err := sldVars.localCM.LoadChains([]string{srcChain.Config().PChainId /*sldVars.chainId*/})
		if err != nil {
			log.Info("local chain may not created, wait for 10 seconds")
			time.Sleep(10 * time.Second)
		}

		cmChildChain := sldVars.localCM.GetChildChain(srcChain.Config().PChainId /*sldVars.chainId*/)
		if cmChildChain == nil {
			log.Info("local chain may not created, wait for 10 seconds")
			time.Sleep(10 * time.Second)
		}

		var ethereum *eth.Ethereum
		cmChildChain.EthNode.ServiceNoRunning(&ethereum)
		dstChain = ethereum.BlockChain()

		if dstChain != nil {
			hookupChildRpc(cmChildChain)
		}
	}

	return dstChain
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
	if err != nil {
		return err
	}

	if srcMainChain == nil || srcChain == nil || localMainChain == nil /*|| localChain == nil*/ {
		return errors.New("some chain load failed")
	}

	sldVars.srcMainChain = srcMainChain
	sldVars.srcChain = srcChain
	sldVars.localMainChain = localMainChain
	sldVars.localChain = localChain

	return nil
}

func loadBlockChain(ctx *cli.Context, chainId, datadir string, isLocal bool) (*core.BlockChain, *core.BlockChain, error) {

	ctx.GlobalSet(utils.DataDirFlag.Name, datadir)

	chainMgr := chain.NewCMInstance(ctx)
	err := chainMgr.LoadMainChain(ctx)

	defer func() {
		if isLocal {
			err1 := chainMgr.StartRPC()
			if err1 != nil {
				log.Error("start rpc failed")
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

	var ethereum *eth.Ethereum
	chainMgr.GetMainChain().EthNode.ServiceNoRunning(&ethereum)
	mainChain := ethereum.BlockChain()

	if mainChain.Config().PChainId == chainId {
		return mainChain, mainChain, nil
	}

	err = chainMgr.LoadChains([]string{chainId})
	if err != nil {
		return mainChain, nil, err
	}

	cmChildChain := chainMgr.GetChildChain(chainId)
	if cmChildChain == nil {
		return mainChain, nil, errors.New("no child chain")
	}
	cmChildChain.EthNode.ServiceNoRunning(&ethereum)
	childChain := ethereum.BlockChain()

	return mainChain, childChain, err
}

func hookupChildRpc(chain *chain.Chain) {
	if rpc.IsHTTPRunning() {
		if h, err := chain.EthNode.GetHTTPHandler(); err == nil {
			rpc.HookupHTTP(chain.Id, h)
		} else {
			log.Errorf("Load Child Chain RPC HTTP handler failed: %v", err)
		}
	}
	if rpc.IsWSRunning() {
		if h, err := chain.EthNode.GetWSHandler(); err == nil {
			rpc.HookupWS(chain.Id, h)
		} else {
			log.Errorf("Load Child Chain RPC WS handler failed: %v", err)
		}
	}
}
