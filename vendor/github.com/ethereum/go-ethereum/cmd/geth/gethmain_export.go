package gethmain

import (
	"github.com/ethereum/go-ethereum/node"
	"gopkg.in/urfave/cli.v1"
)

//var ConsoleCommand = consoleCommand
var AttachCommand = attachCommand

//var JavascriptCommand = javascriptCommand

var ImportChainCommand = importCommand
var ExportChainCommand = exportCommand
var ImportPreimagesCommand = importPreimagesCommand
var ExportPreimagesCommand = exportPreimagesCommand
var CountBlockStateCommand = countBlockStateCommand

var ConfigFileFlag = configFileFlag

type GethConfig = gethConfig

func LoadConfig(file string, cfg *GethConfig) error {
	return loadConfig(file, cfg)
}

func MakeConfigNode(ctx *cli.Context, chainId string) (*node.Node, gethConfig) {
	return makeConfigNode(ctx, chainId)
}

func DefaultNodeConfig() node.Config {
	return defaultNodeConfig()
}
