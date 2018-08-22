package epoch

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/pchain/common/plogger"
	abciTypes "github.com/tendermint/abci/types"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

var plog = plogger.GetLogger("Epoch")

var NextEpochNotExist = errors.New("next epoch parameters do not exist, fatal error")
var NextEpochNotEXPECTED = errors.New("next epoch parameters are not excepted, fatal error")

const (
	EPOCH_NOT_EXIST          = iota // value --> 0
	EPOCH_PROPOSED_NOT_VOTED        // value --> 1
	EPOCH_VOTED_NOT_SAVED           // value --> 2
	EPOCH_SAVED                     // value --> 3

	NextEpochProposeStartPercent  = 0.75
	NextEpochHashVoteEndPercent   = 0.85
	NextEpochRevealVoteEndPercent = 0.95

	MinimumValidatorsSize = 10
)

type Epoch struct {
	mtx sync.Mutex
	db  dbm.DB

	Number         int
	RewardPerBlock *big.Int
	StartBlock     int
	EndBlock       int
	StartTime      time.Time
	EndTime        time.Time //not accurate for current epoch
	BlockGenerated int       //agreed in which block
	Status         int       //checked if this epoch has been saved
	Validators     *tmTypes.ValidatorSet

	//TODO Make it private
	// The VoteSet will be used just before Epoch Start
	validatorVoteSet *EpochValidatorVoteSet
	RS               *RewardScheme
	PreviousEpoch    *Epoch
	NextEpoch        *Epoch
}

func calcEpochKeyWithHeight(number int) []byte {
	return []byte(fmt.Sprintf("EPOCH:%v", number))
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

// Load Full Epoch By EpochNumber
func LoadOneEpoch(db dbm.DB, epochNumber int) *Epoch {
	epoch := loadOneEpoch(db, epochNumber)
	// Set Reward Scheme
	rewardscheme := LoadRewardScheme(db)
	epoch.RS = rewardscheme
	// Set Previous Epoch
	epoch.PreviousEpoch = loadOneEpoch(db, epochNumber-1)
	if epoch.PreviousEpoch != nil {
		epoch.PreviousEpoch.RS = rewardscheme
	}
	// Set Next Epoch
	epoch.NextEpoch = loadOneEpoch(db, epochNumber+1)
	if epoch.NextEpoch != nil {
		epoch.NextEpoch.RS = rewardscheme
		// Set ValidatorVoteSet
		epoch.NextEpoch.validatorVoteSet = LoadEpochVoteSet(db, epochNumber+1)
	}

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

// Convert from OneEpochDoc (Json) to Epoch
func MakeOneEpoch(db dbm.DB, oneEpoch *tmTypes.OneEpochDoc) *Epoch {

	number, _ := strconv.Atoi(oneEpoch.Number)
	RewardPerBlock, _ := new(big.Int).SetString(oneEpoch.RewardPerBlock, 10)
	StartBlock, _ := strconv.Atoi(oneEpoch.StartBlock)
	EndBlock, _ := strconv.Atoi(oneEpoch.EndBlock)
	StartTime, _ := time.Parse(time.RFC3339Nano, oneEpoch.StartTime)
	EndTime, _ := time.Parse(time.RFC3339Nano, oneEpoch.EndTime)
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
			Accum:       big.NewInt(0),
		}
	}

	te := &Epoch{
		db: db,

		Number:         number,
		RewardPerBlock: RewardPerBlock,
		StartBlock:     StartBlock,
		EndBlock:       EndBlock,
		StartTime:      StartTime,
		EndTime:        EndTime,
		BlockGenerated: BlockGenerated,
		Status:         Status,
		Validators:     tmTypes.NewValidatorSet(validators),
	}

	return te
}

// Convert the Epoch to OneEpochDoc, for Json Marshal/UnMarshal
func (epoch *Epoch) MakeOneEpochDoc() *tmTypes.OneEpochDoc {

	validators := make([]tmTypes.GenesisValidator, len(epoch.Validators.Validators))
	for i, val := range epoch.Validators.Validators {
		validators[i] = tmTypes.GenesisValidator{
			EthAccount: common.BytesToAddress(val.Address),
			PubKey:     val.PubKey,
			Amount:     val.VotingPower,
			Name:       "",
		}
	}

	epochDoc := &tmTypes.OneEpochDoc{
		Number:         fmt.Sprintf("%v", epoch.Number),
		RewardPerBlock: fmt.Sprintf("%v", epoch.RewardPerBlock),
		StartBlock:     fmt.Sprintf("%v", epoch.StartBlock),
		EndBlock:       fmt.Sprintf("%v", epoch.EndBlock),
		StartTime:      epoch.StartTime.Format(time.RFC3339Nano),
		EndTime:        epoch.EndTime.Format(time.RFC3339Nano),
		BlockGenerated: fmt.Sprintf("%v", epoch.BlockGenerated),
		Status:         fmt.Sprintf("%v", epoch.Status),
		Validators:     validators,
	}

	return epochDoc
}

