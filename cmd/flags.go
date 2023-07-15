package main

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"gopkg.in/urfave/cli.v1"
	"runtime"
)

var (
	// ----------------------------
	// PChain Flags

	// Log Folder
	LogDirFlag = utils.DirectoryFlag{
		Name:  "logDir",
		Usage: "PChain Log Data directory",
		Value: utils.DirectoryString{"log"},
	}

	// ----------------------------
	// Tendermint Flags

	MonikerFlag = cli.StringFlag{
		Name:  "moniker",
		Value: "",
		Usage: "Node's moniker",
	}

	NodeLaddrFlag = cli.StringFlag{
		Name:  "node_laddr",
		Value: "tcp://0.0.0.0:46656",
		Usage: "Node listen address. (0.0.0.0:0 means any interface, any port)",
	}

	SeedsFlag = cli.StringFlag{
		Name:  "seeds",
		Value: "",
		Usage: "Comma delimited host:port seed nodes",
	}

	FastSyncFlag = cli.BoolFlag{
		Name:  "fast_sync",
		Usage: "Fast blockchain syncing",
	}

	SkipUpnpFlag = cli.BoolFlag{
		Name:  "skip_upnp",
		Usage: "Skip UPNP configuration",
	}

	RpcLaddrFlag = cli.StringFlag{
		Name:  "rpc_laddr",
		Value: "unix://@pchainrpcunixsock", //"tcp://0.0.0.0:46657",
		Usage: "RPC listen address. Port required",
	}

	AddrFlag = cli.StringFlag{
		Name:  "addr",
		Value: "unix://@pchainappunixsock", //"tcp://0.0.0.0:46658",
		Usage: "TMSP app listen address",
	}

	// Flags holds all command-line flags required for debugging.
	DebugFlags = []cli.Flag{
		verbosityFlag, vmoduleFlag, backtraceAtFlag, debugFlag,
		pprofFlag, pprofAddrFlag, pprofPortFlag,
		memprofilerateFlag, blockprofilerateFlag, cpuprofileFlag, traceFlag,
	}

	//from debug module
	// Not exposed by go-ethereum
	verbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		Value: 3,
	}
	vmoduleFlag = cli.StringFlag{
		Name:  "vmodule",
		Usage: "Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. eth/*=5,p2p=4)",
		Value: "",
	}
	backtraceAtFlag = cli.StringFlag{
		Name:  "backtrace",
		Usage: "Request a stack trace at a specific logging statement (e.g. \"block.go:271\")",
		Value: "",
	}
	debugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "Prepends log messages with call-site location (file and line number)",
	}
	pprofFlag = cli.BoolFlag{
		Name:  "pprof",
		Usage: "Enable the pprof HTTP server",
	}
	pprofPortFlag = cli.IntFlag{
		Name:  "pprofport",
		Usage: "pprof HTTP server listening port",
		Value: 6060,
	}
	pprofAddrFlag = cli.StringFlag{
		Name:  "pprofaddr",
		Usage: "pprof HTTP server listening interface",
		Value: "127.0.0.1",
	}
	memprofilerateFlag = cli.IntFlag{
		Name:  "memprofilerate",
		Usage: "Turn on memory profiling with the given rate",
		Value: runtime.MemProfileRate,
	}
	blockprofilerateFlag = cli.IntFlag{
		Name:  "blockprofilerate",
		Usage: "Turn on block profiling with the given rate",
	}
	cpuprofileFlag = cli.StringFlag{
		Name:  "cpuprofile",
		Usage: "Write CPU profile to the given file",
	}
	traceFlag = cli.StringFlag{
		Name:  "trace",
		Usage: "Write execution trace to the given file",
	}
)
