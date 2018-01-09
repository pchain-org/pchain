package main

import (
	"os"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"

	cmn "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"github.com/pkg/errors"
	"io/ioutil"
	"encoding/json"
	"io"
	"fmt"
)

func initCmd(ctx *cli.Context) error {

	// ethereum genesis.json
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("must supply path to genesis JSON file")
	}

	chainDb, err := ethdb.NewLDBDatabase(filepath.Join(utils.MakeDataDir(ctx), "chaindata"), 0, 0)
	if err != nil {
		utils.Fatalf("could not open database: %v", err)
	}

	genesisFile, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}

	block, err := core.WriteGenesisBlock(chainDb, genesisFile)
	if err != nil {
		utils.Fatalf("failed to write genesis block: %v", err)
	}

	genesisFile, err = os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}

	err = init_files(genesisFile)
	if(err != nil) {
		utils.Fatalf("failed to initialize ethermint's priv_validators/genesis file: %v", err)
	}

	glog.V(logger.Info).Infof("successfully wrote genesis block and/or chain rule set: %x", block.Hash())

	return nil
}

func init_files(ethGenesisFile io.Reader) error {

	contents, err := ioutil.ReadAll(ethGenesisFile)
	if err != nil {
		return err
	}

	var coreGenesis = core.Genesis{}

	if err := json.Unmarshal(contents, &coreGenesis); err != nil {
		return err
	}

	// if no priv val, make it
	privValFile := config.GetString("priv_validator_file")
	if _, err := os.Stat(privValFile); os.IsNotExist(err) {
		privValidator := types.GenPrivValidator()
		privValidator.SetFile(privValFile)
		privValidator.Save()

		// if no genesis, make it using the priv val
		genFile := config.GetString("genesis_file")
		if _, err := os.Stat(genFile); os.IsNotExist(err) {
			genDoc := types.GenesisDoc{
				ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
				Consensus: types.CONSENSUS_POS,
			}

			coinbase, amount, checkErr := checkAccount(coreGenesis)
			if(checkErr != nil) {
				glog.V(logger.Error).Infof(checkErr.Error())
				cmn.Exit(checkErr.Error())
			}

			genDoc.Validators = []types.GenesisValidator{types.GenesisValidator{
				EthAccount: coinbase,
				PubKey: privValidator.PubKey,
				Amount: amount,
			}}
			genDoc.SaveAs(genFile)
		}
	}

	// TODO: if there is a priv val but no genesis
	return nil
}

func checkAccount(coreGenesis core.Genesis) (common.Address, int64, error) {

	coinbase := common.HexToAddress(coreGenesis.Coinbase)
	amount := int64(10)
	balance := big.NewInt(-1)
	found := false
	fmt.Printf("checkAccount(), coinbase is %x\n", coinbase)
	for addr, account := range coreGenesis.Alloc {
		address := common.HexToAddress(addr)
		fmt.Printf("checkAccount(), address is %x\n", address)
		if coinbase == address {
			balance = common.String2Big(account.Balance)
			found = true
			break
		}
	}

	if( !found ) {
		fmt.Printf("invalidate eth_account\n")
		return common.Address{}, int64(0), errors.New("invalidate eth_account")
	}

	if ( balance.Cmp(big.NewInt(amount)) < 0) {
		fmt.Printf("balance is not enough to be support validator's amount, balance is %v, amount is %v\n",
			balance, amount)
		return common.Address{}, int64(0), errors.New("no enough balance")
	}

	return coinbase, amount, nil
}
