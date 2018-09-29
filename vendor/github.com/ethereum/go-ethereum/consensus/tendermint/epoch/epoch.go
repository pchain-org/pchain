package epoch

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/log"
	abciTypes "github.com/tendermint/abci/types"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"math"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"time"
)

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
	StartBlock     uint64
	EndBlock       uint64
	StartTime      time.Time
	EndTime        time.Time //not accurate for current epoch
	BlockGenerated int       //agreed in which block
	Status         int       //checked if this epoch has been saved
	Validators     *tmTypes.ValidatorSet

	// The VoteSet will be used just before Epoch Start
	validatorVoteSet *EpochValidatorVoteSet
	rs               *RewardScheme
	previousEpoch    *Epoch
	nextEpoch        *Epoch

	logger log.Logger
}

func calcEpochKeyWithHeight(number int) []byte {
	return []byte(fmt.Sprintf("EPOCH:%v", number))
}

// Load Full Epoch By EpochNumber
func LoadOneEpoch(db dbm.DB, epochNumber int, logger log.Logger) *Epoch {
	epoch := loadOneEpoch(db, epochNumber, logger)
	// Set Reward Scheme
	rewardscheme := LoadRewardScheme(db)
	epoch.rs = rewardscheme
	// Set Previous Epoch
	epoch.previousEpoch = loadOneEpoch(db, epochNumber-1, logger)
	if epoch.previousEpoch != nil {
		epoch.previousEpoch.rs = rewardscheme
	}
	// Set Next Epoch
	epoch.nextEpoch = loadOneEpoch(db, epochNumber+1, logger)
	if epoch.nextEpoch != nil {
		epoch.nextEpoch.rs = rewardscheme
		// Set ValidatorVoteSet
		epoch.nextEpoch.validatorVoteSet = LoadEpochVoteSet(db, epochNumber+1)
	}

	return epoch
}

func loadOneEpoch(db dbm.DB, epochNumber int, logger log.Logger) *Epoch {

	if epochNumber < 0 {
		return nil
	}

	buf := db.Get(calcEpochKeyWithHeight(epochNumber))
	ep := FromBytes(buf)
	if ep != nil {
		ep.db = db
		ep.logger = logger
	}
	return ep
	/*
		if len(buf) == 0 {
			return nil
		} else {

			r, n, err := bytes.NewReader(buf), new(int), new(error)
			wire.ReadBinaryPtr(&oneEpoch, r, 0, n, err)
			if *err != nil {
				// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
				logger.Errorf("LoadState: Data has been corrupted or its spec has changed: %v", *err)
				os.Exit(1)
			}
			// TODO: ensure that buf is completely read.
			ts := MakeOneEpoch(db, oneEpoch, logger)
			logger.Infof("loadOneEpoch. epoch is: %v", ts)
			return ts


		}
	*/
}

// Convert from OneEpochDoc (Json) to Epoch
func MakeOneEpoch(db dbm.DB, oneEpoch *tmTypes.OneEpochDoc, logger log.Logger) *Epoch {

	number, _ := strconv.Atoi(oneEpoch.Number)
	RewardPerBlock, _ := new(big.Int).SetString(oneEpoch.RewardPerBlock, 10)
	StartBlock, _ := strconv.ParseUint(oneEpoch.StartBlock, 0, 64)
	EndBlock, _ := strconv.ParseUint(oneEpoch.EndBlock, 0, 64)
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

		logger: logger,
	}

	return te
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

func (epoch *Epoch) GetRewardScheme() *RewardScheme {
	return epoch.rs
}

func (epoch *Epoch) SetRewardScheme(rs *RewardScheme) {
	epoch.rs = rs
}

// Save the Epoch to Level DB
func (epoch *Epoch) Save() {
	epoch.mtx.Lock()
	defer epoch.mtx.Unlock()
	//fmt.Printf("(epoch *Epoch) Save(), (EPOCH, ts.Bytes()) are: (%s,%v\n", calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())
	epoch.db.SetSync(calcEpochKeyWithHeight(epoch.Number), epoch.Bytes())

	if epoch.nextEpoch != nil && epoch.nextEpoch.Status == EPOCH_VOTED_NOT_SAVED {
		epoch.nextEpoch.Status = EPOCH_SAVED
		// Save the next epoch
		epoch.db.SetSync(calcEpochKeyWithHeight(epoch.nextEpoch.Number), epoch.nextEpoch.Bytes())
	}

	if epoch.nextEpoch != nil && epoch.nextEpoch.validatorVoteSet != nil {
		// Save the next epoch vote set
		SaveEpochVoteSet(epoch.db, epoch.nextEpoch.Number, epoch.nextEpoch.validatorVoteSet)
	}
}

