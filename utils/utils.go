package utils

import (
	"os"
	cfg "github.com/tendermint/go-config"
	"io/ioutil"
	"github.com/tendermint/tendermint/types"
	"fmt"
)

func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}

var Fmt = fmt.Sprintf
func GetGenDocFromFile(config cfg.Config) *types.GenesisDoc {

	genDocFile := config.GetString("genesis_file")
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		Exit(Fmt("MakeGenesisValidatorsFromFile(), Couldn't read GenesisDoc file: %v", err))
	}

	genDoc, err := types.GenesisDocFromJSON(genDocJSON)
	if err != nil {
		Exit(Fmt("MakeGenesisValidatorsFromFile(), Error reading GenesisDoc: %v", err))
	}

	if len(genDoc.Validators) == 0 {
		Exit(Fmt("MakeGenesisValidatorsFromFile(), The genesis file has no validators"))
	}

	return genDoc
}
