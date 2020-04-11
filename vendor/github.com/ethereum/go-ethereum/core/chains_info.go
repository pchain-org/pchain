package core

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	ep "github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"math/big"
	"os"
	"strings"
	"sync"
)

const (
	OFFICIAL_MINIMUM_VALIDATORS = 1
	OFFICIAL_MINIMUM_DEPOSIT    = "100000000000000000000000" // 100,000 * e18
)

type CoreChainInfo struct {
	db dbm.DB

	// Common Info
	Owner   common.Address
	ChainId string

	// Setup Info
	MinValidators    uint16
	MinDepositAmount *big.Int
	StartBlock       *big.Int
	EndBlock         *big.Int

	//joined - during creation phase
	JoinedValidators []JoinedValidator

	//validators - for stable phase; should be Epoch information
	EpochNumber uint64

	//the statitics for balance in & out
	//depositInMainChain >= depositInChildChain
	//withdrawFromChildChain >= withdrawFromMainChain
	//depositInMainChain >= withdrawFromChildChain
	DepositInMainChain     *big.Int //total deposit by users from main
	DepositInChildChain    *big.Int //total deposit allocated to users in child chain
	WithdrawFromChildChain *big.Int //total withdraw by users from child chain
	WithdrawFromMainChain  *big.Int //total withdraw refund to users in main chain
}

type JoinedValidator struct {
	PubKey        crypto.PubKey
	Address       common.Address
	DepositAmount *big.Int
}

type ChainInfo struct {
	CoreChainInfo

	//be careful, this Epoch could be different with the current epoch in the child chain
	//it is just for cache
	Epoch *ep.Epoch
}

const (
	chainInfoKey  = "CHAIN"
	ethGenesisKey = "ETH_GENESIS"
	tdmGenesisKey = "TDM_GENESIS"
)

var allChainKey = []byte("AllChainID")

const specialSep = ";"

var mtx sync.RWMutex

func calcCoreChainInfoKey(chainId string) []byte {
	return []byte(chainInfoKey + ":" + chainId)
}

func calcEpochKey(number uint64, chainId string) []byte {
	return []byte(chainInfoKey + fmt.Sprintf("-%v-%s", number, chainId))
}

func calcETHGenesisKey(chainId string) []byte {
	return []byte(ethGenesisKey + ":" + chainId)
}

func calcTDMGenesisKey(chainId string) []byte {
	return []byte(tdmGenesisKey + ":" + chainId)
}

func GetChainInfo(db dbm.DB, chainId string) *ChainInfo {
	mtx.RLock()
	defer mtx.RUnlock()

	cci := loadCoreChainInfo(db, chainId)
	if cci == nil {
		return nil
	}

	ci := &ChainInfo{
		CoreChainInfo: *cci,
	}

	epoch := loadEpoch(db, cci.EpochNumber, chainId)
	if epoch != nil {
		ci.Epoch = epoch
	}

	log.Debugf("LoadChainInfo(), chainInfo is: %v\n", ci)

	return ci
}

func SaveChainInfo(db dbm.DB, ci *ChainInfo) error {
	mtx.Lock()
	defer mtx.Unlock()

	log.Debugf("ChainInfo Save(), info is: (%v)\n", ci)

	err := saveCoreChainInfo(db, &ci.CoreChainInfo)
	if err != nil {
		return err
	}

	if ci.Epoch != nil {
		err = saveEpoch(db, ci.Epoch, ci.ChainId)
		if err != nil {
			return err
		}
	}

	saveId(db, ci.ChainId)

	return nil
}

