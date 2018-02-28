package epoch

import (
	"fmt"
	tmTypes "github.com/tendermint/tendermint/types"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	wire "github.com/tendermint/go-wire"
	"sync"
	"bytes"
	"os"
	"io/ioutil"
	"errors"
	"time"
	"github.com/tendermint/go-logger"
	abciTypes "github.com/tendermint/abci/types"
	"strconv"
	"github.com/ethereum/go-ethereum/common"
)

var log = logger.New("module", "epoch")

var NextEpochNotExist = errors.New("next epoch parameters do not exist, fatal error")
var NextEpochNotEXPECTED = errors.New("next epoch parameters are not excepted, fatal error")

const (
	EPOCH_NOT_EXIST = iota	     		// value --> 0
	EPOCH_PROPOSED_NOT_VOTED = iota	// value --> 1
	EPOCH_VOTED_NOT_SAVED     		// value --> 2
	EPOCH_SAVED                		// value --> 3
)

type Epoch struct {
	mtx sync.Mutex
	db dbm.DB

	RS *RewardScheme

	Number int
	RewardPerBlock int
	StartBlock int
	EndBlock int
	StartTime time.Time
	EndTime time.Time	//not accurate for current epoch
	BlockGenerated int      //agreed in which block
	Status int		//checked if this epoch has been saved
	Validators *tmTypes.ValidatorSet

	PreviousEpoch *Epoch
	NextEpoch *Epoch
}

/*


	ShouldEnterNewEpoch(height int) (bool, error)
	EnterNewEpoch(height int) error //enter new epoch and apply its parameters

	SaveCurrentEpochNumber() //save to db
	Save() //save to db
	Copy() Epoch //copy another instance
	String() string //format to string

	Validate(other Epoch, checkNextEpoch bool) error //check if equal to anoter rewardscheme
 */

var epochKey = []byte("EPOCH")

func calcEpochKeyWithHeight(number int) []byte {
	return []byte(string(epochKey) + fmt.Sprintf(":%v", number))
}



//roughly one epoch one month
//var rewardPerEpoch = rewardThisYear / 12

//var epoches = []Epoch{}


// Load the most recent state from "state" db,
// or create a new one (and save) from genesis.
func GetEpoch(config cfg.Config, epochDB dbm.DB, number int) *Epoch {

	epoch := LoadOneEpoch(epochDB, number)
	if epoch == nil {
		epoch = MakeEpochFromFile(epochDB, config.GetString("epoch_file"))
		if epoch != nil {
			epoch.Save()
			fmt.Printf("GetEpoch() 0, epoch is: %v\n", epoch)
		} else {
			fmt.Printf("GetEpoch() 1, epoch read from file failed\n")
			os.Exit(1)
		}
	}

	if epoch.Number < 0 {
		fmt.Printf("GetEpoch() 2, epoch checked failed\n")
		os.Exit(1)
	}

	rewardScheme := GetRewardScheme(config, epochDB)
	if rewardScheme == nil {
		fmt.Printf("GetEpoch() 3, reward scheme failed\n")
		os.Exit(1)
	}
	epoch.RS = rewardScheme

	fmt.Printf("GetEpoch() 4, epoch is: %v\n", epoch)

	return epoch
}

func LoadOneEpoch(db dbm.DB, epochNumber int) *Epoch {
	epoch := loadOneEpoch(db, epochNumber)

	epoch.PreviousEpoch = loadOneEpoch(db, epochNumber-1)
	epoch.NextEpoch = loadOneEpoch(db, epochNumber+1)

	return epoch
}

func loadOneEpoch(db dbm.DB, epochNumber int) *Epoch {

	oneEpoch := &tmTypes.OneEpochDoc{}
	buf := db.Get(calcEpochKeyWithHeight(epochNumber))
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&oneEpoch, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadState: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
		// TODO: ensure that buf is completely read.
		ts := MakeOneEpoch(db, oneEpoch)
		fmt.Printf("loadEpoch(), reward scheme is: %v\n", ts)
		return ts
	}
}

