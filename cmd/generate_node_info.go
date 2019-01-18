package main

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"gopkg.in/urfave/cli.v1"
	"net"
	"strconv"
)

func GenerateNodeInfoCmd(ctx *cli.Context) error {

	number, err := strconv.Atoi(ctx.Args().First())
	if err != nil {
		return nil
	}

	for i := 0; i < number; i++ {

		nodeKey, err := crypto.GenerateKey()
		if err != nil {
			fmt.Printf("could not generate key: %v\n", err)
			return err
		}
		keyString := hex.EncodeToString(crypto.FromECDSA(nodeKey))
		fmt.Printf("%s\n", keyString)

		//put p2p.Server here to notice that this logic should keep consistent with p2p module
		srv := &p2p.Server{}
		srv.Config.PrivateKey = nodeKey
		nodeInfo := discover.Node{IP: net.ParseIP("0.0.0.0"), ID: discover.PubkeyID(&srv.PrivateKey.PublicKey)}

		fmt.Printf("%s\n", nodeInfo.String())
	}

	return nil
}
