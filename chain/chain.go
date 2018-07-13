package chain

import (
	"net/http"
	eth "github.com/ethereum/go-ethereum/node"
	"gopkg.in/urfave/cli.v1"
	"fmt"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/cmd/utils"
	cfg "github.com/tendermint/go-config"
	cmn "github.com/tendermint/go-common"
	"github.com/pchain/p2p"
	"github.com/pchain/ethereum"
	"github.com/pchain/version"
)

const (
	// Client identifier to advertise over the network
	MainChain = "pchain"
)

type Chain struct{
	Id string
	Config cfg.Config
	EthNode *eth.Node
	RpcHandler http.Handler
}

func LoadMainChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	chain := &Chain {Id:chainId}
	config := GetTendermintConfig(chainId, ctx)
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
	config := GetTendermintConfig(chainId, ctx)
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

func StartChain(ctx *cli.Context, chain *Chain, quit chan int) error {

	fmt.Printf("start main chain: %s\n", chain.Id)
	go func(){
		fmt.Println("StartChain()->utils.StartNode(stack)")
		utils.StartNodeEx(ctx, chain.EthNode)
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

	config := GetTendermintConfig(chainId, ctx)
	err := init_eth_genesis(config, balStr)
	if err != nil {
		return err
	}

	init_cmd(ctx, config, chainId, config.GetString("eth_genesis_file"))

	return nil
}
