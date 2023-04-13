package main

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	pabi "github.com/pchain/abi"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	CountPerSecFlag = cli.IntFlag{
		Name:  "countpersec",
		Usage: "the number of tx3 per-second\n",
	}

	TotalTimeFlag = cli.IntFlag{
		Name:  "totaltime",
		Usage: "the total time(seconds) to send tx3\n",
	}

	massWithdrawCommand = cli.Command{
		Name:     "masswithdraw",
		Usage:    "withdraw from child chain to main chain",
		Category: "TOOLkIT COMMANDS",
		Action:   utils.MigrateFlags(massWithdraw),
		Flags: []cli.Flag{
			ToolkitDirFlag,
			MainChainUrlFlag,
			ChainUrlFlag,
			CountPerSecFlag,
			TotalTimeFlag,
		},
		Description: `
Send tx3/tx4 in bunch to perform stress test.`,
	}
)

//constants
const privFilePrefix = "UTC--"
const password = "pchain"
const maxHandleTimes = 5

var withdrawAmount = big.NewInt(int64(100))

//variables
var (
	//enviorment variables
	toolkitDir      string
	mainChainUrl    string
	mainChainId     string
	childChainUrl   string
	childChainId    string
	privkeyFileName string
	keyStore        *keystore.KeyStore
	account         accounts.Account
	countPerSec     = 10
	totalTime       = 10

	//state control variables
	sendTx3done  = false
	checkTx3done = false
	sendTx4done  = false
	checkTx4done = false
	stop         = make(chan struct{}) // Channel wait to stop

)

type StatisticHolder struct {
	startTime       time.Time
	endTime         time.Time
	tx3PeekValue    int
	tx3ValleyValue  int
	tx3PredictCount int
	tx3ActualCount  int
	tx3CheckedCount int
	tx4SentCount    int
	tx4CheckedCount int
}

var statisticHolder = StatisticHolder{}

func massWithdraw(ctx *cli.Context) error {

	initEnv(ctx)

	/*
		tx3hash, err := sendOneTx3()
		if err != nil {
			Exit(err)
		}

		tx3hash := common.HexToHash("6d3032699cfb05be52ecee80b38096a43264630d53b6417cea0144969c6ac345")
		err := checkOneTx(tx3hash, childChainUrl)
		if err != nil {
			Exit(err)
		}

		tx4hash, err := sendOneTx4(tx3hash)
		if err != nil {
			Exit(err)
		}

		tx4hash := common.HexToHash("0f3900ab318f32b380e500edd0e41adb9482daf05692d400dec7e5daa20b0d4d")
		err := checkOneTx(tx4hash, mainChainUrl)
		if err != nil {
			Exit(err)
		}
	*/
	go sendTx3()
	go checkTx3()
	go sendTx4()
	go checkTx4()

	<-stop

	printStatic()

	return nil
}

func initEnv(ctx *cli.Context) {

	toolkitDir = ctx.GlobalString(ToolkitDirFlag.Name)
	mainChainUrl = ctx.GlobalString(MainChainUrlFlag.Name)
	mainChainId = mainChainUrl[strings.LastIndex(mainChainUrl, "/")+1:]
	childChainUrl = ctx.GlobalString(ChainUrlFlag.Name)
	childChainId = childChainUrl[strings.LastIndex(childChainUrl, "/")+1:]
	countPerSec = ctx.GlobalInt(CountPerSecFlag.Name)
	totalTime = ctx.GlobalInt(TotalTimeFlag.Name)

	if countPerSec < 0 || countPerSec > 1000 {
		Exit(fmt.Errorf("countPerSec should be in [1, 1000]"))
	}

	if totalTime < 0 || totalTime > 10 {
		Exit(fmt.Errorf("countPerSec should be in [1, 10]"))
	}

	err := error(nil)
	files, err := ioutil.ReadDir(toolkitDir)
	if err != nil {
		Exit(err)
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), privFilePrefix) {
			privkeyFileName = toolkitDir + string(os.PathSeparator) + file.Name()
		}
	}
	if privkeyFileName == "" {
		Exit(fmt.Errorf("there is no private key file under %s", toolkitDir))
	}
	keyStore = keystore.NewKeyStore(toolkitDir, keystore.StandardScryptN, keystore.StandardScryptP)
	keyJSON, err := ioutil.ReadFile(privkeyFileName)
	if err != nil {
		Exit(err)
	}
	privKey, err := keystore.DecryptKey(keyJSON, password)
	account = accounts.Account{Address: privKey.Address, URL: accounts.URL{Scheme: keystore.KeyStoreScheme, Path: privkeyFileName}}
	err = keyStore.Unlock(account, password)
	if err != nil {
		Exit(err)
	}
}

