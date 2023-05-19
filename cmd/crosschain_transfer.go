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

	mainChainUrl string
	fromChainId  string
	fromChainUrl string
	toChainId    string
	toChainUrl   string
	amount       *big.Int
	cctHash      common.Hash
}

const defaultAmount = 1000

func crosschainTransfer(ctx *cli.Context) error {

	cctInitEnv(ctx)

	sendAndCheckCCT()

	cctPrintStatistic()

	return nil
}

func cctInitEnv(ctx *cli.Context) {

	cctVars.toolkitDir = ctx.GlobalString(ToolkitDirFlag.Name)

	localRpcPrefix := calLocalRpcPrefix(ctx)
	if cctVars.mainChainUrl == "" {
		cctVars.mainChainUrl = localRpcPrefix + params.MainnetChainConfig.PChainId
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
	return
}

func sendAndCheckCCT() {

	getBalance()

	nonce := uint64(0)
	hash, err := sendOneCCT(&nonce)
	if err != nil {
		return
	}
	cctVars.cctHash = hash
	checkCCT(cctVars.cctHash)

	getBalance()
}

func getBalance() {

	addr := cctVars.account.Address
	balance, err := ethclient.WrpGetBalance(cctVars.fromChainUrl, addr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err == nil {
		fmt.Printf("getBalance: address(%x)'s balance is %v in chain %s\n", addr, balance, cctVars.fromChainUrl)
	}

	balance, err = ethclient.WrpGetBalance(cctVars.toChainUrl, addr, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err == nil {
		fmt.Printf("getBalance: address(%x)'s balance is %v in chain %s\n", addr, balance, cctVars.toChainUrl)
	}
}

func sendOneCCT(nonceCache *uint64) (common.Hash, error) {

	bs, err := pabi.ChainABI.Pack(pabi.CrossChainTransferRequest.String(), cctVars.fromChainId, cctVars.toChainId, cctVars.amount)
	if err != nil {
		log.Errorf("sendOneTx3, pack err: %v", err)
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
		log.Errorf("sendOneTx3, WrpSuggestGasPrice err: %v", err)
		return common.Hash{}, err
	}

	nonce := *nonceCache
	if nonce == 0 {
		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err = ethclient.WrpNonceAt(cctVars.mainChainUrl, cctVars.account.Address, big.NewInt(int64(rpc.PendingBlockNumber)))
		if err != nil {
			log.Errorf("sendOneTx3, WrpNonceAt err: %v", err)
			return common.Hash{}, err
		}
		*nonceCache = nonce
	}

	// tx
	tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, defaultGas, gasPrice, bs)

	// sign the tx
	signedTx, err := cctVars.keyStore.SignTx(cctVars.account, tx, chainId)
	if err != nil {
		log.Errorf("sendOneTx3, SignTx err: %v", err)
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

func checkCCT(hash common.Hash) error {

	times := 0
	for ; times < 5; times++ {
		_, isPending, err := ethclient.WrpTransactionByHash(cctVars.mainChainUrl, hash)
		if !isPending && err == nil {
			fmt.Printf("checkOneTx: tx %x packaged in block in chain %s\n", hash, cctVars.mainChainUrl)
			break
		}
		time.Sleep(time.Second * 3)
	}
	if times >= 5 {
		return errors.New("not executed, abort check work")
	}

	for times = 0; times < 11; times++ {
		cts, err := ethclient.WrpGetLatestCCTStatusByHash(cctVars.mainChainUrl, hash)
		if err == nil {
			fmt.Printf("checkCCT: tx %x packaged in block in chain %s with (mainBlock, status, toChainOperated): (%v, %v, %v)\n",
				hash, cctVars.mainChainUrl, cts.MainBlockNumber, cts.Status, cts.ToChainOperated)
			if cts.Status == types.CCTFAILED || (cts.Status == types.CCTSUCCEEDED && cts.ToChainOperated) {
				return nil
			}
		}
		time.Sleep(time.Second * 3)
	}

	return errors.New("not executed yet")
}

func cctPrintStatistic() {
	return
}
