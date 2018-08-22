package chain

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/pchain/p2p"
	"github.com/pchain/rpc"
	"github.com/pkg/errors"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"gopkg.in/urfave/cli.v1"
	"net"
	"os"
	"strconv"
)

type ChainManager struct {
	ctx *cli.Context

	mainChain     *Chain
	mainQuit      chan int
	mainStartDone chan int

	childChains map[string]*Chain
	childQuits  map[string]chan int

	p2pObj *p2p.PChainP2P
	ethP2P *p2p.EthP2PServer
	cch    *CrossChainHelper
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
	mainChainConfig := GetTendermintConfig(MainChain, cm.ctx)
	p2pObj, err := p2p.StartP2P(mainChainConfig)
	if err != nil {
		return err
	}

	cm.p2pObj = p2pObj
	return nil
}

func (cm *ChainManager) LoadAndStartMainChain(ctx *cli.Context) error {
	// Load Main Chain
	cm.mainChain = LoadMainChain(cm.ctx, MainChain, cm.p2pObj)
	if cm.mainChain == nil {
		return errors.New("Load main chain failed")
	}

	//set the event.TypeMutex to cch
	cm.InitCrossChainHelper(cm.mainChain.EthNode.EventMux())

	// ethereum p2p needs to be started after loading the main chain.
	cm.StartEthP2P()

	// Start the Main Chain
	cm.mainQuit = make(chan int)
	cm.mainStartDone = make(chan int)
	err := StartChain(ctx, cm.mainChain, cm.mainStartDone, cm.mainQuit)
	return err
}

func (cm *ChainManager) LoadChains() error {

	// Wait for Main Chain Start Complete
	<-cm.mainStartDone

	childChainIds := core.GetChildChainIds(cm.cch.chainInfoDB)
	logger.Printf("Before Load Child Chains, childChainIds is %v, len is %d", childChainIds, len(childChainIds))

	for _, chainId := range childChainIds {
		ci := core.GetChainInfo(cm.cch.chainInfoDB, chainId)
		// Check if we are in this child chain
		if ci.Epoch == nil || !cm.checkCoinbaseInChildChain(ci.Epoch) {
			continue
		}

		logger.Infof("Start to Load Child Chain - %s", chainId)
		chain := LoadChildChain(cm.ctx, chainId, cm.p2pObj)
		if chain == nil {
			logger.Errorf("Load Child Chain - %s Failed.", chainId)
			continue
		}

		cm.childChains[chainId] = chain
		logger.Infof("Load Child Chain - %s Success!", chainId)
	}

	return nil
}

func (cm *ChainManager) InitCrossChainHelper(typeMut *event.TypeMux) {
	cm.cch.typeMut = typeMut
	cm.cch.chainInfoDB = dbm.NewDB("chaininfo",
		cm.mainChain.Config.GetString("db_backend"),
		cm.ctx.GlobalString(DataDirFlag.Name))
	if cm.ctx.GlobalBool(utils.RPCEnabledFlag.Name) {
		host := "127.0.0.1" //cm.ctx.GlobalString(utils.RPCListenAddrFlag.Name)
		port := cm.ctx.GlobalInt(utils.RPCPortFlag.Name)
		url := net.JoinHostPort(host, strconv.Itoa(port))
		url = "http://" + url + "/pchain"
		client, err := ethclient.Dial(url)
		if err != nil {
			logger.Errorf("can't connect to %s, err: %v, exit", url, err)
			os.Exit(0)
		}

		cm.cch.client = client
	}
}

func (cm *ChainManager) StartChains() error {

	for _, chain := range cm.childChains {
		// Start each Chain
		quit := make(chan int)
		cm.childQuits[chain.Id] = quit
		err := StartChain(cm.ctx, chain, nil, quit)
		if err != nil {
			return err
		}
	}

	// Dial the Seeds after network has been added into NodeInfo
	mainChainConfig := GetTendermintConfig(cm.mainChain.Id, cm.ctx)
	err := cm.p2pObj.DialSeeds(mainChainConfig)
	if err != nil {
		return err
	}

	return nil
}

func (cm *ChainManager) StartEthP2P() error {

	cm.ethP2P = p2p.NewEthP2PServer(cm.mainChain.EthNode)
	if cm.ethP2P == nil {
		return errors.New("p2p server is empty after creation")
	}

	for _, chain := range cm.childChains {
		cm.ethP2P.AddNodeConfig(chain.Id, chain.EthNode)
	}

	return cm.ethP2P.Start()
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

	go func() {
		createChildChainSub := cm.mainChain.EthNode.EventMux().Subscribe(core.CreateChildChainEvent{})

		for obj := range createChildChainSub.Chan() {
			event := obj.Data.(core.CreateChildChainEvent)
			logger.Infof("CreateChildChainEvent received: %v", event)
			chainId := event.ChainId

			_, ok := cm.childChains[chainId]
			if ok {
				logger.Infof("CreateChildChainEvent has been received: %v, and chain has been loaded, just continue", event)
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
		logger.Errorf("child chain: %s does not exist, can't load", chainId)
		return
	}

	validators := make([]types.GenesisValidator, 0, len(cci.JoinedValidators))

	validator := false

	// TODO CHECK Ethereum backend
	var ethereum *eth.Ethereum
	cm.mainChain.EthNode.Service(&ethereum)
	localEtherbase, _ := ethereum.Etherbase()

	for _, v := range cci.JoinedValidators {
		if v.Address == localEtherbase {
			validator = true
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
		logger.Warnf("You are not in the validators of child chain %v, no need to start the child chain", chainId)
		return
	}

	chain := LoadChildChain(cm.ctx, chainId, cm.p2pObj)
	if chain == nil {
		// child chain uses the same validator with the main chain.
		privValidatorFile := cm.mainChain.Config.GetString("priv_validator_file")
		keydir := cm.mainChain.Config.GetString("keystore")
		self := types.LoadOrGenPrivValidator(privValidatorFile, keydir)
		err := CreateChildChain(cm.ctx, chainId, *self, validators)
		if err != nil {
			logger.Errorf("Create Child Chain %v failed! %v", chainId, err)
			return
		}

		chain = LoadChildChain(cm.ctx, chainId, cm.p2pObj)
		if chain == nil {
			logger.Errorf("Child Chain %v load failed!", chainId)
			return
		}
	}

	cm.childChains[chainId] = chain

	//StartChildChain to attach p2p and rpc

	cm.ethP2P.Hookup(chain.Id, chain.EthNode)

	// Start the new Child Chain, and it will start child chain reactors as well
	quit := make(chan int)
	cm.childQuits[chain.Id] = quit
	err := StartChain(cm.ctx, chain, nil, quit)
	if err != nil {
		return
	}

	// Child Chain start success, then delete the pending data in chain info db
	core.DeletePendingChildChainData(cm.cch.chainInfoDB, chainId)

	// Broadcast Child ID to all peers
	cm.p2pObj.BroadcastChildChainID(chainId)

	//hookup rpc
	rpc.Hookup(chain.Id, chain.RpcHandler)

	<-quit
}

func (cm *ChainManager) checkCoinbaseInChildChain(childEpoch *epoch.Epoch) bool {
	var ethereum *eth.Ethereum
	cm.mainChain.EthNode.Service(&ethereum)
	localEtherbase, _ := ethereum.Etherbase()

	return childEpoch.Validators.HasAddress(localEtherbase[:])
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
	cm.ethP2P.Stop()
}
