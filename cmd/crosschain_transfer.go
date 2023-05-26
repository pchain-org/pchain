package main

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	pabi "github.com/pchain/abi"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"
)

var (
	FromChainIdFlag = cli.StringFlag{
		Name:  "fromchain",
		Usage: "chain where transfer PI out\n",
	}

	ToChainIdFlag = cli.StringFlag{
		Name:  "tochain",
		Usage: "chain where transfer PI in\n",
	}

	ToFlag = cli.StringFlag{
		Name:  "to",
		Usage: "address where to transfer, in tochain\n",
	}

	AmountFlag = cli.IntFlag{
		Name:  "amount",
		Usage: "amount to transfer\n",
	}

	crossChainTransferCommand = cli.Command{
		Name:     "crosschaintransfer",
		Usage:    "tranfer PI from one chain to another",
		Category: "TOOLkIT COMMANDS",
		Action:   utils.MigrateFlags(crosschainTransfer),
		Flags: []cli.Flag{
			utils.RPCListenAddrFlag,
			utils.RPCPortFlag,
			FromChainIdFlag,
			ToChainIdFlag,
			ToFlag,
			AmountFlag,
			ToolkitDirFlag,
			CountPerSecFlag,
			TotalTimeFlag,
		},
		Description: `
transfer PI cross different chains.`,
	}
)

var cctVars struct {
	toolkitDir      string
	privkeyFileName string
	keyStore        *keystore.KeyStore
	account         accounts.Account
	fromAddr        common.Address
	toAddr          common.Address

	mainChainId  string
	mainChainUrl string
	fromChainId  string
	fromChainUrl string
	toChainId    string
	toChainUrl   string
	amount       *big.Int

	cctHash common.Hash

	countPerSec int
	totalTime   int

	sendCCTdone  bool //false
	checkCCTdone bool //false
	stop         chan struct{}
}

const defaultAmount = 1000

type CCTStatisticHolder struct {
	startTime       time.Time
	endTime         time.Time
	cctPeekValue    int
	cctValleyValue  int
	cctPredictCount int
	cctActualCount  int
	cctCheckedCount int

	cctSuceedCount int
	cctFailedCount int

	fromBlcInMCBefore *big.Int //fromBalanceInMainChainBeforeSent
	fromBlcInFCBefore *big.Int //fromBalanceInFromChainBeforeSent
	toBlcInTCBefore   *big.Int //toBalanceInToChainBeforeSent
	fromBlcInMCAfter  *big.Int //fromBalanceInMainChainAfterSent
	fromBlcInFCAfter  *big.Int //fromBalanceInFromChainAfterSent
	toBlcInTCAfter    *big.Int //toBalanceInToChainAfterSent

	consumedBalance *big.Int
}

var cctSH = CCTStatisticHolder{}

func crosschainTransfer(ctx *cli.Context) error {

	cctInitEnv(ctx)

	cctSH.fromBlcInMCBefore, cctSH.fromBlcInFCBefore, cctSH.toBlcInTCBefore = getBalance()

	go sendCCT()
	go checkCCT()
	<-cctVars.stop

	cctSH.fromBlcInMCAfter, cctSH.fromBlcInFCAfter, cctSH.toBlcInTCAfter = getBalance()

	cctPrintStatistic()

	return nil
}

