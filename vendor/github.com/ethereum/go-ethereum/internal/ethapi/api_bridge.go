package ethapi

import (
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/net/context"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/aristanetworks/goarista/monotime"
)


type APIBridge struct {
	txapi *PublicTransactionPoolAPI
}

var ApiBridge APIBridge


func (ab *APIBridge)SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error){

	if (ab.txapi == nil) {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	if args.ExtendTxData != nil {
		args.ExtendTxData.Params.Set("time", monotime.Now()) //make the tx hash different
	}

	return ab.txapi.SendTransaction(ctx, args)
}
