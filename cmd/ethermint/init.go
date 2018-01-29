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
	"fmt"
	ep "github.com/tendermint/tendermint/epoch"
)

func initCmd(ctx *cli.Context) error {

	// ethereum genesis.json
	ethGenesisPath := ctx.Args().First()
	if len(ethGenesisPath) == 0 {
		utils.Fatalf("must supply path to genesis JSON file")
	}

	init_eth_blockchain(ethGenesisPath, ctx)

	init_em_files(ethGenesisPath)

	rsTmplPath := ctx.Args().Get(1)
	init_reward_scheme_files(rsTmplPath)

	return nil
}

func init_eth_blockchain(ethGenesisPath string, ctx *cli.Context) {

	chainDb, err := ethdb.NewLDBDatabase(filepath.Join(utils.MakeDataDir(ctx), "chaindata"), 0, 0)
	if err != nil {
		utils.Fatalf("could not open database: %v", err)
	}

	genesisFile, err := os.Open(ethGenesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}

	block, err := core.WriteGenesisBlock(chainDb, genesisFile)
	if err != nil {
		utils.Fatalf("failed to write genesis block: %v", err)
	}

	glog.V(logger.Info).Infof("successfully wrote genesis block and/or chain rule set: %x", block.Hash())
}

func init_em_files(ethGenesisPath string) {

	ethGenesisFile, err := os.Open(ethGenesisPath)
	if err != nil {
		utils.Fatalf("failed to read eth_genesis file: %v", err)
	}

	contents, err := ioutil.ReadAll(ethGenesisFile)
	if err != nil {
		utils.Fatalf("failed to read eth_genesis file: %v", err)
	}

	var coreGenesis = core.Genesis{}

	if err := json.Unmarshal(contents, &coreGenesis); err != nil {
		utils.Fatalf("failed to unmarshal content from eth_genesis file: %v", err)
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
}


func init_reward_scheme_files(rsTmplPath string) {

	rsPath := config.GetString("epoch_file")

	if _, err := os.Stat(rsTmplPath); os.IsNotExist(err) {

		rsDoc := ep.EpochDoc{
			RewardScheme : ep.RewardSchemeDoc{
				TotalReward : "210000000000000000000000000",
				PreAllocated : "100000000000000000000000000",
				AddedPerYear : "0",
				RewardFirstYear : "20000000000000000000000000", //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PAI by 10 years
				DescendPerYear : "2000000000000000000000000",
				Allocated : "0",
			},
			CurrentEpoch : ep.OneEpochDoc {
				Number :		"0",
				RewardPerBlock :	"0",
				StartBlock :		"0",
				EndBlock :		"0",
				StartTime :		"0",
				EndTime :		"0",//not accurate for current epoch
				BlockGenerated :	"0",
				Status :		"0",
			},
		}

		rsDoc.SaveAs(rsPath)

	} else if rsFile, err := os.Open(rsTmplPath); err == nil {

		contents, err := ioutil.ReadAll(rsFile)
		if err != nil {
			utils.Fatalf("failed to read ethermint's reward scheme file: %v", err)
		}

		//fmt.Printf("content is: %s\n", string(contents))

		rsDoc := &ep.RewardSchemeDoc{}
		if err := json.Unmarshal(contents, &rsDoc); err != nil {
			utils.Fatalf("failed to unmarshal ethermint's reward scheme file: %v", err)
		}

		//fmt.Printf("rsDoc is: %v\n", rsDoc)
		//rsDoc.SaveAs(rsPath)
	} else {
		utils.Fatalf("failed to read ethermint's reward scheme file: %v", err)
	}
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
