package chain

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	eth "github.com/ethereum/go-ethereum/node"
	"github.com/pchain/common/plogger"
	"github.com/pchain/ethereum"
	"github.com/pchain/p2p"
	"github.com/pchain/version"
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"gopkg.in/urfave/cli.v1"
	"net/http"
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
	RpcHandler http.Handler
}

func LoadMainChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	chain := &Chain{Id: chainId}
	config := GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	logger.Infoln("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, pNode, GetCMInstance(ctx).cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		logger.Error("rpc_handler got failed, return")
	}

	chain.RpcHandler = rpcHandler

	return chain
}

func LoadChildChain(ctx *cli.Context, chainId string, pNode *p2p.PChainP2P) *Chain {

	logger.Infof("now load child: %s", chainId)

	chainDir := ChainDir(ctx, chainId)
	empty, err := cmn.IsDirEmpty(chainDir)
	logger.Infof("chainDir is : %s, empty is %v", chainDir, empty)
	if empty || err != nil {
		logger.Errorf("directory %s not exist or with error %v", chainDir, err)
		return nil
	}
	chain := &Chain{Id: chainId}
	config := GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	logger.Infof("chainId: %s, ethereum.MakeSystemNode", chainId)
	cch := GetCMInstance(ctx).cch
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, pNode, cch)
	chain.EthNode = stack

	rpcHandler, err := stack.GetRPCHandler()
	if err != nil {
		logger.Infoln("rpc_handler got failed, return")
		return nil
	}

	chain.RpcHandler = rpcHandler

	return chain
}

func StartChain(ctx *cli.Context, chain *Chain, startDone, quit chan int) error {

	logger.Infof("start chain: %s\n", chain.Id)
	go func() {
		logger.Infoln("StartChain()->utils.StartNode(stack)")
		utils.StartNodeEx(ctx, chain.EthNode)

		// Add ChainID to Tendermint P2P Node Info
		chainMgr.p2pObj.AddNetwork(chain.Id)

		if startDone != nil {
			startDone <- 1
		}
	}()

	return nil
}

func testEthereumApi() {
	coinbase, err := ethereum.Coinbase()
	if err != nil {
		logger.Infof("ethereum.Coinbase err with: %v\n", err)
		return
	}
	logger.Infof("testEthereumApi: coinbase is: %x\n", coinbase)

	balance, err := ethereum.GetBalance(coinbase)
	if err != nil {
		logger.Infof("ethereum.GetBalance err with: %v\n", err)
	}
	logger.Infof("testEthereumApi: balance is: %x\n", balance)
}

func CreateChildChain(ctx *cli.Context, chainId string, validator tdmTypes.PrivValidator, validators []tdmTypes.GenesisValidator) error {

	// Get Tendermint config base on chain id
	config := GetTendermintConfig(chainId, ctx)

	// Save the Validator Json File
	privValFile := config.GetString("priv_validator_file_root")
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
