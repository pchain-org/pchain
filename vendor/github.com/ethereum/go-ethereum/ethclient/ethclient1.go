package ethclient

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

// SaveBlockToMainChain save a block to main chain
func (ec *Client) SaveBlockToMainChain(ctx context.Context, from common.Address, data []byte) (common.Hash, error) {

	var res common.Hash

	// 'from' here is the validator of child-chain, we need to ensure this account exists&unlocked in main-chain because we use this account to sign tx in main-chain.
	// TODO: Consider to send raw tx to main-chain.
	err := ec.c.CallContext(ctx, &res, "chain_saveBlockToMainChain", from, data)
	if err != nil {
		return common.Hash{}, err
	}

	return res, nil
}