func cctInitEnv(ctx *cli.Context) {

	cctVars.toolkitDir = ctx.GlobalString(ToolkitDirFlag.Name)

	localRpcPrefix := calLocalRpcPrefix(ctx)
	if cctVars.mainChainId == "" {
		cctVars.mainChainId = params.MainnetChainConfig.PChainId
		cctVars.mainChainUrl = localRpcPrefix + cctVars.mainChainId
	}
	cctVars.fromChainId = ctx.GlobalString("fromchain")
	cctVars.fromChainUrl = localRpcPrefix + cctVars.fromChainId
	cctVars.toChainId = ctx.GlobalString("tochain")
	cctVars.toChainUrl = localRpcPrefix + cctVars.toChainId
	amount := ctx.GlobalInt(AmountFlag.Name)
	if amount == 0 {
		amount = defaultAmount
	}
	cctVars.amount = new(big.Int).SetInt64(int64(amount))

	files, err := ioutil.ReadDir(cctVars.toolkitDir)
	if err != nil {
		Exit(err)
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), privFilePrefix) {
			cctVars.privkeyFileName = cctVars.toolkitDir + string(os.PathSeparator) + file.Name()
		}
	}
	if cctVars.privkeyFileName == "" {
		Exit(fmt.Errorf("there is no private key file under %s", cctVars.toolkitDir))
	}
	cctVars.keyStore = keystore.NewKeyStore(cctVars.toolkitDir, keystore.StandardScryptN, keystore.StandardScryptP)
	keyJSON, err := ioutil.ReadFile(cctVars.privkeyFileName)
	if err != nil {
		Exit(err)
	}
	privKey, err := keystore.DecryptKey(keyJSON, password)
	cctVars.account = accounts.Account{Address: privKey.Address, URL: accounts.URL{Scheme: keystore.KeyStoreScheme, Path: cctVars.privkeyFileName}}
	err = cctVars.keyStore.Unlock(cctVars.account, password)
	if err != nil {
		Exit(err)
	}
	cctVars.fromAddr = cctVars.account.Address
	cctVars.toAddr = cctVars.account.Address

	cctVars.countPerSec = ctx.GlobalInt(CountPerSecFlag.Name)
	cctVars.totalTime = ctx.GlobalInt(TotalTimeFlag.Name)

	if cctVars.countPerSec < 0 || cctVars.countPerSec > 1000 {
		Exit(fmt.Errorf("countPerSec should be in [1, 1000]"))
	}

	if cctVars.totalTime < 0 || cctVars.totalTime > 100 {
		Exit(fmt.Errorf("countPerSec should be in [1, 100]"))
	}

	cctVars.stop = make(chan struct{})

	cctSH.consumedBalance = new(big.Int).SetInt64(0)

	return
}

