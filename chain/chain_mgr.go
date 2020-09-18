package chain

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/pchain/p2p"
	"github.com/pchain/rpc"
	"github.com/pkg/errors"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"net"
	"path"
	"strconv"
	"sync"
)

type ChainManager struct {
	ctx *cli.Context

	mainChain     *Chain
	mainQuit      <-chan struct{}
	mainStartDone chan struct{}

	createChildChainLock sync.Mutex
	childChains          map[string]*Chain
	childQuits           map[string]<-chan struct{}

	stop chan struct{} // Channel wait for PCHAIN stop

	server *p2p.PChainP2PServer
	cch    *CrossChainHelper
}

var chainMgr *ChainManager
var once sync.Once

func GetCMInstance(ctx *cli.Context) *ChainManager {

	once.Do(func() {
		chainMgr = &ChainManager{ctx: ctx}
		chainMgr.stop = make(chan struct{})
		chainMgr.childChains = make(map[string]*Chain)
		chainMgr.childQuits = make(map[string]<-chan struct{})
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

	childChainIds := core.GetChildChainIds(cm.cch.chainInfoDB)
	log.Infof("Before Load Child Chains, childChainIds is %v, len is %d", childChainIds, len(childChainIds))

	readyToLoadChains := make(map[string]bool) // Key: Child Chain ID, Value: Enable Mining (deprecated)

	// Check we are belong to the validator of Child Chain in DB first (Mining Mode)
	for _, chainId := range childChainIds {
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

	for chainId := range readyToLoadChains {
		chain := LoadChildChain(cm.ctx, chainId)
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
	cm.cch.localTX3CacheDB, _ = rawdb.NewLevelDBDatabase(path.Join(cm.ctx.GlobalString(utils.DataDirFlag.Name), "tx3cache"), 0, 0, "pchain/db/tx3/")

	chainId := MainChain
	if cm.ctx.GlobalBool(utils.TestnetFlag.Name) {
		chainId = TestnetChain
	}
	cm.cch.mainChainId = chainId

	if cm.ctx.GlobalBool(utils.RPCEnabledFlag.Name) {
		host := "127.0.0.1" //cm.ctx.GlobalString(utils.RPCListenAddrFlag.Name)
		port := cm.ctx.GlobalInt(utils.RPCPortFlag.Name)
		url := net.JoinHostPort(host, strconv.Itoa(port))
		url = "http://" + url + "/" + chainId
		cm.cch.mainChainUrl = url
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
	cm.mainStartDone = make(chan struct{})

	cm.mainChain.EthNode.SetP2PServer(cm.server.Server())

	if address, ok := cm.getNodeValidator(cm.mainChain.EthNode); ok {
		cm.server.AddLocalValidator(cm.mainChain.Id, address)
	}

	err := StartChain(cm.ctx, cm.mainChain, cm.mainStartDone)

	// Wait for Main Chain Start Complete
	<-cm.mainStartDone
	cm.mainQuit = cm.mainChain.EthNode.StopChan()

	return err
}

func (cm *ChainManager) StartChains() error {

	for _, chain := range cm.childChains {
		// Start each Chain
		srv := cm.server.Server()
		childProtocols := chain.EthNode.GatherProtocols()
		// Add Child Protocols to P2P Server Protocols
		srv.Protocols = append(srv.Protocols, childProtocols...)
		// Add Child Protocols to P2P Server Caps
		srv.AddChildProtocolCaps(childProtocols)

		chain.EthNode.SetP2PServer(srv)

		if address, ok := cm.getNodeValidator(chain.EthNode); ok {
			cm.server.AddLocalValidator(chain.Id, address)
		}

		startDone := make(chan struct{})
		StartChain(cm.ctx, chain, startDone)
		<-startDone

		cm.childQuits[chain.Id] = chain.EthNode.StopChan()

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
		if rpc.IsHTTPRunning() {
			if h, err := cm.mainChain.EthNode.GetHTTPHandler(); err == nil {
				rpc.HookupHTTP(cm.mainChain.Id, h)
			} else {
				log.Errorf("Load Main Chain RPC HTTP handler failed: %v", err)
			}
			for _, chain := range cm.childChains {
				if h, err := chain.EthNode.GetHTTPHandler(); err == nil {
					rpc.HookupHTTP(chain.Id, h)
				} else {
					log.Errorf("Load Child Chain RPC HTTP handler failed: %v", err)
				}
			}
		}

		if rpc.IsWSRunning() {
			if h, err := cm.mainChain.EthNode.GetWSHandler(); err == nil {
				rpc.HookupWS(cm.mainChain.Id, h)
			} else {
				log.Errorf("Load Main Chain RPC WS handler failed: %v", err)
			}
			for _, chain := range cm.childChains {
				if h, err := chain.EthNode.GetWSHandler(); err == nil {
					rpc.HookupWS(chain.Id, h)
				} else {
					log.Errorf("Load Child Chain RPC WS handler failed: %v", err)
				}
			}
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

	var ethereum *eth.Ethereum
	cm.mainChain.EthNode.Service(&ethereum)

	var localEtherbase common.Address
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		localEtherbase = tdm.PrivateValidator()
	}

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

	// Write down the genesis into chain info db when exit the routine
	defer writeGenesisIntoChainInfoDB(cm.cch.chainInfoDB, chainId, validators)

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

	// Load the KeyStore file from MainChain (Optional)
	var keyJson []byte
	wallet, walletErr := cm.mainChain.EthNode.AccountManager().Find(accounts.Account{Address: localEtherbase})
	if walletErr == nil {
		var readKeyErr error
		keyJson, readKeyErr = ioutil.ReadFile(wallet.URL().Path)
		if readKeyErr != nil {
			log.Errorf("Failed to Read the KeyStore %v, Error: %v", localEtherbase, readKeyErr)
		}
	}

	// child chain uses the same validator with the main chain.
	privValidatorFile := cm.mainChain.Config.GetString("priv_validator_file")
	self := types.LoadPrivValidator(privValidatorFile)

	err := CreateChildChain(cm.ctx, chainId, *self, keyJson, validators)
	if err != nil {
		log.Errorf("Create Child Chain %v failed! %v", chainId, err)
		return
	}

	chain := LoadChildChain(cm.ctx, chainId)
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

	if address, ok := cm.getNodeValidator(chain.EthNode); ok {
		srv.AddLocalValidator(chain.Id, address)
	}

	// Start the new Child Chain, and it will start child chain reactors as well
	startDone := make(chan struct{})
	err = StartChain(cm.ctx, chain, startDone)
	<-startDone
	if err != nil {
		return
	}

	cm.childQuits[chain.Id] = chain.EthNode.StopChan()

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
	if rpc.IsHTTPRunning() {
		if h, err := chain.EthNode.GetHTTPHandler(); err == nil {
			rpc.HookupHTTP(chain.Id, h)
		} else {
			log.Errorf("Unable Hook up Child Chain (%v) RPC HTTP Handler: %v", chainId, err)
		}
	}
	if rpc.IsWSRunning() {
		if h, err := chain.EthNode.GetWSHandler(); err == nil {
			rpc.HookupWS(chain.Id, h)
		} else {
			log.Errorf("Unable Hook up Child Chain (%v) RPC WS Handler: %v", chainId, err)
		}
	}

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

	var localEtherbase common.Address
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		localEtherbase = tdm.PrivateValidator()
	}

	return childEpoch.Validators.HasAddress(localEtherbase[:])
}

func (cm *ChainManager) StopChain() {
	go func() {
		mainChainError := cm.mainChain.EthNode.Close()
		if mainChainError != nil {
			log.Error("Error when closing main chain", "err", mainChainError)
		} else {
			log.Info("Main Chain Closed")
		}
	}()
	for _, child := range cm.childChains {
		go func() {
			childChainError := child.EthNode.Close()
			if childChainError != nil {
				log.Error("Error when closing child chain", "child id", child.Id, "err", childChainError)
			}
		}()
	}
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
	cm.cch.localTX3CacheDB.Close()
	cm.cch.chainInfoDB.Close()

	// Release the main routine
	close(cm.stop)
}

func (cm *ChainManager) Wait() {
	<-cm.stop
}

func (cm *ChainManager) getNodeValidator(ethNode *node.Node) (common.Address, bool) {

	log.Debug("getNodeValidator")
	var ethereum *eth.Ethereum
	ethNode.Service(&ethereum)

	var etherbase common.Address
	if tdm, ok := ethereum.Engine().(consensus.Tendermint); ok {
		epoch := ethereum.Engine().(consensus.Tendermint).GetEpoch()
		etherbase = tdm.PrivateValidator()
		log.Debugf("getNodeValidator() etherbase is :%v", etherbase)
		return etherbase, epoch.Validators.HasAddress(etherbase[:])
	} else {
		return etherbase, false
	}

}

func writeGenesisIntoChainInfoDB(db dbm.DB, childChainId string, validators []types.GenesisValidator) {
	ethByte, _ := generateETHGenesis(childChainId, validators)
	tdmByte, _ := generateTDMGenesis(childChainId, validators)
	core.SaveChainGenesis(db, childChainId, ethByte, tdmByte)
}