type HandleHashSet struct {
	hashHandledMap map[common.Hash]bool
	mu             sync.Mutex
	needHandle     bool
	lastGotHash    map[common.Hash]int
	gotMinTimes    int
	gotMaxTimes    int
}

func (ha *HandleHashSet) addOrUpdate(hash common.Hash, handled bool) {
	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !handled {
		ha.needHandle = true
	}

	ha.hashHandledMap[hash] = handled
}

func (ha *HandleHashSet) getHashToHandle() common.Hash {
	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !ha.needHandle {
		return common.Hash{}
	}

	needHandle := false
	firstFoundHash := common.Hash{}
	firstFoundGotTimes := 0
	for hash, handled := range ha.hashHandledMap {
		if !handled {
			needHandle = true

			if common.EmptyHash(firstFoundHash) {
				firstFoundHash = hash
				firstFoundGotTimes = ha.lastGotHash[firstFoundHash]
				continue
			}

			lastGotNumber := ha.lastGotHash[hash]
			if lastGotNumber == 0 || lastGotNumber < firstFoundGotTimes {
				lastGotNumber++
				if lastGotNumber > ha.gotMaxTimes {
					ha.gotMaxTimes = lastGotNumber
				}
				ha.lastGotHash[hash] = lastGotNumber
				return hash
			}
		}
	}

	if !common.EmptyHash(firstFoundHash) {
		firstFoundGotTimes++
		if firstFoundGotTimes > ha.gotMaxTimes {
			ha.gotMaxTimes = firstFoundGotTimes
		}
		ha.lastGotHash[firstFoundHash] = firstFoundGotTimes
	}

	/*
		if ha.gotMaxTimes > maxHandleTimes {
			ha.needHandle = false
			return common.Hash{}
		}
	*/

	ha.needHandle = needHandle
	return firstFoundHash
}

func newHandleHashSet() *HandleHashSet {
	return &HandleHashSet{
		hashHandledMap: make(map[common.Hash]bool),
		lastGotHash:    make(map[common.Hash]int),
	}
}

var tx3CheckHHS = newHandleHashSet()
var tx4SendHHS = newHandleHashSet()
var tx4CheckHHS = newHandleHashSet()

func sendTx3() error {

	startTime := time.Now()
	lastSecond := startTime
	lastCount := 0
	totalCount := 0
	statisticHolder.startTime = startTime

	nonceCache := uint64(0)
	//within total time
	for !time.Now().After(startTime.Add(time.Second * time.Duration(totalTime))) {
		fmt.Printf("send tx3 in one second, start at: %v\n", time.Now())
		//within one second
		for !time.Now().After(lastSecond.Add(time.Second * 1)) {
			if lastCount < countPerSec {
				hash, err := sendOneTx3(&nonceCache)
				if err == nil {
					tx3CheckHHS.addOrUpdate(hash, false)
					nonceCache++
					lastCount++
				}
			} else {
				time.Sleep(time.Millisecond)
			}
		}
		totalCount += lastCount

		if statisticHolder.tx3PeekValue < lastCount {
			statisticHolder.tx3PeekValue = lastCount
		}

		if statisticHolder.tx3ValleyValue == 0 || statisticHolder.tx3ValleyValue > lastCount {
			statisticHolder.tx3ValleyValue = lastCount
		}

		fmt.Printf("send tx3 in one second, sent %v txs in one second, total %v txs, end at: %v\n", lastCount, totalCount, time.Now())
		lastSecond = time.Now()
		lastCount = 0
	}

	statisticHolder.tx3PredictCount = countPerSec * totalTime
	statisticHolder.tx3ActualCount = totalCount

	sendTx3done = true
	//tx3CheckHHS.addOrUpdate(common.HexToHash("56fcd79581628cdb65eb2f489cd6fdbfa3605d8582eded5ae5cf44405f8b157e"), false)

	return nil
}

