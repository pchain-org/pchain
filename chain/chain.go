package chain

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	eth "github.com/ethereum/go-ethereum/node"
	etmApp "github.com/pchain/ethermint/app"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"github.com/pchain/ethermint/ethereum"
	validatorsStrategy "github.com/pchain/ethermint/strategies/validators"
	tdm "github.com/pchain/ethermint/tendermint"
	"github.com/pchain/ethermint/version"
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	tdmTypes "github.com/tendermint/tendermint/types"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pchain/common/plogger"
	"github.com/pchain/p2p"
	"github.com/tendermint/go-rpc/server"
	"github.com/tendermint/tendermint/proxy"
	rpcTxHook "github.com/tendermint/tendermint/rpc/core/txhook"
)

const (
	// Client identifier to advertise over the network
	MainChain = "pchain"
)

var logger = plogger.GetLogger("chain")

type Chain struct {
	Id         string
	Config     cfg.Config
	EthNode    *eth.Node
	TdmNode    *tdm.Node
	AbciServer cmn.Service
	EtmApp     *etmApp.EthermintApplication
	RpcHandler http.Handler
}

func LoadMainChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	chain := &Chain{Id: chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	// Create Tendermint RPC Channel Listener first
	listener := rpcserver.NewChannelListener()

	//always start ethereum
	fmt.Println("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, listener, ctx, GetCMInstance(ctx).cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
	}

	chain.RpcHandler = rpcHandler

	consensus, err := getConsensus(config)
	if err != nil {
		cmn.Exit(cmn.Fmt("Couldn't get consensus with: %v", err))
	}
	fmt.Printf("consensus is: %s\n", consensus)

	if consensus != tdmTypes.CONSENSUS_POS {
		fmt.Println("consensus is not pos, so not start the pos prototol")
		return nil
	}

	fmt.Println("tm node")
	chain.TdmNode = MakeTendermintNode(config, pNode, listener, GetCMInstance(ctx).cch)

	return chain
}

func LoadChildChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	fmt.Printf("now load child: %s\n", chainId)

	chainDir := ChainDir(ctx, chainId)
	empty, err := cmn.IsDirEmpty(chainDir)
	fmt.Printf("chainDir is : %s, empty is %v\n", chainDir, empty)
	if empty || err != nil {
		fmt.Printf("directory %s not exist or with error %v\n", chainDir, err)
		return nil
	}
	chain := &Chain{Id: chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	// Create Tendermint RPC Channel Listener first
	listener := rpcserver.NewChannelListener()

	//always start ethereum
	fmt.Printf("chainId: %s, ethereum.MakeSystemNode", chainId)
	cch := GetCMInstance(ctx).cch
	stack := ethereum.MakeSystemNode(chainId, version.Version, listener, ctx, cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
		return nil
	}

	chain.RpcHandler = rpcHandler

	consensus, err := getConsensus(config)
	if err != nil {
		fmt.Printf("Couldn't get consensus with: %v\n", err)
		stack.Stop()
		return nil
	}
	fmt.Printf("consensus is: %s\n", consensus)

	if consensus != tdmTypes.CONSENSUS_POS {
		fmt.Println("consensus is not pos, so not start the pos prototol")
		return nil
	}

	fmt.Println("tm node")
	tdmNode := MakeTendermintNode(config, pNode, listener, cch)
	if tdmNode == nil {
		fmt.Println("make tendermint node failed")
		return nil
	}
	chain.TdmNode = tdmNode

	return chain
}

