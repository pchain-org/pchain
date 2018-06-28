package main

import (
	"fmt"
	"github.com/pchain/ethermint/version"
	"os"
	"path/filepath"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"gopkg.in/urfave/cli.v1"
	etm "github.com/pchain/ethermint/cmd/ethermint"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/pchain/chain"
	"github.com/ethereum/go-ethereum/cmd/geth"
)


func main() {

	glog.V(logger.Info).Infof("Starting pchain")

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
			Action:		chain.InitEthGenesis,
			Name:		"init_eth_genesis",
			Usage:		"init_eth_genesis balance:\"10,10,10\"",
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
		utils.BootnodesV4Flag,
		utils.BootnodesV5Flag,
		//utils.DataDirFlag,
		utils.KeyStoreDirFlag,
		utils.NoUSBFlag,
		utils.DashboardEnabledFlag,
		utils.DashboardAddrFlag,
		utils.DashboardPortFlag,
		utils.DashboardRefreshFlag,
		utils.EthashCacheDirFlag,
		utils.EthashCachesInMemoryFlag,
		utils.EthashCachesOnDiskFlag,
		utils.EthashDatasetDirFlag,
		utils.EthashDatasetsInMemoryFlag,
		utils.EthashDatasetsOnDiskFlag,
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		utils.FastSyncFlag,
		utils.LightModeFlag,
		utils.SyncModeFlag,
		utils.GCModeFlag,
		utils.LightServFlag,
		utils.LightPeersFlag,
		utils.LightKDFFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheGCFlag,
		utils.TrieCacheGenFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.EtherbaseFlag,
		utils.GasPriceFlag,
		utils.MinerThreadsFlag,
		utils.MiningEnabledFlag,
		utils.TargetGasLimitFlag,
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.DeveloperFlag,
		utils.DeveloperPeriodFlag,
		utils.TestnetFlag,
		utils.RinkebyFlag,
		utils.OttomanFlag,
		utils.VMEnableDebugFlag,
		utils.NetworkIdFlag,
		utils.RPCCORSDomainFlag,
		utils.RPCVirtualHostsFlag,
		utils.EthStatsURLFlag,
		utils.MetricsEnabledFlag,
		utils.FakePoWFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.ExtraDataFlag,
		gethmain.ConfigFileFlag,
		utils.IstanbulRequestTimeoutFlag,
		utils.IstanbulBlockPeriodFlag,
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
		utils.IPCPathFlag,
		
		utils.WhisperEnabledFlag,
		utils.SolcPathFlag,
		utils.WhisperMaxMessageSizeFlag,
		utils.WhisperMinPOWFlag,
		
		chain.VerbosityFlag, // not exposed by go-ethereum
		chain.DataDirFlag,   // so we control defaults

		//ethermint flags
		chain.MonikerFlag,
		chain.NodeLaddrFlag,
		chain.LogLevelFlag,
		chain.SeedsFlag,
		chain.FastSyncFlag,
		chain.SkipUpnpFlag,
		chain.RpcLaddrFlag,
		chain.AddrFlag,
		chain.AbciFlag,
	}
	return app
}

func versionCmd() error {
	fmt.Println(version.Version)
	return nil
}


