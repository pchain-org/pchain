package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pdbft/consensus"
	csTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	pabi "github.com/pchain/abi"
	"github.com/pchain/chain"
	tmdcrypto "github.com/tendermint/go-crypto"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	blockFileName         = "block.json"
	privValidatorFileName = "priv_validator.json"

	SendDataDirFlag = utils.DirectoryFlag{
		Name: "senddatadir",
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
		Name:     "sendBlockToMainChain",
		Usage:    "send block to main chain",
		Category: "TOOL COMMANDS",
		Action:   utils.MigrateFlags(sendBlockToMainChain),
		Flags: []cli.Flag{
			SendDataDirFlag,
			MainChainUrlFlag,
		},
		Description: `
When tx3 or Epoch information are not send to main chain in time, main chain
may deny processing of withdraw from child chain.
this command is to help send the block which holds the tx3es or epoch to main
chain.`,
	}
)

func sendBlockToMainChain(ctx *cli.Context) error {

	sendDataDir := ctx.GlobalString(SendDataDirFlag.Name)
	mainChainUrl := ctx.GlobalString(MainChainUrlFlag.Name)

	block := loadBlock(sendDataDir + string(os.PathSeparator) + blockFileName)
	prvValidator := csTypes.LoadPrivValidator(sendDataDir + string(os.PathSeparator) + privValidatorFileName)

	proofData, err := consensus.NewChildChainProofDataV1(block)
	if err != nil {
		Exit(fmt.Sprintf("sendBlockToMainChain: failed to create proof data, block: %v, err: %v\n", block, err))
	}

	pdBytes, err := rlp.EncodeToBytes(proofData)
	if err != nil {
		Exit(fmt.Sprintf("saveDataToMainChain: failed to encode proof data, proof data: %v, err: %v\n", proofData, err))
	}
	fmt.Printf("saveDataToMainChain proof data length: %d\n", len(pdBytes))

	// We use BLS Consensus PrivateKey to sign the digest data
	prv, err := crypto.ToECDSA(prvValidator.PrivKey.(tmdcrypto.BLSPrivKey).Bytes())
	if err != nil {
		Exit(fmt.Sprintf("saveDataToMainChain: failed to get PrivateKey, err: %v\n", err))
	}

	hash, err := sendDataToMainChain(mainChainUrl, pdBytes, prv, chain.MainChain)
	if err != nil {
		Exit(fmt.Sprintf("saveDataToMainChain(rpc) failed, err:%v\n", err))
	} else {
		Exit(fmt.Sprintf("saveDataToMainChain(rpc) success, hash:%v\n", hash))
	}

	//we wait for 15 seconds, if not write to main chain, just return
	seconds := 0
	for seconds < 15 {

		_, isPending, err := ethclient.WrpTransactionByHash(mainChainUrl, hash)
		if !isPending && err == nil {
			fmt.Println("saveDataToMainChain: tx packaged in block in main chain")
			return nil
		}

		time.Sleep(3 * time.Second)
		seconds += 3
	}

	fmt.Println("saveDataToMainChain: tx not packaged within 15 seconds in main chain, stop tracking")

	return nil
}

func loadBlock(filePath string) *types.Block {

	blockJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}

	blockDoc, err := BlockDocFromJSON(blockJSONBytes)
	if err != nil {
		Exit(fmt.Sprintf("Error reading Block from %v: %v\n", filePath, err))
	}

	block := blockDocToBlock(blockDoc)
	if block == nil {
		Exit(fmt.Sprintf("Error reading block from %v: %v\n", filePath, err))
	}

	return block
}

func BlockDocFromJSON(jsonBlob []byte) (*BlockDoc, error) {

	blockDoc := &BlockDoc{}
	if err := json.Unmarshal(jsonBlob, &blockDoc); err != nil {
		return nil, err
	}

	return blockDoc, nil
}

type TransactionDoc struct {
	TransactionIndex uint64          `json:"transactionIndex"`
	Nonce            uint64          `json:"nonce"`
	GasPrice         *big.Int        `json:"gasPrice"`
	Gas              uint64          `json:"gas"`
	To               *common.Address `json:"to"`
	Value            *big.Int        `json:"value"`
	Data             []byte          `json:"input"`
	V                *big.Int        `json:"v"`
	R                *big.Int        `json:"r"`
	S                *big.Int        `json:"s"`
}

