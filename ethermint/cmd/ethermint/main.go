package etmmain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/pchain/common/plogger"
	"github.com/pchain/ethermint/version"
	cfg "github.com/tendermint/go-config"
	"gopkg.in/urfave/cli.v1"
	"os"
	"path/filepath"
)

var logger = plogger.GetLogger("etmmain")

const (
	// Client identifier to advertise over the network
	clientIdentifier = "Ethermint"
)

var (
	// tendermint config
	config cfg.Config
)

//Deprecated
func main() {

	logger.Infof("Starting ethermint")

	cliApp := newCliApp(version.Version, "the ethermint command line interface")
	cliApp.Action = ethermintCmd
	cliApp.Commands = []cli.Command{
		{
			Action:      initCmd,
			Name:        "init",
			Usage:       "init genesis.json",
			Description: "Initialize the files",
		},

		{
			Action:      versionCmd,
			Name:        "version",
			Usage:       "",
			Description: "Print the version",
		},

		{
			Action:      initEthGenesis,
			Name:        "init_eth_genesis",
			Usage:       "init_eth_genesis balance:\"10,10,10\"",
			Description: "Initialize the balance of accounts",
		},

		// See consolecmd.go:
		gethmain.ConsoleCommand,
		gethmain.AttachCommand,
		gethmain.JavascriptCommand,

		gethmain.WalletCommand,
		gethmain.AccountCommand,
	}
	cliApp.HideVersion = true // we have a command to print the version

	cliApp.Before = func(ctx *cli.Context) error {
		config = GetTendermintConfig("", ctx)
		return nil
	}
	cliApp.After = func(ctx *cli.Context) error {
		// logger.Flush()
		return nil
	}

	if err := cliApp.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newCliApp(version, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = version
	app.Usage = usage
	app.Flags = []cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.KeyStoreDirFlag,
		// utils.BlockchainVersionFlag,
		utils.CacheFlag,
		utils.LightKDFFlag,
		utils.JSpathFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.EtherbaseFlag,
		utils.TargetGasLimitFlag,
		utils.GasPriceFlag,
		utils.NATFlag,
		// utils.NatspecEnabledFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.RPCEnabledFlag,
		utils.RPCListenAddrFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.IPCDisabledFlag,
		utils.IPCApiFlag,
		utils.IPCPathFlag,
		utils.ExecFlag,
		utils.PreloadJSFlag,
		utils.TestNetFlag,
		utils.VMForceJitFlag,
		utils.VMJitCacheFlag,
		utils.VMEnableJitFlag,
		utils.NetworkIdFlag,
		utils.RPCCORSDomainFlag,
		utils.MetricsEnabledFlag,
		utils.SolcPathFlag,
		utils.GpoMinGasPriceFlag,
		utils.GpoMaxGasPriceFlag,
		utils.GpoFullBlockRatioFlag,
		utils.GpobaseStepDownFlag,
		utils.GpobaseStepUpFlag,
		utils.GpobaseCorrectionFactorFlag,
		VerbosityFlag, // not exposed by go-ethereum
		DataDirFlag,   // so we control defaults

		//ethermint flags
		MonikerFlag,
		NodeLaddrFlag,
		LogLevelFlag,
		SeedsFlag,
		FastSyncFlag,
		SkipUpnpFlag,
		RpcLaddrFlag,
		AddrFlag,
		AbciFlag,
	}
	return app
}

func versionCmd(ctx *cli.Context) error {
	fmt.Println(version.Version)
	return nil
}