func StartChain(chain *Chain, startDone, quit chan int) error {

	fmt.Printf("start chain: %s\n", chain.Id)
	go func() {
		fmt.Println("ethermintCmd->utils.StartNode(stack)")
		utils.StartNode1(chain.EthNode)

		stack := chain.EthNode
		var backend *ethereum.Backend
		if err := stack.Service(&backend); err != nil {
			utils.Fatalf("backend service not running: %v", err)
		}
		client, err := stack.Attach()
		if err != nil {
			utils.Fatalf("Failed to attach to the inproc geth: %v", err)
		}

		ethereum.ReloadEthApi(stack, backend)

		testEthereumApi()

		//strategy := &emtTypes.Strategy{new(minerRewardStrategies.RewardConstant),nil}
		strategy := &validatorsStrategy.ValidatorsStrategy{}
		etmApp, err := etmApp.NewEthermintApplication(backend, client, strategy)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		chain.EtmApp = etmApp

		// Create ABCI Local Client Creator
		proxy.SetAppClientCreator(chain.TdmNode.ProxyApp(), proxy.NewLocalClientCreator(etmApp))

		/* ABCI Server is no longer required
		addr := config.GetString("proxy_app")
		abci := config.GetString("abci")
		abciServer, err := server.NewServer(addr, abci, etmApp)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		chain.AbciServer = abciServer
		*/

		fmt.Println("tm node")
		err = chain.TdmNode.OnStart1()
		if err != nil {
			cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
		}
		if startDone != nil {
			startDone <- 1
		}

		//fmt.Printf("Started node", "nodeInfo", chain.TdmNode.sw.NodeInfo())

		// Sleep forever and then...
		cmn.TrapSignal(func() {
			chain.TdmNode.Stop()
		})

		quit <- 1
	}()

	return nil
}

func getConsensus(config cfg.Config) (string, error) {

	genDocFile := config.GetString("genesis_file")
	var genDoc *tdmTypes.GenesisDoc = nil
	if !cmn.FileExists(genDocFile) {
		return "", errors.New("Couldn't read GenesisDoc file")
	}

	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return "", errors.New("Couldn't read GenesisDoc file")
	}

	genDoc, err = tdmTypes.GenesisDocFromJSON(jsonBlob)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Genesis doc parse json error: %v", err))
	}

	return genDoc.Consensus, nil
}

func testEthereumApi() {
	coinbase, err := ethereum.Coinbase()
	if err != nil {
		fmt.Printf("ethereum.Coinbase err with: %v\n", err)
		return
	}
	fmt.Printf("testEthereumApi: coinbase is: %x\n", coinbase)

	balance, err := ethereum.GetBalance(coinbase)
	if err != nil {
		fmt.Printf("ethereum.GetBalance err with: %v\n", err)
	}
	fmt.Printf("testEthereumApi: balance is: %x\n", balance)
}

func MakeTendermintNode(config cfg.Config, pNode *p2p.PChainP2P, cl *rpcserver.ChannelListener,
	cch rpcTxHook.CrossChainHelper) *tdm.Node {

	genDocFile := config.GetString("genesis_file")
	if !cmn.FileExists(genDocFile) {
		//log.Notice(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		fmt.Printf(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !cmn.FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				cmn.Exit(cmn.Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc, err := tdmTypes.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				cmn.PanicSanity(cmn.Fmt("Genesis doc parse json error: %v", err))
			}
			if genDoc.ChainID == "" {
				cmn.PanicSanity(cmn.Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	return tdm.NewNodeNotStart(config, pNode.Switch(), pNode.AddrBook(), cl, cch)
}

func CreateChildChain(ctx *cli.Context, chainId string, validator tdmTypes.PrivValidator, validators []tdmTypes.GenesisValidator) error {

	// Get Tendermint config base on chain id
	config := etm.GetTendermintConfig(chainId, ctx)

	// Save the Validator Json File
	privValFile := config.GetString("priv_validator_file_root")
	validator.LastHeight = 0
	validator.LastRound = 0
	validator.LastStep = 0
	validator.LastSignature = nil
	validator.LastSignBytes = nil
	validator.SetFile(privValFile + ".json")
	validator.Save()

	// Init the Ethereum Genesis
	err := initEthGenesisFromExistValidator(config, validators)
	if err != nil {
		return err
	}

	// Init the Ethereum Blockchain
	init_eth_blockchain(chainId, config.GetString("eth_genesis_file"), ctx)

	// Init the Tendermint Genesis
	init_em_files(config, chainId, config.GetString("eth_genesis_file"), validators)

	return nil
}
