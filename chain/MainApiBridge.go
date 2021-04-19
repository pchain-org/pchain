package chain

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/rpc"
)

type MainChainAPIBridge struct {
	mainNode *eth.Ethereum
}

func (ab *MainChainAPIBridge) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	return ab.mainNode.ApiBackend.GetInnerAPIBridge().GasPrice(ctx)
}

func (ab *MainChainAPIBridge) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	return ab.mainNode.ApiBackend.GetInnerAPIBridge().
		GetTransactionCount(ctx, address, blockNr)
}

func (ab *MainChainAPIBridge) SendTransactionEx(ctx context.Context, From common.Address, To *common.Address,
	Gas *hexutil.Uint64, GasPrice *hexutil.Big, Value *hexutil.Big,
	Nonce *hexutil.Uint64, Data *hexutil.Bytes, Input *hexutil.Bytes) (common.Hash, error) {
	return ab.mainNode.ApiBackend.GetInnerAPIBridge().
		SendTransactionEx(ctx, From, To, Gas, GasPrice, Value, Nonce, Data, Input)
}

func (ab *MainChainAPIBridge) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	return ab.mainNode.ApiBackend.GetInnerAPIBridge().
		SendRawTransaction(ctx, encodedTx)
}

func (ab *MainChainAPIBridge) GetTransactionByHash(ctx context.Context, hash common.Hash) (isPending bool, err error) {
	return ab.mainNode.ApiBackend.GetInnerAPIBridge().
		GetTransactionByHash(ctx, hash)
}
