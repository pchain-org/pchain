package ethclient

import (
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/net/context"
)

/****************
 */
// SaveBlockToMainChain save a block to main chain
//
// be careful, this method just work for main chain
func (ec *Client) SaveBlockToMainChain(ctx context.Context, from common.Address, data []byte) (common.Hash, error) {

	var res common.Hash

	err := ec.c.CallContext(ctx, &res, "chain_saveBlockToMainChain", from, data)
	if err != nil {
		return common.Hash{}, err
	}

	return res, nil
}
