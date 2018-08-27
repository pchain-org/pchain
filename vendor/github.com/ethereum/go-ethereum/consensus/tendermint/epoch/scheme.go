package epoch

import (
	//"time"
	//"errors"
	"bytes"
	"fmt"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	wire "github.com/tendermint/go-wire"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"sync"
)

//var totalReward          = 210000000e+18
//var preAllocated         = 178500000e+18
//var rewardFirstYear      =  5727300e+18 //release all left 31500000 PCH by 10 years
//var descendPerYear 	 =   572730e+18
//var addedPerYear         = 0
//var allocated            = 0
//var epochNumberPerYear	 = 525600

type RewardScheme struct {
	mtx sync.Mutex
	db  dbm.DB

	totalReward        *big.Int
	preAllocated       *big.Int
	rewardFirstYear    *big.Int
	addedPerYear       *big.Int
	descendPerYear     *big.Int
	allocated          *big.Int
	epochNumberPerYear int
}

const rewardSchemeKey = "REWARDSCHEME"

//roughly one epoch one month
//var rewardPerEpoch = rewardThisYear / 12

//var epoches = []Epoch{}

// Load the most recent state from "state" db,
// or create a new one (and save) from genesis.
func GetRewardScheme(config cfg.Config, rsDB dbm.DB) *RewardScheme {
	rs := LoadRewardScheme(rsDB)
	if rs == nil {
		rs = MakeRewardSchemeFromFile(rsDB, config.GetString("epoch_file"))
		if rs != nil {
			rs.Save()
		} else {
			logger.Errorf("GetRewardScheme. reward scheme read from file failed")
			os.Exit(1)
		}
	}

	logger.Infof("GetRewardScheme. reward scheme is: %v", rs)

	if rs.totalReward.Sign() != 1 { // total reward <= 0
		logger.Errorf("GetRewardScheme. reward scheme checked failed")
		os.Exit(1)
	}

	return rs
}

func LoadRewardScheme(db dbm.DB) *RewardScheme {
	return loadRewardScheme(db, []byte(rewardSchemeKey))
}

func loadRewardScheme(db dbm.DB, key []byte) *RewardScheme {
	rsDoc := &tmTypes.RewardSchemeDoc{}
	buf := db.Get(key)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&rsDoc, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			logger.Errorf("loadRewardScheme: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
		// TODO: ensure that buf is completely read.
		rs := MakeRewardScheme(db, rsDoc)
		logger.Infof("loadRewardScheme. reward scheme is: %v", rs)
		return rs
	}
}

// Used during replay and in tests.
func MakeRewardSchemeFromFile(db dbm.DB, genFile string) *RewardScheme {
	genJSON, err := ioutil.ReadFile(genFile)
	if err != nil {
		logger.Errorf("Couldn't read GenesisDoc file: %v", err)
		os.Exit(1)
	}
	genDoc, err := tmTypes.GenesisDocFromJSON(genJSON)
	if err != nil {
		logger.Errorf("Error reading GenesisDoc: %v", err)
		os.Exit(1)
	}
	return MakeRewardScheme(db, &genDoc.RewardScheme)
}

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
		totalReward:        totalReward,
		preAllocated:       preAllocated,
		addedPerYear:       addedPerYear,
		rewardFirstYear:    rewardFirstYear,
		descendPerYear:     descendPerYear,
		allocated:          allocated,
		epochNumberPerYear: epochNumberPerYear,
	}

	return rs
}

func (rs *RewardScheme) MakeRewardSchemeDoc() *tmTypes.RewardSchemeDoc {

	rsDoc := &tmTypes.RewardSchemeDoc{
		TotalReward:        fmt.Sprintf("%v", rs.totalReward),
		PreAllocated:       fmt.Sprintf("%v", rs.preAllocated),
		AddedPerYear:       fmt.Sprintf("%v", rs.addedPerYear),
		RewardFirstYear:    fmt.Sprintf("%v", rs.rewardFirstYear),
		DescendPerYear:     fmt.Sprintf("%v", rs.descendPerYear),
		Allocated:          fmt.Sprintf("%v", rs.allocated),
		EpochNumberPerYear: fmt.Sprintf("%v", rs.epochNumberPerYear),
	}

	return rsDoc
}

/*
func (rs *TxScheme) saveTotalReward(height int) []byte {
	rs.db.SetSync([]byte(rewardSchemeKey + ":TotalReward"), []byte(fmt.Sprintf("%v", rs.totalReward))
}
*/
func (rs *RewardScheme) Save() {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()
	logger.Infof("(rs *RewardScheme) Save(), (rewardSchemeKey, ts.Bytes()) are: (%s,%s\n", rewardSchemeKey, rs.Bytes())
	rs.db.SetSync([]byte(rewardSchemeKey), rs.Bytes())
}

func (rs *RewardScheme) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)

	rsDoc := rs.MakeRewardSchemeDoc()
	wire.WriteBinary(rsDoc, buf, n, err)
	if *err != nil {
		logger.Warnf("Epoch get bytes error: %v", err)
	}
	logger.Infof("(rs *RewardScheme) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	return buf.Bytes()
}

func (rs *RewardScheme) String() string {

	return fmt.Sprintf("RewardScheme : {"+
		"db : _,\n"+
		"totalReward : %v,\n"+
		"preAllocated : %v,\n"+
		"addedPerYear : %v,\n"+
		"rewardFirstYear : %v,\n"+
		"descendAmountPerYear : %v,\n"+
		"allocated : %v,\n"+
		"epochNumberPerYear : %v,\n"+
		"}",
		rs.totalReward,
		rs.preAllocated,
		rs.addedPerYear,
		rs.rewardFirstYear,
		rs.descendPerYear,
		rs.allocated,
		rs.epochNumberPerYear)
}
