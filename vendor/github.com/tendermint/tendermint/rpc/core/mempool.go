package core

import (
	"fmt"
	"time"

	abci "github.com/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"github.com/ethereum/go-ethereum/rlp"
	"bytes"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	rpcTxHook "github.com/tendermint/tendermint/rpc/core/txhook"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// Returns right away, with no response
func BroadcastTxAsync(context *RPCDataContext, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := context.mempool.CheckTx(tx, nil)
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash().Bytes()}, nil
}

// Returns with the response from CheckTx
func BroadcastTxSync(context *RPCDataContext, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {

	ethtx := new(ethTypes.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(tx), 0)
	if err := ethtx.DecodeRLP(rlpStream); err != nil {
		fmt.Printf("BroadcastTxSync() ethtx.DecodeRLP(rlpStream) error with: %s\n", err.Error())
	}
	fmt.Printf("BroadcastTxSync(), tx is %x\n, decoded eth.transaction is %s\n", tx, ethtx.String())

	etd := ethtx.ExtendTxData()
	if etd != nil && etd.FuncName != "" {
		receiveTxCb := rpcTxHook.GetReceiveTxCb(etd.FuncName)
		if receiveTxCb != nil {
			receiveTxCb()
		}
	}

	resCh := make(chan *abci.Response, 1)
	err := context.mempool.CheckTx(tx, func(res *abci.Response) {
		resCh <- res
	})
	if err != nil {
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
		Hash: tx.Hash().Bytes(),
	}, nil
}

// CONTRACT: only returns error if mempool.BroadcastTx errs (ie. problem with the app)
// or if we timeout waiting for tx to commit.
// If CheckTx or DeliverTx fail, no error will be returned, but the returned result
// will contain a non-OK ABCI code.
func BroadcastTxCommit(context *RPCDataContext, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {

	// subscribe to tx being committed in block
	deliverTxResCh := make(chan types.EventDataTx, 1)
	types.AddListenerForEvent(context.eventSwitch, "rpc", types.EventStringTx(tx), func(data types.TMEventData) {
		deliverTxResCh <- data.(types.EventDataTx)
	})

	// broadcast the tx and register checktx callback
	checkTxResCh := make(chan *abci.Response, 1)
	err := context.mempool.CheckTx(tx, func(res *abci.Response) {
		checkTxResCh <- res
	})
	if err != nil {
		logger.Error("err:", err)
		return nil, fmt.Errorf("Error broadcasting transaction: %v", err)
	}
	checkTxRes := <-checkTxResCh
	checkTxR := checkTxRes.GetCheckTx()
	if checkTxR.Code != abci.CodeType_OK {
		// CheckTx failed!
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   checkTxR,
			DeliverTx: nil,
			Hash:      tx.Hash().Bytes(),
		}, nil
	}

	// Wait for the tx to be included in a block,
	// timeout after something reasonable.
	// TODO: configureable?
	timer := time.NewTimer(60 * 2 * time.Second)
	select {
	case deliverTxRes := <-deliverTxResCh:
		// The tx was included in a block.
		deliverTxR := &abci.ResponseDeliverTx{
			Code: deliverTxRes.Code,
			Data: deliverTxRes.Data,
			Log:  deliverTxRes.Log,
		}
		logger.Info("DeliverTx passed ", " tx:", []byte(tx), " response:", deliverTxR)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   checkTxR,
			DeliverTx: deliverTxR,
			Hash:      tx.Hash().Bytes(),
			Height:    deliverTxRes.Height,
		}, nil
	case <-timer.C:
		logger.Error("failed to include tx")
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   checkTxR,
			DeliverTx: nil,
			Hash:      tx.Hash().Bytes(),
		}, fmt.Errorf("Timed out waiting for transaction to be included in a block")
	}

	panic("Should never happen!")
}

func UnconfirmedTxs(context *RPCDataContext) (*ctypes.ResultUnconfirmedTxs, error) {
	txs := context.mempool.Reap(-1)
	return &ctypes.ResultUnconfirmedTxs{len(txs), txs}, nil
}

func NumUnconfirmedTxs(context *RPCDataContext) (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{N: context.mempool.Size()}, nil
}
