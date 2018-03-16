package state

import (
	"fmt"
	"bytes"
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	. "github.com/tendermint/go-common"
	"github.com/ethereum/go-ethereum/rlp"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	rpcTxHook "github.com/tendermint/tendermint/rpc/core/txhook"
	"github.com/tendermint/tendermint/proxy"
)


//--------------------------------------------------
// ABCIResponses holds intermediate state during block processing
type ABCIResponses struct {

	State *State
	Height int

	DeliverTx []*abci.ResponseDeliverTx
	EndBlock  abci.ResponseEndBlock

	txs types.Txs // reference for indexing results by hash

	eventCache types.Fireable

	ValidTxs int
	InvalidTxs int
	TxIndex int
	Commited bool
}

var instance *ABCIResponses = &ABCIResponses{
	Height:    -1,
	DeliverTx: nil,
	EndBlock:  abci.ResponseEndBlock{},
	txs:       nil,
	eventCache:nil,
	ValidTxs:  0,
	InvalidTxs:0,
	TxIndex:   0,
	Commited:  true,
}

func GetABCIResponses() *ABCIResponses {
	return instance
}

func RefreshABCIResponses(block *types.Block, state *State,
		eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus) *ABCIResponses {

	if !instance.Commited {
		fmt.Printf("The block begin-deliver-end-commit process not finished normally\n")
	}

	instance.State = state
	instance.Height = block.Height
	instance.DeliverTx = make([]*abci.ResponseDeliverTx, block.NumTxs)
	instance.EndBlock = abci.ResponseEndBlock{}
	instance.txs = block.Data.Txs
	instance.eventCache = eventCache
	instance.ValidTxs = 0
	instance.InvalidTxs = 0
	instance.TxIndex = 0
	instance.Commited = false

	refreshABCIResponseCbMap := rpcTxHook.GetRefreshABCIResponseCbMap()
	for _, refreshABCIResponseCb := range refreshABCIResponseCbMap {
		refreshABCIResponseCb()
	}

	proxyAppConn.SetResponseCallback(instance.ResCb)

	return instance
}

// Serialize the ABCIResponse
func (a *ABCIResponses) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(*a, buf, n, err)
	if *err != nil {
		PanicCrisis(*err)
	}
	return buf.Bytes()
}


func (a *ABCIResponses) ResCb(req *abci.Request, res *abci.Response) {

	switch r := res.Value.(type) {
	case *abci.Response_DeliverTx:

		// TODO: make use of res.Log
		// TODO: make use of this info
		// Blocks may include invalid txs.
		// reqDeliverTx := req.(abci.RequestDeliverTx)
		txError := ""
		txResult := r.DeliverTx
		if txResult.Code == abci.CodeType_OK {
			ethtx := new(ethTypes.Transaction)
			txBytes := req.GetDeliverTx().GetTx()
			rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
			if err := ethtx.DecodeRLP(rlpStream); err != nil {
				fmt.Printf("ethtx.DecodeRLP(rlpStream) error with: %s\n", err.Error())
			}
			fmt.Printf("abci.Response_DeliverTx, tx is %x\n, decoded eth.transaction is %s\n", txBytes, ethtx.String())

			etd := ethtx.ExtendTxData()
			if etd != nil && etd.FuncName != "" {
				deliverTxCb := rpcTxHook.GetDeliverTxCb(etd.FuncName)
				if deliverTxCb != nil {
					deliverTxCb(ethtx)
				}
			}

			a.ValidTxs++
		} else {
			log.Debug("Invalid tx", "code", txResult.Code, "log", txResult.Log)
			a.InvalidTxs++
			txError = txResult.Code.String()
		}

		a.DeliverTx[a.TxIndex] = txResult
		a.TxIndex++

		// NOTE: if we count we can access the tx from the block instead of
		// pulling it from the req
		event := types.EventDataTx{
			Height: a.Height,
			Tx:     types.Tx(req.GetDeliverTx().Tx),
			Data:   txResult.Data,
			Code:   txResult.Code,
			Log:    txResult.Log,
			Error:  txError,
		}
		types.FireEventTx(a.eventCache, event)

	case *abci.Response_Commit:

		fmt.Printf("abci.Response_Commit, a.Commited is %v\n", a.Commited)

		result := r.Commit
		if result.Code == abci.CodeType_OK {
			fmt.Printf("abci.Response_Commit, result ok\n")
			a.Commited = true
		}

		commitCbMap := rpcTxHook.GetCommitCbMap()
		fmt.Printf("abci.Response_Commit, len of commitCbMap is %v\n", len(commitCbMap))
		for _, commitCb := range commitCbMap {
			commitCb(a.State, a.Height)
		}

		vals := req.GetCommit().Validators
		fmt.Printf("abci.Response_Commit, Validators are %v, a.Commited is %v\n", vals, a.Commited)

	}
}
