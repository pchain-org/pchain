package chain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/abi"
	"os"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	"io/ioutil"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	POSReward  = "315000000000000000000000000"
	LockReward = "11500000000000000000000000" // 11.5m

	DefaultAccountPassword = "pchain"
)

type BalaceAmount struct {
	balance string
	amount  string
}

type InvalidArgs struct {
	args string
}

func (invalid InvalidArgs) Error() string {
	return "invalid args:" + invalid.args
}

func parseBalaceAmount(s string) ([]*BalaceAmount, error) {
	r, _ := regexp.Compile("\\{[\\ \\t]*\\d+(\\.\\d+)?[\\ \\t]*\\,[\\ \\t]*\\d+(\\.\\d+)?[\\ \\t]*\\}")
	parse_strs := r.FindAllString(s, -1)
	if len(parse_strs) == 0 {
		return nil, InvalidArgs{s}
	}
	balanceAmounts := make([]*BalaceAmount, len(parse_strs))
	for i, v := range parse_strs {
		length := len(v)
		balanceAmount := strings.Split(v[1:length-1], ",")
		if len(balanceAmount) != 2 {
			return nil, InvalidArgs{s}
		}
		balanceAmounts[i] = &BalaceAmount{strings.TrimSpace(balanceAmount[0]), strings.TrimSpace(balanceAmount[1])}
	}
	return balanceAmounts, nil
}

func InitCmd(ctx *cli.Context) error {

	// ethereum genesis.json
	ethGenesisPath := ctx.Args().First()
	if len(ethGenesisPath) == 0 {
		utils.Fatalf("must supply path to genesis JSON file")
	}

	chainId := ctx.Args().Get(1)
	if chainId == "" {
		chainId = MainChain
		if ctx.GlobalBool(utils.TestnetFlag.Name) {
			chainId = TestnetChain
		}
	}

	return init_cmd(ctx, GetTendermintConfig(chainId, ctx), chainId, ethGenesisPath)
}

func InitChildChainCmd(ctx *cli.Context) error {
	// Load ChainInfo db
	chainInfoDb := dbm.NewDB("chaininfo", "leveldb", ctx.GlobalString(utils.DataDirFlag.Name))
	if chainInfoDb == nil {
		return errors.New("could not open chain info database")
	}
	defer chainInfoDb.Close()

	// Initial Child Chain Genesis
	childChainIds := ctx.GlobalString("childChain")
	if childChainIds == "" {
		return errors.New("please provide child chain id to initialization")
	}

	chainIds := strings.Split(childChainIds, ",")
	for _, chainId := range chainIds {
		ethGenesis, tdmGenesis := core.LoadChainGenesis(chainInfoDb, chainId)
		if ethGenesis == nil || tdmGenesis == nil {
			return errors.New(fmt.Sprintf("unable to retrieve the genesis file for child chain %s", chainId))
		}

		childConfig := GetTendermintConfig(chainId, ctx)

		// Write down genesis and get the genesis path
		ethGenesisPath := childConfig.GetString("eth_genesis_file")
		if err := ioutil.WriteFile(ethGenesisPath, ethGenesis, 0644); err != nil {
			utils.Fatalf("write eth_genesis_file failed")
			return err
		}

		// Init the blockchain from genesis path
		init_eth_blockchain(chainId, ethGenesisPath, ctx)

		// Write down TDM Genesis directly
		if err := ioutil.WriteFile(childConfig.GetString("genesis_file"), tdmGenesis, 0644); err != nil {
			utils.Fatalf("write tdm genesis_file failed")
			return err
		}

	}

	return nil
}

func init_cmd(ctx *cli.Context, config cfg.Config, chainId string, ethGenesisPath string) error {

	init_eth_blockchain(chainId, ethGenesisPath, ctx)

	init_em_files(config, chainId, ethGenesisPath, nil)

	return nil
}

func InitEthGenesis(ctx *cli.Context) error {
	log.Info("this is init_eth_genesis")
	args := ctx.Args()
	if len(args) != 1 {
		utils.Fatalf("len of args is %d", len(args))
		return nil
	}
	bal_str := args[0]

	chainId := MainChain
	if ctx.GlobalBool(utils.TestnetFlag.Name) {
		chainId = TestnetChain
	}
	return init_eth_genesis(GetTendermintConfig(chainId, ctx), bal_str)
}

func init_eth_genesis(config cfg.Config, balStr string) error {

	balanceAmounts, err := parseBalaceAmount(balStr)
	if err != nil {
		utils.Fatalf("init eth_genesis_file failed")
		return err
	}

	validators := createPriValidators(config, len(balanceAmounts))

	var coreGenesis = core.Genesis{
		Config:     params.MainnetChainConfig,
		Nonce:      0xdeadbeefdeadbeef,
		Timestamp:  0x0,
		ParentHash: common.Hash{},
		ExtraData:  []byte("0x0"),
		GasLimit:   0x8000000,
		Difficulty: new(big.Int).SetUint64(0x400),
		Mixhash:    common.Hash{},
		Coinbase:   common.Address{},
		Alloc:      core.GenesisAlloc{},
	}
	for i, validator := range validators {
		coreGenesis.Alloc[validator.Address] = core.GenesisAccount{
			Balance: math.MustParseBig256(balanceAmounts[i].balance),
			Amount:  math.MustParseBig256(balanceAmounts[i].amount),
		}
	}

	contents, err := json.MarshalIndent(coreGenesis, "", "\t")
	if err != nil {
		utils.Fatalf("marshal coreGenesis failed")
		return err
	}
	ethGenesisPath := config.GetString("eth_genesis_file")
	if err = ioutil.WriteFile(ethGenesisPath, contents, 0654); err != nil {
		utils.Fatalf("write eth_genesis_file failed")
		return err
	}
	return nil
}

