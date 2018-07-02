package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	ep "github.com/tendermint/tendermint/epoch"
	"math/big"
	"os"
	"strings"
	"sync"
)

type CoreChainInfo struct {
	db dbm.DB

	// Common Info
	Owner   common.Address
	ChainId string

	// Setup Info
	MinValidators    uint16
	MinDepositAmount *big.Int
	StartBlock       uint64
	EndBlock         uint64

	//joined - during creation phase
	JoinedValidators []JoinedValidator

	//validators - for stable phase; should be Epoch information
	EpochNumber int

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

const chainInfoKey = "CHAIN"

var allChainKey = []byte("AllChainID")

const specialSep = ";"

var mtx sync.Mutex

func calcCoreChainInfoKey(chainId string) []byte {
	return []byte(chainInfoKey + ":" + chainId)
}

func calcEpochKey(number int, chainId string) []byte {
	return []byte(chainInfoKey + fmt.Sprintf("-%v-%s", number, chainId))
}

func GetChainInfo(db dbm.DB, chainId string) *ChainInfo {

	cci := loadCoreChainInfo(db, chainId)
	if cci == nil {
		return nil
	}

	ci := &ChainInfo{
		CoreChainInfo: *cci,
	}

	if cci.EpochNumber != 0 {
		epoch := loadEpoch(db, cci.EpochNumber, chainId)
		if epoch == nil {
			return nil
		}
		ci.Epoch = epoch
	}

	fmt.Printf("LoadChainInfo(), chainInfo is: %v\n", ci)

	return ci
}

func SaveChainInfo(db dbm.DB, ci *ChainInfo) error {

	mtx.Lock()
	defer mtx.Unlock()
	fmt.Printf("ChainInfo Save(), info is: (%v, %v)\n", ci)

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

	SaveId(db, ci.ChainId)

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
			fmt.Printf("LoadChainInfo: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
	}
	return &cci
}

func saveCoreChainInfo(db dbm.DB, cci *CoreChainInfo) error {

	db.SetSync(calcCoreChainInfoKey(cci.ChainId), cci.Bytes())
	return nil
}

func (cci *CoreChainInfo) Bytes() []byte {

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(*cci, buf, n, err)
	if *err != nil {
		return nil
	}
	return buf.Bytes()
}

func (cci *CoreChainInfo) TotalDeposit() *big.Int {
	sum := big.NewInt(0)
	for _, v := range cci.JoinedValidators {
		sum.Add(sum, v.DepositAmount)
	}
	return sum
}

func loadEpoch(db dbm.DB, number int, chainId string) *ep.Epoch {

	mtx.Lock()
	defer mtx.Unlock()

	epochBytes := db.Get(calcEpochKey(number, chainId))
	return ep.FromBytes(epochBytes)
}

func saveEpoch(db dbm.DB, epoch *ep.Epoch, chainId string) error {

	db.SetSync(calcEpochKey(epoch.Number, chainId), epoch.Bytes())
	return nil
}

func (ci *ChainInfo) GetEpochByBlockNumber(blockNumber int) *ep.Epoch {

	if blockNumber < 0 {
		return ci.Epoch
	} else {
		epoch := ci.Epoch
		if blockNumber >= epoch.StartBlock && blockNumber <= epoch.EndBlock {
			return epoch
		}

		for number := epoch.Number - 1; number >= 0; number-- {

			ep := loadEpoch(ci.db, number, ci.ChainId)
			if ep == nil {
				return nil
			}

			if blockNumber >= ep.StartBlock && blockNumber <= ep.EndBlock {
				return ep
			}
		}
	}
	return nil
}

func SaveId(db dbm.DB, chainId string) {

	buf := db.Get(allChainKey)

	if len(buf) == 0 {
		db.SetSync(allChainKey, []byte(chainId))
		fmt.Printf("ChainInfo SaveId(), chainId is: %s\n", chainId)
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

			fmt.Printf("ChainInfo SaveId(), strIds is: %s\n", strIds)
		}
	}
}

func GetChildChainIds(db dbm.DB) []string {

	buf := db.Get(allChainKey)

	fmt.Printf("GetChildChainIds 0, buf is %v, len is %d\n", buf, len(buf))

	if len(buf) == 0 {
		return []string{}
	}

	strIdArr := strings.Split(string(buf), specialSep)

	fmt.Printf("GetChildChainIds 1, strIdArr is %v, len is %d\n", strIdArr, len(strIdArr))

	return strIdArr
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
	Start   uint64
	End     uint64
}

// GetPendingChildChainData get the pending child chain data from db with key pending chain
func GetPendingChildChainData(db dbm.DB, chainId string) *CoreChainInfo {

	pendingChainByteSlice := db.Get(calcPendingChainInfoKey(chainId))
	if pendingChainByteSlice != nil {
		var cci CoreChainInfo
		gobReadBinaryBytes(pendingChainByteSlice, &cci)
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
	db.SetSync(calcPendingChainInfoKey(cci.ChainId), gobBinaryBytes(*cci))

	if create {
		// index the data
		var idx []pendingIdxData
		pendingIdxByteSlice := db.Get(pendingChainIndexKey)
		if pendingIdxByteSlice != nil {
			gobReadBinaryBytes(pendingIdxByteSlice, &idx)
		}
		idx = append(idx, pendingIdxData{cci.ChainId, cci.StartBlock, cci.EndBlock})
		db.SetSync(pendingChainIndexKey, gobBinaryBytes(idx))
	}
}

// DeletePendingChildChainData delete the pending child chain data from db with chain id
func DeletePendingChildChainData(db dbm.DB, chainId string) {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	db.DeleteSync(calcPendingChainInfoKey(chainId))
}

// GetChildChainForLaunch get the child chain for pending db for launch
func GetChildChainForLaunch(db dbm.DB, height uint64) []string {
	pendingChainMtx.Lock()
	defer pendingChainMtx.Unlock()

	// Get the Pending Index from db
	var idx []pendingIdxData
	pendingIdxByteSlice := db.Get(pendingChainIndexKey)
	if pendingIdxByteSlice != nil {
		gobReadBinaryBytes(pendingIdxByteSlice, &idx)
	}

	if len(idx) == 0 {
		return nil
	}

	newPendingIdx := idx[:0]
	readyForLaunch := make([]string, 0)

	for _, v := range idx {
		if v.Start > height {
			// skip it
			newPendingIdx = append(newPendingIdx, v)
		} else if v.End < height {
			// remove it
			DeletePendingChildChainData(db, v.ChainID)
		} else {
			// check condition
			cci := GetPendingChildChainData(db, v.ChainID)
			if len(cci.JoinedValidators) >= int(cci.MinValidators) && cci.TotalDeposit().Cmp(cci.MinDepositAmount) >= 0 {
				readyForLaunch = append(readyForLaunch, v.ChainID)
			} else {
				newPendingIdx = append(newPendingIdx, v)
			}
		}
	}

	if len(newPendingIdx) != len(idx) {
		// Update the Pending Idx
		db.SetSync(pendingChainIndexKey, gobBinaryBytes(newPendingIdx))
	}

	// Return the ready for launch Child Chain
	return readyForLaunch
}

func gobBinaryBytes(o interface{}) []byte {
	gob.Register(&crypto.EtherumPubKey{})
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	enc.Encode(o)
	return b.Bytes()
}

func gobReadBinaryBytes(d []byte, ptr interface{}) error {
	dec := gob.NewDecoder(bytes.NewReader(d))
	return dec.Decode(ptr)
}