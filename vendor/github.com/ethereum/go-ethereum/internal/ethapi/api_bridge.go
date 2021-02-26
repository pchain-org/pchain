package ethapi

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type InnerAPIBridge interface {

	GasPrice(ctx context.Context) (*hexutil.Big, error)

	GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) 

	SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error)

	SendTransactionEx(ctx context.Context, From common.Address, To *common.Address,
		Gas *hexutil.Uint64, GasPrice *hexutil.Big, Value *hexutil.Big,
		Nonce *hexutil.Uint64, Data  *hexutil.Bytes, Input *hexutil.Bytes) (common.Hash, error)

	SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error)

	GetTransactionByHash(ctx context.Context, hash common.Hash) (isPending bool, err error)
}

type InnerAPIBridgeImp struct {
	txapi *PublicTransactionPoolAPI
	peapi *PublicEthereumAPI
}

func (ab *InnerAPIBridgeImp) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	return ab.peapi.GasPrice(ctx)
}

func (ab *InnerAPIBridgeImp) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	if ab.txapi == nil {
		return nil, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	return ab.txapi.GetTransactionCount(ctx, address, blockNr)
}


func (ab *InnerAPIBridgeImp) SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error) {

	if ab.txapi == nil {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	return ab.txapi.SendTransaction(ctx, args)
}

func (ab *InnerAPIBridgeImp) SendTransactionEx(ctx context.Context,
									From     common.Address,
									To       *common.Address,
									Gas      *hexutil.Uint64,
									GasPrice *hexutil.Big,
									Value    *hexutil.Big,
									Nonce    *hexutil.Uint64,
									Data  *hexutil.Bytes,
									Input *hexutil.Bytes) (common.Hash, error) {

	if ab.txapi == nil {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	args := SendTxArgs {
		From,
		To,
		Gas,
		GasPrice,
		Value,
		Nonce,
		Data,
		Input,
	}

	return ab.txapi.SendTransaction(ctx, args)
}

func (ab *InnerAPIBridgeImp) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	if ab.txapi == nil {
		return common.Hash{}, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	return ab.txapi.SendRawTransaction(ctx, encodedTx)
}

func (ab *InnerAPIBridgeImp) GetTransactionByHash(ctx context.Context, hash common.Hash) (isPending bool, err error) {
	if ab.txapi == nil {
		return true, errors.New("PublicTransactionPoolAPI not initialized yet")
	}

	rpcTx := ab.txapi.GetTransactionByHash(ctx, hash)

	if rpcTx == nil {
		return true, errors.New("NotFound")
	}
	return rpcTx.BlockNumber == nil, nil
}