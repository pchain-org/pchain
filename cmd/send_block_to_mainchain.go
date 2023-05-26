package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pdbft/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	csTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	pabi "github.com/pchain/abi"
	"github.com/pchain/chain"
	tmdcrypto "github.com/tendermint/go-crypto"
	"gopkg.in/urfave/cli.v1"

	"context"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"time"
)

var (
	blockFileName         = "block.json"
	privValidatorFileName = "priv_validator.json"

	ToolkitDirFlag = utils.DirectoryFlag{
		Name: "toolkitdir",
		Usage: "directory for one block.json and one priv_validator.json. \n" +
			"block.json file should contains only one block's json data, \n" +
			"priv_validator.json file should contain only one priv_validator's json data, " +
			"and the private validator should be one of the validators of the related epoch, " +
			"to which epoch the block belongs.\n",
	}

	sendBlockToMainChainCommand = cli.Command{
		Name:     "sendblocktomainchain",
		Usage:    "send block to main chain",
		Category: "TOOLkIT COMMANDS",
		Action:   utils.MigrateFlags(sendBlockToMainChain),
		Flags: []cli.Flag{
			utils.RPCListenAddrFlag,
			utils.RPCPortFlag,
			ToolkitDirFlag,
		},
		Description: `
When tx or Epoch information are not send to main chain in time, main chain
may deny processing of withdraw from child chain.
this command is to help send the block which holds the txs or epoch to main
chain.`,
	}
)

var (
	ExceedTxCount = errors.New("exceed the tx count")
)

var sendTxVars struct {
	mainChainUrl string
	account      common.Address
	signer       types.Signer
	prv          *ecdsa.PrivateKey
	ctx          context.Context
	nonceCache   uint64
}

func sendBlockToMainChain(ctx *cli.Context) error {

	toolkitDir := ctx.GlobalString(ToolkitDirFlag.Name)

	//init sendTxVars
	prvValidator := csTypes.LoadPrivValidator(toolkitDir + string(os.PathSeparator) + privValidatorFileName)
	// We use BLS Consensus PrivateKey to sign the digest data
	prv, err := crypto.ToECDSA(prvValidator.PrivKey.(tmdcrypto.BLSPrivKey).Bytes())
	if err != nil {
		Exit(fmt.Errorf("saveDataToMainChain: failed to get PrivateKey, err: %v\n", err))
	}

	localRpcPrefix := calLocalRpcPrefix(ctx)
	sendTxVars.mainChainUrl = localRpcPrefix + params.MainnetChainConfig.PChainId
	sendTxVars.prv = prv
	sendTxVars.account = crypto.PubkeyToAddress(sendTxVars.prv.PublicKey)
	digest := crypto.Keccak256([]byte(chain.MainChain))
	sendTxVars.signer = types.LatestSignerForChainID(new(big.Int).SetBytes(digest[:]))
	sendTxVars.ctx, _ = context.WithTimeout(context.Background(), 3*time.Second)
	sendTxVars.nonceCache = 0

	//load block and send
	block := loadBlock(toolkitDir + string(os.PathSeparator) + blockFileName)
	err = sendOneBlock(block, 1)
	if err != nil {
		Exit(fmt.Errorf("saveDataToMainChain(rpc) failed, err:%v\n", err))
	}

	fmt.Printf("\n")
	fmt.Printf("block %v has been sent to main chain successfully!\n", block.NumberU64())

	return nil
}

func loadBlock(filePath string) *types.Block {

	blockJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err)
	}

	consoleBlock, err := consoleBlockFromJSON(blockJSONBytes)
	if err != nil {
		Exit(fmt.Errorf("Error reading Block from %v: %v\n", filePath, err))
	}

	block := consoleBlockToBlock(consoleBlock)
	if block == nil {
		Exit(fmt.Errorf("Error reading block from %v: %v\n", filePath, err))
	}

	return block
}

func consoleBlockFromJSON(jsonBlob []byte) (*ConsoleBlock, error) {

	consoleBlock := &ConsoleBlock{}
	if err := json.Unmarshal(jsonBlob, &consoleBlock); err != nil {
		return nil, err
	}

	return consoleBlock, nil
}

func Exit(err error) {
	fmt.Println(err)
	os.Exit(1)
}

type MPDRet struct {
	proofDataBytes []byte
	epochNumber    int
	txHashes       []common.Hash
	txCount        uint64
	iterCount      uint64
	ended          bool
	err            error
}

func sendOneBlock(block *types.Block, version int) error {

	txBeginIndex := uint64(0)
	ended := false
	for !ended {
		ret := makeProofData(block, txBeginIndex, version)
		if ret.err != nil {
			return ret.err
		}

		if len(ret.proofDataBytes) != 0 {

			if ret.txCount > 0 {
				fmt.Printf("send data to main chain with (blocknumber-txBeginIndex-txCount): (%v-%v-%v)\n",
					block.NumberU64(), txBeginIndex, ret.txCount)
				//fmt.Printf("%x \n", ret.txHashes[0])
				//for _,hash := range picked.txHashes {
				//	sd.Logger().Infof("%x \n", hash)
				//}
			}

			hash, err := sendDataToMainChain(ret.proofDataBytes, &sendTxVars.nonceCache)
			if err == nil {
				sendTxVars.nonceCache++
			} else {
				sendTxVars.nonceCache = 0
			}

			//we wait for 30 seconds, if not write to main chain, just return
			seconds := 0
			packaged := false
			for seconds < 30 {
				_, isPending, err := ethclient.WrpTransactionByHash(sendTxVars.mainChainUrl, hash)
				if !isPending && err == nil {
					fmt.Printf("tx %x packaged in block in main chain\n", hash)
					packaged = true
					break
				}
				time.Sleep(3 * time.Second)
				seconds += 3
			}

			if !packaged {
				fmt.Println("saveDataToMainChain: tx not packaged within 30 seconds in main chain, stop tracking")
			}
		}

		txBeginIndex += ret.iterCount
		ended = ret.ended
	}
	return nil
}