func FromBytes(buf []byte) *Epoch {

	if len(buf) == 0 {
		return nil
	} else {
		ep := &Epoch{}
		err := wire.ReadBinaryBytes(buf, ep)
		if err != nil {
			log.Errorf("Load Epoch from Bytes Failed, error: %v", err)
			return nil
		}
		return ep
	}
}

func (epoch *Epoch) Bytes() []byte {
	return wire.BinaryBytes(*epoch)
}

func (epoch *Epoch) ValidateNextEpoch(next *Epoch, height uint64) error {

	if epoch.nextEpoch == nil {
		epoch.nextEpoch = epoch.ProposeNextEpoch(height)
	}

	if !epoch.nextEpoch.Equals(next, false) {
		return NextEpochNotEXPECTED
	}

	return nil
}

//check if need propose next epoch
func (epoch *Epoch) ShouldProposeNextEpoch(curBlockHeight uint64) bool {
	// If next epoch already proposed, then no need propose again
	if epoch.nextEpoch != nil {
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

func (epoch *Epoch) ProposeNextEpoch(curBlockHeight uint64) *Epoch {

	if epoch != nil {

		rewardPerBlock, blocks := epoch.estimateForNextEpoch(curBlockHeight)

		next := &Epoch{
			mtx: epoch.mtx,
			db:  epoch.db,

			rs: epoch.rs,

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

			previousEpoch: epoch,
			nextEpoch:     nil,

			logger: epoch.logger,
		}

		return next
	}
	return nil
}

func (epoch *Epoch) GetVoteStartHeight() uint64 {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochProposeStartPercent
	return uint64(math.Ceil(percent)) + epoch.StartBlock
}

func (epoch *Epoch) GetVoteEndHeight() uint64 {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochHashVoteEndPercent
	if _, frac := math.Modf(percent); frac == 0 {
		return uint64(percent) - 1 + epoch.StartBlock
	} else {
		return uint64(math.Floor(percent)) + epoch.StartBlock
	}
}

func (epoch *Epoch) GetRevealVoteStartHeight() uint64 {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochHashVoteEndPercent
	return uint64(math.Ceil(percent)) + epoch.StartBlock
}

func (epoch *Epoch) GetRevealVoteEndHeight() uint64 {
	percent := float64(epoch.EndBlock-epoch.StartBlock) * NextEpochRevealVoteEndPercent
	return uint64(math.Floor(percent)) + epoch.StartBlock
}

func (epoch *Epoch) CheckInHashVoteStage(height uint64) bool {
	fCurBlockHeight := float64(height)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	return (NextEpochProposeStartPercent <= passRate) && (passRate < NextEpochHashVoteEndPercent)
}

func (epoch *Epoch) CheckInRevealVoteStage(height uint64) bool {
	fCurBlockHeight := float64(height)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	return (NextEpochHashVoteEndPercent <= passRate) && (passRate < NextEpochRevealVoteEndPercent)
}

func (epoch *Epoch) GetNextEpoch() *Epoch {
	return epoch.nextEpoch
}

func (epoch *Epoch) SetNextEpoch(next *Epoch) {
	epoch.nextEpoch = next
	if next != nil {
		epoch.nextEpoch.db = epoch.db
	}
}

func (epoch *Epoch) GetPreviousEpoch() *Epoch {
	return epoch.previousEpoch
}

func (epoch *Epoch) ShouldEnterNewEpoch(height uint64) (bool, error) {

	if height == epoch.EndBlock {
		if epoch.nextEpoch != nil {
			return true, nil
		} else {
			return false, NextEpochNotExist
		}
	}
	return false, nil
}

// Move to New Epoch
func (epoch *Epoch) EnterNewEpoch(height uint64) (*Epoch, []*abciTypes.RefundValidatorAmount, error) {

	if height == epoch.EndBlock {
		if epoch.nextEpoch != nil {

			// Set the End Time for current Epoch and Save it
			epoch.EndTime = time.Now()
			epoch.Save()
			// Old Epoch Ended

			// Now move to Next Epoch
			nextEpoch := epoch.nextEpoch
			// Store the Previous Epoch Validators only
			nextEpoch.previousEpoch = &Epoch{Validators: epoch.Validators}
			nextEpoch.StartTime = time.Now()

			// update the validator set with the latest abciResponses
			refund, err := updateEpochValidatorSet(nextEpoch.Validators, nextEpoch.validatorVoteSet)
			if err != nil {
				epoch.logger.Warn("Error changing validator set", "error", err)
				return nil, nil, err
			}
			// Update validator accums and set state variables
			nextEpoch.Validators.IncrementAccum(1)

			nextEpoch.nextEpoch = nil //suppose we will not generate a more epoch after next-epoch
			epoch.logger.Infof("Enter into New Epoch %v", nextEpoch)
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
				return nil, fmt.Errorf("Failed to add new validator %x with voting power %d", v.Address, v.Amount)
			}
			newValSize++
		} else if v.Amount.Sign() == 0 {
			refund = append(refund, &abciTypes.RefundValidatorAmount{Address: v.Address, Amount: validator.VotingPower})
			// Remove the Validator
			_, removed := validators.Remove(validator.Address)
			if !removed {
				return nil, fmt.Errorf("Failed to remove validator %x", validator.Address)
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
				return nil, fmt.Errorf("Failed to update validator %x with voting power %d", validator.Address, v.Amount)
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

func (epoch *Epoch) GetEpochByBlockNumber(blockNumber uint64) *Epoch {

	if blockNumber >= epoch.StartBlock && blockNumber <= epoch.EndBlock {
		return epoch
	}

	for number := epoch.Number - 1; number >= 0; number-- {

		ep := loadOneEpoch(epoch.db, number, epoch.logger)
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
		if epoch.previousEpoch != nil {
			previousEpoch = epoch.previousEpoch.copy(false)
		}

		if epoch.nextEpoch != nil {
			nextEpoch = epoch.nextEpoch.copy(false)
		}
	}

	return &Epoch{
		mtx:    epoch.mtx,
		db:     epoch.db,
		logger: epoch.logger,

		rs: epoch.rs,

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

		previousEpoch: previousEpoch,
		nextEpoch:     nextEpoch,
	}
}

func (epoch *Epoch) estimateForNextEpoch(curBlockHeight uint64) (rewardPerBlock *big.Int, blocksOfNextEpoch uint64) {

	//var totalReward          = 210000000e+18
	//var preAllocated         = 100000000e+18
	var rewardFirstYear = epoch.rs.RewardFirstYear //20000000e+18 //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PAI by 10 years
	var addedPerYear = epoch.rs.AddedPerYear       //0
	var descendPerYear = epoch.rs.DescendPerYear   //2000000e+18
	//var allocated            = epoch.RS.allocated //0
	var epochNumberPerYear = epoch.rs.EpochNumberPerYear //12

	zeroEpoch := loadOneEpoch(epoch.db, 0, epoch.logger)
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

		blocksOfNextEpoch = uint64(epochTimePerEpochLeftNextYear / timePerBlockThisEpoch)
		if blocksOfNextEpoch == 0 {
			epoch.logger.Crit("EstimateForNextEpoch Failed: Please check the epoch_no_per_year setup in Genesis")
		}

		rewardPerEpochNextYear := calculateRewardPerEpochByYear(rewardFirstYear, addedPerYear, descendPerYear, int64(nextYear), int64(epochNumberPerYear))

		rewardPerBlock = new(big.Int).Div(rewardPerEpochNextYear, big.NewInt(int64(blocksOfNextEpoch)))

	} else {

		nextYearStartTime := initStartTime.AddDate(nextYear, 0, 0)

		timeLeftThisYear := nextYearStartTime.Sub(time.Now())

		epochTimePerEpochLeftThisYear := timeLeftThisYear.Nanoseconds() / int64(epochLeftThisYear)

		blocksOfNextEpoch = uint64(epochTimePerEpochLeftThisYear / timePerBlockThisEpoch)
		if blocksOfNextEpoch == 0 {
			epoch.logger.Crit("EstimateForNextEpoch Failed: Please check the epoch_no_per_year setup in Genesis")
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
		if !epoch.previousEpoch.Equals(other.previousEpoch, false) ||
			!epoch.nextEpoch.Equals(other.nextEpoch, false) {
			return false
		}
	}

	return true
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
		epoch.nextEpoch,
		epoch.previousEpoch,
		epoch.rs != nil,
	)
}
