package chain

import (
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/log"
	eth "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pchain/ethereum"
	"github.com/pchain/version"
	cfg "github.com/tendermint/go-config"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"
)

const (
	// Client identifier to advertise over the network
	MainChain    = params.MAINNETPCHAINID
	TestnetChain = params.TESTNETPCHAINID
)

type Chain struct {
	Id      string
	Config  cfg.Config
	EthNode *eth.Node
}

func LoadMainChain(ctx *cli.Context, chainId string) *Chain {

	chain := &Chain{Id: chainId}
	config := GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	log.Info("ethereum.MakeSystemNode")
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, GetCMInstance(ctx).cch)
	chain.EthNode = stack

	return chain
}

func LoadChildChain(ctx *cli.Context, chainId string) *Chain {

	log.Infof("now load child: %s", chainId)

	//chainDir := ChainDir(ctx, chainId)
	//empty, err := cmn.IsDirEmpty(chainDir)
	//log.Infof("chainDir is : %s, empty is %v", chainDir, empty)
	//if empty || err != nil {
	//	log.Errorf("directory %s not exist or with error %v", chainDir, err)
	//	return nil
	//}
	chain := &Chain{Id: chainId}
	config := GetTendermintConfig(chainId, ctx)
	chain.Config = config

	//always start ethereum
	log.Infof("chainId: %s, ethereum.MakeSystemNode", chainId)
	cch := GetCMInstance(ctx).cch
	stack := ethereum.MakeSystemNode(chainId, version.Version, ctx, cch)
	if stack == nil {
		return nil
	} else {
		chain.EthNode = stack
		return chain
	}
}

func StartChain(ctx *cli.Context, chain *Chain, startDone chan<- struct{}) error {

	log.Infof("Start Chain: %s", chain.Id)
	go func() {
		log.Info("StartChain()->utils.StartNode(stack)")
		utils.StartNodeEx(ctx, chain.EthNode)

		if startDone != nil {
			startDone <- struct{}{}
		}
	}()

	return nil
}

func CreateChildChain(ctx *cli.Context, chainId string, validator tdmTypes.PrivValidator, keyJson []byte, validators []tdmTypes.GenesisValidator) error {

	// Get Tendermint config base on chain id
	config := GetTendermintConfig(chainId, ctx)

	// Save the KeyStore File (Optional)
	if len(keyJson) > 0 {
		keystoreDir := config.GetString("keystore")
		keyJsonFilePath := filepath.Join(keystoreDir, keystore.KeyFileName(validator.Address))
		saveKeyError := keystore.WriteKeyStore(keyJsonFilePath, keyJson)
		if saveKeyError != nil {
			return saveKeyError
		}
	}

	// Save the Validator Json File
	privValFile := config.GetString("priv_validator_file_root")
	validator.SetFile(privValFile + ".json")
	validator.Save()

	// Init the Ethereum Genesis
	err := initEthGenesisFromExistValidator(chainId, config, validators)
	if err != nil {
		return err
	}

	// Init the Ethereum Blockchain
	init_eth_blockchain(chainId, config.GetString("eth_genesis_file"), ctx)

	// Init the Tendermint Genesis
	init_tdm_files(config, chainId, config.GetString("eth_genesis_file"), validators)

	return nil
}
