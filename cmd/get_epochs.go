package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/ethclient"
	"gopkg.in/urfave/cli.v1"
	"os"
)

var (
	getEpochsCommand = cli.Command{
		Name:     "getEpochs",
		Usage:    "get all epochs for one chain",
		Category: "TOOLKIT COMMANDS",
		Action:   utils.MigrateFlags(getEpochs),
		Flags: []cli.Flag{
			ToolkitDirFlag,
			ChainIdFlag,
			utils.RPCListenAddrFlag,
			utils.RPCPortFlag,
		},
		Description: `
Get all epochs for one chain. Just get their (startBlock, endBlock, startTime, endTime) for now.`,
	}
)

func getEpochs(ctx *cli.Context) error {

	toolkitDir := ctx.GlobalString(ToolkitDirFlag.Name)
	if toolkitDir == "" {
		toolkitDir = "."
	}

	if !ctx.IsSet(ChainIdFlag.Name) {
		Exit(fmt.Errorf("--chainId needed"))
	}
	chainId := ctx.GlobalString(ChainIdFlag.Name)
	localRpcPrefix := calLocalRpcPrefix(ctx)
	chainUrl := localRpcPrefix + chainId

	curEpochNumber, err := ethclient.WrpGetCurrentEpochNumber(chainUrl)
	if err != nil {
		Exit(err)
	}

	epochFileName := toolkitDir + string(os.PathSeparator) + chainId + "_epochs.txt"

	// Open the file handle and potentially wrap with a gzip stream
	fh, err := os.OpenFile(epochFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer fh.Close()

	timeFormat := "2006-01-02 15:04:05"
	for number := uint64(0); number < curEpochNumber; number++ {
		rpcEpoch, err := ethclient.WrpGetEpoch(chainUrl, number)
		if err != nil {
			Exit(err)
		}

		fh.WriteString(fmt.Sprintf("epoch_%d\n", rpcEpoch.Number))
		fh.WriteString(fmt.Sprintf("StartBlock: %d	EndBlock: %d\n", rpcEpoch.StartBlock, rpcEpoch.EndBlock))
		fh.WriteString(fmt.Sprintf("StartTime: %s	EndTime: %s\n", rpcEpoch.StartTime.Format(timeFormat), rpcEpoch.EndTime.Format(timeFormat)))
		fh.WriteString("\n")
	}

	return nil
}
