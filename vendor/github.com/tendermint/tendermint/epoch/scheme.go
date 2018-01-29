package epoch

import (
	//"time"
	//"errors"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	wire "github.com/tendermint/go-wire"
	"fmt"
	"bytes"
	"os"
	"io/ioutil"
	"sync"
	"strconv"
)

var totalReward          = 210000000e+18
var preAllocated         = 100000000e+18
var rewardFirstYear      =  20000000e+18 //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PAI by 10 years
var addedPerYear         = 0
var descendPerYear 	 =   2000000e+18
var allocated            = 0
var epochNumberPerYear	 = 12

type RewardScheme struct {
	mtx sync.Mutex
	db dbm.DB

	totalReward int
	preAllocated int
	rewardFirstYear int
	addedPerYear int
	descendPerYear int
	allocated int
	epochNumberPerYear int
}

var rewardSchemeKey = []byte("REWARDSCHEME")


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
			fmt.Printf("GetRewardScheme() 0, reward scheme is: %v\n", rs)
		} else {
			fmt.Printf("GetRewardScheme() 1, epoch read from file failed\n")
			os.Exit(1)
		}
	}

	fmt.Printf("GetRewardScheme() 2, reward scheme is: %v\n", rs)

	if rs.totalReward <= 0 {
		fmt.Printf("GetRewardScheme() 3, reward scheme checked failed\n")
		os.Exit(1)
	}

	return rs
}

func LoadRewardScheme(db dbm.DB) *RewardScheme {
	return loadRewardScheme(db, rewardSchemeKey)
}

func loadRewardScheme(db dbm.DB, key []byte) *RewardScheme {
	rsDoc := &RewardSchemeDoc{}
	buf := db.Get(key)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&rsDoc, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadState: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
		// TODO: ensure that buf is completely read.
		rs := MakeRewardScheme(db, rsDoc)
		fmt.Printf("loadEpoch(), reward scheme is: %v\n", rs)
		return rs
	}
}

// Used during replay and in tests.
func MakeRewardSchemeFromFile(db dbm.DB, epochFile string) *RewardScheme {
	epochJSON, err := ioutil.ReadFile(epochFile)
	if err != nil {
		fmt.Printf("Couldn't read GenesisDoc file: %v\n", err)
		os.Exit(1)
	}
	epochDoc, err := epochFromJSON(epochJSON)
	if err != nil {
		fmt.Printf("Error reading GenesisDoc: %v\n", err)
		os.Exit(1)
	}
	return MakeRewardScheme(db, &epochDoc.RewardScheme)
}


func MakeRewardScheme(db dbm.DB, rsDoc *RewardSchemeDoc) *RewardScheme {

	totalReward,_ := strconv.Atoi(rsDoc.TotalReward)
	preAllocated,_ := strconv.Atoi(rsDoc.PreAllocated)
	addedPerYear,_ := strconv.Atoi(rsDoc.AddedPerYear)
	rewardFirstYear,_ := strconv.Atoi(rsDoc.RewardFirstYear)
	descendPerYear,_ := strconv.Atoi(rsDoc.DescendPerYear)
	allocated,_ := strconv.Atoi(rsDoc.Allocated)

	rs := &RewardScheme{
		db : db,
		totalReward : totalReward,
		preAllocated : preAllocated,
		addedPerYear : addedPerYear,
		rewardFirstYear : rewardFirstYear,
		descendPerYear : descendPerYear,
		allocated : allocated,
	}

	return rs
}

func (rs *RewardScheme) MakeRewardSchemeDoc() *RewardSchemeDoc {

	rsDoc := &RewardSchemeDoc{
		TotalReward : fmt.Sprintf("%v", rs.totalReward),
		PreAllocated : fmt.Sprintf("%v", rs.preAllocated),
		AddedPerYear : fmt.Sprintf("%v", rs.addedPerYear),
		RewardFirstYear	: fmt.Sprintf("%v", rs.rewardFirstYear),
		DescendPerYear : fmt.Sprintf("%v", rs.descendPerYear),
		Allocated : fmt.Sprintf("%v", rs.allocated),
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
	fmt.Printf("(rs *RewardScheme) Save(), (rewardSchemeKey, ts.Bytes()) are: (%v,%v\n", rewardSchemeKey, rs.Bytes())
	rs.db.SetSync(rewardSchemeKey, rs.Bytes())
}

func (rs *RewardScheme) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	fmt.Printf("(rs *RewardScheme) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)

	rsDoc := rs.MakeRewardSchemeDoc()
	wire.WriteBinary(rsDoc, buf, n, err)
	if *err != nil {
		fmt.Printf("Epoch get bytes error: %v", err)
	}
	fmt.Printf("(rs *TxScheme) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	return buf.Bytes()
}


func (rs *RewardScheme) String() string {

	return fmt.Sprintf("TxScheme : {" +
		"db : _,\n" +
		"totalReward : %v,\n" +
		"preAllocated : %v,\n" +
		"addedPerYear : %v,\n" +
		"rewardFirstYear : %v,\n" +
		"descendAmountPerYear : %v,\n" +
		"allocated : %v,\n" +
		"}",
		rs.totalReward,
		rs.preAllocated,
		rs.addedPerYear,
		rs.rewardFirstYear,
		rs.descendPerYear,
		rs.allocated)
}
/*
func (rs *RewardScheme)Current() (uint64, error) {

	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)CurrentRewardPerBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)CurrentStartBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)CurrentEndBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)CurrentStartTime() (time.Time, error) {
	if !epoch.agreedBy23 {
		return time.Now(), errors.New("not synchronized by all validators")
	}
	return time.Now()
}

func (rs *RewardScheme)CurrentEstimatedEndTime() (time.Time, error) {
	if !epoch.agreedBy23 {
		return time.Now(), errors.New("not synchronized by all validators")
	}
	return time.Now()
}

func (rs *RewardScheme)Next() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)NextStartBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)NextEstimatedEndBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}

func (rs *RewardScheme)NextRewardPerBlock() (uint64, error) {
	if !epoch.agreedBy23 {
		return 0, errors.New("not synchronized by all validators")
	}
	return 0
}
*/
