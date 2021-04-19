package bridge

//this file export modules/functions from go-ethereum/internal
import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/rpc"
	"gopkg.in/urfave/cli.v1"
)

func Debug_Setup(ctx *cli.Context, logdir string) error {
	return debug.Setup(ctx, logdir)
}

func Debug_Exit() {
	debug.Exit()
}

func Debug_LoadPanic(x interface{}) {
	debug.LoudPanic(x)
}

var DebugFlags = debug.Flags

type APIBridge interface {

	GasPrice(ctx context.Context) (*hexutil.Big, error)

	GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint64, error)

	SendTransactionEx(ctx context.Context, From common.Address, To *common.Address,
		Gas *hexutil.Uint64, GasPrice *hexutil.Big, Value *hexutil.Big,
		Nonce *hexutil.Uint64, Data *hexutil.Bytes, Input *hexutil.Bytes) (common.Hash, error)

	SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error)

	GetTransactionByHash(ctx context.Context, hash common.Hash) (isPending bool, err error)
}