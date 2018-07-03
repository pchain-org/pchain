package core

import (
	"github.com/ethereum/go-ethereum/common"
	"fmt"
	"os"
	wire "github.com/tendermint/go-wire"
	dbm "github.com/tendermint/go-db"
	"bytes"
	"sync"
	"strings"
	"math/big"
	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
)

type CoreChainInfo struct {
	db dbm.DB

	Owner	common.Address
	ChainId	string

	//joined - during creation phase
	Joined  []common.Address

	//validators - for stable phase; should be Epoch information
	EpochNumber int

	//the statitics for balance in & out
	//depositInMainChain >= depositInChildChain
	//withdrawFromChildChain >= withdrawFromMainChain
	//depositInMainChain >= withdrawFromChildChain
	DepositInMainChain *big.Int      //total deposit by users from main
	DepositInChildChain *big.Int     //total deposit allocated to users in child chain
	WithdrawFromChildChain *big.Int  //total withdraw by users from child chain
	WithdrawFromMainChain *big.Int   //total withdraw refund to users in main chain
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
	if cci == nil {return nil}

	epoch := loadEpoch(db, cci.EpochNumber, chainId)
	if epoch == nil {return nil}

	ci := &ChainInfo {
		CoreChainInfo: *cci,
		Epoch: epoch,
	}

	fmt.Printf("LoadChainInfo(), chainInfo is: %v\n", ci)

	return nil
}

func SaveChainInfo(db dbm.DB, ci *ChainInfo) error{

	mtx.Lock()
	defer mtx.Unlock()
	fmt.Printf("ChainInfo Save(), info is: (%v, %v)\n", ci)

	err := saveCoreChainInfo(db, &ci.CoreChainInfo)
	if err != nil {return err}

	err = saveEpoch(db, ci.Epoch, ci.ChainId)
	if err != nil {return err}

	SaveId(db, ci.ChainId)

	return nil
}

func loadCoreChainInfo(db dbm.DB, chainId string) *CoreChainInfo {

	cci := CoreChainInfo{db:db}
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

	mtx.Lock()
	defer mtx.Unlock()

	db.SetSync(calcCoreChainInfoKey(cci.ChainId), cci.Bytes())
	return nil
}

func (cci *CoreChainInfo) Bytes() []byte {

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(cci, buf, n, err)
	if *err != nil {
		return nil
	}
	return buf.Bytes()
}

func loadEpoch(db dbm.DB, number int, chainId string) *ep.Epoch {

	mtx.Lock()
	defer mtx.Unlock()

	epochBytes := db.Get(calcEpochKey(number,chainId))
	return ep.FromBytes(epochBytes)
}

func saveEpoch(db dbm.DB, epoch *ep.Epoch, chainId string) error {

	mtx.Lock()
	defer mtx.Unlock()

	db.SetSync(calcEpochKey(epoch.Number, chainId), epoch.Bytes())
	return nil
}


func (ci *ChainInfo)GetEpochByBlockNumber(blockNumber int) *ep.Epoch {

	if blockNumber < 0 {
		return ci.Epoch
	} else {
		epoch := ci.Epoch
		if blockNumber >= epoch.StartBlock && blockNumber <= epoch.EndBlock {
			return epoch
		}

		for number:=epoch.Number-1; number>=0; number-- {

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

func GetChildChainIds(db dbm.DB) []string{

	buf := db.Get(allChainKey)

	fmt.Printf("GetChildChainIds 0, buf is %v, len is %d\n", buf, len(buf))

	if len(buf) == 0 {return []string{}}

	strIdArr := strings.Split(string(buf), specialSep)

	fmt.Printf("GetChildChainIds 1, strIdArr is %v, len is %d\n", strIdArr, len(strIdArr))

	return strIdArr
}


