package chain

import (
	"net/http"
	tdm "github.com/ethereum/go-ethereum/consensus/tendermint"
	eth "github.com/ethereum/go-ethereum/node"
	etmApp "github.com/pchain/ethermint/app"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"gopkg.in/urfave/cli.v1"
	"fmt"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/cmd/utils"
	cfg "github.com/tendermint/go-config"
	"github.com/pchain/ethermint/ethereum"
	"github.com/pchain/ethermint/version"
	cmn "github.com/tendermint/go-common"

	"github.com/pchain/p2p"
)

const (
	// Client identifier to advertise over the network
	MainChain = "pchain"
)

type Chain struct{
	Id string
	Config cfg.Config
	EthNode *eth.Node
	TdmNode *tdm.Node
	AbciServer cmn.Service
	EtmApp  *etmApp.EthermintApplication
	RpcHandler http.Handler
}

func LoadMainChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	chain := &Chain {Id:chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	fmt.Println("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, pNode, GetCMInstance(ctx).cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
	}

	chain.RpcHandler = rpcHandler

	return chain
}

func LoadChildChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	fmt.Printf("now load child: %s\n", chainId)

	chainDir := ChainDir(ctx, chainId)
	empty, err :=cmn.IsDirEmpty(chainDir)
	fmt.Printf("chainDir is : %s, empty is %v\n", chainDir, empty)
	if empty || err != nil{
		fmt.Printf("directory %s not exist or with error %v\n", chainDir, err)
		return nil
	}
	chain := &Chain {Id:chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	fmt.Printf("chainId: %s, ethereum.MakeSystemNode", chainId)
	cch := GetCMInstance(ctx).cch
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, pNode, cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
		return nil
	}

	chain.RpcHandler = rpcHandler

	//set verbosity level for go-ethereum
	glog.SetToStderr(true)
	glog.SetV(ctx.GlobalInt(VerbosityFlag.Name))

	return chain
}

func StartChain(chain *Chain, quit chan int) error {

	fmt.Printf("start main chain: %s\n", chain.Id)
	go func(){
		fmt.Println("ethermintCmd->utils.StartNode(stack)")
		utils.StartNode1(chain.EthNode)

		/*
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
		*/
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
		/*
		fmt.Println("tm node")
		err = chain.TdmNode.OnStart1()
		if err != nil {
			cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
		}

		//fmt.Printf("Started node", "nodeInfo", chain.TdmNode.sw.NodeInfo())

		// Sleep forever and then...
		cmn.TrapSignal(func() {
			chain.TdmNode.Stop()
		})

		quit <- 1
		*/
	}()

	return nil
}

func testEthereumApi() {
	coinbase, err := ethereum.Coinbase()
	if(err != nil) {
		fmt.Printf("ethereum.Coinbase err with: %v\n", err)
		return
	}
	fmt.Printf("testEthereumApi: coinbase is: %x\n", coinbase)

	balance, err := ethereum.GetBalance(coinbase)
	if(err != nil) {
		fmt.Printf("ethereum.GetBalance err with: %v\n", err)
	}
	fmt.Printf("testEthereumApi: balance is: %x\n", balance)
}

func CreateChildChain(ctx *cli.Context, chainId string, balStr string) error{
	//validators: json format, like {[{pubkey: pk1, balance:b1, amount: am1},{pubkey: pk2, balance: b2, amount: am2}]}

	config := etm.GetTendermintConfig(chainId, ctx)
	err := init_eth_genesis(config, balStr)
	if err != nil {
		return err
	}

	init_cmd(ctx, config, chainId, config.GetString("eth_genesis_file"))

	return nil
}
