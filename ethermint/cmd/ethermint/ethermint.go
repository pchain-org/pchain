package etmmain

import (
	"fmt"
	"os"

	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/sirupsen/logrus"

	"github.com/pchain/ethermint/app"
	"github.com/pchain/ethermint/ethereum"
	"github.com/pchain/ethermint/version"

	validatorsStrategy "github.com/pchain/ethermint/strategies/validators"

	"errors"
	"github.com/pchain/common/plogger"
	"github.com/pchain/ethermint/tendermint"
	"github.com/tendermint/abci/server"
	cmn "github.com/tendermint/go-common"
	tmTypes "github.com/tendermint/tendermint/types"
	"io/ioutil"
)

var log = plogger.GetLogger("cmd")

var EthermintCmd = ethermintCmd

func ethermintCmd(chainId string, ctx *cli.Context, quit chan int) error {

	config = GetTendermintConfig(chainId, ctx)
	/*
		glog.SetV(ctx.GlobalInt(VerbosityFlag.Name))
		glog.V(logger.Info).Infoln("try to enable glog/logger")
		fmt.Println("pow recover: ethermintCmd(), before Geth")
		gethmain.Geth(ctx)
		fmt.Println("pow recover: ethermintCmd(), after Geth")
		return nil
	*/
	verbosity := ctx.GlobalInt(VerbosityFlag.Name)
	plogger.SetVerbosity(logrus.Level(verbosity))
	log.Info("setVerbosity level ", verbosity)

	//always start ethereum
	fmt.Println("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, nil, ctx, nil)
	//stack := ethereum.MakeSystemNode(chainId, version.Version, config.GetString(RpcLaddrFlag.Name), ctx, nil)

	//emmark
	fmt.Println("ethermintCmd->utils.StartNode(stack)")
	utils.StartNode(stack)

	consensus, err := getConsensus()
	if err != nil {
		cmn.Exit(cmn.Fmt("Couldn't get consensus with: %v", err))
	}
	fmt.Printf("consensus is: %s\n", consensus)

	if consensus != tmTypes.CONSENSUS_POS {
		fmt.Println("consensus is not pos, so not start the pos prototol")
		return nil
	}

	//addr := ctx.GlobalString("addr")
	//abci := ctx.GlobalString("abci")
	addr := config.GetString("proxy_app")
	abci := config.GetString("abci")

	//set verbosity level for go-ethereum
	// glog.SetToStderr(true)
	// glog.SetV(ctx.GlobalInt(VerbosityFlag.Name))

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
	ethApp, err := app.NewEthermintApplication(backend, client, strategy)
	//ethApp, err := app.NewEthermintApplication(backend, client, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, err = server.NewServer(addr, abci, ethApp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("tm node")
	tendermint.RunNode(config, ethApp)

	quit <- 1
	return nil
}

func getConsensus() (string, error) {

	genDocFile := config.GetString("genesis_file")
	var genDoc *tmTypes.GenesisDoc = nil
	if !cmn.FileExists(genDocFile) {
		return "", errors.New("Couldn't read GenesisDoc file")
	}

	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return "", errors.New("Couldn't read GenesisDoc file")
	}

	genDoc, err = tmTypes.GenesisDocFromJSON(jsonBlob)
	if err != nil {
		return "", errors.New("Genesis doc parse json error: %v")
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