func init_eth_blockchain(chainId string, ethGenesisPath string, ctx *cli.Context) {

	dbPath := filepath.Join(utils.MakeDataDir(ctx), chainId, "geth/chaindata")
	log.Infof("init_eth_blockchain 0 with dbPath: %s", dbPath)

	chainDb, err := rawdb.NewLevelDBDatabase(filepath.Join(utils.MakeDataDir(ctx), chainId, gethmain.ClientIdentifier, "chaindata"), 0, 0, "eth/db/chaindata/")
	if err != nil {
		utils.Fatalf("could not open database: %v", err)
	}
	defer chainDb.Close()

	log.Info("init_eth_blockchain 1")
	genesisFile, err := os.Open(ethGenesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}
	defer genesisFile.Close()

	log.Info("init_eth_blockchain 2")
	block, err := core.WriteGenesisBlock(chainDb, genesisFile)
	if err != nil {
		utils.Fatalf("failed to write genesis block: %v", err)
	}

	log.Info("init_eth_blockchain end")
	log.Infof("successfully wrote genesis block and/or chain rule set: %x", block.Hash())
}

func init_em_files(config cfg.Config, chainId string, genesisPath string, validators []types.GenesisValidator) error {
	gensisFile, err := os.Open(genesisPath)
	defer gensisFile.Close()
	if err != nil {
		utils.Fatalf("failed to read Ethereum genesis file: %v", err)
		return err
	}
	contents, err := ioutil.ReadAll(gensisFile)
	if err != nil {
		utils.Fatalf("failed to read Ethereum genesis file: %v", err)
		return err
	}
	var coreGenesis = core.Genesis{}
	if err := json.Unmarshal(contents, &coreGenesis); err != nil {
		return err
	}

	var privValidator *types.PrivValidator
	// validators == nil means we are init the Genesis from priv_validator, not from runtime GenesisValidator
	if validators == nil {
		privValPath := config.GetString("priv_validator_file")
		if _, err := os.Stat(privValPath); os.IsNotExist(err) {
			log.Info("priv_validator_file not exist, probably you are running in non-mining mode")
			return nil
		}
		// Now load the priv_validator_file
		privValidator = types.LoadPrivValidator(privValPath)
	}

	// Create the Genesis Doc
	if err := createGenesisDoc(config, chainId, &coreGenesis, privValidator, validators); err != nil {
		utils.Fatalf("failed to write genesis file: %v", err)
		return err
	}
	return nil
}

func createGenesisDoc(config cfg.Config, chainId string, coreGenesis *core.Genesis, privValidator *types.PrivValidator, validators []types.GenesisValidator) error {
	genFile := config.GetString("genesis_file")
	if _, err := os.Stat(genFile); os.IsNotExist(err) {

		var rewardScheme types.RewardSchemeDoc
		if chainId == MainChain || chainId == TestnetChain {
			posReward, _ := new(big.Int).SetString(POSReward, 10)
			LockReward, _ := new(big.Int).SetString(LockReward, 10)
			totalReward := new(big.Int).Sub(posReward, LockReward)
			rewardScheme = types.RewardSchemeDoc{
				TotalReward:        totalReward,
				RewardFirstYear:    new(big.Int).Div(totalReward, big.NewInt(8)),
				EpochNumberPerYear: 12,
				TotalYear:          23,
			}
		} else {
			rewardScheme = types.RewardSchemeDoc{
				TotalReward:        big.NewInt(0),
				RewardFirstYear:    big.NewInt(0),
				EpochNumberPerYear: 12,
				TotalYear:          0,
			}
		}

		var rewardPerBlock *big.Int
		if chainId == MainChain || chainId == TestnetChain {
			rewardPerBlock = big.NewInt(1219698431069958847)
		} else {
			rewardPerBlock = big.NewInt(0)
		}

		genDoc := types.GenesisDoc{
			ChainID:      chainId,
			Consensus:    types.CONSENSUS_POS,
			GenesisTime:  time.Now(),
			RewardScheme: rewardScheme,
			CurrentEpoch: types.OneEpochDoc{
				Number:         0,
				RewardPerBlock: rewardPerBlock,
				StartBlock:     0,
				EndBlock:       657000,
				Status:         0,
			},
		}

		if privValidator != nil {
			coinbase, amount, checkErr := checkAccount(*coreGenesis)
			if checkErr != nil {
				log.Infof(checkErr.Error())
				cmn.Exit(checkErr.Error())
			}

			genDoc.CurrentEpoch.Validators = []types.GenesisValidator{{
				EthAccount: coinbase,
				PubKey:     privValidator.PubKey,
				Amount:     amount,
			}}
		} else if validators != nil {
			genDoc.CurrentEpoch.Validators = validators
		}
		genDoc.SaveAs(genFile)
	}
	return nil
}

