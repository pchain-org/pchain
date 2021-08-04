package main

import (
	"fmt"
	gethmain "github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"gopkg.in/urfave/cli.v1"
	"strconv"
)

func dumpCmd(ctx *cli.Context) error {

	strChildChain := ctx.GlobalString(StringChainIdFlag.Name)
	stack, _ := gethmain.MakeConfigNode(ctx, strChildChain)

	chain, chainDb := utils.MakeChain(ctx, stack)
	for _, arg := range ctx.Args() {
		var block *types.Block
		if _, err := strconv.Atoi(arg); err != nil {
			block = chain.GetBlockByHash(common.HexToHash(arg))
		} else {
			num, _ := strconv.Atoi(arg)
			block = chain.GetBlockByNumber(uint64(num))
		}
		if block == nil {
			fmt.Println("{}")
			utils.Fatalf("block not found")
		} else {
			state, err := state.New(block.Root(), state.NewDatabase(chainDb))
			if err != nil {
				utils.Fatalf("could not create new state: %v", err)
			}
			fmt.Printf("%s\n", state.Dump())
		}
	}

	chainDb.Close()
	return nil
}
