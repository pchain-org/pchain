package chain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/pchain/common/plogger"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"github.com/pchain/ethermint/ethereum"
	"github.com/pchain/p2p"
	"github.com/pchain/rpc"
	"github.com/pkg/errors"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/types"
	"gopkg.in/urfave/cli.v1"
	"math/big"
	"os"
)

var plog = plogger.GetLogger("ChainManager")

type ChainManager struct {
	ctx *cli.Context

	mainChain   *Chain
	childChains map[string]*Chain
	mainQuit    chan int
	childQuits  map[string]chan int
	p2pObj      *p2p.PChainP2P
	cch         *CrossChainHelper
}

var chainMgr *ChainManager

func GetCMInstance(ctx *cli.Context) *ChainManager {

	if chainMgr == nil {
		chainMgr = &ChainManager{ctx: ctx}
		chainMgr.childChains = make(map[string]*Chain)
		chainMgr.childQuits = make(map[string]chan int)
		chainMgr.cch = &CrossChainHelper{}
	}
	return chainMgr
}

func (cm *ChainManager) StartP2P() error {

	// Start PChain P2P Node
	mainChainConfig := etm.GetTendermintConfig(MainChain, cm.ctx)
	p2pObj, err := p2p.StartP2P(mainChainConfig)
	if err != nil {
		return err
	}

	cm.p2pObj = p2pObj
	return nil
}

func (cm *ChainManager) LoadChains() error {

	cm.mainChain = LoadMainChain(cm.ctx, MainChain, cm.p2pObj)
	if cm.mainChain == nil {
		return errors.New("Load main chain failed")
	}

	//set the event.TypeMutex to cch
	cm.InitCrossChainHelper(cm.mainChain.EthNode.EventMux())

	childChainIds := core.GetChildChainIds(cm.cch.chainInfoDB)
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

func (cm *ChainManager) InitCrossChainHelper(typeMut *event.TypeMux) {
	cm.cch.typeMut = typeMut
	cm.cch.chainInfoDB = dbm.NewDB("chaininfo",
		cm.mainChain.Config.GetString("db_backend"),
		cm.ctx.GlobalString(DataDirFlag.Name))
	client, err := ethclient.Dial("http://localhost:6969/pchain")
	if err != nil {
		fmt.Printf("can't connect to localhost:6969/pchain, exit")
		os.Exit(0)
	}

	cm.cch.client = client
}

func (cm *ChainManager) StartChains() error {

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

/*
func (cm *ChainManager) StartRPC1() error {

	ids := make([]string, 0)
	handlers := make([]http.Handler, 0)

	ids = append(ids, cm.mainChain.Id)
	handlers = append(handlers, cm.mainChain.RpcHandler)

	for _, chain := range cm.childChains {
		ids = append(ids, chain.Id)
		handlers = append(handlers, chain.RpcHandler)
	}

	// Start PChain RPC
	err := rpc.StartRPC1(cm.ctx, ids, handlers)
	if err != nil {
		return err
	}

	return nil
}
*/

func (cm *ChainManager) StartInspectEvent() {

	go func() {
		txSub := cm.mainChain.EthNode.EventMux().Subscribe(core.CreateChildChainEvent{})

		for obj := range txSub.Chan() {
			event := obj.Data.(core.CreateChildChainEvent)
			plog.Infof("CreateChildChainEvent received: %v\n", event)
			chainId := event.ChainId

			_, ok := cm.childChains[chainId]
			if ok {
				plog.Infof("CreateChildChainEvent has been received: %v, and chain has been loaded, just continue\n", event)
				continue
			}

			go cm.LoadChildChainInRT(event.ChainId)
		}
	}()
}

func (cm *ChainManager) LoadChildChainInRT(chainId string) {

	// Load Child Chain data from pending data
	cci := core.GetPendingChildChainData(cm.cch.chainInfoDB, chainId)
	if cci == nil {
		plog.Errorf("child chain: %s does not exist, can't load", chainId)
		return
	}

	validators := make([]types.GenesisValidator, 0, len(cci.JoinedValidators))

	validator := false
	coinbase, _ := ethereum.Coinbase()
	self := cm.mainChain.TdmNode.PrivValidator()
	var selfDeposit *big.Int
	for _, v := range cci.JoinedValidators {
		if v.Address == coinbase {
			validator = true
			selfDeposit = v.DepositAmount
		}

		// dereference the PubKey
		if pubkey, ok := v.PubKey.(*crypto.EtherumPubKey); ok {
			v.PubKey = *pubkey
		}

		// append the Validator
		validators = append(validators, types.GenesisValidator{
			EthAccount: v.Address,
			PubKey:     v.PubKey,
			Amount:     v.DepositAmount,
		})
	}

	if !validator {
		plog.Warnf("You are not in the validators of child chain %v, no need to start the child chain", chainId)
		return
	}

	chain := LoadChildChain(cm.ctx, chainId, cm.p2pObj)
	if chain == nil {
		// Find KeyStore URL for own myself
		wallet, wallet_err := cm.mainChain.EthNode.AccountManager().Find(accounts.Account{Address: coinbase})
		if wallet_err != nil {
			plog.Errorf("Create Child Chain %v failed! %v", chainId, wallet_err)
			return
		}

		mainChainKeyStorePath := cm.mainChain.Config.GetString("keystore")
		err := CreateChildChain(cm.ctx, chainId, mainChainKeyStorePath, wallet.URL().Path, *self, selfDeposit, validators)
		if err != nil {
			plog.Errorf("Create Child Chain %v failed! %v", chainId, err)
			return
		} else {
			// Create success, store the chain info into db
			core.SaveChainInfo(cm.cch.chainInfoDB, &core.ChainInfo{CoreChainInfo: *cci})

			core.DeletePendingChildChainData(cm.cch.chainInfoDB, chainId)
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

	<-quit
}

func (cm *ChainManager) WaitChainsStop() {

	<-cm.mainQuit
	for _, quit := range cm.childQuits {
		<-quit
	}
}

func (cm *ChainManager) Stop() {
	rpc.StopRPC()
	cm.p2pObj.StopP2P()
}