func makeProofData(block *types.Block, beginIndex uint64, version int) (mpdRet *MPDRet) {

	mpdRet = &MPDRet{ended: true}

	tdmExtra, err := csTypes.ExtractTendermintExtra(block.Header())
	if err != nil {
		return
	}

	if tdmExtra.EpochBytes != nil && len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil {
			mpdRet.epochNumber = int(ep.Number)
		} else {
			mpdRet.epochNumber = -1
		}
	}

	proofDataBytes := []byte{}

	if version == 0 {
		proofData, err := types.NewChildChainProofData(block)
		if err != nil {
			mpdRet.err = err
			log.Error("MakeProofData: failed to create proof data", "block", block, "err", err)
			return
		}
		proofDataBytes, err = rlp.EncodeToBytes(proofData)
		if err != nil {
			mpdRet.err = err
			log.Error("MakeProofData: failed to encode proof data", "proof data", proofData, "err", err)
			return
		}

		mpdRet.proofDataBytes = proofDataBytes
		return
	} else {

		mpdRet.ended = false
		lastTxHashes := make([]common.Hash, 0)
		lastTxCount := uint64(0)
		lastIterCount := uint64(0)
		lastProofDataBytes := []byte{}
		step := uint64(10)
		count := step
		for {
			proofData, hashes, iCount, err := consensus.NewChildChainProofDataV1(block, beginIndex, count)
			if err == ExceedTxCount {
				mpdRet.err = err
				return
			} else if err != nil {
				mpdRet.err = err
				log.Error("MakeProofData: failed to create proof data", "block", block, "err", err)
				return
			}
			proofDataBytes, err = rlp.EncodeToBytes(proofData)
			if err != nil {
				mpdRet.err = err
				log.Error("MakeProofData: failed to encode proof data", "proof data", proofData, "err", err)
				return
			}

			if estimateTxOverSize(proofDataBytes) {
				break
			}

			lastProofDataBytes = proofDataBytes
			lastTxHashes = hashes
			lastTxCount = uint64(len(proofData.TxProofs))
			lastIterCount = iCount

			if iCount < count {
				mpdRet.ended = true
				break
			}

			count += step //try to contain 10 more tx
		}

		mpdRet.proofDataBytes = lastProofDataBytes
		mpdRet.txHashes = lastTxHashes
		mpdRet.txCount = lastTxCount
		mpdRet.iterCount = lastIterCount
	}
	log.Debugf("MakeProofData proof data length: %d， beginIndex is %v, txCount is %v",
		len(proofDataBytes), beginIndex, mpdRet.txCount)

	return
}

func estimateTxOverSize(input []byte) bool {

	tx := types.NewTransaction(uint64(0), pabi.ChainContractMagicAddr, nil, 0, common.Big256, input)

	signedTx, _ := types.SignTx(tx, sendTxVars.signer, sendTxVars.prv)

	return signedTx.Size() >= core.TxMaxSize
}

func sendDataToMainChain(data []byte, nonce *uint64) (common.Hash, error) {

	// data
	bs, err := pabi.ChainABI.Pack(pabi.SaveDataToMainChain.String(), data)
	if err != nil {
		log.Errorf("SendDataToMainChain, pack err: %v", err)
		return common.Hash{}, err
	}

	var hash = common.Hash{}
	//should send successfully, let's wait longer time
	err = retry(30, time.Second*3, func() error {
		// gasPrice
		gasPrice, err := ethclient.WrpSuggestGasPrice(sendTxVars.mainChainUrl)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpSuggestGasPrice err: %v", err)
			return err
		}

		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err := ethclient.WrpNonceAt(sendTxVars.mainChainUrl, sendTxVars.account, new(big.Int).SetInt64(int64(rpc.PendingBlockNumber)))
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpNonceAt err: %v", err)
			return err
		}

		// tx
		tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, 0, gasPrice, bs)

		// sign the tx
		signedTx, err := types.SignTx(tx, sendTxVars.signer, sendTxVars.prv)
		if err != nil {
			log.Errorf("SendDataToMainChain, SignTx err: %v", err)
			return err
		}

		// eth_sendRawTransaction
		err = ethclient.WrpSendTransaction(sendTxVars.mainChainUrl, signedTx)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpSendTransaction err: %v", err)
			return err
		}

		hash = signedTx.Hash()
		return nil
	})

	if err != nil {
		log.Errorf("SendDataToMainChain, 30 times of failure, last err: %v", err)
	} else {
		fmt.Printf("send data to main chain, succeeded with hash: %x\n", hash)
	}

	return hash, err
}

func calLocalRpcPrefix(ctx *cli.Context) string {
	localRpcPrefix := "http://" + ctx.GlobalString(utils.RPCListenAddrFlag.Name) + ":"
	localRpcPrefix += fmt.Sprintf("%v", ctx.GlobalInt(utils.RPCPortFlag.Name)) + "/"
	return localRpcPrefix
}

//attemps: this parameter means the total amount of operations
func retry(attemps int, sleep time.Duration, fn func() error) error {

	for attemps > 0 {

		err := fn()
		if err == nil {
			return nil
		}

		attemps--
		if attemps == 0 {
			return err
		}

		// Add some randomness to prevent creating a Thundering Herd
		jitter := time.Duration(rand.Int63n(int64(sleep)))
		sleep = sleep + jitter/2

		time.Sleep(sleep)
	}

	return nil
}
