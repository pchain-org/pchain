package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

var CONSENSUS_POS string = "pos"
var CONSENSUS_POW string = "pow"

type GenesisValidator struct {
	EthAccount     common.Address `json:"address"`
	PubKey         crypto.PubKey  `json:"pub_key"`
	Amount         *big.Int       `json:"amount"`
	Name           string         `json:"name"`
	RemainingEpoch uint64         `json:"epoch"`
}

type OneEpochDoc struct {
	Number         uint64             `json:"number"`
	RewardPerBlock *big.Int           `json:"reward_per_block"`
	StartBlock     uint64             `json:"start_block"`
	EndBlock       uint64             `json:"end_block"`
	Status         int                `json:"status"`
	Validators     []GenesisValidator `json:"validators"`
}

type RewardSchemeDoc struct {
	TotalReward        *big.Int `json:"total_reward"`
	RewardFirstYear    *big.Int `json:"reward_first_year"`
	EpochNumberPerYear uint64   `json:"epoch_no_per_year"`
	TotalYear          uint64   `json:"total_year"`
}

type GenesisDoc struct {
	ChainID      string          `json:"chain_id"`
	Consensus    string          `json:"consensus"` //should be 'pos' or 'pow'
	GenesisTime  time.Time       `json:"genesis_time"`
	RewardScheme RewardSchemeDoc `json:"reward_scheme"`
	CurrentEpoch OneEpochDoc     `json:"current_epoch"`
}

// Utility method for saving GenensisDoc as JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.MarshalIndent(genDoc, "", "\t")
	if err != nil {
		fmt.Println(err)
	}

	return WriteFile(file, genDocBytes, 0644)
}

//------------------------------------------------------------
// Make genesis state from file

func GenesisDocFromJSON(jsonBlob []byte) (genDoc *GenesisDoc, err error) {
	err = json.Unmarshal(jsonBlob, &genDoc)
	return
}

