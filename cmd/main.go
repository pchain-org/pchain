package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/bridge"
	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pchain/chain"
	"github.com/pchain/version"
	"gopkg.in/urfave/cli.v1"
)

const (
	clientIdentifier = "pchain" // Client identifier to advertise over the network; it also is the main chain's id
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
			Action:      chain.InitChildChainCmd,
			Name:        "init_child_chain",
			Usage:       "./pchain --datadir=.pchain --childChain=child_0,child_1,child_2 init_child_chain",
			Description: "Initialize child chain genesis from chain info db",
		},

		{
			Action:      GenerateNodeInfoCmd,
			Name:        "gen_node_info",
			Usage:       "gen_node_info number", //generate node info for 'number' nodes
			Description: "Generate node info for static-nodes.json",
		},

		{
			//Action: GeneratePrivateValidatorCmd,
			Action: utils.MigrateFlags(GeneratePrivateValidatorCmd),
			Name:   "gen_priv_validator",
			Usage:  "gen_priv_validator address", //generate priv_validator.json for address
			Flags: []cli.Flag{
				utils.DataDirFlag,
			},
			Description: "Generate priv_validator.json for address",
		},

		{
			Action:    utils.MigrateFlags(dumpCmd),
			Name:      "dump",
			Usage:     "Dump a specific block from storage",
			ArgsUsage: "[<blockHash> | <blockNum>]...",
			Flags: []cli.Flag{
				utils.DataDirFlag,
				StringChainIdFlag,
				//utils.CacheFlag,
				//utils.LightModeFlag,
			},
			Category: "BLOCKCHAIN COMMANDS",
			Description: `
The arguments are interpreted as block numbers or hashes.
Use "ethereum dump 0" to dump the genesis block.`,
		},

		// See consolecmd.go:
		//gethmain.ConsoleCommand,
		gethmain.AttachCommand,

		//gethmain.JavascriptCommand,
		gethmain.ImportChainCommand,
		gethmain.ExportChainCommand,
		gethmain.ImportPreimagesCommand,
		gethmain.ExportPreimagesCommand,
		gethmain.CountBlockStateCommand,

		//walletCommand,
		accountCommand,
		snapshotCommand,
	}
	cliApp.HideVersion = true // we have a command to print the version

	cliApp.Before = func(ctx *cli.Context) error {

		// Log Folder
		logFolderFlag := ctx.GlobalString(LogDirFlag.Name)

		// Setup the Global Logger
		commonLogDir := path.Join(logFolderFlag, "common")
		log.NewLogger("", commonLogDir, ctx.GlobalInt(verbosityFlag.Name), ctx.GlobalBool(debugFlag.Name), ctx.GlobalString(vmoduleFlag.Name), ctx.GlobalString(backtraceAtFlag.Name))

		// Tendermint Config
		chainId := params.MainnetChainConfig.PChainId
		if ctx.GlobalBool(utils.TestnetFlag.Name) {
			chainId = params.TestnetChainConfig.PChainId
		}
		chain.Config = chain.GetTendermintConfig(chainId, ctx)

		runtime.GOMAXPROCS(runtime.NumCPU())

		if err := bridge.Debug_Setup(ctx, logFolderFlag); err != nil {
			return err
		}

		// Start system runtime metrics collection
		go metrics.CollectProcessMetrics(3 * time.Second)

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
		//utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.BootnodesV4Flag,
		utils.BootnodesV5Flag,
		utils.DataDirFlag,
		utils.KeyStoreDirFlag,
		utils.NoUSBFlag,
		/*
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
		*/
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
		//utils.FastSyncFlag,
		//utils.LightModeFlag,
		utils.SyncModeFlag,
		utils.GCModeFlag,
		//utils.LightServFlag,
		//utils.LightPeersFlag,
		//utils.LightKDFFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheTrieFlag,
		utils.CacheGCFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.MinerThreadsFlag,
		utils.MinerGasTargetFlag,
		utils.MinerGasLimitFlag,
		utils.MinerGasPriceFlag,
		utils.MinerEtherbaseFlag,
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		//utils.DeveloperFlag,
		//utils.DeveloperPeriodFlag,
		utils.TestnetFlag,
		//utils.RinkebyFlag,
		//utils.OttomanFlag,
		utils.VMEnableDebugFlag,
		utils.NetworkIdFlag,
		utils.PruneFlag,
		//utils.PruneBlockFlag,

		utils.EthStatsURLFlag,
		utils.MetricsEnabledFlag,
		utils.FakePoWFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.ExtraDataFlag,
		//gethmain.ConfigFileFlag,
		//utils.IstanbulRequestTimeoutFlag,
		//utils.IstanbulBlockPeriodFlag,
		// RPC HTTP Flag
		utils.RPCEnabledFlag,
		utils.RPCListenAddrFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.RPCCORSDomainFlag,
		utils.RPCVirtualHostsFlag,
		// RPC WS Flag
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,

		utils.IPCDisabledFlag,
		utils.IPCPathFlag,

		utils.SolcPathFlag,
		//utils.WhisperEnabledFlag,
		//utils.WhisperMaxMessageSizeFlag,
		//utils.WhisperMinPOWFlag,

		utils.PerfTestFlag,

		LogDirFlag,
		StringChainIdFlag,
		ChildChainFlag,

		/*
			//Tendermint flags
			MonikerFlag,
			NodeLaddrFlag,
			SeedsFlag,
			FastSyncFlag,
			SkipUpnpFlag,
			RpcLaddrFlag,
			AddrFlag,
		*/
	}
	app.Flags = append(app.Flags, DebugFlags...)

	return app
}

func versionCmd(ctx *cli.Context) error {
	fmt.Println(version.Version)
	return nil
}
