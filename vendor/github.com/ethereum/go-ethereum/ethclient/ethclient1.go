package ethclient

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"math/big"
)

func (ec *Client) BlockNumber(ctx context.Context) (*big.Int, error) {

	var hex hexutil.Big

	err := ec.c.CallContext(ctx, &hex, "eth_blockNumber")
	if err != nil {
		return nil, err
	}
	return (*big.Int)(&hex), nil
}

// SaveBlockToMainChain save a block to main chain through eth_sendRawTransaction
func (ec *Client) SendBlockToMainChain(ctx context.Context, chainId string, data []byte, signer types.Signer, account common.Address, prv *ecdsa.PrivateKey) (common.Hash, error) {

	if chainId == "" || chainId == "pchain" {
		return common.Hash{}, errors.New("invalid child chainId")
	}

	// extend tx data
	etd := &types.ExtendTxData{
		FuncName: "SaveBlockToMainChain",
	}

	// nonce
	nonce, err := ec.NonceAt(ctx, account, nil)
	if err != nil {
		return common.Hash{}, err
	}

	// tx
	tx := types.NewTransactionEx(nonce, nil, nil, 0, nil, data, etd)

	// sign the tx
	signedTx, err := types.SignTx(tx, signer, prv)
	if err != nil {
		return common.Hash{}, err
	}

	// eth_sendRawTransaction
	err = ec.SendTransaction(ctx, signedTx)
	if err != nil {
		return common.Hash{}, err
	}

	return tx.Hash(), nil
}