var MainnetGenesisJSON string = `{
	"chain_id": "pchain",
	"consensus": "pos",
	"genesis_time": "2019-03-04T10:07:44.721697528Z",
	"reward_scheme": {
		"total_reward": "0xfb0c9ff7abf3b85f800000",
		"reward_first_year": "0x1f6193fef57e770bf00000",
		"epoch_no_per_year": "0x6540",
		"total_year": "0x17"
	},
	"current_epoch": {
		"number": "0x0",
		"reward_per_block": "0x10ed3d42c506b2bf",
		"start_block": "0x0",
		"end_block": "0x258",
		"validators": [{
				"amount": "0x898ad220390e380030000",
				"epoch": "0xf",
				"address": "0xfbd64f3e2b9a40e3892e79af615f76e15f772440",
				"name": "",
				"pub_key": "0x2D985CC4CD3317DCA4BD45AADBE4F686BBD1C38C90FAFE761D99F5B45D848B2285CFDB2798310C07B0716ABB1A1B138CAA557D2DDDD3B854C3243DBB7E08B3EA8DD96C6C81ACBECB5069C095A4E2640EC667CC33BB6CC14FC6D114719AC7DA973E0985A0F78171B5EA872D4D7FD7D650B2E30878F0EB1C66B4FB197BA011B2EB"
			}, {
				"amount": "0x62a1310adfc7f40000000",
				"epoch": "0xf",
				"address": "0x49fab03ffaa398057507e0c08228315af30b55ea",
				"name": "",
				"pub_key": "0x81D5C819FA78B8AD0D780D753964EE29A3BCC7A2CF159D65B4F3B7302C328E978B6442C11175E8C80478AFCCFA9F1B31A343B8DCC4D12209153FA862B2B4564D722168D50CE1230D6D99EE31EC858342ADF316FD305184FC399232840BDBA9222DA5865436910BC6EBE28083BBDAF120E492E891EFE5EADB3ADEE660603AD14E"
			}, {
				"amount": "0x8459520ab06af00000000",
				"epoch": "0xf",
				"address": "0xb62073fd7055f30fd883d950651377c52158ad44",
				"name": "",
				"pub_key": "0x07707BBC0FE01C54A88AF75E59214AC695097FBF091D53B6E2DEA4A00CF513980A71A4C36C3A92BD48D740608CE60D39AC97A0D1FA5642C7CDC82B459A9A9CD36242805FCDB310DD255C089331D8C53D6C368BC2B258AD6267F0B26AC9160C5B3E5B82FD211BF52936F3EA96DA82CA07901BE4B45465EFE169C792FE868F81AB"
			},
			{
				"address": "0x826b6e6a868a62508989666fa80f4b099a36f951",
				"pub_key": "0x65D6182CDD778A93D9C47EF9E4B30863B71AA0672237E9239EA19C37DA42AF14142D63F4035A5E9C2C5E1DE91CBEAB1630440D74D3C340251C8623D0B16D86052A5A109F9F6EB761B6AB5D0BF97AF0A07154C57CD26E3880AD235B744E076FBF38D8F0D74C0C9418AA91C61A5DFAA06A927B7124B512F888AFAFF72871DD0F86",
				"amount": "0x493ca50db0fcac0000000",
				"name": "",
				"epoch": "0x0"
			},
			{
				"address": "0x9db7b33f775b96d12a252e7eb10cfc643c5ea3fd",
				"pub_key": "0x3227C4C7E5BC5F7C1259BA23E584EECB4C27A0712367A231A246AE29D59902575CB7C2A0C381AD22D65541EDD540956053FD0779ACE309CEF666EDE3D54B323913C9A3F54771DFA4F915EC5211A220D5959C0E4E0E66360EA75C85232D8F68B35CE1E6A482BBC1C2B535D6071187CDF4C2A60F83DF13BF7AF54AF0BD18A27C5B",
				"amount": "0x5d48e40a0bf8c00000000",
				"name": "",
				"epoch": "0x0"
			},
			{
				"address": "0x9632ea925d39fda19146fceb646b38ec66a5993a",
				"pub_key": "0x725D95C4F958E458D20CC2FE3275D1C0E8376F7F83BAB5713264D28DB8E68E5D2168B9B7D20A5B0B3904F916EB2B014B6FE611F29619C156838634A33A94653D2F3995B685C95CF3453AD338DD8DD2C8F884956AE57F935A8694E5211646FC3A7E6F9E2B4E4287B9412516BE02355114DFF93B3B7E4A76B15732E6C7DCA31B44",
				"amount": "0x4ef075355c96700000000",
				"name": "",
				"epoch": "0x0"
			},
			{
				"address": "0x3839d4958a8d930101eccb8089bcd2d9ee9c597e",
				"pub_key": "0x0A2C9F325874BBC248D9DE013447829B8A7F964DC12EE841C7F551EB87C43A9A5ED25F4B676291A7D5A56EE75054BFFD0E8A724CC635723817AFE946BC116DFB035E8C7ED77AA6420801AABB91B6E2E0D9C32EFF0FD254DD9598684261FE467E799805E6545D5F22D5334E7677AAFDF561AD634DB941E1DC40E0474F5D1F0FB6",
				"amount": "0x3796274caf64c80000000",
				"name": "",
				"epoch": "0x0"
			},
			{
				"address": "0x8035a9b057055d4d117b3aba071e9db7d6055412",
				"pub_key": "0x391DF89C35EC507F4B5040BE9CBEE9E6F674BE186D6713851556DD507E8ED9A5582B916F1856A92A579ECD0AA3AEDD1E4A8AA8AA24A18B88FBD1A79B0313FA9D067000BA8F25F128985FDF9FC46743AA70704D485A9F4721CC92ECC8EFD6D5C7327063C62FED9BBD880DD5FA3389657E633E9CEC05E2CC7DA81EA8D254BA2688",
				"amount": "0x32eb01744459de0000000",
				"name": "",
				"epoch": "0x0"
			}
		]
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
    "validators": [
      {
        "amount": "200000000000000000000000",
        "epoch": 11,
        "eth_account": "D2FD09246E2CED295411F1F863E6E7B5929BCC59",
        "name": "",
        "pub_key": [
          4,
          "0C75143EB5952A46803215DAA3E8F53E2245501C2BCBA0285E624DE74C6DF703157710EBA5023EF1B082FC8ECA27670E131745ED01954A48892AD1F8CF2602303B2D37035353410982E31AC15BEF80902B38591593A312C9D917F2ED5C941164425FE4B19C1C65C3E2C31B544A3BABF85EE0BEEDE5D9E1F8C004C28A9C93E47F"
        ]
      },
      {
        "amount": "200000000000000000000000",
        "epoch": 11,
        "eth_account": "C6179A651918888251380A4E3FEE6AF81CF091D1",
        "name": "",
        "pub_key": [
          4,
          "72747AFEE21059D6BE594C815A280EF64620EE0BA502DF5A064AB45AC2D633CA106E6439414C8CB688FC3699566472F3A056471581713EA09B5E9C6216744CC68CCCA0A390A0088248DBC2679C9B9CD699631472C4000F64D77919A8472A75716D690F669F53DE3ED2919A1457D75FABDFB4ACD7BD8BAB35B7D6CD8B59CFC4A5"
        ]
      },
      {
        "amount": "200000000000000000000000",
        "epoch": 11,
        "eth_account": "79CD31B59E3FAAB6DEEA68FBBAAFA4DA748BBDF6",
        "name": "",
        "pub_key": [
          4,
          "7315DF293B07C52EF6C1FC05018A1CA4FB630F6DBD4F1216804FEDDC2F04CD2932A5AB72B6910145ED97A5FFA0CDCB818F928A8921FDAE8033BF4259AC3400552065951D2440C25A6994367E1DC60EE34B34CB85CD95304B24F9A07473163F1F24C79AC5CBEC240B5EAA80907F6B3EDD44FD8341BF6EB8179334105FEDE6E790"
        ]
      },
      {
        "amount": "200000000000000000000000",
        "epoch": 0,
        "eth_account": "4CACBCBF218679DCC9574A90A2061BCA4A8D8B6C",
        "name": "",
        "pub_key": [
          4,
          "085586D41F70435700850E19B7DE54B3E793C5EC4C6EC502D19030EF4F2122823E5A765E56CBA7B4C57E50561F77B022313C39895CA303F3C95D7B7282412F334778B95ACE046A79AEA4DB148334527250C8895AC5DB80459BF5D367236B59AF2DB5C0254E30A6D8CD1FA10AB8A5D872F5EBD312D3160D3E4DD496973BDC75E0"
        ]
      }
    ]
  },
  "genesis_time": "2019-02-18T07:12:15.424Z",
  "reward_scheme": {
    "epoch_no_per_year": "183",
    "reward_first_year": "37937500000000000000000000",
    "total_reward": "303500000000000000000000000",
    "total_year": "23"
  }
}
`

