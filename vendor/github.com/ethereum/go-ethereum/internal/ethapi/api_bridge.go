package ethapi

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"golang.org/x/net/context"
)

type InnerAPIBridge interface {
	SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error)
}

type APIBridge struct {
	txapi *PublicTransactionPoolAPI
}

func (ab *APIBridge) SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error) {

	if ab.txapi == nil {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	return ab.txapi.SendTransaction(ctx, args)
}
