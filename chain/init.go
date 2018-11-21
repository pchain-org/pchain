package chain

import (
	"github.com/ethereum/go-ethereum/log"
	"os"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethdb"

	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
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
	}

	return init_cmd(ctx, GetTendermintConfig(chainId, ctx), chainId, ethGenesisPath)
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

	return init_eth_genesis(GetTendermintConfig(MainChain, ctx), bal_str)
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
		ParentHash: common.StringToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		ExtraData:  []byte("0x0"),
		GasLimit:   0x8000000,
		Difficulty: new(big.Int).SetUint64(0x400),
		Mixhash:    common.StringToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		Coinbase:   common.BytesToAddress((*validators[0]).EthereumAddress),
		Alloc:      core.GenesisAlloc{},
	}
	for i, validator := range validators {
		otherConPub, l := validator.PubKey.(crypto.BLSPubKey)
		_, r := validator.EthereumPrivKey.(crypto.EthereumPrivKey)
		if l && r {
			coreGenesis.Alloc[common.BytesToAddress(validator.EthereumAddress)] = core.GenesisAccount{
				Balance:         math.MustParseBig256(balanceAmounts[i].balance),
				Amount:          math.MustParseBig256(balanceAmounts[i].amount),
				ConsensusPubKey: common.ToHex(otherConPub[:]),
			}
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

	chainDb, err := ethdb.NewLDBDatabase(filepath.Join(utils.MakeDataDir(ctx), chainId, gethmain.ClientIdentifier, "chaindata"), 0, 0)
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
	if validators == nil {
		privValPath := config.GetString("priv_validator_file")
		keydir := config.GetString("keystore")
		if _, err := os.Stat(privValPath); os.IsNotExist(err) {
			utils.Fatalf("failed to read privValidator file: %v", err)
			return err
		}
		privValidator = types.LoadOrGenPrivValidator(privValPath, keydir)
	}

	//privValPath := config.GetString("priv_validator_file")
	//if _, err := os.Stat(privValPath); os.IsNotExist(err) {
	//	utils.Fatalf("failed to read privValidator file: %v", err)
	//	return err
	//}
	//privValidator := types.LoadOrGenPrivValidator(privValPath)

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
		if chainId == MainChain {
			posReward, _ := new(big.Int).SetString(POSReward, 10)
			LockReward, _ := new(big.Int).SetString(LockReward, 10)
			totalReward := new(big.Int).Sub(posReward, LockReward)
			rewardScheme = types.RewardSchemeDoc{
				TotalReward:        totalReward.String(),
				RewardFirstYear:    new(big.Int).Div(totalReward, big.NewInt(8)).String(),
				EpochNumberPerYear: "12",
				TotalYear:          "23",
			}
		} else {
			rewardScheme = types.RewardSchemeDoc{
				TotalReward:        "0",
				RewardFirstYear:    "0",
				EpochNumberPerYear: "12",
				TotalYear:          "0",
			}
		}

		var rewardPerBlock string
		if chainId == MainChain {
			rewardPerBlock = "1219698431069958847"
		} else {
			rewardPerBlock = "0"
		}

		genDoc := types.GenesisDoc{
			ChainID:      chainId, //cmn.Fmt("pchain-%v", cmn.RandStr(6)),
			Consensus:    types.CONSENSUS_POS,
			GenesisTime:  time.Now(),
			RewardScheme: rewardScheme,
			CurrentEpoch: types.OneEpochDoc{
				Number:         "0",
				RewardPerBlock: rewardPerBlock,
				StartBlock:     "0",
				EndBlock:       "2592000",
				BlockGenerated: "0",
				Status:         "0",
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

func createPriValidators(config cfg.Config, num int) []*types.PrivValidator {
	validators := make([]*types.PrivValidator, num)
	var newKey *keystore.Key
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStoreByTenermint(config.GetString("keystore"), scryptN, scryptP)
	privValFile := config.GetString("priv_validator_file_root")
	for i := 0; i < num; i++ {
		validators[i], newKey = types.GenPrivValidatorKey()
		pwd := "pchain"
		log.Info("createPriValidators", "account:", common.ToHex(validators[i].Address), "pwd:", pwd)
		a := accounts.Account{Address: newKey.Address, URL: accounts.URL{Scheme: keystore.KeyStoreScheme, Path: ks.Ks.JoinPath(keystore.KeyFileName(newKey.Address))}}
		if err := ks.StoreKey(a.URL.Path, newKey, pwd); err != nil {
			utils.Fatalf("store key failed")
			return nil
		}
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
	amount := big.NewInt(10)
	balance := big.NewInt(-1)
	found := false
	log.Infof("checkAccount(), coinbase is %x", coinbase)
	for address, account := range coreGenesis.Alloc {
		log.Infof("checkAccount(), address is %x, balance is %v, amount is %v", address, account.Balance, account.Amount)
		if coinbase == address {
			balance = account.Balance
			amount = account.Amount
			found = true
			break
		}
	}

	if !found {
		log.Error("invalidate eth_account")
		return common.Address{}, nil, errors.New("invalidate eth_account")
	}

	if balance.Sign() == -1 || amount.Sign() == -1 {
		log.Errorf("balance / amount can't be negative integer, balance is %v, amount is %v", balance, amount)
		return common.Address{}, nil, errors.New("no enough balance")
	}

	return coinbase, amount, nil
}

func initEthGenesisFromExistValidator(childChainID string, childConfig cfg.Config, validators []types.GenesisValidator) error {

	var coreGenesis = core.Genesis{
		Config:     params.NewChildChainConfig(childChainID),
		Nonce:      0xdeadbeefdeadbeef,
		Timestamp:  0x0,
		ParentHash: common.StringToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		ExtraData:  []byte("0x0"),
		GasLimit:   0x8000000,
		Difficulty: new(big.Int).SetUint64(0x400),
		Mixhash:    common.StringToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		Coinbase:   validators[0].EthAccount,
		Alloc:      core.GenesisAlloc{},
	}
	for _, validator := range validators {
		coreGenesis.Alloc[validator.EthAccount] = core.GenesisAccount{
			Balance: big.NewInt(0),
			Amount:  validator.Amount,
		}
	}

	contents, err := json.Marshal(coreGenesis)
	if err != nil {
		utils.Fatalf("marshal coreGenesis failed")
		return err
	}
	ethGenesisPath := childConfig.GetString("eth_genesis_file")
	if err = ioutil.WriteFile(ethGenesisPath, contents, 0654); err != nil {
		utils.Fatalf("write eth_genesis_file failed")
		return err
	}
	return nil
}