func getBalance() (mainBalance, fromBalance, toBalance *big.Int) {

	fromBalance, err := ethclient.WrpGetBalance(cctVars.fromChainUrl, cctVars.fromAddr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err == nil {
		fmt.Printf("getBalance: address(%x)'s balance is %v in chain %s\n", cctVars.fromAddr, fromBalance, cctVars.fromChainId)
	}

	toBalance, err = ethclient.WrpGetBalance(cctVars.toChainUrl, cctVars.toAddr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err == nil {
		fmt.Printf("getBalance: address(%x)'s balance is %v in chain %s\n", cctVars.toAddr, toBalance, cctVars.toChainId)
	}

	if cctVars.fromChainId == cctVars.mainChainId {
		mainBalance = fromBalance
	} else if cctVars.toChainId == cctVars.mainChainId {
		mainBalance = toBalance
	} else {
		mainBalance, err = ethclient.WrpGetBalance(cctVars.mainChainUrl, cctVars.fromAddr, big.NewInt(int64(rpc.LatestBlockNumber)))
		if err == nil {
			fmt.Printf("getBalance: address(%x)'s balance is %v in chain %s\n", cctVars.fromAddr, mainBalance, cctVars.mainChainId)
		}
	}

	return
}

var cctCheckHHS = newHandleHashSet()

func sendCCT() error {

	startTime := time.Now()
	lastSecond := startTime
	lastCount := 0
	totalCount := 0
	cctSH.startTime = startTime

	nonceCache := uint64(0)
	//within total time
	for !time.Now().After(startTime.Add(time.Second * time.Duration(cctVars.totalTime))) {
		fmt.Printf("send CCT in one second, start at: %v\n", time.Now())
		//within one second
		for !time.Now().After(lastSecond.Add(time.Second * 1)) {
			if lastCount < cctVars.countPerSec {
				hash, err := sendOneCCT(&nonceCache)
				if err == nil {
					cctCheckHHS.addOrUpdate(hash, false)
					nonceCache++
					lastCount++
				}
			} else {
				time.Sleep(time.Millisecond)
			}
		}
		totalCount += lastCount

		if cctSH.cctPeekValue < lastCount {
			cctSH.cctPeekValue = lastCount
		}

		if cctSH.cctValleyValue == 0 || cctSH.cctValleyValue > lastCount {
			cctSH.cctValleyValue = lastCount
		}

		fmt.Printf("send CCT in one second, sent %v txs in one second, total %v txs, end at: %v\n", lastCount, totalCount, time.Now())
		lastSecond = time.Now()
		lastCount = 0
	}

	cctSH.cctPredictCount = cctVars.countPerSec * cctVars.totalTime
	cctSH.cctActualCount = totalCount

	cctVars.sendCCTdone = true
	//cctCheckHHS.addOrUpdate(common.HexToHash("56fcd79581628cdb65eb2f489cd6fdbfa3605d8582eded5ae5cf44405f8b157e"), false)

	return nil
}

func sendOneCCT(nonceCache *uint64) (common.Hash, error) {

	bs, err := pabi.ChainABI.Pack(pabi.CrossChainTransferRequest.String(), cctVars.fromChainId, cctVars.toChainId, cctVars.amount)
	if err != nil {
		log.Errorf("sendOneCCT, pack err: %v", err)
		return common.Hash{}, err
	}

	// tx signer for the main chain
	digest := crypto.Keccak256([]byte(params.MainnetChainConfig.PChainId))
	chainId := new(big.Int).SetBytes(digest[:])

	// gasLimit
	defaultGas := pabi.CrossChainTransferRequest.RequiredGas()

	// gasPrice
	gasPrice, err := ethclient.WrpSuggestGasPrice(cctVars.mainChainUrl)
	if err != nil {
		log.Errorf("sendOneCCT, WrpSuggestGasPrice err: %v", err)
		return common.Hash{}, err
	}

	cctSH.consumedBalance = new(big.Int).Add(cctSH.consumedBalance, new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(defaultGas)))

	nonce := *nonceCache
	if nonce == 0 {
		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err = ethclient.WrpNonceAt(cctVars.mainChainUrl, cctVars.fromAddr, big.NewInt(int64(rpc.PendingBlockNumber)))
		if err != nil {
			log.Errorf("sendOneCCT, WrpNonceAt err: %v", err)
			return common.Hash{}, err
		}
		*nonceCache = nonce
	}

	// tx
	tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, defaultGas, gasPrice, bs)

	// sign the tx
	signedTx, err := cctVars.keyStore.SignTx(cctVars.account, tx, chainId)
	if err != nil {
		log.Errorf("sendOneCCT, SignTx err: %v", err)
		return common.Hash{}, err
	}

	// eth_sendRawTransaction
	err = ethclient.WrpSendTransaction(cctVars.mainChainUrl, signedTx)
	if err != nil {
		log.Errorf("sendOneCCT, WrpSendTransaction err: %v", err)
		return common.Hash{}, err
	}

	hash := signedTx.Hash()
	fmt.Printf("sendOneCCT, returned (nonce, hash): (%v, %x)\n", nonce, hash)

	return hash, nil
}

