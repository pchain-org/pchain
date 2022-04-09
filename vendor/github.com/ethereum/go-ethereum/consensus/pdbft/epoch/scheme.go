package epoch

import (
	"fmt"
	tmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/log"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"math"
	"math/big"
	"sync"
)

const rewardSchemeKey = "REWARDSCHEME"

var (
	mainnetChild0RewardFirstYear    = big.NewInt(math.MaxInt64)
	mainnetChild0EpochNumberPerYear = uint64(8760)
	mainnetChild0TotalYear          = uint64(23)

	testnetChild0RewardFirstYear    = big.NewInt(math.MaxInt64)
	testnetChild0EpochNumberPerYear = uint64(8760)
	testnetChild0TotalYear          = uint64(23)
)

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
		rs.db = db
		return rs
	}
}

// Convert Reward Scheme from json to struct
func MakeRewardScheme(db dbm.DB, rsDoc *tmTypes.RewardSchemeDoc) *RewardScheme {

	rs := &RewardScheme{
		db:                 db,
		TotalReward:        rsDoc.TotalReward,
		RewardFirstYear:    rsDoc.RewardFirstYear,
		EpochNumberPerYear: rsDoc.EpochNumberPerYear,
		TotalYear:          rsDoc.TotalYear,
	}

	return rs
}

func (rs *RewardScheme) copy(deepcopy bool) *RewardScheme {

	rsCopy := &RewardScheme{
		db:                 rs.db,
		TotalReward:        rs.TotalReward,
		RewardFirstYear:    rs.RewardFirstYear,
		EpochNumberPerYear: rs.EpochNumberPerYear,
		TotalYear:          rs.TotalYear,
	}

	if deepcopy {
		rsCopy.TotalReward = new(big.Int).Set(rs.TotalReward)
		rsCopy.RewardFirstYear = new(big.Int).Set(rs.RewardFirstYear)
	}

	return rsCopy
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