func sendOneTx3(nonceCache *uint64) (common.Hash, error) {

	bs, err := pabi.ChainABI.Pack(pabi.WithdrawFromChildChain.String(), childChainId)
	if err != nil {
		log.Errorf("sendOneTx3, pack err: %v", err)
		return common.Hash{}, err
	}

	// tx signer for the main chain
	digest := crypto.Keccak256([]byte(childChainId))
	chainId := new(big.Int).SetBytes(digest[:])

	// gasLimit
	defaultGas := pabi.WithdrawFromChildChain.RequiredGas()

	// gasPrice
	gasPrice, err := ethclient.WrpSuggestGasPrice(childChainUrl)
	if err != nil {
		log.Errorf("sendOneTx3, WrpSuggestGasPrice err: %v", err)
		return common.Hash{}, err
	}

	nonce := *nonceCache
	if nonce == 0 {
		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err = ethclient.WrpNonceAt(childChainUrl, account.Address, big.NewInt(int64(rpc.PendingBlockNumber)))
		if err != nil {
			log.Errorf("sendOneTx3, WrpNonceAt err: %v", err)
			return common.Hash{}, err
		}
		*nonceCache = nonce
	}

	// tx
	tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, withdrawAmount, defaultGas, gasPrice, bs)

	// sign the tx
	signedTx, err := keyStore.SignTx(account, tx, chainId)
	if err != nil {
		log.Errorf("sendOneTx3, SignTx err: %v", err)
		return common.Hash{}, err
	}

	// eth_sendRawTransaction
	err = ethclient.WrpSendTransaction(childChainUrl, signedTx)
	if err != nil {
		log.Errorf("sendOneTx3, WrpSendTransaction err: %v", err)
		return common.Hash{}, err
	}

	hash := signedTx.Hash()
	fmt.Printf("sendOneTx3, returned (nonce, hash): (%v, %x)\n", nonce, hash)

	return hash, nil
}