func (epoch *Epoch) GetDB() dbm.DB {
	return epoch.db
}

func (epoch *Epoch) GetEpochValidatorVoteSet() *EpochValidatorVoteSet {
	return epoch.validatorVoteSet
}

func (epoch *Epoch) SetEpochValidatorVoteSet(voteSet *EpochValidatorVoteSet) {
	epoch.validatorVoteSet = voteSet
}

// Save the Epoch to Level DB
func (epoch *Epoch) Save() {
	epoch.mtx.Lock()
	defer epoch.mtx.Unlock()
	//fmt.Printf("(epoch *Epoch) Save(), (EPOCH, ts.Bytes()) are: (%s,%v\n", calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())
	epoch.db.SetSync(calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())

	if epoch.NextEpoch != nil && epoch.NextEpoch.Status == EPOCH_VOTED_NOT_SAVED {
		epoch.NextEpoch.Status = EPOCH_SAVED
		// Save the next epoch
		epoch.db.SetSync(calcEpochKeyWithHeight(epoch.NextEpoch.Number), epoch.NextEpoch.Bytes())
	}

	if epoch.NextEpoch != nil && epoch.NextEpoch.validatorVoteSet != nil {
		// Save the next epoch vote set
		SaveEpochVoteSet(epoch.db, epoch.NextEpoch.Number, epoch.NextEpoch.validatorVoteSet)
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
	}
}

func (epoch *Epoch) Bytes() []byte {

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	//fmt.Printf("(ts *EPOCH) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)

	epochDoc := epoch.MakeOneEpochDoc()
	wire.WriteBinary(epochDoc, buf, n, err)
	if *err != nil {
		fmt.Printf("Epoch get bytes error: %v", err)
		return nil
	}
	//fmt.Printf("(ts *EPOCH) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
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
	// If next epoch already proposed, then no need propose again
	if epoch.NextEpoch != nil {
		return false
	}

	//the epoch's end time is too rough to estimate,
	//so use generated block number in this epoch to decide if should propose next epoch parameters
	fCurBlockHeight := float64(curBlockHeight)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	shouldPropose := (NextEpochProposeStartPercent <= passRate) && (passRate < 1.0)
	return shouldPropose
}

func (epoch *Epoch) ProposeNextEpoch(curBlockHeight int) *Epoch {

	if epoch != nil {

		rewardPerBlock, blocks := epoch.estimateForNextEpoch(curBlockHeight)

		next := &Epoch{
			mtx: epoch.mtx,
			db:  epoch.db,

			RS: epoch.RS,

			Number:         epoch.Number + 1,
			RewardPerBlock: rewardPerBlock,
			StartBlock:     epoch.EndBlock + 1,
			EndBlock:       epoch.EndBlock + blocks,
			//StartTime *big.Int
			//EndTime *big.Int	//not accurate for current epoch
			BlockGenerated:   0,
			Status:           EPOCH_PROPOSED_NOT_VOTED,
			Validators:       epoch.Validators.Copy(),    // Old Validators
			validatorVoteSet: NewEpochValidatorVoteSet(), // New Validators with vote

			PreviousEpoch: epoch,
			NextEpoch:     nil,
		}

		return next
	}
	return nil
}

func (epoch *Epoch) GetVoteStartHeight() int {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochProposeStartPercent
	return int(math.Ceil(percent)) + epoch.StartBlock
}

func (epoch *Epoch) GetVoteEndHeight() int {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochHashVoteEndPercent
	if _, frac := math.Modf(percent); frac == 0 {
		return int(percent) - 1 + epoch.StartBlock
	} else {
		return int(math.Floor(percent)) + epoch.StartBlock
	}
}

func (epoch *Epoch) GetRevealVoteStartHeight() int {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochHashVoteEndPercent
	return int(math.Ceil(percent)) + epoch.StartBlock
}

