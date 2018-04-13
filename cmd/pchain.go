package main

import (
	"fmt"
	"gopkg.in/urfave/cli.v1"
)

func pchainCmd(ctx *cli.Context) error {

	if ctx == nil {
		fmt.Printf("oh, ctx is null, how pchain works?")
		return nil
	}

	fmt.Printf("pchain supports large scale block-chain applicaitons with multi-chain\n")
	return nil
}