func checkCCT() error {

	receiveTime := time.Now()
	for !time.Now().After(receiveTime.Add(waitDuration)) {
		hash := common.Hash{}
		if cctVars.sendCCTdone {
			hash = cctCheckHHS.getHashToHandle()
			if common.EmptyHash(hash) {
				cctVars.checkCCTdone = true
				cctSH.endTime = time.Now()
				close(cctVars.stop)
				return nil
			}
		} else {
			hash = cctCheckHHS.getHashToHandle()
		}

		if !common.EmptyHash(hash) {
			err := checkOneCCT(hash)
			if err == nil {
				cctCheckHHS.addOrUpdate(hash, true)
				cctSH.cctCheckedCount++
				receiveTime = time.Now()
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	cctSH.endTime = time.Now()
	close(cctVars.stop)
	return nil
}

func checkOneCCT(hash common.Hash) error {

	_, isPending, err := ethclient.WrpTransactionByHash(cctVars.mainChainUrl, hash)
	if !isPending && err == nil {
		fmt.Printf("checkOneTx: tx %x packaged in block in chain %s\n", hash, cctVars.mainChainUrl)
	} else {
		return errors.New("not executed yet")
	}

	cts, err := ethclient.WrpGetLatestCCTStatusByHash(cctVars.mainChainUrl, hash)
	if err == nil {
		fmt.Printf("checkCCT: tx %x packaged in block in chain %s with (mainBlock, status, toChainOperated): (%v, %v, %v)\n",
			hash, cctVars.mainChainUrl, cts.MainBlockNumber, cts.Status, cts.LastOperationDone)
		if cts.LastOperationDone {
			if cts.Status == types.CCTFAILED {
				cctSH.cctFailedCount++
			} else if cts.Status == types.CCTSUCCEEDED {
				cctSH.cctSuceedCount++
			}
			return nil
		} else {
			return errors.New("not finalized yet")
		}
	} else {
		return err
	}
}

func checkBalance() bool {

	sc := new(big.Int).SetInt64(int64(cctSH.cctSuceedCount))
	txConsumedBalance := cctSH.consumedBalance
	transferAmount := new(big.Int).Mul(sc, new(big.Int).SetInt64(defaultAmount))

	expectedFromBlcInMC := (*big.Int)(nil)
	expectedFromBlcInFC := new(big.Int).Sub(cctSH.fromBlcInFCBefore, transferAmount)
	expectedToBlcInTC := new(big.Int).Add(cctSH.toBlcInTCBefore, transferAmount)

	if cctVars.fromChainId == params.MainnetChainConfig.PChainId {
		expectedFromBlcInFC = new(big.Int).Sub(expectedFromBlcInFC, txConsumedBalance)
		expectedFromBlcInMC = expectedFromBlcInFC
	} else if cctVars.toChainId == params.MainnetChainConfig.PChainId {
		expectedToBlcInTC = new(big.Int).Sub(expectedToBlcInTC, txConsumedBalance)
		expectedFromBlcInMC = expectedToBlcInTC
	} else {
		expectedFromBlcInMC = new(big.Int).Sub(cctSH.fromBlcInMCBefore, txConsumedBalance)
	}

	if expectedFromBlcInMC.Cmp(cctSH.fromBlcInMCAfter) == 0 &&
		expectedFromBlcInFC.Cmp(cctSH.fromBlcInFCAfter) == 0 &&
		expectedToBlcInTC.Cmp(cctSH.toBlcInTCAfter) == 0 {
		return true
	}

	return false
}

func cctPrintStatistic() {
	fmt.Printf("\n\n\n\n\n\n"+
		"this statistic of massive cct test\n"+
		"startTime: %v\n"+
		"endTime:%v\n"+
		"total running time:%v\n"+
		"cct peek value(txs/sec): %v\n"+
		"cct valley value(txs/sec: %v\n"+
		"cct predicted number for sending: %v\n"+
		"cct acctuall number of sent: %v\n"+
		"cct checked number: %v, (succeed, failed) is (%v, %v)\n"+
		"balance before sent(from:%v, to:%v): (%v, %v)\n"+
		"balance after sent(from:%v, to:%v): (%v, %v)\n"+
		"the result is as expected: %v\n",
		cctSH.startTime, cctSH.endTime, cctSH.endTime.Sub(cctSH.startTime).Seconds(),
		cctSH.cctPeekValue, cctSH.cctValleyValue,
		cctSH.cctPredictCount, cctSH.cctActualCount,
		cctSH.cctCheckedCount, cctSH.cctSuceedCount, cctSH.cctFailedCount,
		cctVars.fromChainId, cctVars.toChainId, cctSH.fromBlcInFCBefore, cctSH.toBlcInTCBefore,
		cctVars.fromChainId, cctVars.toChainId, cctSH.fromBlcInFCAfter, cctSH.toBlcInTCAfter,
		checkBalance())

	if count := cctSH.cctActualCount - cctSH.cctSuceedCount - cctSH.cctFailedCount; count > 0 {
		if count > 5 {
			count = 5
		}
		cctCheckHHS.printUnhandedHashes(count)
	}

}