func (epoch *Epoch) GetRevealVoteEndHeight() int {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochRevealVoteEndPercent
	return int(math.Floor(percent)) + epoch.StartBlock
}

func (epoch *Epoch) CheckInHashVoteStage(height int) bool {
	fCurBlockHeight := float64(height)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	return (NextEpochProposeStartPercent <= passRate) && (passRate < NextEpochHashVoteEndPercent)
}

func (epoch *Epoch) CheckInRevealVoteStage(height int) bool {
	fCurBlockHeight := float64(height)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	return (NextEpochHashVoteEndPercent <= passRate) && (passRate < NextEpochRevealVoteEndPercent)
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

func (epoch *Epoch) ShouldEnterNewEpoch(height int) (bool, error) {

	if height == epoch.EndBlock {
		if epoch.NextEpoch != nil {
			return true, nil
		} else {
			return false, NextEpochNotExist
		}
	}
	return false, nil
}

// Move to New Epoch
func (epoch *Epoch) EnterNewEpoch(height int) (*Epoch, []*abciTypes.RefundValidatorAmount, error) {

	if height == epoch.EndBlock {
		if epoch.NextEpoch != nil {

			// Set the End Time for current Epoch and Save it
			epoch.EndTime = time.Now()
			epoch.Save()
			// Old Epoch Ended

			// Now move to Next Epoch
			nextEpoch := epoch.NextEpoch
			// Store the Previous Epoch Validators only
			nextEpoch.PreviousEpoch = &Epoch{Validators: epoch.Validators}
			nextEpoch.StartTime = time.Now()

			// update the validator set with the latest abciResponses
			refund, err := updateEpochValidatorSet(nextEpoch.Validators, nextEpoch.validatorVoteSet)
			if err != nil {
				plog.Warnln("Error changing validator set", "error", err)
				return nil, nil, err
			}
			// Update validator accums and set state variables
			nextEpoch.Validators.IncrementAccum(1)

			nextEpoch.NextEpoch = nil //suppose we will not generate a more epoch after next-epoch
			plog.Infof("Enter into New Epoch %v", nextEpoch)
			return nextEpoch, refund, nil
		} else {
			return nil, nil, NextEpochNotExist
		}
	}

	return nil, nil, nil
}

// updateEpochValidatorSet Update the Current Epoch Validator by vote
//
func updateEpochValidatorSet(validators *tmTypes.ValidatorSet, voteSet *EpochValidatorVoteSet) ([]*abciTypes.RefundValidatorAmount, error) {
	if voteSet.IsEmpty() {
		// No vote, keep the current validator set
		return nil, nil
	}

	// Refund List will be vaildators contain from Vote (exit validator or less amount than previous amount) and Knockout after sort by amount
	var refund []*abciTypes.RefundValidatorAmount
	oldValSize, newValSize := validators.Size(), 0
	// Process the Votes and merge into the Validator Set
	for _, v := range voteSet.Votes {
		// If vote not reveal, bypass this vote
		if v.Amount == nil || v.Salt == "" || v.PubKey == nil {
			continue
		}

		_, validator := validators.GetByAddress(v.Address[:])
		if validator == nil {
			// Add the new validator
			added := validators.Add(tmTypes.NewValidator(v.PubKey, v.Amount))
			if !added {
				return nil, errors.New(fmt.Sprintf("Failed to add new validator %x with voting power %d", v.Address, v.Amount))
			}
			newValSize++
		} else if v.Amount.Sign() == 0 {
			refund = append(refund, &abciTypes.RefundValidatorAmount{Address: v.Address, Amount: validator.VotingPower})
			// Remove the Validator
			_, removed := validators.Remove(validator.Address)
			if !removed {
				return nil, errors.New(fmt.Sprintf("Failed to remove validator %x", validator.Address))
			}
		} else {
			//refund if new amount less than the voting power
			if v.Amount.Cmp(validator.VotingPower) == -1 {
				refundAmount := new(big.Int).Sub(validator.VotingPower, v.Amount)
				refund = append(refund, &abciTypes.RefundValidatorAmount{Address: v.Address, Amount: refundAmount})
			}

			// Update the Validator Amount
			validator.VotingPower = v.Amount
			updated := validators.Update(validator)
			if !updated {
				return nil, errors.New(fmt.Sprintf("Failed to update validator %x with voting power %d", validator.Address, v.Amount))
			}
		}
	}

	// Determine the Validator Size
	valSize := oldValSize + newValSize/2
	if valSize > MinimumValidatorsSize {
		valSize = MinimumValidatorsSize
	}

	// If actual size of Validators greater than Determine Validator Size
	// then sort the Validators with VotingPower and return the most top Validators
	if validators.Size() > valSize {
		// Sort the Validator Set with Amount
		sort.Slice(validators.Validators, func(i, j int) bool {
			return validators.Validators[i].VotingPower.Cmp(validators.Validators[j].VotingPower) == 1
		})
		// Add knockout validator to refund list
		knockout := validators.Validators[valSize:]
		for _, k := range knockout {
			refund = append(refund, &abciTypes.RefundValidatorAmount{Address: common.BytesToAddress(k.Address), Amount: k.VotingPower})
		}

		validators.Validators = validators.Validators[:valSize]
	}

	return refund, nil
}

func (epoch *Epoch) GetEpochByBlockNumber(blockNumber int) *Epoch {

	if blockNumber >= epoch.StartBlock && blockNumber <= epoch.EndBlock {
		return epoch
	}

	for number := epoch.Number - 1; number >= 0; number-- {

		ep := loadOneEpoch(epoch.db, number)
		if ep == nil {
			return nil
		}

		if blockNumber >= ep.StartBlock && blockNumber <= ep.EndBlock {
			return ep
		}
	}

	return nil
}

func (epoch *Epoch) Copy() *Epoch {
	return epoch.copy(true)
}

func (epoch *Epoch) copy(copyPrevNext bool) *Epoch {

	var previousEpoch, nextEpoch *Epoch
	if copyPrevNext {
		if epoch.PreviousEpoch != nil {
			previousEpoch = epoch.PreviousEpoch.copy(false)
		}

		if epoch.NextEpoch != nil {
			nextEpoch = epoch.NextEpoch.copy(false)
		}
	}

	return &Epoch{
		mtx: epoch.mtx,
		db:  epoch.db,

		RS: epoch.RS,

		Number:           epoch.Number,
		RewardPerBlock:   epoch.RewardPerBlock,
		StartBlock:       epoch.StartBlock,
		EndBlock:         epoch.EndBlock,
		StartTime:        epoch.StartTime,
		EndTime:          epoch.EndTime,
		BlockGenerated:   epoch.BlockGenerated,
		Status:           epoch.Status,
		Validators:       epoch.Validators.Copy(),
		validatorVoteSet: epoch.validatorVoteSet.Copy(),

		PreviousEpoch: previousEpoch,
		NextEpoch:     nextEpoch,
	}
}

func (epoch *Epoch) estimateForNextEpoch(curBlockHeight int) (rewardPerBlock *big.Int, blocksOfNextEpoch int) {

	//var totalReward          = 210000000e+18
	//var preAllocated         = 100000000e+18
	var rewardFirstYear = epoch.RS.rewardFirstYear //20000000e+18 //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PAI by 10 years
	var addedPerYear = epoch.RS.addedPerYear       //0
	var descendPerYear = epoch.RS.descendPerYear   //2000000e+18
	//var allocated            = epoch.RS.allocated //0
	var epochNumberPerYear = epoch.RS.epochNumberPerYear //12

	zeroEpoch := loadOneEpoch(epoch.db, 0)
	initStartTime := zeroEpoch.StartTime

	//from 0 year
	thisYear := (epoch.Number / epochNumberPerYear)
	nextYear := thisYear + 1

	timePerBlockThisEpoch := time.Now().Sub(epoch.StartTime).Nanoseconds() / int64(curBlockHeight-epoch.StartBlock)

	epochLeftThisYear := epochNumberPerYear - epoch.Number%epochNumberPerYear - 1

	blocksOfNextEpoch = 0

	if epochLeftThisYear == 0 { //to another year

		nextYearStartTime := initStartTime.AddDate(nextYear, 0, 0)

		timeLeftNextYear := nextYearStartTime.AddDate(1, 0, 0).Sub(nextYearStartTime)

		epochLeftNextYear := epochNumberPerYear

		epochTimePerEpochLeftNextYear := timeLeftNextYear.Nanoseconds() / int64(epochLeftNextYear)

		blocksOfNextEpoch = int(epochTimePerEpochLeftNextYear / timePerBlockThisEpoch)
		if blocksOfNextEpoch == 0 {
			plog.Panicln("EstimateForNextEpoch Failed: Please check the epoch_no_per_year setup in Genesis")
		}

		rewardPerEpochNextYear := calculateRewardPerEpochByYear(rewardFirstYear, addedPerYear, descendPerYear, int64(nextYear), int64(epochNumberPerYear))

		rewardPerBlock = new(big.Int).Div(rewardPerEpochNextYear, big.NewInt(int64(blocksOfNextEpoch)))

	} else {

		nextYearStartTime := initStartTime.AddDate(nextYear, 0, 0)

		timeLeftThisYear := nextYearStartTime.Sub(time.Now())

		epochTimePerEpochLeftThisYear := timeLeftThisYear.Nanoseconds() / int64(epochLeftThisYear)

		blocksOfNextEpoch = int(epochTimePerEpochLeftThisYear / timePerBlockThisEpoch)
		if blocksOfNextEpoch == 0 {
			plog.Panicln("EstimateForNextEpoch Failed: Please check the epoch_no_per_year setup in Genesis")
		}

		rewardPerEpochThisYear := calculateRewardPerEpochByYear(rewardFirstYear, addedPerYear, descendPerYear, int64(thisYear), int64(epochNumberPerYear))

		rewardPerBlock = new(big.Int).Div(rewardPerEpochThisYear, big.NewInt(int64(blocksOfNextEpoch)))

	}
	return rewardPerBlock, blocksOfNextEpoch
}

/*
	Abstract function to calculate the reward of each Epoch by year
	rewardPerEpochNextYear := (rewardFirstYear + (addedPerYear - descendPerYear) * year) / epochNumberPerYear
*/
func calculateRewardPerEpochByYear(rewardFirstYear, addedPerYear, descendPerYear *big.Int, year, epochNumberPerYear int64) *big.Int {
	result := new(big.Int).Sub(addedPerYear, descendPerYear)
	return result.Mul(result, big.NewInt(year)).Add(result, rewardFirstYear).Div(result, big.NewInt(epochNumberPerYear))
}

func (epoch *Epoch) Equals(other *Epoch, checkPrevNext bool) bool {

	if (epoch == nil && other != nil) || (epoch != nil && other == nil) {
		return false
	}

	if epoch == nil && other == nil {
		return true
	}

	if !(epoch.Number == other.Number && epoch.RewardPerBlock.Cmp(other.RewardPerBlock) == 0 &&
		epoch.StartBlock == other.StartBlock && epoch.EndBlock == other.EndBlock &&
		epoch.Validators.Equals(other.Validators)) {
		return false
	}

	if checkPrevNext {
		if !epoch.PreviousEpoch.Equals(other.PreviousEpoch, false) ||
			!epoch.NextEpoch.Equals(other.NextEpoch, false) {
			return false
		}
	}

	return true
}

func (epoch *Epoch) ProposeTransactions(sender string, blockHeight int) (tmTypes.Txs, error) {

	txs := make([]tmTypes.Tx, 0)

	if blockHeight == epoch.EndBlock && epoch.Number > 1 {
		voSet := LoadValidatorOperationSet(epoch.Number - 2)
		if voSet != nil {
			for validator, voArr := range voSet.Operations {
				for i := 0; i < len(voArr); i++ {
					vo := voArr[i]
					if vo.Action == SVM_WITHDRAW {
						tx, err := NewUnlockAssetTransaction(sender, validator, vo.Amount)
						if err != nil {
							return tmTypes.Txs{}, err
						}
						txs = append(txs, tx)
					}
				}
			}
		}
	}

	return txs, nil
}

func (epoch *Epoch) String() string {
	return fmt.Sprintf("Epoch : {"+
		"Number : %v,\n"+
		"RewardPerBlock : %v,\n"+
		"StartBlock : %v,\n"+
		"EndBlock : %v,\n"+
		"StartTime : %v,\n"+
		"EndTime : %v,\n"+
		"BlockGenerated : %v,\n"+
		"Status : %v,\n"+
		"Next Epoch : %v,\n"+
		"Prev Epoch : %v,\n"+
		"Contains RS : %v, \n"+
		"}",
		epoch.Number,
		epoch.RewardPerBlock,
		epoch.StartBlock,
		epoch.EndBlock,
		epoch.StartTime,
		epoch.EndTime,
		epoch.BlockGenerated,
		epoch.Status,
		epoch.NextEpoch,
		epoch.PreviousEpoch,
		epoch.RS != nil,
	)
}
