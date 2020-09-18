package ethclient

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	pabi "github.com/pchain/abi"
	"github.com/pkg/errors"
	"math/big"
	"math/rand"
	"time"
)

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
	nonce, err := client.NonceAt(ctx, account, nil)
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
		log.Errorf("WrpSendTransaction, err: %v", err)
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
		log.Errorf("WrpTransactionByHash, err: %v", err)
		return nil, false, err
	}

	Close(client)
	return tx, isPending, nil
}



// SendDataToMainChain send epoch data to main chain through eth_sendRawTransaction
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

// BroadcastDataToMainChain send tx3 proof data to MainChain via rpc call, then broadcast it via p2p network
func BroadcastDataToMainChain(chainUrl string, chainId string, data []byte) error {
	if chainId == "" || chainId == params.MainnetChainConfig.PChainId || chainId == params.TestnetChainConfig.PChainId {
		return errors.New("invalid child chainId")
	}

	err := retry(2, time.Millisecond*200, func() error {
		return WrpBroadcastTX3ProofData(chainUrl, data)
	})

	return err
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
