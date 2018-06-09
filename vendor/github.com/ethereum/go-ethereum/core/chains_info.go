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
)

type ChainInfo struct {
	Owner	common.Address
	ChainId	string

	//joined - during creation phase
	Joined  []common.Address

	//validators - for stable phase; should be Epoch information
	Validators []common.Address

	//the statitics for balance in & out
	//depositInMainChain >= depositInChildChain
	//withdrawFromChildChain >= withdrawFromMainChain
	//depositInMainChain >= withdrawFromChildChain
	DepositInMainChain *big.Int      //total deposit by users from main
	DepositInChildChain *big.Int     //total deposit allocated to users in child chain
	WithdrawFromChildChain *big.Int  //total withdraw by users from child chain
	WithdrawFromMainChain *big.Int   //total withdraw refund to users in main chain
}


const chainInfoKey = "CHAIN"
var allChainKey = []byte("AllChainID")
const specialSep = ";"

var mtx sync.Mutex


func calcChainInfoKey(chainId string) []byte {
	return []byte(chainInfoKey + ":" + chainId)
}

func GetChainInfo(db dbm.DB, chainId string) *ChainInfo {

	ci := &ChainInfo{}
	buf := db.Get(calcChainInfoKey(chainId))
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&ci, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadChainInfo: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}

		fmt.Printf("LoadChainInfo(), chainInfo is: %v\n", ci)
		return ci
	}

	return nil
}

func SaveChainInfo(db dbm.DB, ci *ChainInfo) error{

	mtx.Lock()
	defer mtx.Unlock()
	fmt.Printf("ChainInfo Save(), info is: (%v, %v)\n", ci)

	db.SetSync(calcChainInfoKey(ci.ChainId), ci.Bytes())
	SaveId(db, ci.ChainId)
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

func (ci *ChainInfo) Bytes() []byte {

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	fmt.Printf("(ci *ChainInfo) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	wire.WriteBinary(ci, buf, n, err)
	if *err != nil {
		fmt.Printf("ChainInfo get bytes error: %v", err)
		return nil
	}
	fmt.Printf("(ci *ChainInfo) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	return buf.Bytes()
}

