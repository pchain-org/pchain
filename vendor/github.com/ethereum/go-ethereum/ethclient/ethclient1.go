package ethclient

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	pabi "github.com/pchain/abi"
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
func (ec *Client) SendDataToMainChain(ctx context.Context, chainId string, data []byte, signer types.Signer, account common.Address, prv *ecdsa.PrivateKey) (common.Hash, error) {

	if chainId == "" || chainId == "pchain" {
		return common.Hash{}, errors.New("invalid child chainId")
	}

	// data
	bs, err := pabi.ChainABI.Pack(pabi.SaveDataToMainChain.String(), data)
	if err != nil {
		return common.Hash{}, err
	}

	// nonce
	nonce, err := ec.NonceAt(ctx, account, nil)
	if err != nil {
		return common.Hash{}, err
	}

	// tx
	tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, 0, nil, bs)

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

	return signedTx.Hash(), nil
}
