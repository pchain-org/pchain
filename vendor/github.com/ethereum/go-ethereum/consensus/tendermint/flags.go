package tendermint

import (
	"gopkg.in/urfave/cli.v1"
)

var (
	// ----------------------------
	// go-ethereum flags

	// So we can control the DefaultDir
	DataDirFlag = cli.StringFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DefaultDataDir(),
	}

	// Not exposed by go-ethereum
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Verbosity of go-ethereum",
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
)
