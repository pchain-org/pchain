package chain

import (
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/urfave/cli.v1"

	tmcfg "github.com/ethereum/go-ethereum/consensus/tendermint/config/tendermint"
	cfg "github.com/tendermint/go-config"
)

var (
	// tendermint config
	Config cfg.Config
)

func GetTendermintConfig(chainId string, ctx *cli.Context) cfg.Config {
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

func homeDir() string {
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
	home := homeDir()
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

// DefaultLogDir return the default relative Log folder
func defaultLogDir() string {
	return "log"
}
