package epoch

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
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

	epochKey       = "Epoch:%v"
	latestEpochKey = "LatestEpoch"
)

type Epoch struct {
	mtx sync.Mutex
	db  dbm.DB

	Number         uint64
	RewardPerBlock *big.Int
	StartBlock     uint64
	EndBlock       uint64
	StartTime      time.Time
	EndTime        time.Time //not accurate for current epoch
	BlockGenerated int       //agreed in which block
	Status         int       //checked if this epoch has been saved
	Validators     *tmTypes.ValidatorSet

	// The VoteSet will be used just before Epoch Start
	validatorVoteSet *EpochValidatorVoteSet // VoteSet store with key prefix EpochValidatorVote_
	rs               *RewardScheme          // RewardScheme store with key REWARDSCHEME
	previousEpoch    *Epoch
	nextEpoch        *Epoch

	logger log.Logger
}

func calcEpochKeyWithHeight(number uint64) []byte {
	return []byte(fmt.Sprintf(epochKey, number))
}

// InitEpoch either initial the Epoch from DB or from genesis file
func InitEpoch(db dbm.DB, genDoc *tmTypes.GenesisDoc, logger log.Logger) *Epoch {

	epochNumber := db.Get([]byte(latestEpochKey))
	if epochNumber == nil {
		// Read Epoch from Genesis
		rewardScheme := MakeRewardScheme(db, &genDoc.RewardScheme)
		rewardScheme.Save()

		ep := MakeOneEpoch(db, &genDoc.CurrentEpoch, logger)
		ep.Save()

		ep.SetRewardScheme(rewardScheme)
		return ep
	} else {
		// Load Epoch from DB
		epNo, _ := strconv.ParseUint(string(epochNumber), 10, 64)
		return LoadOneEpoch(db, epNo, logger)
	}
}

