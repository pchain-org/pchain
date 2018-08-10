package chain

import (
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/urfave/cli.v1"

	cfg "github.com/tendermint/go-config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

var (
	// tendermint config
	Config cfg.Config
)

//Deprecated
func getTendermintConfig(chainId string, ctx *cli.Context) cfg.Config {
	datadir := ctx.GlobalString(DataDirFlag.Name)
	config := tmcfg.GetConfig(datadir, chainId)

	checkAndSet(config, ctx, "moniker")
	checkAndSet(config, ctx, "node_laddr")
	checkAndSet(config, ctx, "log_level")
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

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if user, err := user.Current(); err == nil {
			p = user.HomeDir + p[1:]
		}
	}
	return path.Clean(os.ExpandEnv(p))
}

func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := HomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Pchain")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Pchain")
		} else {
			return filepath.Join(home, ".pchain")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func ChainDir(ctx *cli.Context, chainId string) string {
	datadir := ctx.GlobalString(DataDirFlag.Name)
	return filepath.Join(datadir, chainId)
}
