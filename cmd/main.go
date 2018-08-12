package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/pchain/chain"
	"github.com/pchain/common/plogger"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"github.com/pchain/ethermint/version"
	"github.com/sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"
)

func main() {

	cliApp := newCliApp(version.Version, "the ethermint command line interface")
	cliApp.Action = pchainCmd
	cliApp.Commands = []cli.Command{

		{
			Action:      versionCmd,
			Name:        "version",
			Usage:       "",
			Description: "Print the version",
		},

		{
			Action:      chain.InitEthGenesis,
			Name:        "init_eth_genesis",
			Usage:       "init_eth_genesis balance:\"10,10,10\"",
			Description: "Initialize the balance of accounts",
		},

		{
			Action:      chain.InitCmd,
			Name:        "init",
			Usage:       "init genesis.json",
			Description: "Initialize the files",
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
		// Log Folder
		logFolderFlag := ctx.GlobalString(chain.LogDirFlag.Name)
		plogger.SetLogFolder(logFolderFlag)

		// Log Level
		logLevelFlag := ctx.GlobalString(chain.LogLevelFlag.Name)
		logLevel, err := logrus.ParseLevel(logLevelFlag)
		if err != nil {
			fmt.Printf("unknown log level, default level should be info\n")
			return err
		}
		plogger.SetVerbosity(logLevel)

		plogger.InitLogWriter()

		chain.Config = etm.GetTendermintConfig(chain.MainChain, ctx)
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

		chain.LogLevelFlag,
		chain.LogDirFlag,
		chain.DataDirFlag, // so we control defaults

		//ethermint flags
		chain.MonikerFlag,
		chain.NodeLaddrFlag,
		chain.SeedsFlag,
		chain.FastSyncFlag,
		chain.SkipUpnpFlag,
		chain.RpcLaddrFlag,
		chain.AddrFlag,
		chain.AbciFlag,
	}
	return app
}

func versionCmd(ctx *cli.Context) error {
	fmt.Println(version.Version)
	return nil
}
