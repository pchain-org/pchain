package chain

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	tmcfg "github.com/ethereum/go-ethereum/consensus/pdbft/config/pdbft"
	cfg "github.com/tendermint/go-config"
	"gopkg.in/urfave/cli.v1"
)

var (
	// tendermint config
	Config cfg.Config
)

func GetTendermintConfig(chainId string, ctx *cli.Context) cfg.Config {
	datadir := ctx.GlobalString(utils.DataDirFlag.Name)
	config := tmcfg.GetConfig(datadir, chainId)

	return config
}

func contains(a []string, s string) bool {
	for _, e := range a {
		if s == e {
			return true
		}
	}
	return false
}
