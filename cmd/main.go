package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/bridge"
	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/pchain/chain"
	"github.com/pchain/version"
	"gopkg.in/urfave/cli.v1"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"
)

func main() {

	cliApp := newCliApp(version.Version, "the pchain command line interface")
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
			Usage:       "init_eth_genesis balance:{\"1000000\",\"100\"}",
			Description: "Initialize the balance of accounts",
		},

		{
			Action:      chain.InitCmd,
			Name:        "init",
			Usage:       "init genesis.json",
			Description: "Initialize the files",
		},

		{
			Action:      chain.GenerateNodeInfoCmd,
			Name:        "gen_node_info",
			Usage:       "gen_node_info number", //generate node info for 'number' nodes
			Description: "Generate node info for static-nodes.json",
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
		logFolderFlag := ctx.GlobalString(LogDirFlag.Name)

		// Setup the Global Logger
		commonLogDir := path.Join(logFolderFlag, "common")
		log.NewLogger("", commonLogDir, ctx.GlobalInt(verbosityFlag.Name), ctx.GlobalBool(debugFlag.Name), ctx.GlobalString(vmoduleFlag.Name), ctx.GlobalString(backtraceAtFlag.Name))

		// Tendermint Config
		chain.Config = chain.GetTendermintConfig(chain.MainChain, ctx)

		runtime.GOMAXPROCS(runtime.NumCPU())

		if err := bridge.Debug_Setup(ctx, logFolderFlag); err != nil {
			return err
		}

		// Start system runtime metrics collection
		go metrics.CollectProcessMetrics(3 * time.Second)

		utils.SetupNetwork(ctx)
		return nil
	}

	cliApp.After = func(ctx *cli.Context) error {
		bridge.Debug_Exit()
		console.Stdin.Close() // Resets terminal mode.
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
		utils.DataDirFlag,
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

		utils.PerfTestFlag,

		LogDirFlag,
		ChildChainFlag,

		//Tendermint flags
		MonikerFlag,
		NodeLaddrFlag,
		SeedsFlag,
		FastSyncFlag,
		SkipUpnpFlag,
		RpcLaddrFlag,
		AddrFlag,
	}
	app.Flags = append(app.Flags, DebugFlags...)

	return app
}

func versionCmd(ctx *cli.Context) error {
	fmt.Println(version.Version)
	return nil
}
