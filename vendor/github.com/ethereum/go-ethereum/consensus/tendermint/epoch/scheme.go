package epoch

import (
	"fmt"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/log"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"math/big"
	"strconv"
	"sync"
)

const rewardSchemeKey = "REWARDSCHEME"

type RewardScheme struct {
	mtx sync.Mutex
	db  dbm.DB

	TotalReward        *big.Int
	PreAllocated       *big.Int
	RewardFirstYear    *big.Int
	AddedPerYear       *big.Int
	DescendPerYear     *big.Int
	Allocated          *big.Int
	EpochNumberPerYear int
}

// Load Reward Scheme
func LoadRewardScheme(db dbm.DB) *RewardScheme {
	buf := db.Get([]byte(rewardSchemeKey))
	if len(buf) == 0 {
		return nil
	} else {
		rs := &RewardScheme{}
		err := wire.ReadBinaryBytes(buf, rs)
		if err != nil {
			log.Errorf("LoadRewardScheme Failed, error: %v", err)
			return nil
		}
		return rs
	}
}

// Convert Reward Scheme from json to struct
func MakeRewardScheme(db dbm.DB, rsDoc *tmTypes.RewardSchemeDoc) *RewardScheme {

	totalReward, _ := new(big.Int).SetString(rsDoc.TotalReward, 10)
	preAllocated, _ := new(big.Int).SetString(rsDoc.PreAllocated, 10)
	addedPerYear, _ := new(big.Int).SetString(rsDoc.AddedPerYear, 10)
	rewardFirstYear, _ := new(big.Int).SetString(rsDoc.RewardFirstYear, 10)
	descendPerYear, _ := new(big.Int).SetString(rsDoc.DescendPerYear, 10)
	allocated, _ := new(big.Int).SetString(rsDoc.Allocated, 10)
	epochNumberPerYear, _ := strconv.Atoi(rsDoc.EpochNumberPerYear)

	rs := &RewardScheme{
		db:                 db,
		TotalReward:        totalReward,
		PreAllocated:       preAllocated,
		RewardFirstYear:    rewardFirstYear,
		AddedPerYear:       addedPerYear,
		DescendPerYear:     descendPerYear,
		Allocated:          allocated,
		EpochNumberPerYear: epochNumberPerYear,
	}

	return rs
}

// Save the Reward Scheme to DB
func (rs *RewardScheme) Save() {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()
	rs.db.SetSync([]byte(rewardSchemeKey), wire.BinaryBytes(*rs))
}

func (rs *RewardScheme) String() string {

	return fmt.Sprintf("RewardScheme : {"+
		"totalReward : %v,\n"+
		"preAllocated : %v,\n"+
		"rewardFirstYear : %v,\n"+
		"addedPerYear : %v,\n"+
		"descendPerYear : %v,\n"+
		"allocated : %v,\n"+
		"epochNumberPerYear : %v,\n"+
		"}",
		rs.TotalReward,
		rs.PreAllocated,
		rs.RewardFirstYear,
		rs.AddedPerYear,
		rs.DescendPerYear,
		rs.Allocated,
		rs.EpochNumberPerYear)
}