// Used during replay and in tests.
func MakeEpochFromFile(db dbm.DB, genesisFile string) *Epoch {
	epochJSON, err := ioutil.ReadFile(genesisFile)
	if err != nil {
		fmt.Printf("Couldn't read GenesisDoc file: %v\n", err)
		os.Exit(1)
	}
	genDoc, err := tmTypes.GenesisDocFromJSON(epochJSON)
	if err != nil {
		fmt.Printf("Error reading GenesisDoc: %v\n", err)
		os.Exit(1)
	}
	return MakeOneEpoch(db, &genDoc.CurrentEpoch)
}


func MakeOneEpoch(db dbm.DB, oneEpoch *tmTypes.OneEpochDoc) *Epoch {

	number, _ := strconv.Atoi(oneEpoch.Number)
	RewardPerBlock, _ := strconv.Atoi(oneEpoch.RewardPerBlock)
	StartBlock, _ := strconv.Atoi(oneEpoch.StartBlock)
	EndBlock, _ := strconv.Atoi(oneEpoch.EndBlock)
	StartTime, _ := time.Parse(tmTypes.TimeLayout, oneEpoch.StartTime)
	EndTime, _ := time.Parse(tmTypes.TimeLayout, oneEpoch.EndTime)
	BlockGenerated, _ := strconv.Atoi(oneEpoch.BlockGenerated)
	Status, _ := strconv.Atoi(oneEpoch.Status)

	validators := make([]*tmTypes.Validator, len(oneEpoch.Validators))
	for i, val := range oneEpoch.Validators {
		pubKey := val.PubKey
		address := pubKey.Address()
		//TODO: very important, here the address should be the ethereum account,
		//TODO: at least, should add one additional ethereum account
		//address := val.EthAccount.Bytes()

		// Make validator
		validators[i] = &tmTypes.Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Amount,
		}
	}

	te := &Epoch{
		db : db,

		Number: number,
		RewardPerBlock : RewardPerBlock,
		StartBlock : StartBlock,
		EndBlock : EndBlock,
		StartTime : StartTime,
		EndTime : EndTime,
		BlockGenerated : BlockGenerated,
		Status : Status,
		Validators : tmTypes.NewValidatorSet(validators),
	}

	return te
}

func (epoch *Epoch) MakeOneEpochDoc() *tmTypes.OneEpochDoc {

	validators := make([]tmTypes.GenesisValidator, len(epoch.Validators.Validators))
	for i, val := range epoch.Validators.Validators {
		validators[i] = tmTypes.GenesisValidator {
			EthAccount: common.BytesToAddress(val.Address),
			PubKey: val.PubKey,
			Amount: val.VotingPower,
			Name: "",
		}
	}

	epochDoc := &tmTypes.OneEpochDoc{
		Number : fmt.Sprintf("%v", epoch.Number),
		RewardPerBlock : fmt.Sprintf("%v", epoch.RewardPerBlock),
		StartBlock : fmt.Sprintf("%v", epoch.StartBlock),
		EndBlock : fmt.Sprintf("%v", epoch.EndBlock),
		StartTime : epoch.StartTime.Format(tmTypes.TimeLayout),
		EndTime : epoch.EndTime.Format(tmTypes.TimeLayout),
		BlockGenerated : fmt.Sprintf("%v", epoch.BlockGenerated),
		Status : fmt.Sprintf("%v", epoch.Status),
		Validators: validators,
	}

	return epochDoc
}

func (epoch *Epoch) Save() {

	epoch.mtx.Lock()
	defer epoch.mtx.Unlock()
	fmt.Printf("(ts *TxScheme) Save(), (rewardSchemeKey, ts.Bytes()) are: (%v,%v\n", calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())
	epoch.db.SetSync(calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())

	if epoch.NextEpoch != nil && epoch.NextEpoch.Status == EPOCH_VOTED_NOT_SAVED {
		epoch.db.SetSync(calcEpochKeyWithHeight(epoch.NextEpoch.Number), epoch.NextEpoch.Bytes())
		epoch.NextEpoch.Status = EPOCH_SAVED
	}
}

func FromBytes(buf []byte) *Epoch {

	oneEpoch := &tmTypes.OneEpochDoc{}
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&oneEpoch, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadState: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
		// TODO: ensure that buf is completely read.
		ts := MakeOneEpoch(nil, oneEpoch)
		fmt.Printf("loadEpoch(), reward scheme is: %v\n", ts)
		return ts
	}}