func (td *TransactionDoc) UnmarshalJSON(input []byte) error {
	type hexTransactionDoc struct {
		TransactionIndex uint64          `json:"transactionIndex"`
		Nonce            uint64          `json:"nonce"`
		GasPrice         *big.Int        `json:"gasPrice"`
		Gas              *big.Int        `json:"gas"`
		To               *common.Address `json:"to"`
		Value            *big.Int        `json:"value"`
		Data             string          `json:"input"`
		V                string          `json:"v"` //hexutil.Big
		R                string          `json:"r"` //hexutil.Big
		S                string          `json:"s"` //hexutil.Big
	}

	var dec hexTransactionDoc
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	td.TransactionIndex = dec.TransactionIndex
	td.Nonce = dec.Nonce
	td.GasPrice = dec.GasPrice
	td.Gas = dec.Gas.Uint64()
	td.To = dec.To
	td.Value = dec.Value
	td.Data = common.FromHex(dec.Data)
	td.V, _ = hexStringToBig(dec.V, 16)
	td.R, _ = hexStringToBig(dec.R, 16)
	td.S, _ = hexStringToBig(dec.S, 16)

	return nil
}

func hexStringToBig(str string, base int) (*big.Int, bool) {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	return new(big.Int).SetString(str, base)
}

type BlockDoc struct {
	ParentHash  common.Hash      `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash      `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address   `json:"miner"            gencodec:"required"`
	Root        common.Hash      `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash      `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash      `json:"receiptsRoot"     gencodec:"required"`
	Bloom       string           `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int         `json:"difficulty"       gencodec:"required"`
	Number      *big.Int         `json:"number"           gencodec:"required"`
	GasLimit    uint64           `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64           `json:"gasUsed"          gencodec:"required"`
	Time        *big.Int         `json:"timestamp"        gencodec:"required"`
	Extra       string           `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash      `json:"mixHash"`
	Nonce       types.BlockNonce `json:"nonce"`

	// For Child Chain only
	MainChainNumber *big.Int `json:"mainchainNumber"      gencodec:"required"`

	Transactions []TransactionDoc `json:"transactions"`
}

func blockDocToBlock(blockDoc *BlockDoc) *types.Block {

	//fill headers
	header := &types.Header{}
	header.ParentHash = blockDoc.ParentHash
	header.UncleHash = blockDoc.UncleHash
	header.Coinbase = blockDoc.Coinbase
	header.Root = blockDoc.Root
	header.TxHash = blockDoc.TxHash
	header.ReceiptHash = blockDoc.ReceiptHash
	header.Bloom = types.BytesToBloom(common.FromHex(blockDoc.Bloom))
	header.Difficulty = blockDoc.Difficulty
	header.Number = blockDoc.Number
	header.GasLimit = blockDoc.GasLimit
	header.GasUsed = blockDoc.GasUsed
	header.Time = blockDoc.Time
	header.Extra = common.FromHex(blockDoc.Extra)
	header.MixDigest = blockDoc.MixDigest
	header.Nonce = blockDoc.Nonce
	header.MainChainNumber = blockDoc.MainChainNumber

	//fill txs
	txs := make([]*types.Transaction, len(blockDoc.Transactions))
	for _, txDoc := range blockDoc.Transactions {
		tx := &types.LegacyTx{
			Nonce:    txDoc.Nonce,
			To:       txDoc.To,
			Value:    txDoc.Value,
			Gas:      txDoc.Gas,
			GasPrice: txDoc.GasPrice,
			Data:     txDoc.Data,
			V:        txDoc.V,
			R:        txDoc.R,
			S:        txDoc.S,
		}
		txs[txDoc.TransactionIndex] = types.NewTx(tx)
	}

	return types.NewBlock(header, txs, nil, nil, new(trie.Trie))
}

func Exit(err string) {
	fmt.Println(err)
	os.Exit(1)
}

func sendDataToMainChain(chainUrl string, data []byte, prv *ecdsa.PrivateKey, mainChainId string) (common.Hash, error) {

	// data
	bs, err := pabi.ChainABI.Pack(pabi.SaveDataToMainChain.String(), data)
	if err != nil {
		log.Errorf("SendDataToMainChain, pack err: %v", err)
		return common.Hash{}, err
	}

	account := crypto.PubkeyToAddress(prv.PublicKey)

	// tx signer for the main chain
	digest := crypto.Keccak256([]byte(mainChainId))
	signer := types.LatestSignerForChainID(new(big.Int).SetBytes(digest[:]))

	var hash = common.Hash{}
	//should send successfully, let's wait longer time
	err = retry(30, time.Second*3, func() error {
		// gasPrice
		gasPrice, err := ethclient.WrpSuggestGasPrice(chainUrl)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpSuggestGasPrice err: %v", err)
			return err
		}

		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err := ethclient.WrpNonceAt(chainUrl, account, nil)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpNonceAt err: %v", err)
			return err
		}

		// tx
		tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, 0, gasPrice, bs)

		// sign the tx
		signedTx, err := types.SignTx(tx, signer, prv)
		if err != nil {
			log.Errorf("SendDataToMainChain, SignTx err: %v", err)
			return err
		}

		// eth_sendRawTransaction
		err = ethclient.WrpSendTransaction(chainUrl, signedTx)
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
		log.Errorf("SendDataToMainChain, succeeded with hash: %v", hash)
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
