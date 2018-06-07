package chain

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

type SimpleBalance struct {
	//when deposit, hash is from main chain;
	//when withdraw, hash is from child chain
	Txhash		common.Hash
	Balance		*big.Int
	LockedBalance   *big.Int
}

type ChainInfo struct {
	owner	common.Address
	chainId	string

	//for creation
	joined  []common.Address

	//for ledger check, only change when there are PAI flew between main chain and this child chain
	//not consistent with the realtime chain accounts
	deposit map[common.Address]*SimpleBalance
	withdraw map[common.Address]*SimpleBalance

	//totalDeposit should always be greater or equal than totalWithdraw
	totalDeposit *big.Int
	totalWithdraw *big.Int
}


const chainInfoKey = "CHAIN"
var allChainKey = []byte("AllChainID")
const specialSep = ";"

var mtx sync.Mutex


func calcChainInfoKey(chainId string) []byte {
	return []byte(chainInfoKey + ":" + chainId)
}

func GetChainInfo(db dbm.DB, chainId string) *ChainInfo {

	ac := &adaptChainInfo{}
	buf := db.Get(calcChainInfoKey(chainId))
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&ac, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadChainInfo: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}

		ci := toChainInfo(ac)
		fmt.Printf("LoadChainInfo(), chainInfo is: %v\n", ci)
		return ci
	}

	return nil
}

func SaveChainInfo(db dbm.DB, ci *ChainInfo) error{

	mtx.Lock()
	defer mtx.Unlock()
	fmt.Printf("ChainInfo Save(), info is: (%v, %v)\n", ci)

	db.SetSync(calcChainInfoKey(ci.chainId), ci.Bytes())
	SaveId(db, ci.chainId)
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

	ac := fromChainInfo(ci)

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	fmt.Printf("(ci *ChainInfo) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	wire.WriteBinary(ac, buf, n, err)
	if *err != nil {
		fmt.Printf("ChainInfo get bytes error: %v", err)
		return nil
	}
	fmt.Printf("(ci *ChainInfo) Bytes(), (buf, n) are: (%v,%v)\n", buf.Bytes(), *n)
	return buf.Bytes()
}


/*
Here are the adapted types to make wire.XXX() works; because go-wire not support 'map'
 */
type adaptSimpleBalance struct {
	addr common.Address
	sb *SimpleBalance
}

type adaptChainInfo struct {
	owner	common.Address
	chainId	string

	//for creation
	joined  []common.Address

	//for ledger
	accounts []*adaptSimpleBalance
}

func fromChainInfo(ci *ChainInfo) *adaptChainInfo{

	if ci == nil { return nil }

	ac := &adaptChainInfo{
		owner: ci.owner,
		chainId: ci.chainId,
		joined: ci.joined,
	}

	ac.accounts = make([]*adaptSimpleBalance, len(ci.accounts))
	for addr, sb := range ci.accounts {
		ac.accounts = append(ac.accounts,
					&adaptSimpleBalance{
						addr: addr,
						sb: sb,
					})
	}

	return ac
}

func toChainInfo(ac *adaptChainInfo) *ChainInfo {
	if ac == nil {return nil}

	ci := &ChainInfo {
		owner: ac.owner,
		chainId: ac.chainId,
		joined: ac.joined,
	}

	ci.accounts = make(map[common.Address]*SimpleBalance, len(ac.accounts))
	for _, account := range ac.accounts {
		ci.accounts[account.addr] = account.sb
	}

	return ci
}