func (epoch *Epoch) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	fmt.Printf("(ts *TxScheme) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)

	epochDoc := epoch.MakeOneEpochDoc()
	wire.WriteBinary(epochDoc, buf, n, err)
	if *err != nil {
		fmt.Printf("Epoch get bytes error: %v", err)
	}
	fmt.Printf("(ts *TxScheme) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	return buf.Bytes()
}

func (epoch *Epoch) ValidateNextEpoch(next *Epoch, height int) error {

	if epoch.NextEpoch == nil {
		epoch.NextEpoch = epoch.ProposeNextEpoch(height)
	}

	if !epoch.NextEpoch.Equals(next, false) {
		return NextEpochNotEXPECTED
	}

	return nil
}

//check if need propose next epoch
func (epoch *Epoch) ShouldProposeNextEpoch(curBlockHeight int) bool {

	//the epoch's end time is too rough to estimate,
	//so use generated block number in this epoch to decide if should propose next epoch parameters
	fCurBlockHeight := float64(curBlockHeight)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	shouldPropose := (0.75 <= passRate) && (passRate < 1.0)

	return shouldPropose
}

func (epoch *Epoch) ProposeNextEpoch(curBlockHeight int) *Epoch {

	if epoch != nil {

		rewardPerBlock, blocks := epoch.estimateForNextEpoch(curBlockHeight)

		next := &Epoch{
			mtx: epoch.mtx,
			db: epoch.db,

			RS: epoch.RS,

			Number: epoch.Number + 1,
			RewardPerBlock: rewardPerBlock,
			StartBlock: epoch.EndBlock + 1,
			EndBlock: epoch.EndBlock + blocks,
			//StartTime *big.Int
			//EndTime *big.Int	//not accurate for current epoch
			BlockGenerated: 0,
			Status: EPOCH_PROPOSED_NOT_VOTED,
			Validators: epoch.Validators.Copy(),

			PreviousEpoch: epoch,
			NextEpoch: nil,
		}

		return next
	}
	return nil
}

func (epoch *Epoch) GetNextEpoch() *Epoch {
	return epoch.NextEpoch
}

func (epoch *Epoch) SetNextEpoch(next *Epoch) {
	epoch.NextEpoch = next
	if next != nil {
		epoch.NextEpoch.db = epoch.db
	}
}

func (epoch *Epoch) GetNextEpochStatus() int {

	if epoch.NextEpoch == nil {
		return EPOCH_NOT_EXIST
	}

	return epoch.Status
}

func (epoch *Epoch) ShouldEnterNewEpoch(height int) (bool, error) {

	if height == epoch.EndBlock + 1 {
		if epoch.NextEpoch != nil {
			return true, nil
		} else {
			return false, NextEpochNotExist
		}
	}
	return false, nil
}

func (epoch *Epoch) EnterNewEpoch(height int, diffs []*abciTypes.Validator) error {

	if height == epoch.EndBlock + 1 {
		if epoch.NextEpoch != nil {

			epoch.NextEpoch.PreviousEpoch = epoch
			epoch = epoch.NextEpoch

			// update the validator set with the latest abciResponses
			err := tmTypes.UpdateValidators(epoch.Validators, diffs)
			if err != nil {
				log.Warn("Error changing validator set", "error", err)
				// TODO: err or carry on?
			}
			// Update validator accums and set state variables
			epoch.Validators.IncrementAccum(1)

			epoch.NextEpoch = nil //suppose we will not generate a more epoch after next-epoch
		} else {
			return NextEpochNotExist
		}
	}

	return nil
}

func (epoch *Epoch) Copy() *Epoch {

	var previousEpoch *Epoch = nil
	if epoch.PreviousEpoch != nil {
		previousEpoch = epoch.PreviousEpoch.Copy()
	}

	var nextEpoch *Epoch = nil
	if epoch.NextEpoch != nil {
		nextEpoch = epoch.NextEpoch.Copy()
	}

	return &Epoch {
		mtx: epoch.mtx,
		db: epoch.db,

		RS: epoch.RS,

		Number: epoch.Number,
		RewardPerBlock: epoch.RewardPerBlock,
		StartBlock: epoch.StartBlock,
		EndBlock: epoch.EndBlock,
		StartTime: epoch.StartTime,
		EndTime: epoch.EndTime,
		BlockGenerated: epoch.BlockGenerated,
		Status: epoch.Status,
		Validators: epoch.Validators.Copy(),

		PreviousEpoch: previousEpoch,
		NextEpoch: nextEpoch,
	}
}

func (epoch *Epoch) estimateForNextEpoch(curBlockHeight int) (rewardPerBlock, blocksOfNextEpoch int) {

	//var totalReward          = 210000000e+18
	//var preAllocated         = 100000000e+18
	var rewardFirstYear      = epoch.RS.rewardFirstYear //20000000e+18 //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PAI by 10 years
	var addedPerYear         = epoch.RS.addedPerYear	//0
	var descendPerYear = epoch.RS.descendPerYear //2000000e+18
	//var allocated            = epoch.RS.allocated //0
	var epochNumberPerYear	 = epoch.RS.epochNumberPerYear //12

	zeroEpoch := loadOneEpoch(epoch.db, 1)
	initStartTime := zeroEpoch.StartTime

	//from 0 year
	thisYear := (epoch.Number / epochNumberPerYear)
	nextYear := thisYear + 1

	timePerBlockThisEpoch := time.Now().Sub(epoch.StartTime).Nanoseconds() / int64(curBlockHeight - epoch.StartBlock)

	epochLeftThisYear := epochNumberPerYear - epoch.Number % epochNumberPerYear - 1

	rewardPerBlock = 0
	blocksOfNextEpoch = 0

	if epochLeftThisYear == 0 { //to another year

		rewardPerEpochNextYear := (rewardFirstYear + (addedPerYear - descendPerYear) * nextYear) / epochNumberPerYear

		nextYearStartTime := initStartTime.AddDate(nextYear, 0, 0)

		timeLeftNextYear := nextYearStartTime.AddDate(1, 0, 0).Sub(nextYearStartTime)

		epochLeftNextYear := epochNumberPerYear

		epochTimePerEpochLeftNextYear := timeLeftNextYear.Nanoseconds() / int64(epochLeftNextYear)

		blocksOfNextEpoch = int(epochTimePerEpochLeftNextYear / timePerBlockThisEpoch)

		rewardPerBlock = rewardPerEpochNextYear / blocksOfNextEpoch

	} else {

		nextYearStartTime := initStartTime.AddDate(nextYear, 0, 0)

		timeLeftThisYear := nextYearStartTime.Sub(time.Now())

		epochTimePerEpochLeftThisYear := timeLeftThisYear.Nanoseconds() / int64(epochLeftThisYear)

		blocksOfNextEpoch = int(epochTimePerEpochLeftThisYear / timePerBlockThisEpoch)

		rewardPerEpochThisYear := (rewardFirstYear + (addedPerYear - descendPerYear) * thisYear) / epochNumberPerYear

		rewardPerBlock = rewardPerEpochThisYear / blocksOfNextEpoch

	}
	return rewardPerBlock, blocksOfNextEpoch
}

func (epoch *Epoch) Equals(other *Epoch, checkPrevNext bool) bool{

	if (epoch == nil && other != nil) || (epoch != nil && other == nil) {
		return false
	}

	if epoch == nil && other == nil {
		return true
	}

	if !(epoch.Number == other.Number && epoch.RewardPerBlock == other.RewardPerBlock &&
		epoch.StartBlock == other.StartBlock && epoch.EndBlock == other.EndBlock &&
		epoch.Validators.Equals(other.Validators)) {
		return false
	}

	if checkPrevNext {
		if !epoch.PreviousEpoch.Equals(other.PreviousEpoch, false) ||
			!epoch.NextEpoch.Equals(other.NextEpoch, false){
			return false
		}
	}

	return true
}

func (epoch *Epoch) String() string {
	return fmt.Sprintf("Epoch : {" +
		"Height : %v,\n" +
		"RewardPerBlock : %v,\n" +
		"StartBlock : %v,\n" +
		"EndBlock : %v,\n" +
		"StartTime : %v,\n" +
		"EndTime : %v,\n" +
		"BlockGenerated : %v\n" +
		"Status : %v\n" +
		"}",
		epoch.Number,
		epoch.RewardPerBlock,
		epoch.StartBlock,
		epoch.EndBlock,
		epoch.StartTime,
		epoch.EndTime,
		epoch.BlockGenerated,
		epoch.Status)
}
