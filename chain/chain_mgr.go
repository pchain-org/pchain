package chain

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/p2p"
	"github.com/pchain/rpc"
	"github.com/pkg/errors"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
)

type ChainManager struct {
	ctx *cli.Context

	mainChain     *Chain
	mainQuit      chan int
	mainStartDone chan struct{}

	createChildChainLock sync.Mutex
	childChains          map[string]*Chain
	childQuits           map[string]chan int

	server *p2p.PChainP2PServer
	cch    *CrossChainHelper
}

var chainMgr *ChainManager
var once sync.Once

func GetCMInstance(ctx *cli.Context) *ChainManager {

	once.Do(func() {
		chainMgr = &ChainManager{ctx: ctx}
		chainMgr.childChains = make(map[string]*Chain)
		chainMgr.childQuits = make(map[string]chan int)
		chainMgr.cch = &CrossChainHelper{}
	})
	return chainMgr
}

func (cm *ChainManager) GetNodeID() string {
	return cm.server.Server().NodeInfo().ID
}

func (cm *ChainManager) InitP2P() {
	cm.server = p2p.NewP2PServer(cm.ctx)
}

func (cm *ChainManager) LoadMainChain(ctx *cli.Context) error {
	// Load Main Chain
	chainId := MainChain
	if ctx.GlobalBool(utils.TestnetFlag.Name) {
		chainId = TestnetChain
	}
	cm.mainChain = LoadMainChain(cm.ctx, chainId)
	if cm.mainChain == nil {
		return errors.New("Load main chain failed")
	}

	return nil
}

func (cm *ChainManager) LoadChains(childIds []string) error {

	// Wait for Main Chain Start Complete
	<-cm.mainStartDone

	childChainIds := core.GetChildChainIds(cm.cch.chainInfoDB)
	log.Infof("Before Load Child Chains, childChainIds is %v, len is %d", childChainIds, len(childChainIds))

	readyToLoadChains := make(map[string]bool) // Key: Child Chain ID, Value: Enable Mining

	// Check we are belong to the validator of Child Chain in DB first (Mining Mode)
	for _, chainId := range childChainIds {
		// TODO Check Validator Address in Tendermint
		// Check Current Validator is Child Chain Validator
		ci := core.GetChainInfo(cm.cch.chainInfoDB, chainId)
		// Check if we are in this child chain
		if ci.Epoch != nil && cm.checkCoinbaseInChildChain(ci.Epoch) {
			readyToLoadChains[chainId] = true
		}
	}

	// Check request from Child Chain
	for _, requestId := range childIds {
		if requestId == "" {
			// Ignore the Empty ID
			continue
		}

		if _, present := readyToLoadChains[requestId]; present {
			// Already loaded, ignore
			continue
		} else {
			// Launch in non-mining mode, including both correct and wrong chain id
			// Wrong chain id will be ignore after loading failed
			readyToLoadChains[requestId] = false
		}
	}

	log.Infof("Number of Child Chain to be load - %v", len(readyToLoadChains))
	log.Infof("Start to Load Child Chain - %v", readyToLoadChains)

	for chainId, mining := range readyToLoadChains {
		chain := LoadChildChain(cm.ctx, chainId, mining)
		if chain == nil {
			log.Errorf("Load Child Chain - %s Failed.", chainId)
			continue
		}

		cm.childChains[chainId] = chain
		log.Infof("Load Child Chain - %s Success!", chainId)
	}
	return nil
}

func (cm *ChainManager) InitCrossChainHelper() {
	cm.cch.chainInfoDB = dbm.NewDB("chaininfo",
		cm.mainChain.Config.GetString("db_backend"),
		cm.ctx.GlobalString(utils.DataDirFlag.Name))
	cm.cch.localTX3CacheDB, _ = ethdb.NewLDBDatabase(path.Join(cm.ctx.GlobalString(utils.DataDirFlag.Name), "tx3cache"), 0, 0)

	chainId := MainChain
	if cm.ctx.GlobalBool(utils.TestnetFlag.Name) {
		chainId = TestnetChain
	}
	if cm.ctx.GlobalBool(utils.RPCEnabledFlag.Name) {
		host := "127.0.0.1" //cm.ctx.GlobalString(utils.RPCListenAddrFlag.Name)
		port := cm.ctx.GlobalInt(utils.RPCPortFlag.Name)
		url := net.JoinHostPort(host, strconv.Itoa(port))
		url = "http://" + url + "/" + chainId
		client, err := ethclient.Dial(url)
		if err != nil {
			log.Errorf("can't connect to %s, err: %v, exit", url, err)
			os.Exit(0)
		}

		cm.cch.client = client
	}
}

func (cm *ChainManager) StartP2PServer() error {
	srv := cm.server.Server()
	// Append Main Chain Protocols
	srv.Protocols = append(srv.Protocols, cm.mainChain.EthNode.GatherProtocols()...)
	// Append Child Chain Protocols
	//for _, chain := range cm.childChains {
	//	srv.Protocols = append(srv.Protocols, chain.EthNode.GatherProtocols()...)
	//}
	// Start the server
	return srv.Start()
}

func (cm *ChainManager) StartMainChain() error {
	// Start the Main Chain
	cm.mainQuit = make(chan int)
	cm.mainStartDone = make(chan struct{})

	cm.mainChain.EthNode.SetP2PServer(cm.server.Server())
	err := StartChain(cm.ctx, cm.mainChain, cm.mainStartDone)
	return err
}

