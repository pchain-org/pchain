package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/go-wire"
	"gopkg.in/urfave/cli.v1"
	"os"
	"path/filepath"
)

func GeneratePrivateValidatorCmd(ctx *cli.Context) error {

	address := ctx.Args().First()

	if address == "" {
		log.Info("address is empty, need an address")
		return nil
	}

	datadir := ctx.GlobalString(utils.DataDirFlag.Name);
	if err := os.MkdirAll(datadir, 0700); err != nil {
		return err
	}

	privValFile := filepath.Join(ctx.GlobalString(utils.DataDirFlag.Name), "priv_validator.json")

	validator := types.GenPrivValidatorKey(common.HexToAddress(address))
	fmt.Printf(string(wire.JSONBytesPretty(validator)))
	validator.SetFile(privValFile)
	validator.Save()

	return nil
}
