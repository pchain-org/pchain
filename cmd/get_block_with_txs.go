package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"os"
)

var (
	ChainIdFlag = cli.StringFlag{
		Name:  "chainid",
		Usage: "chain's url to get block and its transactions\n",
	}

	BlockNumberFlag = cli.IntFlag{
		Name:  "blocknumber",
		Usage: "the number of the block which to get\n",
	}

	getBlockWithTxsCommand = cli.Command{
		Name:     "getblockwithtxs",
		Usage:    "get block and its transactions",
		Category: "TOOLKIT COMMANDS",
		Action:   utils.MigrateFlags(getBlockWithTxs),
		Flags: []cli.Flag{
			ToolkitDirFlag,
			utils.RPCListenAddrFlag,
			utils.RPCPortFlag,
			ChainIdFlag,
			BlockNumberFlag,
		},
		Description: `
When tx3 or Epoch information are not send to main chain in time, main chain
may deny processing of withdraw from child chain.
this command is to help send the block which holds the tx3es or epoch to main
chain.`,
	}
)

func getBlockWithTxs(ctx *cli.Context) error {

	toolkitDir := ctx.GlobalString(ToolkitDirFlag.Name)
	if toolkitDir == "" {
		toolkitDir = "."
	}

	if !ctx.IsSet(ChainIdFlag.Name) {
		Exit(fmt.Errorf("--chainId needed"))
	}
	chainId := ctx.GlobalString(ChainIdFlag.Name)
	localRpcPrefix := calLocalRpcPrefix(ctx)
	chainUrl := localRpcPrefix + chainId

	if !ctx.IsSet(BlockNumberFlag.Name) {
		Exit(fmt.Errorf("--blocknumber needed"))
	}
	blockNumber := ctx.Uint64(BlockNumberFlag.Name)

	rpcBlock, err := ethclient.WrpGetBlock(chainUrl, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		Exit(err)
	}

	consoleBlock := rpcBlockToConsoleBlock(rpcBlock)

	blockFileName := toolkitDir + string(os.PathSeparator) + "block_" + fmt.Sprintf("%d", blockNumber) + ".json"
	blockBytes, err := json.MarshalIndent(consoleBlock, "", "\t")
	if err != nil {
		Exit(err)
	}

	err = ioutil.WriteFile(blockFileName, blockBytes, 0644)
	if err != nil {
		Exit(err)
	}

	return nil
}

type ConsoleBlock struct {
	Difficulty       *big.Int              `json:"difficulty"`
	ExtraData        hexutil.Bytes         `json:"extraData"`
	GasLimit         uint64                `json:"gasLimit"`
	GasUsed          uint64                `json:"gasUsed"`
	Hash             common.Hash           `json:"hash"`
	LogsBloom        types.Bloom           `json:"logsBloom"`
	MainchainNumber  *big.Int              `json:"mainchainNumber"`
	Miner            common.Address        `json:"miner"`
	MixHash          common.Hash           `json:"mixHash"`
	Nonce            types.BlockNonce      `json:"nonce"`
	Number           *big.Int              `json:"number"`
	ParentHash       common.Hash           `json:"parentHash"`
	ReceiptsRoot     common.Hash           `json:"receiptsRoot"`
	Sha3Uncles       common.Hash           `json:"sha3Uncles"`
	Size             uint64                `json:"size"`
	StateRoot        common.Hash           `json:"stateRoot"`
	Timestamp        *big.Int              `json:"timestamp"`
	TotalDifficulty  *big.Int              `json:"totalDifficulty"`
	Transactions     []*ConsoleTransaction `json:"transactions"`
	TransactionsRoot common.Hash           `json:"transactionsRoot"`
	Uncles           []common.Hash         `json:"uncles"`
}

type ConsoleTransaction struct {
	Accesses         *types.AccessList `json:"accessList,omitempty"`
	BlockHash        common.Hash       `json:"blockHash"`
	BlockNumber      *big.Int          `json:"blockNumber"`
	From             common.Address    `json:"from"`
	Gas              uint64            `json:"gas"`
	GasPrice         *big.Int          `json:"gasPrice"`
	Hash             common.Hash       `json:"hash"`
	Input            hexutil.Bytes     `json:"input"`
	Nonce            uint64            `json:"nonce"`
	R                *hexutil.Big      `json:"r"`
	S                *hexutil.Big      `json:"s"`
	To               *common.Address   `json:"to"`
	TransactionIndex uint64            `json:"transactionIndex"`
	V                *hexutil.Big      `json:"v"`
	Value            *big.Int          `json:"value"`
}