func SaveFutureEpoch(db dbm.DB, futureEpoch *ep.Epoch, chainId string) error {
	mtx.Lock()
	defer mtx.Unlock()

	if futureEpoch != nil {
		err := saveEpoch(db, futureEpoch, chainId)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadCoreChainInfo(db dbm.DB, chainId string) *CoreChainInfo {

	cci := CoreChainInfo{db: db}
	buf := db.Get(calcCoreChainInfoKey(chainId))
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&cci, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			log.Debugf("LoadChainInfo: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
	}
	return &cci
}

func saveCoreChainInfo(db dbm.DB, cci *CoreChainInfo) error {

	db.SetSync(calcCoreChainInfoKey(cci.ChainId), wire.BinaryBytes(*cci))
	return nil
}

func (cci *CoreChainInfo) TotalDeposit() *big.Int {
	sum := big.NewInt(0)
	for _, v := range cci.JoinedValidators {
		sum.Add(sum, v.DepositAmount)
	}
	return sum
}

func loadEpoch(db dbm.DB, number uint64, chainId string) *ep.Epoch {
	epochBytes := db.Get(calcEpochKey(number, chainId))
	return ep.FromBytes(epochBytes)
}

func saveEpoch(db dbm.DB, epoch *ep.Epoch, chainId string) error {

	db.SetSync(calcEpochKey(epoch.Number, chainId), epoch.Bytes())
	return nil
}

func (ci *ChainInfo) GetEpochByBlockNumber(blockNumber uint64) *ep.Epoch {
	mtx.RLock()
	defer mtx.RUnlock()

	if blockNumber < 0 {
		return ci.Epoch
	} else {
		epoch := ci.Epoch
		if epoch == nil {
			return nil
		}
		if blockNumber >= epoch.StartBlock && blockNumber <= epoch.EndBlock {
			return epoch
		}

		number := epoch.Number
		// If blockNumber > epoch EndBlock, find future epoch
		if blockNumber > epoch.EndBlock {
			for {
				number ++

				ep := loadEpoch(ci.db, number, ci.ChainId)
				if ep == nil {
					return nil
				}

				if blockNumber >= ep.StartBlock && blockNumber <= ep.EndBlock {
					return ep
				}
			}

		} else { // If blockNumber < epoch StartBlock, find history epoch
			for {
				if number == 0 {
					break
				}
				number--

				ep := loadEpoch(ci.db, number, ci.ChainId)
				if ep == nil {
					return nil
				}

				if blockNumber >= ep.StartBlock && blockNumber <= ep.EndBlock {
					return ep
				}
			}
		}
	}
	return nil
}

func saveId(db dbm.DB, chainId string) {

	buf := db.Get(allChainKey)

	if len(buf) == 0 {
		db.SetSync(allChainKey, []byte(chainId))
		log.Debugf("ChainInfo SaveId(), chainId is: %s\n", chainId)
	} else {

		strIdArr := strings.Split(string(buf), specialSep)

		found := false
		for _, id := range strIdArr {
			if id == chainId {
				found = true
				break
			}
		}

		if !found {
			strIdArr = append(strIdArr, chainId)
			strIds := strings.Join(strIdArr, specialSep)
			db.SetSync(allChainKey, []byte(strIds))

			log.Debugf("ChainInfo SaveId(), strIds is: %s\n", strIds)
		}
	}
}

func GetChildChainIds(db dbm.DB) []string {
	mtx.RLock()
	defer mtx.RUnlock()

	buf := db.Get(allChainKey)

	log.Debugf("GetChildChainIds 0, buf is %v, len is %d\n", buf, len(buf))

	if len(buf) == 0 {
		return []string{}
	}

	return strings.Split(string(buf), specialSep)
}

func CheckChildChainRunning(db dbm.DB, chainId string) bool {
	ids := GetChildChainIds(db)

	for _, id := range ids {
		if id == chainId {
			return true
		}
	}

	return false
}

// SaveChainGenesis save the genesis file for child chain
func SaveChainGenesis(db dbm.DB, chainId string, ethGenesis, tdmGenesis []byte) {
	mtx.Lock()
	defer mtx.Unlock()

	// Save the eth genesis
	db.SetSync(calcETHGenesisKey(chainId), ethGenesis)

	// Save the tdm genesis
	db.SetSync(calcTDMGenesisKey(chainId), tdmGenesis)
}

// LoadChainGenesis load the genesis file for child chain
func LoadChainGenesis(db dbm.DB, chainId string) (ethGenesis, tdmGenesis []byte) {
	mtx.RLock()
	defer mtx.RUnlock()

	ethGenesis = db.Get(calcETHGenesisKey(chainId))
	tdmGenesis = db.Get(calcTDMGenesisKey(chainId))
	return
}

// ---------------------
// Pending Chain
var pendingChainMtx sync.Mutex

var pendingChainIndexKey = []byte("PENDING_CHAIN_IDX")

func calcPendingChainInfoKey(chainId string) []byte {
	return []byte("PENDING_CHAIN:" + chainId)
}

type pendingIdxData struct {
	ChainID string
	Start   *big.Int
	End     *big.Int
}

// GetPendingChildChainData get the pending child chain data from db with key pending chain
func GetPendingChildChainData(db dbm.DB, chainId string) *CoreChainInfo {

	pendingChainByteSlice := db.Get(calcPendingChainInfoKey(chainId))
	if pendingChainByteSlice != nil {
		var cci CoreChainInfo
		wire.ReadBinaryBytes(pendingChainByteSlice, &cci)
		return &cci
	}

	return nil
}

// CreatePendingChildChainData create the pending child chain data with index
func CreatePendingChildChainData(db dbm.DB, cci *CoreChainInfo) {
	storePendingChildChainData(db, cci, true)
}

// UpdatePendingChildChainData update the pending child chain data without index
func UpdatePendingChildChainData(db dbm.DB, cci *CoreChainInfo) {
	storePendingChildChainData(db, cci, false)
}

// storePendingChildChainData save the pending child chain data into db with key pending chain
func storePendingChildChainData(db dbm.DB, cci *CoreChainInfo, create bool) {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	// store the data
	db.SetSync(calcPendingChainInfoKey(cci.ChainId), wire.BinaryBytes(*cci))

	if create {
		// index the data
		var idx []pendingIdxData
		pendingIdxByteSlice := db.Get(pendingChainIndexKey)
		if pendingIdxByteSlice != nil {
			wire.ReadBinaryBytes(pendingIdxByteSlice, &idx)
		}
		// Check if chain id has been added already
		for _, v := range idx {
			if v.ChainID == cci.ChainId {
				return
			}
		}
		// Pass the check, add the key to idx
		idx = append(idx, pendingIdxData{cci.ChainId, cci.StartBlock, cci.EndBlock})
		db.SetSync(pendingChainIndexKey, wire.BinaryBytes(idx))
	}
}

// DeletePendingChildChainData delete the pending child chain data from db with chain id
func DeletePendingChildChainData(db dbm.DB, chainId string) {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	db.DeleteSync(calcPendingChainInfoKey(chainId))
}

// GetChildChainForLaunch get the child chain for pending db for launch
func GetChildChainForLaunch(db dbm.DB, height *big.Int, stateDB *state.StateDB) (readyForLaunch []string, newPendingIdxBytes []byte, deleteChildChainIds []string) {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	// Get the Pending Index from db
	var idx []pendingIdxData
	pendingIdxByteSlice := db.Get(pendingChainIndexKey)
	if pendingIdxByteSlice != nil {
		wire.ReadBinaryBytes(pendingIdxByteSlice, &idx)
	}

	if len(idx) == 0 {
		return
	}

	newPendingIdx := idx[:0]

	for _, v := range idx {
		if v.Start.Cmp(height) > 0 {
			// skip it
			newPendingIdx = append(newPendingIdx, v)
		} else if v.End.Cmp(height) < 0 {
			// Refund the Lock Balance
			cci := GetPendingChildChainData(db, v.ChainID)
			for _, jv := range cci.JoinedValidators {
				stateDB.SubChildChainDepositBalance(jv.Address, v.ChainID, jv.DepositAmount)
				stateDB.AddBalance(jv.Address, jv.DepositAmount)
			}

			officialMinimumDeposit := math.MustParseBig256(OFFICIAL_MINIMUM_DEPOSIT)
			stateDB.AddBalance(cci.Owner, officialMinimumDeposit)
			stateDB.SubChainBalance(cci.Owner, officialMinimumDeposit)
			if stateDB.GetChainBalance(cci.Owner).Sign() != 0 {
				log.Error("the chain balance is not 0 when create chain failed, watch out!!!")
			}

			// Add the Child Chain Id to Remove List, to be removed after the consensus
			deleteChildChainIds = append(deleteChildChainIds, v.ChainID)
			//db.DeleteSync(calcPendingChainInfoKey(v.ChainID))
		} else {
			// check condition
			cci := GetPendingChildChainData(db, v.ChainID)
			if len(cci.JoinedValidators) >= int(cci.MinValidators) && cci.TotalDeposit().Cmp(cci.MinDepositAmount) >= 0 {
				// Deduct the Deposit
				for _, jv := range cci.JoinedValidators {
					// Deposit will move to the Child Chain Account
					stateDB.SubChildChainDepositBalance(jv.Address, v.ChainID, jv.DepositAmount)
					stateDB.AddChainBalance(cci.Owner, jv.DepositAmount)
				}
				// Append the Chain ID to Ready Launch List
				readyForLaunch = append(readyForLaunch, v.ChainID)
			} else {
				newPendingIdx = append(newPendingIdx, v)
			}
		}
	}

	if len(newPendingIdx) != len(idx) {
		// Set the Bytes to Update the Pending Idx
		newPendingIdxBytes = wire.BinaryBytes(newPendingIdx)
		//db.SetSync(pendingChainIndexKey, wire.BinaryBytes(newPendingIdx))
	}

	// Return the ready for launch Child Chain
	return
}

func ProcessPostPendingData(db dbm.DB, newPendingIdxBytes []byte, deleteChildChainIds []string) {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	// Remove the Child Chain
	for _, id := range deleteChildChainIds {
		db.DeleteSync(calcPendingChainInfoKey(id))
	}

	// Update the Idx Bytes
	if newPendingIdxBytes != nil {
		db.SetSync(pendingChainIndexKey, newPendingIdxBytes)
	}
}
