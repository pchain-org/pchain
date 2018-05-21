package chain

import (
	"gopkg.in/urfave/cli.v1"
	"github.com/pkg/errors"
	"github.com/pchain/p2p"
	"github.com/pchain/rpc"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"github.com/ethereum/go-ethereum/core"
	"fmt"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/common"
	dbm "github.com/tendermint/go-db"
)

type ChainManager struct {

	ctx *cli.Context

	mainChain   *Chain
	childChains map[string]*Chain
	mainQuit    chan int
	childQuits  map[string]chan int
	p2pObj	*p2p.PChainP2P
	cch 	*CrossChainHelper
	//leger
}

var chainMgr *ChainManager

func GetCMInstance(ctx *cli.Context) *ChainManager{

	if chainMgr == nil {
		chainMgr = &ChainManager{ctx:ctx}
		chainMgr.childChains = make(map[string]*Chain)
		chainMgr.childQuits = make(map[string]chan int)
		chainMgr.cch = &CrossChainHelper{}
	}
	return chainMgr
}

func (cm *ChainManager)StartP2P() error {

	// Start PChain P2P Node
	mainChainConfig := etm.GetTendermintConfig(MainChain, cm.ctx)
	p2pObj, err := p2p.StartP2P(mainChainConfig)
	if err != nil {
		return err
	}

	cm.p2pObj = p2pObj
	return nil
}

func (cm *ChainManager)LoadChains() error {

	cm.mainChain = LoadMainChain(cm.ctx, MainChain, cm.p2pObj)
	if cm.mainChain == nil {
		return errors.New("Load main chain failed")
	}

	//set the event.TypeMutex to cch
	cm.InitCrossChainHelper(cm.mainChain.EthNode.EventMux())

	childChainIds := GetChildChainIds(cm.cch.chainInfoDB)
	fmt.Printf("LoadChains 0, childChainIds is %v, len is %d\n", childChainIds, len(childChainIds))

	for _, chainId := range childChainIds {

		chain := LoadChildChain(cm.ctx, chainId, cm.p2pObj)
		if chain == nil {
			return errors.New("load child chain failed")
		}

		cm.childChains[chainId] = chain
	}

	return nil
}

func (cm *ChainManager)InitCrossChainHelper(typeMut *event.TypeMux) {
	cm.cch.typeMut = typeMut
	cm.cch.chainInfoDB = dbm.NewDB("chaininfo",
					cm.mainChain.Config.GetString("db_backend"),
					cm.ctx.GlobalString(DataDirFlag.Name))
}

func (cm *ChainManager)StartChains() error{

	cm.p2pObj.AddNetwork(cm.mainChain.Id)
	cm.mainQuit = make(chan int)
	err := StartChain(cm.mainChain, cm.mainQuit)
	if err != nil {
		return err
	}

	for _, chain := range cm.childChains {
		// Add Chain ID to NodeInfo first
		cm.p2pObj.AddNetwork(chain.Id)
		// Start each Chain
		quit := make(chan int)
		cm.childQuits[chain.Id] = quit
		err = StartChain(chain, quit)
		if err != nil {
			return err
		}
	}

	// Dial the Seeds after network has been added into NodeInfo
	mainChainConfig := etm.GetTendermintConfig(cm.mainChain.Id, cm.ctx)
	err = cm.p2pObj.DialSeeds(mainChainConfig)
	if err != nil {
		return err
	}

	return nil
}

func (cm *ChainManager) StartRPC() error {

	// Start PChain RPC
	err := rpc.StartRPC(cm.ctx)
	if err != nil {
		return err
	} else {
		rpc.Hookup(cm.mainChain.Id, cm.mainChain.RpcHandler)
		for _, chain := range cm.childChains {
			rpc.Hookup(chain.Id, chain.RpcHandler)
		}
	}

	return nil
}


func (cm *ChainManager) StartInspectEvent() {

	eventMap := make(map[string]bool)

	go func() {
		txSub := cm.mainChain.EthNode.EventMux().Subscribe(core.CreateChildChainEvent{})

		for obj := range txSub.Chan() {
			event := obj.Data.(core.CreateChildChainEvent)
			fmt.Printf("CreateChildChainEvent received: %v\n", event)
			chainId := event.ChainId
			_, ok := eventMap[chainId]
			if ok {
				fmt.Printf("CreateChildChainEvent has been received: %v, just continue\n", event)
				continue
			}
			eventMap[chainId] = true

			_, ok = cm.childChains[chainId]
			if ok {
				fmt.Printf("CreateChildChainEvent has been received, and chain has been loaded, just continue\n", event)
				continue
			}

			go cm.LoadChildChainInRT(event.From, event.ChainId)
		}
	}()
}

func (cm *ChainManager) LoadChildChainInRT(from common.Address, chainId string) {

	//LoadChildChain if from and chainId matches
	ci := GetChainInfo(cm.cch.chainInfoDB, chainId)
	if ci == nil {
		fmt.Printf("child chain: %s does not exist, can't load", chainId)
		return
	}

	chain := LoadChildChain(cm.ctx, chainId, cm.p2pObj)
	if chain == nil {
		fmt.Printf("child chain load failed, try to create one\n")
		err := CreateChildChain(cm.ctx, chainId, "{10000000000000000000000000000000000, 100}, {10000000000000000000000000000000000, 100}, { 10000000000000000000000000000000000, 100}")
		if err != nil {
			fmt.Printf("child chain creation failed\n")
		}

		chain = LoadChildChain(cm.ctx, chainId, cm.p2pObj)
		if chain == nil {
			fmt.Printf("child chain load failed\n")
			return
		}
	}

	cm.childChains[chainId] = chain

	//StartChildChain to attach p2p and rpc
	cm.p2pObj.AddNetwork(chain.Id)
	// Start each Chain
	quit := make(chan int)
	cm.childQuits[chain.Id] = quit
	err := StartChain(chain, quit)
	if err != nil {
		return
	}

	//hookup rpc
	rpc.Hookup(chain.Id, chain.RpcHandler)

	<- quit
}

func (cm *ChainManager) WaitChainsStop() {

	<- cm.mainQuit
	for _, quit := range cm.childQuits {
		<- quit
	}
}

func (cm *ChainManager) Stop() {
	rpc.StopRPC()
	cm.p2pObj.StopP2P()
}

