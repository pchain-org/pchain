package chain

import (
	"net/http"
	tdm "github.com/pchain/ethermint/tendermint"
	eth "github.com/ethereum/go-ethereum/node"
	etmApp "github.com/pchain/ethermint/app"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"gopkg.in/urfave/cli.v1"
	"fmt"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"os"
	"github.com/tendermint/abci/server"
	cfg "github.com/tendermint/go-config"
	"github.com/pchain/ethermint/ethereum"
	"github.com/pchain/ethermint/version"
	"io/ioutil"
	tdmTypes "github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/go-common"
	"github.com/syndtr/goleveldb/leveldb/errors"
	validatorsStrategy "github.com/pchain/ethermint/strategies/validators"
	"time"

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

func LoadMainChain(ctx *cli.Context, chainId string) *Chain {

	chain := &Chain {Id:chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	fmt.Println("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, config.GetString(RpcLaddrFlag.Name), ctx)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
	}

	chain.RpcHandler = rpcHandler


	consensus, err := getConsensus(config)
	if(err != nil) {
		cmn.Exit(cmn.Fmt("Couldn't get consensus with: %v", err))
	}
	fmt.Printf("consensus is: %s\n", consensus)

	if (consensus != tdmTypes.CONSENSUS_POS) {
		fmt.Println("consensus is not pos, so not start the pos prototol")
		return nil
	}

	//set verbosity level for go-ethereum
	glog.SetToStderr(true)
	glog.SetV(ctx.GlobalInt(VerbosityFlag.Name))

	fmt.Println("tm node")
	chain.TdmNode = MakeTendermintNode(config)

	return chain
}

func LoadChildChain(ctx *cli.Context, chainId string) *Chain {

	fmt.Printf("now load child: %s\n", chainId)

	chainDir := ChainDir(ctx, chainId)
	empty, err :=cmn.IsDirEmpty(chainDir)
	if empty || err != nil{
		fmt.Printf("directory %s not exist or with error %v\n", chainDir, err)
		return nil
	}
	chain := &Chain {Id:chainId}
	config := etm.GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	fmt.Println("chainId: %s, ethereum.MakeSystemNode", chainId)
	stack := ethereum.MakeSystemNode(chainId, version.Version, config.GetString(RpcLaddrFlag.Name), ctx)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		fmt.Println("rpc_handler got failed, return")
		return nil
	}

	chain.RpcHandler = rpcHandler

	consensus, err := getConsensus(config)
	if(err != nil) {
		fmt.Printf("Couldn't get consensus with: %v\n", err)
		stack.Stop()
		return nil
	}
	fmt.Printf("consensus is: %s\n", consensus)

	if (consensus != tdmTypes.CONSENSUS_POS) {
		fmt.Println("consensus is not pos, so not start the pos prototol")
		return nil
	}

	//set verbosity level for go-ethereum
	glog.SetToStderr(true)
	glog.SetV(ctx.GlobalInt(VerbosityFlag.Name))

	fmt.Println("tm node")
	tdmNode := MakeTendermintNode(config)
	if tdmNode == nil {
		fmt.Println("make tendermint node failed")
		return nil
	}
	chain.TdmNode = tdmNode

	return chain
}


func StartMainChain(ctx *cli.Context, chain *Chain, quit chan int) error {

	fmt.Printf("start main chain: %s\n", chain.Id)
	go func(){
		fmt.Println("ethermintCmd->utils.StartNode(stack)")
		utils.StartNode1(chain.EthNode)


		config := chain.Config
		addr := config.GetString("proxy_app")
		abci := config.GetString("abci")

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

		abciServer, err := server.NewServer(addr, abci, etmApp)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		chain.AbciServer = abciServer

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
	}()


	return nil
}

func StartChildChain(ctx *cli.Context, chain *Chain, quit chan int) error {

	fmt.Printf("start child chain: %s\n", chain.Id)
	go func(){
		fmt.Println("ethermintCmd->utils.StartNode(stack)")
		utils.StartNode1(chain.EthNode)

		config := chain.Config
		addr := config.GetString("proxy_app")
		abci := config.GetString("abci")

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

		abciServer, err := server.NewServer(addr, abci, etmApp)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		chain.AbciServer = abciServer

		fmt.Println("tm node")
		err = chain.TdmNode.OnStart2()
		if err != nil {
			cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
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
		return "", errors.New("Genesis doc parse json error: %v")
	}

	return genDoc.Consensus, nil
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

func MakeTendermintNode(config cfg.Config) *tdm.Node{

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

	return tdm.NewNodeNotStart(config)
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