// Load Full Epoch By EpochNumber (Epoch data, Reward Scheme, ValidatorVote, Previous Epoch, Next Epoch)
func LoadOneEpoch(db dbm.DB, epochNumber uint64, logger log.Logger) *Epoch {
	// Load Epoch Data from DB
	epoch := loadOneEpoch(db, epochNumber, logger)
	// Set Reward Scheme
	rewardscheme := LoadRewardScheme(db)
	epoch.rs = rewardscheme
	// Set Validator VoteSet if has
	epoch.validatorVoteSet = LoadEpochVoteSet(db, epochNumber)
	// Set Previous Epoch
	if epochNumber > 0 {
		epoch.previousEpoch = loadOneEpoch(db, epochNumber-1, logger)
		if epoch.previousEpoch != nil {
			epoch.previousEpoch.rs = rewardscheme
		}
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

func loadOneEpoch(db dbm.DB, epochNumber uint64, logger log.Logger) *Epoch {

	buf := db.Get(calcEpochKeyWithHeight(epochNumber))
	ep := FromBytes(buf)
	if ep != nil {
		ep.db = db
		ep.logger = logger
	}
	return ep
}

// Convert from OneEpochDoc (Json) to Epoch
func MakeOneEpoch(db dbm.DB, oneEpoch *tmTypes.OneEpochDoc, logger log.Logger) *Epoch {

	number, _ := strconv.ParseUint(oneEpoch.Number, 10, 64)
	RewardPerBlock, _ := new(big.Int).SetString(oneEpoch.RewardPerBlock, 10)
	StartBlock, _ := strconv.ParseUint(oneEpoch.StartBlock, 10, 64)
	EndBlock, _ := strconv.ParseUint(oneEpoch.EndBlock, 10, 64)
	StartTime := time.Now()
	EndTime := time.Unix(0, 0) //not accurate for current epoch
	BlockGenerated, _ := strconv.Atoi(oneEpoch.BlockGenerated)
	Status, _ := strconv.Atoi(oneEpoch.Status)

	validators := make([]*tmTypes.Validator, len(oneEpoch.Validators))
	for i, val := range oneEpoch.Validators {
		pubKey := val.PubKey
		// address := pubKey.Address()
		//TODO: very important, here the address should be the ethereum account,
		//TODO: at least, should add one additional ethereum account
		address := val.EthAccount.Bytes()

		// Make validator
		validators[i] = &tmTypes.Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Amount,
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
	epoch.db.SetSync([]byte(latestEpochKey), []byte(strconv.FormatUint(epoch.Number, 10)))

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

func (epoch *Epoch) ValidateNextEpoch(next *Epoch, lastHeight uint64, lastBlockTime time.Time) error {

	myNextEpoch := epoch.ProposeNextEpoch(lastHeight, lastBlockTime)

	if !myNextEpoch.Equals(next, false) {
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

func (epoch *Epoch) ProposeNextEpoch(lastBlockHeight uint64, lastBlockTime time.Time) *Epoch {

	if epoch != nil {

		rewardPerBlock, blocks := epoch.estimateForNextEpoch(lastBlockHeight, lastBlockTime)

		next := &Epoch{
			mtx: epoch.mtx,
			db:  epoch.db,

			Number:         epoch.Number + 1,
			RewardPerBlock: rewardPerBlock,
			StartBlock:     epoch.EndBlock + 1,
			EndBlock:       epoch.EndBlock + blocks,
			BlockGenerated: 0,
			Status:         EPOCH_PROPOSED_NOT_VOTED,
			Validators:     epoch.Validators.Copy(), // Old Validators

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

func (epoch *Epoch) CheckInNormalStage(height uint64) bool {
	fCurBlockHeight := float64(height)
	fStartBlock := float64(epoch.StartBlock)
	fEndBlock := float64(epoch.EndBlock)

	passRate := (fCurBlockHeight - fStartBlock) / (fEndBlock - fStartBlock)

	return (0 <= passRate) && (passRate < NextEpochProposeStartPercent)
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
	if next != nil {
		next.db = epoch.db
		next.rs = epoch.rs
		next.logger = epoch.logger
	}
	epoch.nextEpoch = next
}

func (epoch *Epoch) GetPreviousEpoch() *Epoch {
	return epoch.previousEpoch
}

func (epoch *Epoch) ShouldEnterNewEpoch(height uint64, state *state.StateDB) (bool, *tmTypes.ValidatorSet, error) {

	if height == epoch.EndBlock {
		if epoch.nextEpoch != nil {
			// Step 1: Refund the Delegate (subtract the pending refund / deposit proxied amount)
			for refundAddress := range state.GetDelegateAddressRefundSet() {
				state.ForEachProxied(refundAddress, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
					if pendingRefundBalance.Sign() > 0 {
						// Refund Pending Refund
						state.SubDepositProxiedBalanceByUser(refundAddress, key, pendingRefundBalance)
						state.SubPendingRefundBalanceByUser(refundAddress, key, pendingRefundBalance)
						state.SubDelegateBalance(key, pendingRefundBalance)
						state.AddBalance(key, pendingRefundBalance)
					}
					return true
				})
				// reset commission = 0 if not candidate
				if !state.IsCandidate(refundAddress) {
					state.ClearCommission(refundAddress)
				}
			}
			state.ClearDelegateRefundSet()

			// Step 2: Sort the Validators and potential Validators (with success vote) base on deposit amount + deposit proxied amount
			// Step 2.1: Update deposit amount base on the vote (Add/Substract deposit amount base on vote)
			// Step 2.2: Sort the address with deposit + deposit proxied amount
			newValidators := epoch.nextEpoch.Validators.Copy()
			for _, v := range newValidators.Validators {
				vAddr := common.BytesToAddress(v.Address)
				totalProxiedBalance := new(big.Int).Add(state.GetTotalProxiedBalance(vAddr), state.GetTotalDepositProxiedBalance(vAddr))
				// Voting Power = Delegated amount + Deposit amount
				v.VotingPower = new(big.Int).Add(totalProxiedBalance, state.GetDepositBalance(vAddr))
			}

			// Update Validators with vote
			refunds, err := updateEpochValidatorSet(newValidators, epoch.nextEpoch.validatorVoteSet)
			if err != nil {
				epoch.logger.Warn("Error changing validator set", "error", err)
				return false, nil, err
			}

			// Now newValidators become a real new Validators
			// Step 3: Special Case: For the existing Validator + Candidate + no vote, Move proxied amount to deposit proxied amount  (proxied amount -> deposit proxied amount)
			// (if has vote, proxied amount has already move to deposit proxied amount during apply reveal vote)
			for _, v := range newValidators.Validators {
				vAddr := common.BytesToAddress(v.Address)
				if state.IsCandidate(vAddr) && state.GetTotalProxiedBalance(vAddr).Sign() > 0 {
					state.ForEachProxied(vAddr, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
						if proxiedBalance.Sign() > 0 {
							// Deposit the proxied amount
							state.SubProxiedBalanceByUser(vAddr, key, proxiedBalance)
							state.AddDepositProxiedBalanceByUser(vAddr, key, proxiedBalance)
						}
						return true
					})
				}
			}

			// Step 4: For vote out Address, refund deposit (deposit amount -> balance, deposit proxied amount -> proxied amount)
			for _, r := range refunds {
				if !r.Voteout {
					// Normal Refund, refund the deposit back to the self balance
					state.SubDepositBalance(r.Address, r.Amount)
					state.AddBalance(r.Address, r.Amount)
				} else {
					// Voteout Refund, refund the deposit both to self and proxied (if available)
					if state.IsCandidate(r.Address) {
						state.ForEachProxied(r.Address, func(key common.Address, proxiedBalance, depositProxiedBalance, pendingRefundBalance *big.Int) bool {
							if depositProxiedBalance.Sign() > 0 {
								state.SubDepositProxiedBalanceByUser(r.Address, key, depositProxiedBalance)
								state.AddProxiedBalanceByUser(r.Address, key, depositProxiedBalance)
							}
							return true
						})
					}
					// Refund all the self deposit balance
					depositBalance := state.GetDepositBalance(r.Address)
					state.SubDepositBalance(r.Address, depositBalance)
					state.AddBalance(r.Address, depositBalance)
				}
			}

			return true, newValidators, nil
		} else {
			return false, nil, NextEpochNotExist
		}
	}
	return false, nil, nil
}

// Move to New Epoch
func (epoch *Epoch) EnterNewEpoch(newValidators *tmTypes.ValidatorSet) (*Epoch, error) {
	if epoch.nextEpoch != nil {
		now := time.Now()

		// Set the End Time for current Epoch and Save it
		epoch.EndTime = now
		epoch.Save()
		// Old Epoch Ended
		epoch.logger.Infof("Epoch %v reach to his end", epoch.Number)

		// Now move to Next Epoch
		nextEpoch := epoch.nextEpoch
		// Store the Previous Epoch Validators only
		nextEpoch.previousEpoch = &Epoch{Validators: epoch.Validators}
		nextEpoch.StartTime = now
		nextEpoch.Validators = newValidators

		nextEpoch.nextEpoch = nil //suppose we will not generate a more epoch after next-epoch
		nextEpoch.Save()
		epoch.logger.Infof("Enter into New Epoch %v", nextEpoch)
		return nextEpoch, nil
	} else {
		return nil, NextEpochNotExist
	}
}

func DryRunUpdateEpochValidatorSet(validators *tmTypes.ValidatorSet, voteSet *EpochValidatorVoteSet) error {
	_, err := updateEpochValidatorSet(validators, voteSet)
	return err
}

// updateEpochValidatorSet Update the Current Epoch Validator by vote
//
func updateEpochValidatorSet(validators *tmTypes.ValidatorSet, voteSet *EpochValidatorVoteSet) ([]*tmTypes.RefundValidatorAmount, error) {
	if voteSet.IsEmpty() {
		// No vote, keep the current validator set
		return nil, nil
	}

	// Refund List will be vaildators contain from Vote (exit validator or less amount than previous amount) and Knockout after sort by amount
	var refund []*tmTypes.RefundValidatorAmount
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
			refund = append(refund, &tmTypes.RefundValidatorAmount{Address: v.Address, Amount: validator.VotingPower, Voteout: false})
			// Remove the Validator
			_, removed := validators.Remove(validator.Address)
			if !removed {
				return nil, fmt.Errorf("Failed to remove validator %x", validator.Address)
			}
		} else {
			//refund if new amount less than the voting power
			if v.Amount.Cmp(validator.VotingPower) == -1 {
				refundAmount := new(big.Int).Sub(validator.VotingPower, v.Amount)
				refund = append(refund, &tmTypes.RefundValidatorAmount{Address: v.Address, Amount: refundAmount, Voteout: false})
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
			refund = append(refund, &tmTypes.RefundValidatorAmount{Address: common.BytesToAddress(k.Address), Amount: nil, Voteout: true})
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

func (epoch *Epoch) estimateForNextEpoch(lastBlockHeight uint64, lastBlockTime time.Time) (rewardPerBlock *big.Int, blocksOfNextEpoch uint64) {

	var rewardFirstYear = epoch.rs.RewardFirstYear       //20000000e+18 //2 + 1.8 + 1.6 + ... + 0.2；release all left 110000000 PI by 10 years
	var epochNumberPerYear = epoch.rs.EpochNumberPerYear //12
	var totalYear = epoch.rs.TotalYear                   // 23

	const EMERGENCY_BLOCKS_OF_NEXT_EPOCH uint64 = 10

	zeroEpoch := loadOneEpoch(epoch.db, 0, epoch.logger)
	initStartTime := zeroEpoch.StartTime

	//from 0 year
	thisYear := epoch.Number / epochNumberPerYear
	nextYear := thisYear + 1

	timePerBlockThisEpoch := lastBlockTime.Sub(epoch.StartTime).Nanoseconds() / int64(lastBlockHeight-epoch.StartBlock)

	epochLeftThisYear := epochNumberPerYear - epoch.Number%epochNumberPerYear - 1

	blocksOfNextEpoch = 0

	log.Info("estimateForNextEpoch",
		"epochLeftThisYear", epochLeftThisYear,
		"timePerBlockThisEpoch", timePerBlockThisEpoch)

	if epochLeftThisYear == 0 { //to another year

		nextYearStartTime := initStartTime.AddDate(int(nextYear), 0, 0)

		timeLeftNextYear := nextYearStartTime.AddDate(1, 0, 0).Sub(nextYearStartTime)

		epochLeftNextYear := epochNumberPerYear

		epochTimePerEpochLeftNextYear := timeLeftNextYear.Nanoseconds() / int64(epochLeftNextYear)

		blocksOfNextEpoch = uint64(epochTimePerEpochLeftNextYear / timePerBlockThisEpoch)

		log.Info("estimateForNextEpoch 0",
			"timePerBlockThisEpoch", timePerBlockThisEpoch,
			"nextYearStartTime", nextYearStartTime,
			"timeLeftNextYear", timeLeftNextYear,
			"epochLeftNextYear", epochLeftNextYear,
			"epochTimePerEpochLeftNextYear", epochTimePerEpochLeftNextYear,
			"blocksOfNextEpoch", blocksOfNextEpoch)

		if blocksOfNextEpoch == 0 {
			blocksOfNextEpoch = EMERGENCY_BLOCKS_OF_NEXT_EPOCH //make it move ahead
			epoch.logger.Error("EstimateForNextEpoch Error: Please check the epoch_no_per_year setup in Genesis")
		}

		rewardPerEpochNextYear := calculateRewardPerEpochByYear(rewardFirstYear, int64(nextYear), int64(totalYear), int64(epochNumberPerYear))

		rewardPerBlock = new(big.Int).Div(rewardPerEpochNextYear, big.NewInt(int64(blocksOfNextEpoch)))

	} else {

		nextYearStartTime := initStartTime.AddDate(int(nextYear), 0, 0)

		timeLeftThisYear := nextYearStartTime.Sub(lastBlockTime)

		epochTimePerEpochLeftThisYear := timeLeftThisYear.Nanoseconds() / int64(epochLeftThisYear)

		blocksOfNextEpoch = uint64(epochTimePerEpochLeftThisYear / timePerBlockThisEpoch)

		log.Info("estimateForNextEpoch 1",
			"timePerBlockThisEpoch", timePerBlockThisEpoch,
			"nextYearStartTime", nextYearStartTime,
			"timeLeftThisYear", timeLeftThisYear,
			"epochTimePerEpochLeftThisYear", epochTimePerEpochLeftThisYear,
			"blocksOfNextEpoch", blocksOfNextEpoch)

		if blocksOfNextEpoch == 0 {
			blocksOfNextEpoch = EMERGENCY_BLOCKS_OF_NEXT_EPOCH //make it move ahead
			epoch.logger.Error("EstimateForNextEpoch Error: Please check the epoch_no_per_year setup in Genesis")
		}

		epoch.logger.Debugf("Current Epoch Number %v, This Year %v, Next Year %v, Epoch No Per Year %v, Epoch Left This year %v\n"+
			"initStartTime %v ; nextYearStartTime %v\n"+
			"Time Left This year %v, timePerBlockThisEpoch %v, blocksOfNextEpoch %v\n", epoch.Number, thisYear, nextYear, epochNumberPerYear, epochLeftThisYear, initStartTime, nextYearStartTime, timeLeftThisYear, timePerBlockThisEpoch, blocksOfNextEpoch)

		rewardPerEpochThisYear := calculateRewardPerEpochByYear(rewardFirstYear, int64(thisYear), int64(totalYear), int64(epochNumberPerYear))

		rewardPerBlock = new(big.Int).Div(rewardPerEpochThisYear, big.NewInt(int64(blocksOfNextEpoch)))

	}
	return rewardPerBlock, blocksOfNextEpoch
}

/*
	Abstract function to calculate the reward of each Epoch by year

	factor = year / 4
switch factor
case 5
	factor = 4
default:
	rewardYear = rewardFirstYear / 2 ^ factor

	rewardYear / epochNumberPerYear (12)
*/
func calculateRewardPerEpochByYear(rewardFirstYear *big.Int, year, totalYear, epochNumberPerYear int64) *big.Int {
	if year > totalYear {
		return big.NewInt(0)
	}

	var rewardYear *big.Int

	power := year / 4
	switch power {
	case 5:
		power = 4
		fallthrough
	default:
		exp := new(big.Int).Exp(big.NewInt(2), big.NewInt(power), nil)
		rewardYear = new(big.Int).Div(rewardFirstYear, exp)
	}

	return new(big.Int).Div(rewardYear, big.NewInt(epochNumberPerYear))
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

func UpdateEpochEndTime(db dbm.DB, epNumber uint64, endTime time.Time) {
	// Load Epoch from DB
	ep := loadOneEpoch(db, epNumber, nil)
	if ep != nil {
		ep.mtx.Lock()
		defer ep.mtx.Unlock()
		// Set End Time
		ep.EndTime = endTime
		// Save back to DB
		db.SetSync(calcEpochKeyWithHeight(epNumber), ep.Bytes())
	}
}