func (ep OneEpochDoc) MarshalJSON() ([]byte, error) {
	type hexEpoch struct {
		Number         hexutil.Uint64     `json:"number"`
		RewardPerBlock *hexutil.Big       `json:"reward_per_block"`
		StartBlock     hexutil.Uint64     `json:"start_block"`
		EndBlock       hexutil.Uint64     `json:"end_block"`
		Validators     []GenesisValidator `json:"validators"`
	}
	var enc hexEpoch
	enc.Number = hexutil.Uint64(ep.Number)
	enc.RewardPerBlock = (*hexutil.Big)(ep.RewardPerBlock)
	enc.StartBlock = hexutil.Uint64(ep.StartBlock)
	enc.EndBlock = hexutil.Uint64(ep.EndBlock)
	if ep.Validators != nil {
		enc.Validators = ep.Validators
	}
	return json.Marshal(&enc)
}

func (ep *OneEpochDoc) UnmarshalJSON(input []byte) error {
	type hexEpoch struct {
		Number         hexutil.Uint64     `json:"number"`
		RewardPerBlock *hexutil.Big       `json:"reward_per_block"`
		StartBlock     hexutil.Uint64     `json:"start_block"`
		EndBlock       hexutil.Uint64     `json:"end_block"`
		Validators     []GenesisValidator `json:"validators"`
	}
	var dec hexEpoch
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	ep.Number = uint64(dec.Number)
	ep.RewardPerBlock = (*big.Int)(dec.RewardPerBlock)
	ep.StartBlock = uint64(dec.StartBlock)
	ep.EndBlock = uint64(dec.EndBlock)
	if dec.Validators == nil {
		return errors.New("missing required field 'validators' for Genesis/epoch")
	}
	ep.Validators = dec.Validators
	return nil
}

