package tendermint

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"time"
	"github.com/pborman/uuid"
	"strconv"
)

const (
	defaultDataDir         = "data"
	defaultConfigFileName  = "config.toml"
	defaultGenesisJSONName = "genesis.json"
)


func getTMRoot(rootDir string) string {
	if rootDir == "" {
		rootDir = os.Getenv("TMHOME")
	}
	if rootDir == "" {
		// deprecated, use TMHOME (TODO: remove in TM 0.11.0)
		rootDir = os.Getenv("TMROOT")
	}
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}
	return rootDir
}

func initTMRoot(rootDir, chainId string) {
	rootDir = getTMRoot(rootDir)
	EnsureDir(rootDir, 0700)
	EnsureDir(filepath.Join(rootDir, chainId, defaultDataDir), 0700)

	configFilePath := filepath.Join(rootDir, defaultConfigFileName)

	// Write default config file if missing.
	if !FileExists(configFilePath) {
		// Ask user for moniker
		// moniker := cfg.Prompt("Type hostname: ", "anonymous")
		MustWriteFile(configFilePath, []byte(defaultConfig("anonymous")), 0644)
	}
}

func GetConfig(rootDir, chainId string) cfg.Config {
	rootDir = getTMRoot(rootDir)
	initTMRoot(rootDir, chainId)

	configFilePath := filepath.Join(rootDir, defaultConfigFileName)
	mapConfig, err := cfg.ReadMapConfigFromFile(configFilePath)
	if err != nil {
		Exit(Fmt("Could not read config: %v", err))
	}

	// Set defaults or panic
	if mapConfig.IsSet("chain_id") {
		Exit("Cannot set 'chain_id' via config.toml")
	}
	if mapConfig.IsSet("revision_file") {
		Exit("Cannot set 'revision_file' via config.toml. It must match what's in the Makefile")
	}
	mapConfig.SetRequired("chain_id") // blows up if you try to use it before setting.
	mapConfig.SetDefault("chain_id", chainId)
	mapConfig.SetDefault("genesis_file", filepath.Join(rootDir, chainId, "genesis.json"))
	mapConfig.SetDefault("eth_genesis_file", filepath.Join(rootDir, chainId, "eth_genesis.json"))
	mapConfig.SetDefault("keystore", filepath.Join(rootDir, chainId, "keystore"))
	//mapConfig.SetDefault("proxy_app", "tcp://127.0.0.1:46658")
	//mapConfig.SetDefault("abci", "socket")
	//mapConfig.Set("proxy_app", calcAppAddr())
	//mapConfig.Set("abci", defaultAbci())
	mapConfig.SetDefault("moniker", "anonymous")
	mapConfig.SetDefault("node_laddr", "tcp://0.0.0.0:46656")
	mapConfig.SetDefault("seeds", "")
	// mapConfig.SetDefault("seeds", "goldenalchemist.chaintest.net:46656")
	mapConfig.SetDefault("fast_sync", true)
	mapConfig.SetDefault("skip_upnp", false)
	mapConfig.SetDefault("addrbook_file", filepath.Join(rootDir, chainId, "addrbook.json"))
	mapConfig.SetDefault("addrbook_strict", true) // disable to allow connections locally
	mapConfig.SetDefault("pex_reactor", false)    // enable for peer exchange
	mapConfig.SetDefault("priv_validator_file", filepath.Join(rootDir, chainId, "priv_validator.json"))
	mapConfig.SetDefault("priv_validator_file_root", filepath.Join(rootDir, chainId, "priv_validator"))
	mapConfig.SetDefault("db_backend", "leveldb")
	mapConfig.SetDefault("db_dir", filepath.Join(rootDir, chainId, defaultDataDir))
	mapConfig.SetDefault("log_level", "info")
	//mapConfig.SetDefault("rpc_laddr", "tcp://0.0.0.0:46657")
	//mapConfig.SetDefault("rpc_laddr", calcRpcAddr())
	mapConfig.SetDefault("grpc_laddr", "")
	mapConfig.SetDefault("prof_laddr", "")
	mapConfig.SetDefault("revision_file", filepath.Join(rootDir, chainId, "revision"))
	mapConfig.SetDefault("cs_wal_file", filepath.Join(rootDir, chainId, defaultDataDir, "cs.wal", "wal"))
	mapConfig.SetDefault("cs_wal_light", false)
	mapConfig.SetDefault("filter_peers", false)

	//liaoyd
	mapConfig.SetDefault("cs_val_file", filepath.Join(rootDir, chainId, defaultDataDir, "cs.val", "val"))

	mapConfig.SetDefault("block_size", 10000)      // max number of txs
	mapConfig.SetDefault("block_part_size", 65536) // part size 64K
	mapConfig.SetDefault("disable_data_hash", false)

	// all timeouts are in ms
	mapConfig.SetDefault("timeout_handshake", 10000)
	mapConfig.SetDefault("timeout_propose", 3000)
	mapConfig.SetDefault("timeout_propose_delta", 500)
	mapConfig.SetDefault("timeout_prevote", 3000)
	mapConfig.SetDefault("timeout_prevote_delta", 500)
	mapConfig.SetDefault("timeout_precommit", 3000)
	mapConfig.SetDefault("timeout_precommit_delta", 500)
	mapConfig.SetDefault("timeout_commit", 3000)

	// make progress asap (no `timeout_commit`) on full precommit votes
	mapConfig.SetDefault("skip_timeout_commit", false)
	mapConfig.SetDefault("mempool_recheck", true)
	mapConfig.SetDefault("mempool_recheck_empty", true)
	mapConfig.SetDefault("mempool_broadcast", true)
	mapConfig.SetDefault("mempool_wal_dir", filepath.Join(rootDir, chainId, defaultDataDir, "mempool.wal"))

	mapConfig.SetDefault("tx_index", "kv")

	return mapConfig
}

var defaultConfigTmpl = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml
#proxy_app = "tcp://127.0.0.1:46658"
moniker = "__MONIKER__"
node_laddr = "tcp://0.0.0.0:46656"
seeds = ""
fast_sync = true
db_backend = "leveldb"
log_level = "notice"
#rpc_laddr = "tcp://0.0.0.0:46657"
`

func defaultConfig(moniker string) (defaultConfig string) {
	defaultConfig = strings.Replace(defaultConfigTmpl, "__MONIKER__", moniker, -1)
	return
}

func defaultAbci() string {
	return "unix"
}

func calcAppAddr() string {
	return defaultAbci() + "://" + strconv.Itoa(time.Now().Nanosecond()) + "-" + uuid.NewRandom().String()
}

func calcRpcAddr() string {
	return defaultAbci() + "://" + strconv.Itoa(time.Now().Nanosecond()) + "-" + uuid.NewRandom().String()
}
