package core

import (
	"fmt"

	ctypes "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/types"
	"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex/null"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

func Tx(context *RPCDataContext, hash []byte, prove bool) (*ctypes.ResultTx, error) {

	// if index is disabled, return error
	if _, ok := context.txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("Transaction indexing is disabled.")
	}

	r, err := context.txIndexer.Get(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return nil, fmt.Errorf("Tx (%X) not found", hash)
	}

	height := int(r.Height) // XXX
	index := int(r.Index)

	var proof types.TxProof
	if prove {
		block := context.blockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(index)
	}

	return &ctypes.ResultTx{
		Height:   height,
		Index:    index,
		TxResult: r.Result,
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}
