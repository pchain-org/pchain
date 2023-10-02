package ethclient

import (
	"context"
	//"crypto/ecdsa"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	pTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core/types"
	//"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	//pabi "github.com/pchain/abi"
	"github.com/pkg/errors"
	"math/big"
	"math/rand"
	"time"
)

type RPCBlock struct {
	Difficulty       *hexutil.Big      `json:"difficulty"`
	ExtraData        hexutil.Bytes     `json:"extraData"`
	GasLimit         hexutil.Uint64    `json:"gasLimit"`
	GasUsed          hexutil.Uint64    `json:"gasUsed"`
	Hash             common.Hash       `json:"hash"`
	LogsBloom        types.Bloom       `json:"logsBloom"`
	MainchainNumber  *hexutil.Big      `json:"mainchainNumber"`
	Miner            common.Address    `json:"miner"`
	MixHash          common.Hash       `json:"mixHash"`
	Nonce            types.BlockNonce  `json:"nonce"`
	Number           *hexutil.Big      `json:"number"`
	ParentHash       common.Hash       `json:"parentHash"`
	ReceiptsRoot     common.Hash       `json:"receiptsRoot"`
	Sha3Uncles       common.Hash       `json:"sha3Uncles"`
	Size             hexutil.Uint64    `json:"size"`
	StateRoot        common.Hash       `json:"stateRoot"`
	Timestamp        *hexutil.Big      `json:"timestamp"`
	TotalDifficulty  *hexutil.Big      `json:"totalDifficulty"`
	Transactions     []*RPCTransaction `json:"transactions"`
	TransactionsRoot common.Hash       `json:"transactionsRoot"`
	Uncles           []common.Hash     `json:"uncles"`
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	Accesses         *types.AccessList `json:"accessList,omitempty"`
	BlockHash        common.Hash       `json:"blockHash"`
	BlockNumber      *hexutil.Big      `json:"blockNumber"`
	From             common.Address    `json:"from"`
	Gas              hexutil.Uint64    `json:"gas"`
	GasPrice         *hexutil.Big      `json:"gasPrice"`
	Hash             common.Hash       `json:"hash"`
	Input            hexutil.Bytes     `json:"input"`
	Nonce            hexutil.Uint64    `json:"nonce"`
	R                *hexutil.Big      `json:"r"`
	S                *hexutil.Big      `json:"s"`
	To               *common.Address   `json:"to"`
	TransactionIndex hexutil.Uint      `json:"transactionIndex"`
	V                *hexutil.Big      `json:"v"`
	Value            *hexutil.Big      `json:"value"`
}

//Wrapped BlockNumber, with Dial and Close
func WrpBlockNumber(chainUrl string) (*big.Int, error) {

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpBlockNumber dial err: %v", err)
		return nil, err
	}

	var hex hexutil.Big

	err = client.c.CallContext(ctx, &hex, "eth_blockNumber")
	if err != nil {
		Close(client)
		log.Errorf("WrpBlockNumber, err: %v", err)
		return nil, err
	}

	Close(client)
	return (*big.Int)(&hex), nil
}

func WrpNonceAt(chainUrl string, account common.Address, blockNumber *big.Int) (uint64, error) {

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpNonceAt, dial err: %v", err)
		return uint64(0xffffffff), err
	}

	// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
	nonce, err := client.NonceAt(ctx, account, blockNumber)
	if err != nil {
		Close(client)
		log.Errorf("WrpNonceAt, err: %v", err)
		return uint64(0xffffffff), err
	}

	Close(client)
	return nonce, nil
}

func WrpSuggestGasPrice(chainUrl string) (*big.Int, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpSuggestGasPrice, dial err: %v", err)
		return nil, err
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		Close(client)
		log.Errorf("WrpSuggestGasPrice, err: %v", err)
		return nil, err
	}

	Close(client)
	return gasPrice, nil
}

func WrpSendTransaction(chainUrl string, tx *types.Transaction) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpSendTransaction, dial err: %v", err)
		return err
	}

	err = client.SendTransaction(ctx, tx)
	if err != nil {
		Close(client)
		//log.Errorf("WrpSendTransaction, err: %v", err)
		return err
	}

	Close(client)
	return nil
}


func WrpBroadcastTX3ProofData(chainUrl string, data []byte) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpBroadcastTX3ProofData, dial err: %v", err)
		return err
	}

	err = client.c.CallContext(ctx, nil, "chain_broadcastTX3ProofData", common.ToHex(data))
	if err != nil {
		Close(client)
		log.Errorf("WrpBroadcastTX3ProofData, err: %v", err)
		return err
	}

	Close(client)
	return nil
}

