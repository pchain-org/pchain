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
	RewardFirstYear    *big.Int
	EpochNumberPerYear uint64
	TotalYear          uint64
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
	rewardFirstYear, _ := new(big.Int).SetString(rsDoc.RewardFirstYear, 10)
	epochNumberPerYear, _ := strconv.ParseUint(rsDoc.EpochNumberPerYear, 10, 64)
	totalYear, _ := strconv.ParseUint(rsDoc.TotalYear, 10, 64)

	rs := &RewardScheme{
		db:                 db,
		TotalReward:        totalReward,
		RewardFirstYear:    rewardFirstYear,
		EpochNumberPerYear: epochNumberPerYear,
		TotalYear:          totalYear,
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
		"rewardFirstYear : %v,\n"+
		"epochNumberPerYear : %v,\n"+
		"}",
		rs.TotalReward,
		rs.RewardFirstYear,
		rs.EpochNumberPerYear)
}
