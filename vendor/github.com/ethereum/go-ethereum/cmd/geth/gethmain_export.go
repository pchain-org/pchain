package gethmain

import (
	"gopkg.in/urfave/cli.v1"
	"github.com/ethereum/go-ethereum/node"
)

//var ConsoleCommand = consoleCommand
var AttachCommand = attachCommand
//var JavascriptCommand = javascriptCommand

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
