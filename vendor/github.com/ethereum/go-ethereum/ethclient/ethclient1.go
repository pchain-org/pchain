package ethclient

import (
	"context"
	"math/big"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
//
// be careful, this method just work for main chain
func (ec *Client) SaveBlockToMainChain(ctx context.Context, from common.Address, block string) (common.Hash, error) {

	type result struct {
		hash common.Hash
		err  error
	}

	var res result

	err := ec.c.CallContext(ctx, &res, "pchain_saveBlockToMainChain", from, block)
	if err != nil {
		return common.Hash{}, err
	}

	return res.hash, res.err
}
