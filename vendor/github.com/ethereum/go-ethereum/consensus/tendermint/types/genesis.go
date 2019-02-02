package types

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

var CONSENSUS_POS string = "pos"
var CONSENSUS_POW string = "pow"

type GenesisValidator struct {
	EthAccount     common.Address `json:"eth_account"`
	PubKey         crypto.PubKey  `json:"pub_key"`
	Amount         *big.Int       `json:"amount"`
	Name           string         `json:"name"`
	RemainingEpoch uint64         `json:"epoch"`
}

type OneEpochDoc struct {
	Number         string             `json:"number"`
	RewardPerBlock string             `json:"reward_per_block"`
	StartBlock     string             `json:"start_block"`
	EndBlock       string             `json:"end_block"`
	BlockGenerated string             `json:"block_generated"`
	Status         string             `json:"status"`
	Validators     []GenesisValidator `json:"validators"`
}

type RewardSchemeDoc struct {
	TotalReward        string `json:"total_reward"`
	RewardFirstYear    string `json:"reward_first_year"`
	EpochNumberPerYear string `json:"epoch_no_per_year"`
	TotalYear          string `json:"total_year"`
}

type GenesisDoc struct {
	AppHash      []byte          `json:"app_hash"`
	ChainID      string          `json:"chain_id"`
	Consensus    string          `json:"consensus"` //should be 'pos' or 'pow'
	GenesisTime  time.Time       `json:"genesis_time"`
	RewardScheme RewardSchemeDoc `json:"reward_scheme"`
	CurrentEpoch OneEpochDoc     `json:"current_epoch"`
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
		s[i] = GenesisValidator{v.EthAccount, v.PubKey, v.Amount, v.Name}
	}
	return string(wire.JSONBytes(s))
}

var MainnetGenesisJSON string = `{
	"app_hash": "",
	"chain_id": "pchain",
	"consensus": "pos",
	"current_epoch": {
		"block_generated": "0",
		"end_block": "2592000",
		"number": "0",
		"reward_per_block": "1219698431069958847",
		"start_block": "0",
		"status": "0",
		"validators": [
			{
				"amount": "5027221",
				"eth_account": "d498329bda9dd1cddd397910e79baca72fae1e1c",
				"name": "",
				"pub_key": [
					4,
					"6EFECE72321773490F67F5B6F5104F699EE2EEC679EF8BDEE032CC7D2854B64D402933FA0D9A4D1254AD78043AF747DD568A6362E7DDE83267E04C542C053E1A072351B2347267AD33991E8D26D850F411CA490978C7FDDDA436BB13C6B3990D8632CA6E9A90D22D9EC6B370365CCDE58EC77733C78AFA1AD2E970D679174264"
				]
 			},
			{
				"amount": "10392399",
				"eth_account": "e218d242af2159e6aba5176a1c2564291a48da2d",
				"name": "",
				"pub_key": [
					4,
					"5695B283C0B256073F340D1DA58964448FA7CFF970D7D7CABEDC879FBBA2630C46EA0F6C0397E7CEFEE34A26C636A5193DAC095CFFCCA0E98E072551B31338F91E63106C22BAF75D9C6B1937D13C87B74A08F82590163F755B758ABC3AE9D0FE70C50A9A56D4DFCFB6050486A2DD6135C9D5025FE64C1DFBD3B649A36BB446D0"
				]
 			}
		]
	},
	"genesis_time": "2019-01-22T02:26:57.071Z",
	"reward_scheme": {
		"epoch_no_per_year": "12",
		"reward_first_year": "37937500000000000000000000",
		"total_reward": "303500000000000000000000000",
		"total_year": "23"
	}
}`

var TestnetGenesisJSON string = `{
	"app_hash": "",
	"chain_id": "testnet",
	"consensus": "pos",
	"current_epoch": {
		"block_generated": "0",
		"end_block": "172800",
		"number": "0",
		"reward_per_block": "1219698431069958847",
		"start_block": "0",
		"status": "0",
		"validators": [{
				"amount": "100",
				"eth_account": "D2FD09246E2CED295411F1F863E6E7B5929BCC59",
				"name": "",
				"pub_key": [4, "0C75143EB5952A46803215DAA3E8F53E2245501C2BCBA0285E624DE74C6DF703157710EBA5023EF1B082FC8ECA27670E131745ED01954A48892AD1F8CF2602303B2D37035353410982E31AC15BEF80902B38591593A312C9D917F2ED5C941164425FE4B19C1C65C3E2C31B544A3BABF85EE0BEEDE5D9E1F8C004C28A9C93E47F"]
			}, {
				"amount": "100",
				"eth_account": "C6179A651918888251380A4E3FEE6AF81CF091D1",
				"name": "",
				"pub_key": [4, "72747AFEE21059D6BE594C815A280EF64620EE0BA502DF5A064AB45AC2D633CA106E6439414C8CB688FC3699566472F3A056471581713EA09B5E9C6216744CC68CCCA0A390A0088248DBC2679C9B9CD699631472C4000F64D77919A8472A75716D690F669F53DE3ED2919A1457D75FABDFB4ACD7BD8BAB35B7D6CD8B59CFC4A5"]
			}, {
				"amount": "100",
				"eth_account": "79CD31B59E3FAAB6DEEA68FBBAAFA4DA748BBDF6",
				"name": "",
				"pub_key": [4, "7315DF293B07C52EF6C1FC05018A1CA4FB630F6DBD4F1216804FEDDC2F04CD2932A5AB72B6910145ED97A5FFA0CDCB818F928A8921FDAE8033BF4259AC3400552065951D2440C25A6994367E1DC60EE34B34CB85CD95304B24F9A07473163F1F24C79AC5CBEC240B5EAA80907F6B3EDD44FD8341BF6EB8179334105FEDE6E790"]
			},
			{
				"amount": "100",
				"eth_account": "4CACBCBF218679DCC9574A90A2061BCA4A8D8B6C",
				"name": "",
				"pub_key": [
					4,
					"085586D41F70435700850E19B7DE54B3E793C5EC4C6EC502D19030EF4F2122823E5A765E56CBA7B4C57E50561F77B022313C39895CA303F3C95D7B7282412F334778B95ACE046A79AEA4DB148334527250C8895AC5DB80459BF5D367236B59AF2DB5C0254E30A6D8CD1FA10AB8A5D872F5EBD312D3160D3E4DD496973BDC75E0"
				]
			}
		]
	},
	"genesis_time": "2018-12-06T08:16:31.221Z",
	"reward_scheme": {
		"epoch_no_per_year": "183",
		"reward_first_year": "37937500000000000000000000",
		"total_reward": "303500000000000000000000000",
		"total_year": "23"
	}
}`
