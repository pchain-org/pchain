package epoch

import (
	//"time"
	"github.com/tendermint/go-wire"
	. "github.com/tendermint/go-common"
)

var timeLayout = "1970-01-01 00:00:00"

type OneEpochDoc struct {
	Number 		string	`json:"number"`
	RewardPerBlock 	string	`json:"reward_per_block"`
	StartBlock	string	`json:"start_block"`
	EndBlock	string	`json:"end_block"`
	StartTime	string	`json:"start_time"`
	EndTime		string	`json:"end_time"`
	BlockGenerated	string	`json:"block_generated"`
	Status		string	`json:"status"`
}

type RewardSchemeDoc struct {
	TotalReward		string		`json:"total_reward"`
	PreAllocated		string		`json:"pre_allocated"`
	AddedPerYear		string		`json:"added_per_year"`
	RewardFirstYear		string		`json:"reward_first_year"`
	DescendPerYear		string		`json:"descend_per_year"`
	Allocated		string		`json:"allocated"`
}

type EpochDoc struct {
	RewardScheme RewardSchemeDoc
	CurrentEpoch OneEpochDoc
}

func (epochDoc *EpochDoc)SaveAs(file string) error {
	genDocBytes := wire.JSONBytesPretty(epochDoc)
	return WriteFile(file, genDocBytes, 0644)
}

func epochFromJSON(jsonBlob []byte) (epochDoc *EpochDoc, err error) {
	wire.ReadJSONPtr(&epochDoc, jsonBlob, &err)
	return
}