func WrpTransactionByHash(chainUrl string, hash common.Hash) (tx *types.Transaction, isPending bool, err error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpTransactionByHash, dial err: %v", err)
		return nil, false, err
	}

	tx, isPending, err = client.TransactionByHash(ctx, hash)
	if err != nil {
		Close(client)
		//log.Errorf("WrpTransactionByHash, err: %v", err)
		return nil, false, err
	}

	Close(client)
	return tx, isPending, nil
}

func WrpGetLatestCCTStatusByHash(chainUrl string, hash common.Hash) (*types.CCTTxStatus, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpGetLatestCCTStatusByHash, dial err: %v", err)
		return nil, err
	}

	var cts *types.CCTTxStatus
	err = client.c.CallContext(ctx, &cts, "eth_getLatestCCTStatusByHash", hash)
	if err != nil {
		Close(client)
		log.Errorf("WrpGetLatestCCTStatusByHash, err: %v", err)
		return nil, err
	}

	Close(client)
	return cts, err
}


func WrpGetBalance(chainUrl string, addr common.Address, blockNumber *big.Int) (*big.Int, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpGetBalance, dial err: %v", err)
		return nil, err
	}

	var balance hexutil.Big
	err = client.c.CallContext(ctx, &balance, "eth_getBalance", addr, toBlockNumArg(blockNumber))
	if err != nil {
		Close(client)
		log.Errorf("WrpGetBalance, err: %v", err)
		return nil, err
	}

	Close(client)
	return (*big.Int)(&balance), err
}

// SendDataToMainChain send epoch data to main chain through eth_sendRawTransaction
// deprecated
/*
func SendDataToMainChain(chainUrl string, data []byte, prv *ecdsa.PrivateKey, mainChainId string) (common.Hash, error) {

	// data
	bs, err := pabi.ChainABI.Pack(pabi.SaveDataToMainChain.String(), data)
	if err != nil {
		log.Errorf("SendDataToMainChain, pack err: %v", err)
		return common.Hash{}, err
	}

	account := crypto.PubkeyToAddress(prv.PublicKey)

	// tx signer for the main chain
	digest := crypto.Keccak256([]byte(mainChainId))
	signer := types.NewEIP155Signer(new(big.Int).SetBytes(digest[:]))

	var hash = common.Hash{}
	//should send successfully, let's wait longer time
	err = retry(30, time.Second*3, func() error {
		// gasPrice
		gasPrice, err := WrpSuggestGasPrice(chainUrl)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpSuggestGasPrice err: %v", err)
			return err
		}

		// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
		nonce, err := WrpNonceAt(chainUrl, account, nil)
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
		err = WrpSendTransaction(chainUrl, signedTx)
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
*/

// BroadcastDataToMainChain send tx3 proof data to MainChain via rpc call, then broadcast it via p2p network
func BroadcastDataToMainChain(chainUrl string, chainId string, data []byte) error {
	if chainId == "" || params.IsMainChain(chainId) {
		return errors.New("invalid child chainId")
	}

	err := retry(2, time.Millisecond*200, func() error {
		return WrpBroadcastTX3ProofData(chainUrl, data)
	})

	return err
}

func WrpGetBlock(chainUrl string, number *big.Int) (*RPCBlock, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpTransactionByHash, dial err: %v", err)
		return nil, err
	}

	var raw json.RawMessage
	err = client.c.CallContext(ctx, &raw, "eth_getBlockByNumber", hexutil.EncodeBig(number), true)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, errors.New("nil message")
	}
	
	// Decode header and transactions.
	var rpcBlock = &RPCBlock{}
	err = json.Unmarshal(raw, rpcBlock)
	if err != nil {
		return nil, err
	}
	
	Close(client)
	return rpcBlock, nil
}

func WrpGetCurrentEpochNumber(chainUrl string) (uint64, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpTransactionByHash, dial err: %v", err)
		return 0, err
	}

	var curEpochNumber hexutil.Uint64
	err = client.c.CallContext(ctx, &curEpochNumber, "tdm_getCurrentEpochNumber")
	if err != nil {
		return 0, err
	}

	Close(client)
	return uint64(curEpochNumber), nil
}

func WrpGetEpoch(chainUrl string, number uint64) (*pTypes.EpochApi, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := Dial(chainUrl)
	if err != nil {
		log.Errorf("WrpTransactionByHash, dial err: %v", err)
		return nil, err
	}

	var raw json.RawMessage
	err = client.c.CallContext(ctx, &raw, "tdm_getEpoch", hexutil.EncodeUint64(number))
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, errors.New("nil message")
	}

	var rpcEopch = &pTypes.EpochApi{}
	err = json.Unmarshal(raw, rpcEopch)
	if err != nil {
		return nil, err
	}

	Close(client)
	return rpcEopch, nil
}

//attemps: this parameter means the total amount of operations
func retry(attemps int, sleep time.Duration, fn func() error) error {

	for ; attemps > 0; {

		err := fn()
		if err == nil {
			return nil
		}

		attemps --
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