func checkTx3() error {

	receiveTime := time.Now()
	for !time.Now().After(receiveTime.Add(time.Second * 300)) {
		hash := common.Hash{}
		if sendTx3done {
			hash = tx3CheckHHS.getHashToHandle()
			if common.EmptyHash(hash) {
				checkTx3done = true
				return nil
			}
		} else {
			hash = tx3CheckHHS.getHashToHandle()
		}

		if !common.EmptyHash(hash) {
			err := checkOneTx(hash, childChainUrl)
			if err == nil {
				tx3CheckHHS.addOrUpdate(hash, true)
				tx4SendHHS.addOrUpdate(hash, false)
				statisticHolder.tx3CheckedCount++
				receiveTime = time.Now()
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	checkTx3done = true
	return nil
}

func checkOneTx(hash common.Hash, url string) error {

	_, isPending, err := ethclient.WrpTransactionByHash(url, hash)
	if !isPending && err == nil {
		fmt.Printf("checkOneTx: tx %x packaged in block in chain %s\n", hash, url)
		return nil
	}

	return errors.New("not executed yet")
}

func sendTx4() error {

	receiveTime := time.Now()
	nonceCache := uint64(0)
	for !time.Now().After(receiveTime.Add(time.Second * 300)) {
		tx3hash := common.Hash{}
		if checkTx3done {
			tx3hash = tx4SendHHS.getHashToHandle()
			if common.EmptyHash(tx3hash) {
				sendTx4done = true
				return nil
			}
		} else {
			tx3hash = tx4SendHHS.getHashToHandle()
		}
		if !common.EmptyHash(tx3hash) {
			hash, err := sendOneTx4(tx3hash, &nonceCache)
			if err == nil {
				tx4SendHHS.addOrUpdate(tx3hash, true)
				tx4CheckHHS.addOrUpdate(hash, false)
				statisticHolder.tx4SentCount++
				nonceCache++
				receiveTime = time.Now()
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	sendTx4done = true
	//tx4CheckHHS.addOrUpdate(common.HexToHash("0xc2ee2b0ffb2fc8a937d3c7f81e96395399ddfc0a3ad74f2802734523f1739af1"), false)

	return nil
}

func sendOneTx4(tx3hash common.Hash, nonceCache *uint64) (common.Hash, error) {

	bs, err := pabi.ChainABI.Pack(pabi.WithdrawFromMainChain.String(), childChainId, withdrawAmount, tx3hash)
	if err != nil {
		log.Errorf("sendOneTx4, pack err: %v", err)
		return common.Hash{}, err
	}

	// tx signer for the main chain
	digest := crypto.Keccak256([]byte(mainChainId))
	chainId := new(big.Int).SetBytes(digest[:])

	// gasLimit
	defaultGas := pabi.WithdrawFromMainChain.RequiredGas()

	nonce := *nonceCache
	// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
	if nonce == 0 {
		nonce, err = ethclient.WrpNonceAt(mainChainUrl, account.Address, big.NewInt(int64(rpc.PendingBlockNumber)))
		if err != nil {
			log.Errorf("sendOneTx4, WrpNonceAt err: %v", err)
			return common.Hash{}, err
		}
		*nonceCache = nonce
	}

	signedTx := (*types.Transaction)(nil)
	gasPrice := common.Big0
	i := int64(0)
	for ; i < 5; i++ {
		if i == 0 { // gasPrice
			gasPrice, err = ethclient.WrpSuggestGasPrice(mainChainUrl)
			if err != nil {
				log.Errorf("sendOneTx4, WrpSuggestGasPrice err: %v", err)
				return common.Hash{}, err
			}
		} else {
			gasPrice = gasPrice.Mul(gasPrice, new(big.Int).SetInt64(i+2))
		}

		// tx
		tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, withdrawAmount, defaultGas, gasPrice, bs)

		// sign the tx
		signedTx, err = keyStore.SignTx(account, tx, chainId)
		if err != nil {
			log.Errorf("sendOneTx4, SignTx err: %v", err)
			return common.Hash{}, err
		}

		// eth_sendRawTransaction
		err = ethclient.WrpSendTransaction(mainChainUrl, signedTx)
		if err != nil {
			if err.Error() == core.ErrReplaceUnderpriced.Error() {
				continue
			} else {
				log.Errorf("sendOneTx4, WrpSendTransaction err: %v", err)
				return common.Hash{}, err
			}
		} else {
			break
		}
	}

	if err != nil {
		log.Errorf("sendOneTx4, WrpSendTransaction err: %v", err)
	}

	hash := signedTx.Hash()
	fmt.Printf("sendOneTx4(tx3hash: %x), returned (i, nonce, hash): (%v, %v, %x)\n", tx3hash, i, nonce, hash)

	return hash, nil
}

func checkTx4() error {

	receiveTime := time.Now()
	for !time.Now().After(receiveTime.Add(time.Second * 300)) {
		hash := common.Hash{}
		if sendTx4done {
			hash = tx4CheckHHS.getHashToHandle()
			if common.EmptyHash(hash) {
				statisticHolder.endTime = time.Now()
				close(stop)
				return nil
			}
		} else {
			hash = tx4CheckHHS.getHashToHandle()
		}
		if !common.EmptyHash(hash) {
			err := checkOneTx(hash, mainChainUrl)
			if err == nil {
				tx4CheckHHS.addOrUpdate(hash, true)
				statisticHolder.tx4CheckedCount++
				receiveTime = time.Now()
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	statisticHolder.endTime = time.Now()
	close(stop)

	return nil
}

func printStatic() {
	fmt.Printf("\n\n\n\n\n\n"+
		"this statistic of mass tx3/tx4 test\n"+
		"startTime: %v\n"+
		"endTime:%v\n"+
		"total running time:%v\n"+
		"tx3 peek value(txs/sec): %v\n"+
		"tx3 valley value(txs/sec: %v\n"+
		"tx3 predicted number for sending: %v\n"+
		"tx3 acctuall number of sent: %v\n"+
		"tx3 succeeded number: %v\n"+
		"tx4 number of sent: %v\n"+
		"tx4 succeeded number: %v\n",
		statisticHolder.startTime, statisticHolder.endTime, statisticHolder.endTime.Sub(statisticHolder.startTime).Seconds(),
		statisticHolder.tx3PeekValue, statisticHolder.tx3ValleyValue,
		statisticHolder.tx3PredictCount, statisticHolder.tx3ActualCount,
		statisticHolder.tx3CheckedCount,
		statisticHolder.tx4SentCount, statisticHolder.tx4CheckedCount)
}