func (cm *ChainManager) StartChains() error {

	for _, chain := range cm.childChains {
		// Start each Chain
		quit := make(chan int)
		cm.childQuits[chain.Id] = quit

		srv := cm.server.Server()
		childProtocols := chain.EthNode.GatherProtocols()
		// Add Child Protocols to P2P Server Protocols
		srv.Protocols = append(srv.Protocols, childProtocols...)
		// Add Child Protocols to P2P Server Caps
		srv.AddChildProtocolCaps(childProtocols)

		chain.EthNode.SetP2PServer(srv)

		startDone := make(chan struct{})
		StartChain(cm.ctx, chain, startDone)
		<-startDone
		// Tell other peers that we have added into a new child chain
		cm.server.BroadcastNewChildChainMsg(chain.Id)
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

	createChildChainCh := make(chan core.CreateChildChainEvent, 10)
	createChildChainSub := MustGetEthereumFromNode(cm.mainChain.EthNode).BlockChain().SubscribeCreateChildChainEvent(createChildChainCh)

	go func() {
		defer createChildChainSub.Unsubscribe()

		for {
			select {
			case event := <-createChildChainCh:
				log.Infof("CreateChildChainEvent received: %v", event)

				go func() {
					cm.createChildChainLock.Lock()
					defer cm.createChildChainLock.Unlock()

					cm.LoadChildChainInRT(event.ChainId)
				}()
			case <-createChildChainSub.Err():
				return
			}
		}
	}()
}

func (cm *ChainManager) LoadChildChainInRT(chainId string) {

	// Load Child Chain data from pending data
	cci := core.GetPendingChildChainData(cm.cch.chainInfoDB, chainId)
	if cci == nil {
		log.Errorf("child chain: %s does not exist, can't load", chainId)
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
		if pubkey, ok := v.PubKey.(*crypto.BLSPubKey); ok {
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
		log.Warnf("You are not in the validators of child chain %v, no need to start the child chain", chainId)
		// Update Child Chain to formal
		cm.formalizeChildChain(chainId, *cci, nil)
		return
	}

	// if child chain already loaded, just return (For catch-up case)
	if _, ok := cm.childChains[chainId]; ok {
		log.Infof("Child Chain [%v] has been already loaded.", chainId)
		return
	}

	// Load the KeyStore file from MainChain
	wallet, walletErr := cm.mainChain.EthNode.AccountManager().Find(accounts.Account{Address: localEtherbase})
	if walletErr != nil {
		log.Errorf("Failed to Find the Account %v, Error: %v", localEtherbase, walletErr)
		return
	}
	keyJson, readKeyErr := ioutil.ReadFile(wallet.URL().Path)
	if readKeyErr != nil {
		log.Errorf("Failed to Read the KeyStore %v, Error: %v", localEtherbase, readKeyErr)
		return
	}

	// child chain uses the same validator with the main chain.
	privValidatorFile := cm.mainChain.Config.GetString("priv_validator_file")
	self := types.LoadPrivValidator(privValidatorFile)

	err := CreateChildChain(cm.ctx, chainId, *self, keyJson, validators)
	if err != nil {
		log.Errorf("Create Child Chain %v failed! %v", chainId, err)
		return
	}

	chain := LoadChildChain(cm.ctx, chainId, true)
	if chain == nil {
		log.Errorf("Child Chain %v load failed!", chainId)
		return
	}

	//StartChildChain to attach p2p and rpc
	//TODO Hookup new Created Child Chain to P2P server
	srv := cm.server.Server()
	childProtocols := chain.EthNode.GatherProtocols()
	// Add Child Protocols to P2P Server Protocols
	srv.Protocols = append(srv.Protocols, childProtocols...)
	// Add Child Protocols to P2P Server Caps
	srv.AddChildProtocolCaps(childProtocols)

	chain.EthNode.SetP2PServer(srv)

	// Start the new Child Chain, and it will start child chain reactors as well
	quit := make(chan int)
	cm.childQuits[chain.Id] = quit

	err = StartChain(cm.ctx, chain, nil)
	if err != nil {
		return
	}

	var childEthereum *eth.Ethereum
	chain.EthNode.Service(&childEthereum)
	firstEpoch := childEthereum.Engine().(consensus.Tendermint).GetEpoch()
	// Child Chain start success, then delete the pending data in chain info db
	cm.formalizeChildChain(chainId, *cci, firstEpoch)

	// Add Child Chain Id into Chain Manager
	cm.childChains[chainId] = chain

	//TODO Broadcast Child ID to all Main Chain peers
	go cm.server.BroadcastNewChildChainMsg(chainId)

	//hookup rpc
	rpc.Hookup(chain.Id, chain.RpcHandler)
}

func (cm *ChainManager) formalizeChildChain(chainId string, cci core.CoreChainInfo, ep *epoch.Epoch) {
	// Child Chain start success, then delete the pending data in chain info db
	core.DeletePendingChildChainData(cm.cch.chainInfoDB, chainId)
	// Convert the Chain Info from Pending to Formal
	core.SaveChainInfo(cm.cch.chainInfoDB, &core.ChainInfo{CoreChainInfo: cci, Epoch: ep})
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
	cm.server.Stop()
}
