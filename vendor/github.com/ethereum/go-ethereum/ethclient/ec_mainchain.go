package ethclient

import (
	"golang.org/x/net/context"
	"github.com/ethereum/go-ethereum/common"
)

/****************
 */
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
