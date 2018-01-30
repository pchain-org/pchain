package types

import (
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/ethereum/go-ethereum/common"

	abciTypes "github.com/tendermint/abci/types"
	"fmt"
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

func (gv *GenesisValidator) ToAbciValidator () *abciTypes.Validator {

	return &abciTypes.Validator{
		PubKey: gv.PubKey.Bytes(),
		Power: uint64(gv.Amount),
	}
}

func FromAbciValidator (val *abciTypes.Validator) *GenesisValidator {

	pubkey, err := crypto.PubKeyFromBytes(val.PubKey)
	if err != nil {
		fmt.Printf("\n\n\n!!!FromAbciValidator(), pubkey convert failed!!!\n\n\n")
	}
	return &GenesisValidator{
		EthAccount: common.Address{},
		PubKey: pubkey,
		Amount: int64(val.Power),
		Name: "",
	}
}

func FromAbciValidators (vals []*abciTypes.Validator) []*GenesisValidator {

	genVals := make([]*GenesisValidator, len(vals))
	for i, val := range vals {
		genVals[i] = FromAbciValidator(val)
	}

	return genVals
}