func generateTDMGenesis(childChainID string, validators []types.GenesisValidator) ([]byte, error) {
	var rewardScheme = types.RewardSchemeDoc{
		TotalReward:        big.NewInt(0),
		RewardFirstYear:    big.NewInt(0),
		EpochNumberPerYear: 12,
		TotalYear:          0,
	}

	genDoc := types.GenesisDoc{
		ChainID:      childChainID,
		Consensus:    types.CONSENSUS_POS,
		GenesisTime:  time.Now(),
		RewardScheme: rewardScheme,
		CurrentEpoch: types.OneEpochDoc{
			Number:         0,
			RewardPerBlock: big.NewInt(0),
			StartBlock:     0,
			EndBlock:       657000,
			Status:         0,
			Validators:     validators,
		},
	}

	contents, err := json.Marshal(genDoc)
	if err != nil {
		utils.Fatalf("marshal tdm Genesis failed")
		return nil, err
	}
	return contents, nil
}

func createPriValidators(config cfg.Config, num int) []*types.PrivValidator {
	validators := make([]*types.PrivValidator, num)

	ks := keystore.NewKeyStore(config.GetString("keystore"), keystore.StandardScryptN, keystore.StandardScryptP)

	privValFile := config.GetString("priv_validator_file_root")
	for i := 0; i < num; i++ {
		// Create New PChain Account
		account, err := ks.NewAccount(DefaultAccountPassword)
		if err != nil {
			utils.Fatalf("Failed to create PChain account: %v", err)
		}
		// Generate Consensus KeyPair
		validators[i] = types.GenPrivValidatorKey(account.Address)
		log.Info("createPriValidators", "account:", validators[i].Address, "pwd:", DefaultAccountPassword)
		if i > 0 {
			validators[i].SetFile(privValFile + strconv.Itoa(i) + ".json")
		} else {
			validators[i].SetFile(privValFile + ".json")
		}
		validators[i].Save()
	}
	return validators
}

func checkAccount(coreGenesis core.Genesis) (common.Address, *big.Int, error) {

	coinbase := coreGenesis.Coinbase
	log.Infof("checkAccount(), coinbase is %x", coinbase)

	var act common.Address
	amount := big.NewInt(-1)
	balance := big.NewInt(-1)
	found := false
	for address, account := range coreGenesis.Alloc {
		log.Infof("checkAccount(), address is %x, balance is %v, amount is %v", address, account.Balance, account.Amount)
		balance = account.Balance
		amount = account.Amount
		act = address
		found = true
		break
	}

	if !found {
		log.Error("invalidate eth_account")
		return common.Address{}, nil, errors.New("invalidate eth_account")
	}

	if balance.Sign() == -1 || amount.Sign() == -1 {
		log.Errorf("balance / amount can't be negative integer, balance is %v, amount is %v", balance, amount)
		return common.Address{}, nil, errors.New("no enough balance")
	}

	return act, amount, nil
}

func initEthGenesisFromExistValidator(childChainID string, childConfig cfg.Config, validators []types.GenesisValidator) error {

	contents, err := generateETHGenesis(childChainID, validators)
	if err != nil {
		return err
	}
	ethGenesisPath := childConfig.GetString("eth_genesis_file")
	if err = ioutil.WriteFile(ethGenesisPath, contents, 0654); err != nil {
		utils.Fatalf("write eth_genesis_file failed")
		return err
	}
	return nil
}

func generateETHGenesis(childChainID string, validators []types.GenesisValidator) ([]byte, error) {
	var coreGenesis = core.Genesis{
		Config:     params.NewChildChainConfig(childChainID),
		Nonce:      0xdeadbeefdeadbeef,
		Timestamp:  0x0,
		ParentHash: common.Hash{},
		ExtraData:  []byte("0x0"),
		GasLimit:   0x8000000,
		Difficulty: new(big.Int).SetUint64(0x400),
		Mixhash:    common.Hash{},
		Coinbase:   common.Address{},
		Alloc:      core.GenesisAlloc{},
	}
	for _, validator := range validators {
		coreGenesis.Alloc[validator.EthAccount] = core.GenesisAccount{
			Balance: big.NewInt(0),
			Amount:  validator.Amount,
		}
	}

	// Add Child Chain Default Token
	coreGenesis.Alloc[abi.ChildChainTokenIncentiveAddr] = core.GenesisAccount{
		Balance: new(big.Int).Mul(big.NewInt(100000), big.NewInt(1e+18)),
		Amount:  common.Big0,
	}

	contents, err := json.Marshal(coreGenesis)
	if err != nil {
		utils.Fatalf("marshal coreGenesis failed")
		return nil, err
	}
	return contents, nil
}
