package chain

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"

	tmcfg "github.com/ethereum/go-ethereum/consensus/tendermint/config/tendermint"
	cfg "github.com/tendermint/go-config"
)

var (
	// tendermint config
	Config cfg.Config
)

func GetTendermintConfig(chainId string, ctx *cli.Context) cfg.Config {
	datadir := ctx.GlobalString(utils.DataDirFlag.Name)
	config := tmcfg.GetConfig(datadir, chainId)

	checkAndSet(config, ctx, "moniker")
	checkAndSet(config, ctx, "node_laddr")
	checkAndSet(config, ctx, "seeds")
	checkAndSet(config, ctx, "fast_sync")
	checkAndSet(config, ctx, "skip_upnp")

	return config
}

func checkAndSet(config cfg.Config, ctx *cli.Context, opName string) {
	if ctx.GlobalIsSet(opName) {
		config.Set(opName, ctx.GlobalString(opName))

	}
}

func ChainDir(ctx *cli.Context, chainId string) string {
	datadir := ctx.GlobalString(utils.DataDirFlag.Name)
	return filepath.Join(datadir, chainId)
}
