package state

import (
	"fmt"
	"bytes"
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-wire"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	. "github.com/tendermint/go-common"
	dbm "github.com/tendermint/go-db"
	"github.com/ethereum/go-ethereum/rlp"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	rpcTxHook "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/txhook"
	"github.com/ethereum/go-ethereum/consensus/tendermint/proxy"
)


//--------------------------------------------------

func calcABCIResponsesKey(height int) []byte {
	return []byte(Fmt("abciResponsesKey:%v", height))
}

//--------------------------------------------------

// ABCIResponses retains the responses
// of the various ABCI calls during block processing.
// It is persisted to disk for each height before calling Commit.
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

	block	*types.Block
	cch 	rpcTxHook.CrossChainHelper
}

var abciMap map[string]*ABCIResponses = make(map[string]*ABCIResponses)

//var instance *ABCIResponses = &ABCIResponses{
//	Height:    -1,
//	DeliverTx: nil,
//	EndBlock:  abci.ResponseEndBlock{},
//	txs:       nil,
//	eventCache:nil,
//	ValidTxs:  0,
//	InvalidTxs:0,
//	TxIndex:   0,
//	Commited:  true,
//}

// NewABCIResponses returns a new ABCIResponses
func NewABCIResponses(block *types.Block, state *State,
			eventCache types.Fireable, cch 	rpcTxHook.CrossChainHelper) *ABCIResponses {

	return &ABCIResponses{
		State:      state,
		//Height:     block.Height,
		//DeliverTx:  make([]*abci.ResponseDeliverTx, block.NumTxs),
		EndBlock:   abci.ResponseEndBlock{},
		//txs:        block.Data.Txs,
		eventCache: eventCache,
		ValidTxs:   0,
		InvalidTxs: 0,
		TxIndex:    0,
		Commited:   false,
		block:      block,
		cch:	    cch,
	}
}

func RefreshABCIResponses(block *types.Block, state *State,
		eventCache types.Fireable, proxyAppConn proxy.AppConnConsensus,
		cch rpcTxHook.CrossChainHelper) *ABCIResponses {

	chainID := state.ChainID

	instance, ok := abciMap[chainID]
	if !ok {
		instance = NewABCIResponses(block, state, eventCache, cch)
		abciMap[chainID] = instance
	} else {
		instance.State = state
		//instance.Height = block.Height
		//instance.DeliverTx = make([]*abci.ResponseDeliverTx, block.NumTxs)
		instance.EndBlock = abci.ResponseEndBlock{}
		//instance.txs = block.Data.Txs
		instance.eventCache = eventCache
		instance.ValidTxs = 0
		instance.InvalidTxs = 0
		instance.TxIndex = 0
		instance.Commited = false
		instance.block = block
		instance.cch = cch
	}

	if !instance.Commited {
		fmt.Printf("The block begin-deliver-end-commit process not finished normally\n")
	}

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

// LoadABCIResponses loads the ABCIResponses for the given height from the database.
// This is useful for recovering from crashes where we called app.Commit and before we called
// s.Save(). It can also be used to produce Merkle proofs of the result of txs.

// TODO move the state db outside the state struct
// func LoadABCIResponses(db dbm.DB, height int64) (*ABCIResponses, error) {
func (s *State) LoadABCIResponses(height int) (*ABCIResponses, error) {
	buf := s.db.Get(calcABCIResponsesKey(height))
	if len(buf) == 0 {
		return nil, ErrNoABCIResponsesForHeight{height}
	}

	abciResponses := new(ABCIResponses)

	r, n, err := bytes.NewReader(buf), new(int), new(error)
	wire.ReadBinaryPtr(abciResponses, r, 0, n, err)
	if *err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		Exit(Fmt("LoadABCIResponses: Data has been corrupted or its spec has changed: %v\n", *err))
	}
	// TODO: ensure that buf is completely read.

	return abciResponses, nil
}

// SaveABCIResponses persists the ABCIResponses to the database.
// This is useful in case we crash after app.Commit and before s.Save().
// Responses are indexed by height so they can also be loaded later to produce Merkle proofs.
func saveABCIResponses(db dbm.DB, height int, abciResponses *ABCIResponses) {
	db.SetSync(calcABCIResponsesKey(height), abciResponses.Bytes())
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
			commitCb(a)
		}

		vals := req.GetCommit().Validators
		fmt.Printf("abci.Response_Commit, Validators are %v, a.Commited is %v\n", vals, a.Commited)

	}
}

func (a *ABCIResponses) GetChainId() string {
	return a.State.ChainID
}

func (a *ABCIResponses) GetValidators() (*types.ValidatorSet, *types.ValidatorSet, error) {
	return a.State.GetValidators()
}

func (a *ABCIResponses) GetCurrentBlock() (*types.Block) {
	return a.block
}

func (a *ABCIResponses) SaveCurrentBlock2MainChain() {

	//a.State.BlockNumberToSave = a.block.Height
}

func (a *ABCIResponses) GetCrossChainHelper() rpcTxHook.CrossChainHelper {
	return a.cch
}
