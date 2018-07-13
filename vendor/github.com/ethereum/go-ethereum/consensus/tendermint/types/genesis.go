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

var CONSENSUS_POS string = "pos"
var CONSENSUS_POW string = "pow"

type GenesisValidator struct {
	EthAccount common.Address `json:"eth_account"`
	PubKey crypto.PubKey      `json:"pub_key"`
	Amount int64              `json:"amount"`
	Name   string             `json:"name"`
}

type OneEpochDoc struct {
	Number 		string	            `json:"number"`
	RewardPerBlock 	string	            `json:"reward_per_block"`
	StartBlock	string	            `json:"start_block"`
	EndBlock	string	            `json:"end_block"`
	StartTime	string	            `json:"start_time"`
	EndTime		string	            `json:"end_time"`
	BlockGenerated	string	            `json:"block_generated"`
	Status		string	            `json:"status"`
	Validators      []GenesisValidator  `json:"validators"`
}

type RewardSchemeDoc struct {
	TotalReward		string		`json:"total_reward"`
	PreAllocated		string		`json:"pre_allocated"`
	AddedPerYear		string		`json:"added_per_year"`
	RewardFirstYear		string		`json:"reward_first_year"`
	DescendPerYear		string		`json:"descend_per_year"`
	Allocated		string		`json:"allocated"`
	EpochNumberPerYear		string		`json:"epoch_no_per_year"`
}

type GenesisDoc struct {
	//AppHash     []byte              `json:"app_hash"`
	ChainID     string              `json:"chain_id"`
	Consensus   string              `json:"consensus"` //should be 'pos' or 'pow'
	GenesisTime time.Time          	`json:"genesis_time"`
	RewardScheme RewardSchemeDoc    `json:"reward_scheme"`
	CurrentEpoch OneEpochDoc        `json:"current_epoch"`
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
