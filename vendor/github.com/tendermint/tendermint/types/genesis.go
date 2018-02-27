package types

import (
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/ethereum/go-ethereum/common"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

type GenesisValidator struct {
	EthAccount common.Address `json:"eth_account"`
	PubKey crypto.PubKey `json:"pub_key"`
	Amount int64         `json:"amount"`
	Name   string        `json:"name"`
}

var CONSENSUS_POS string = "pos"
var CONSENSUS_POW string = "pow"

type GenesisDoc struct {
	GenesisTime time.Time          `json:"genesis_time"`
	ChainID     string             `json:"chain_id"`
	Consensus   string             `json:"consensus"` //should be 'pos' or 'pow'
	Validators  []GenesisValidator `json:"validators"`
	AppHash     []byte             `json:"app_hash"`
}

// Utility method for saving GenensisDoc as JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes := wire.JSONBytesPretty(genDoc)
	return WriteFile(file, genDocBytes, 0644)
}

//------------------------------------------------------------
// Make genesis state from file

func GenesisDocFromJSON(jsonBlob []byte) (genDoc *GenesisDoc, err error) {
	wire.ReadJSONPtr(&genDoc, jsonBlob, &err)
	return
}

func GenesisValidatorsString(vs []*GenesisValidator) string {
	s := make([]GenesisValidator, len(vs))
	for i, v := range vs {
		s[i] = GenesisValidator{v.EthAccount,v.PubKey,v.Amount,v.Name}
	}
	return string(wire.JSONBytes(s))
}
