package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
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

	MainChainUrlFlag = cli.StringFlag{
		Name:  "mainchainurl",
		Usage: "main chain's url to which send the block, default value is http://localhost:6969/pchain\n",
		Value: "http://localhost:6969/pchain",
	}

	sendBlockToMainChainCommand = cli.Command{
		Name:     "sendblocktomainchain",
		Usage:    "send block to main chain",
		Category: "TOOLkIT COMMANDS",
		Action:   utils.MigrateFlags(sendBlockToMainChain),
		Flags: []cli.Flag{
			ToolkitDirFlag,
			MainChainUrlFlag,
		},
		Description: `
When tx3 or Epoch information are not send to main chain in time, main chain
may deny processing of withdraw from child chain.
this command is to help send the block which holds the tx3es or epoch to main
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

	sendTxVars.mainChainUrl = ctx.GlobalString(MainChainUrlFlag.Name)
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
	tx3Count       uint64
	tx3Hashes      []common.Hash
	ended          bool
	err            error
}

func sendOneBlock(block *types.Block, version int) error {

	tx3BeginIndex := uint64(0)
	ended := false
	for !ended {
		ret := makeProofData(block, tx3BeginIndex, version)
		if ret.err != nil {
			return ret.err
		}

		if len(ret.proofDataBytes) != 0 {

			if ret.tx3Count > 0 {
				fmt.Printf("send data to main chain with (blocknumber-tx3BeginIndex-tx3Count): (%v-%v-%v)\n",
					block.NumberU64(), tx3BeginIndex, ret.tx3Count)
				//fmt.Printf("%x \n", ret.tx3Hashes[0])
				//for _,hash := range picked.tx3Hashes {
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

		tx3BeginIndex += ret.tx3Count
		ended = ret.ended
	}
	return nil
}

func makeProofData(block *types.Block, tx3BeginIndex uint64, version int) (mpdRet *MPDRet) {

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

		lastTx3Count := uint64(0)
		lastTx3Hashes := make([]common.Hash, 0)
		lastProofDataBytes := []byte{}
		ended := false
		step := uint64(10)
		tx3Count := step
		for {
			proofData, tx3Hashes, err := consensus.NewChildChainProofDataV1(block, tx3BeginIndex, tx3Count)
			if err == ExceedTxCount {
				mpdRet.ended = true
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
			lastTx3Hashes = tx3Hashes
			lastTx3Count = uint64(len(tx3Hashes))
			if lastTx3Count < tx3Count {
				ended = true
				break
			}

			tx3Count += step //try to contain 10 more tx3
		}

		mpdRet.proofDataBytes = lastProofDataBytes
		mpdRet.tx3Hashes = lastTx3Hashes
		mpdRet.tx3Count = lastTx3Count

		if ended {
			mpdRet.ended = true
		}
	}
	log.Debugf("MakeProofData proof data length: %d， tx3BeginIndex is %v, tx3Count is %v",
		len(proofDataBytes), tx3BeginIndex, mpdRet.tx3Count)

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
