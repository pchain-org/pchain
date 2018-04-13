package main

import (
	"fmt"
	"github.com/pchain/ethermint/version"
	"os"
	"path/filepath"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"gopkg.in/urfave/cli.v1"
	cfg "github.com/tendermint/go-config"
)

const (
	// Client identifier to advertise over the network
	clientIdentifier = "Pchain"
)

var (
	// tendermint config
	config cfg.Config
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

	}
	cliApp.HideVersion = true // we have a command to print the version


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
	}
	return app
}

func versionCmd(ctx *cli.Context) error {
	fmt.Println(version.Version)
	return nil
}


