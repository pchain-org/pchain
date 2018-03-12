package ethapi

import (
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/net/context"
	"github.com/syndtr/goleveldb/leveldb/errors"
)


type APIBridge struct {
	txapi *PublicTransactionPoolAPI
}

var ApiBridge APIBridge


func (ab *APIBridge)SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error){

	if (ab.txapi == nil) {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	return ab.txapi.SendTransaction(ctx, args)
}