func (gv GenesisValidator) MarshalJSON() ([]byte, error) {
	type hexValidator struct {
		Address        common.Address `json:"address"`
		PubKey         string         `json:"pub_key"`
		Amount         *hexutil.Big   `json:"amount"`
		Name           string         `json:"name"`
		RemainingEpoch hexutil.Uint64 `json:"epoch"`
	}
	var enc hexValidator
	enc.Address = gv.EthAccount
	enc.PubKey = gv.PubKey.KeyString()
	enc.Amount = (*hexutil.Big)(gv.Amount)
	enc.Name = gv.Name
	enc.RemainingEpoch = hexutil.Uint64(gv.RemainingEpoch)

	return json.Marshal(&enc)
}

func (rs *GenesisValidator) UnmarshalJSON(input []byte) error {
	type hexValidator struct {
		Address        common.Address `json:"address"`
		PubKey         string         `json:"pub_key"`
		Amount         *hexutil.Big   `json:"amount"`
		Name           string         `json:"name"`
		RemainingEpoch hexutil.Uint64 `json:"epoch"`
	}
	var dec hexValidator
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	rs.EthAccount = dec.Address

	pubkeyBytes := common.FromHex(dec.PubKey)
	if dec.PubKey == "" || len(pubkeyBytes) != 128 {
		return errors.New("wrong format of required field 'pub_key' for Genesis/epoch/validators")
	}
	var blsPK crypto.BLSPubKey
	copy(blsPK[:], pubkeyBytes)
	rs.PubKey = blsPK

	if dec.Amount == nil {
		return errors.New("missing required field 'amount' for Genesis/epoch/validators")
	}
	rs.Amount = (*big.Int)(dec.Amount)
	rs.Name = dec.Name
	rs.RemainingEpoch = uint64(dec.RemainingEpoch)
	return nil
}

func (rs RewardSchemeDoc) MarshalJSON() ([]byte, error) {
	type hexRewardScheme struct {
		TotalReward        *hexutil.Big   `json:"total_reward"`
		RewardFirstYear    *hexutil.Big   `json:"reward_first_year"`
		EpochNumberPerYear hexutil.Uint64 `json:"epoch_no_per_year"`
		TotalYear          hexutil.Uint64 `json:"total_year"`
	}
	var enc hexRewardScheme
	enc.TotalReward = (*hexutil.Big)(rs.TotalReward)
	enc.RewardFirstYear = (*hexutil.Big)(rs.RewardFirstYear)
	enc.EpochNumberPerYear = hexutil.Uint64(rs.EpochNumberPerYear)
	enc.TotalYear = hexutil.Uint64(rs.TotalYear)

	return json.Marshal(&enc)
}

func (rs *RewardSchemeDoc) UnmarshalJSON(input []byte) error {
	type hexRewardScheme struct {
		TotalReward        *hexutil.Big   `json:"total_reward"`
		RewardFirstYear    *hexutil.Big   `json:"reward_first_year"`
		EpochNumberPerYear hexutil.Uint64 `json:"epoch_no_per_year"`
		TotalYear          hexutil.Uint64 `json:"total_year"`
	}
	var dec hexRewardScheme
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.TotalReward == nil {
		return errors.New("missing required field 'total_reward' for Genesis/reward_scheme")
	}
	rs.TotalReward = (*big.Int)(dec.TotalReward)
	if dec.RewardFirstYear == nil {
		return errors.New("missing required field 'reward_first_year' for Genesis/reward_scheme")
	}
	rs.RewardFirstYear = (*big.Int)(dec.RewardFirstYear)

	rs.EpochNumberPerYear = uint64(dec.EpochNumberPerYear)
	rs.TotalYear = uint64(dec.TotalYear)

	return nil
}