func rpcBlockToConsoleBlock(rpcBlock *ethclient.RPCBlock) *ConsoleBlock {

	cosoleBlock := &ConsoleBlock{}

	cosoleBlock.Difficulty = (*big.Int)(rpcBlock.Difficulty)
	cosoleBlock.ExtraData = rpcBlock.ExtraData
	cosoleBlock.GasLimit = uint64(rpcBlock.GasLimit)
	cosoleBlock.GasUsed = uint64(rpcBlock.GasUsed)
	cosoleBlock.Hash = rpcBlock.Hash
	cosoleBlock.LogsBloom = rpcBlock.LogsBloom
	cosoleBlock.MainchainNumber = (*big.Int)(rpcBlock.MainchainNumber)
	cosoleBlock.Miner = rpcBlock.Miner
	cosoleBlock.MixHash = rpcBlock.MixHash
	cosoleBlock.Nonce = rpcBlock.Nonce
	cosoleBlock.Number = (*big.Int)(rpcBlock.Number)
	cosoleBlock.ParentHash = rpcBlock.ParentHash
	cosoleBlock.ReceiptsRoot = rpcBlock.ReceiptsRoot
	cosoleBlock.Sha3Uncles = rpcBlock.Sha3Uncles
	cosoleBlock.Size = uint64(rpcBlock.Size)
	cosoleBlock.StateRoot = rpcBlock.StateRoot
	cosoleBlock.Timestamp = (*big.Int)(rpcBlock.Timestamp)
	cosoleBlock.TotalDifficulty = (*big.Int)(rpcBlock.TotalDifficulty)
	cosoleBlock.TransactionsRoot = rpcBlock.TransactionsRoot
	cosoleBlock.Uncles = rpcBlock.Uncles

	txs := make([]*ConsoleTransaction, 0)
	for _, rpcTx := range rpcBlock.Transactions {
		consoleTx := rpcTxToConsoleTx(rpcTx)
		txs = append(txs, consoleTx)
	}
	cosoleBlock.Transactions = txs

	return cosoleBlock
}

func rpcTxToConsoleTx(rpcTx *ethclient.RPCTransaction) *ConsoleTransaction {

	consoleTx := &ConsoleTransaction{}
	consoleTx.Accesses = rpcTx.Accesses
	consoleTx.BlockHash = rpcTx.BlockHash
	consoleTx.BlockNumber = (*big.Int)(rpcTx.BlockNumber)
	consoleTx.From = rpcTx.From
	consoleTx.Gas = uint64(rpcTx.Gas)
	consoleTx.GasPrice = (*big.Int)(rpcTx.GasPrice)
	consoleTx.Hash = rpcTx.Hash
	consoleTx.Input = rpcTx.Input
	consoleTx.Nonce = uint64(rpcTx.Nonce)
	consoleTx.R = rpcTx.R
	consoleTx.S = rpcTx.S
	consoleTx.To = rpcTx.To
	consoleTx.TransactionIndex = uint64(rpcTx.TransactionIndex)
	consoleTx.V = rpcTx.V
	consoleTx.Value = (*big.Int)(rpcTx.Value)

	return consoleTx
}

func consoleBlockToBlock(consoleBlock *ConsoleBlock) *types.Block {

	//fill headers
	header := &types.Header{}
	header.ParentHash = consoleBlock.ParentHash
	header.UncleHash = consoleBlock.Sha3Uncles
	header.Coinbase = consoleBlock.Miner
	header.Root = consoleBlock.StateRoot
	header.TxHash = consoleBlock.TransactionsRoot
	header.ReceiptHash = consoleBlock.ReceiptsRoot
	header.Bloom = consoleBlock.LogsBloom
	header.Difficulty = consoleBlock.Difficulty
	header.Number = consoleBlock.Number
	header.GasLimit = consoleBlock.GasLimit
	header.GasUsed = consoleBlock.GasUsed
	header.Time = consoleBlock.Timestamp
	header.Extra = consoleBlock.ExtraData
	header.MixDigest = consoleBlock.MixHash
	header.Nonce = consoleBlock.Nonce
	header.MainChainNumber = consoleBlock.MainchainNumber

	//fill txs
	txs := make([]*types.Transaction, len(consoleBlock.Transactions))
	for _, consoleTx := range consoleBlock.Transactions {
		tx := &types.LegacyTx{
			Nonce:    consoleTx.Nonce,
			To:       consoleTx.To,
			Value:    consoleTx.Value,
			Gas:      consoleTx.Gas,
			GasPrice: consoleTx.GasPrice,
			Data:     consoleTx.Input,
			V:        (*big.Int)(consoleTx.V),
			R:        (*big.Int)(consoleTx.R),
			S:        (*big.Int)(consoleTx.S),
		}
		txs[consoleTx.TransactionIndex] = types.NewTx(tx)
	}

	block := types.NewBlockWithHeader(header).WithBody(txs, nil)
	return block
}
