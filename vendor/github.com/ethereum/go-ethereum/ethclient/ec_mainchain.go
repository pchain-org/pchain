package ethclient

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/pchain/common/plogger"
	"golang.org/x/net/context"
)

var plog = plogger.GetLogger("ethclient")

// SaveBlockToMainChain save a block to main chain
//
// be careful, this method just work for main chain
func (ec *Client) SaveBlockToMainChain(ctx context.Context, from common.Address, data []byte) (common.Hash, error) {

	var res common.Hash

	err := ec.c.CallContext(ctx, &res, "chain_saveBlockToMainChain", from, data)
	if err != nil {
		plog.Warnf("call chain_saveBlockToMainChain fail, err: %v", err)
		return common.Hash{}, err
	}

	plog.Infof("call chain_saveBlockToMainChain success, hash: %v", res)
	return res, nil
}
