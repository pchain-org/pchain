package app

import (
	"bytes"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"

	abciTypes "github.com/tendermint/abci/types"
	"github.com/tendermint/ethermint/ethereum"
	emtTypes "github.com/tendermint/ethermint/types"
	tmTypes "github.com/tendermint/tendermint/types"
	"fmt"
	"math/big"
	"os"
)

// EthermintApplication implements an ABCI application
type EthermintApplication struct {
	backend      *ethereum.Backend              // backend ethereum struct
	currentState func() (*state.StateDB, error) // fetch the latest state from the ethereum blockchain
	rpcClient    *rpc.Client                    // eth rpc cclient
	strategy     emtTypes.Strategy             // strategy for validator compensation

}

// NewEthermintApplication creates the abci application for ethermint
func NewEthermintApplication(backend *ethereum.Backend,
	client *rpc.Client, strategy emtTypes.Strategy) (*EthermintApplication, error) {
	app := &EthermintApplication{
		backend:      backend,
		rpcClient:    client,
		currentState: backend.Ethereum().BlockChain().State,
		strategy:     strategy,
	}
	backend.SetPreCheckInt(app)

	err := app.backend.ResetWork(app.Receiver()) // init the block results
	return app, err
}

// Info returns information about EthermintApplication to the tendermint engine
func (app *EthermintApplication) Info() abciTypes.ResponseInfo {
	blockchain := app.backend.Ethereum().BlockChain()
	currentBlock := blockchain.CurrentBlock()
	height := currentBlock.Number()
	hash := currentBlock.Hash()

	resInfo := abciTypes.ResponseInfo{
		Data:             "ABCIEthereum",
		LastBlockHeight:  height.Uint64(),
		LastBlockAppHash: hash[:],
	}

	if height.Cmp(big.NewInt(0)) == 0 {
		resInfo.LastBlockAppHash = []byte("")
	}

	fmt.Printf("(app *EthermintApplication) Info() with resInfo result: %v\n", resInfo)

	return resInfo
}

// SetOption sets a configuration option for EthermintApplication
func (app *EthermintApplication) SetOption(key string, value string) (log string) {
	return ""
}

// InitChain initalizes the validator set
func (app *EthermintApplication) InitChain(validators []*abciTypes.Validator) {
	glog.V(logger.Debug).Infof("InitChain")
	glog.V(logger.Debug).Infof("Should not invoked. exit")
	os.Exit(0)
	//app.SetValidators(validators)
}


func (app *EthermintApplication) PreCheck(tx *ethTypes.Transaction) error {
	return app.backend.PreCheck(tx)
}

// CheckTx checks a transaction is valid but does not mutate the state
func (app *EthermintApplication) CheckTx(txBytes []byte) abciTypes.Result {
	glog.V(logger.Debug).Infof("Check tx")

	tx, err := decodeTx(txBytes)
	if err != nil {
		return abciTypes.ErrEncodingError
	}

	err = app.validateTx(tx)
	if err != nil {
		return abciTypes.ErrInternalError // TODO
	}

	return abciTypes.OK
}

// DeliverTx processes a transaction in the EthermintApplication state
func (app *EthermintApplication) DeliverTx(txBytes []byte) abciTypes.Result {
	tx, err := decodeTx(txBytes)
	if err != nil {
		return abciTypes.ErrEncodingError
	}
	glog.V(logger.Debug).Infof("Got DeliverTx (tx): %v", tx)
	err = app.backend.DeliverTx(tx)
	if err != nil {
		glog.V(logger.Debug).Infof("DeliverTx error: %v", err)
		return abciTypes.ErrInternalError
	}
	app.CollectTx(tx)
	return abciTypes.OK
}

// BeginBlock starts a new Ethereum block
func (app *EthermintApplication) BeginBlock(hash []byte, tmHeader *abciTypes.Header) {
	glog.V(logger.Debug).Infof("Begin block")

	// update the eth header with the tendermint header
	app.backend.UpdateHeaderWithTimeInfo(tmHeader)
}

// EndBlock accumulates rewards for the validators and updates them
func (app *EthermintApplication) EndBlock(height uint64) abciTypes.ResponseEndBlock {
	app.backend.AccumulateRewards(app.strategy)
	return app.GetUpdatedValidators()
}

// Commit commits the block and returns a hash of the current state
func (app *EthermintApplication) Commit() abciTypes.Result {
	blockHash, err := app.backend.Commit(app.Receiver())
	if err != nil {
		glog.V(logger.Debug).Infof("Error getting latest ethereum state: %v", err)
		return abciTypes.ErrInternalError
	}
	return abciTypes.NewResultOK(blockHash[:], "")
}

// Query queries the state of EthermintApplication
func (app *EthermintApplication) Query(reqQuery abciTypes.RequestQuery) abciTypes.ResponseQuery {

	fmt.Printf("(app *EthermintApplication) Query() with argument: %s\n", reqQuery.String())

	var in jsonRequest
	if err := json.Unmarshal(reqQuery.Data, &in); err != nil {
		panic(abciTypes.ErrInternalError)
		return abciTypes.ResponseQuery{}
	}
	var result interface{}
	if err := app.rpcClient.Call(&result, in.Method, in.Params...); err != nil {
		panic(abciTypes.NewError(abciTypes.ErrInternalError.Code, err.Error()))
		return abciTypes.ResponseQuery{}
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		panic(abciTypes.ErrInternalError)
		return abciTypes.ResponseQuery{}
	}
	//return abciTypes.NewResultOK(bytes, "")
	return abciTypes.ResponseQuery{Log: string(bytes[:])}
}

//-------------------------------------------------------
// convenience methods

func (app *EthermintApplication) Receiver() common.Address {
	if app.strategy != nil {
		return app.strategy.Receiver()
	}
	return common.Address{}
}

func (app *EthermintApplication) SetValidators(validators []*tmTypes.GenesisValidator) {
	if app.strategy != nil {
		app.strategy.SetValidators(validators)
	}
}

func (app *EthermintApplication) GetUpdatedValidators() abciTypes.ResponseEndBlock {
	if app.strategy != nil {

		genValidators := app.strategy.GetUpdatedValidators()
		s := make([]*abciTypes.Validator, len(genValidators))
		for i, v := range genValidators {
			s[i] = &abciTypes.Validator{
				PubKey : v.PubKey.Bytes(),
				Power : uint64(v.Amount),
			}
		}

		return abciTypes.ResponseEndBlock{Diffs: s}
	}
	return abciTypes.ResponseEndBlock{}
}

func (app *EthermintApplication) CollectTx(tx *ethTypes.Transaction) {
	if app.strategy != nil {
		app.strategy.CollectTx(tx)
	}
}

//-------------------------------------------------------
// utility functions

// validateTx checks the validity of a tx against the blockchain's current state.
// it duplicates the logic in ethereum's tx_pool
func (app *EthermintApplication) validateTx(tx *ethTypes.Transaction) error {
	currentState, err := app.currentState()
	if err != nil {
		return err
	}

	var signer ethTypes.Signer = ethTypes.FrontierSigner{}
	if tx.Protected() {
		signer = ethTypes.NewEIP155Signer(tx.ChainId())
	}

	from, err := ethTypes.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	// Make sure the account exist. Non existent accounts
	// haven't got funds and well therefor never pass.
	if !currentState.Exist(from) {
		return core.ErrInvalidSender
	}

	// Last but not least check for nonce errors
	if currentState.GetNonce(from) > tx.Nonce() {
		return core.ErrNonce
	}

	// Check the transaction doesn't exceed the current
	// block limit gas.
	// TODO
	/*if pool.gasLimit().Cmp(tx.Gas()) < 0 {
		return core.ErrGasLimit
	}*/

	// Transactions can't be negative. This may never happen
	// using RLP decoded transactions but may occur if you create
	// a transaction using the RPC for example.
	if tx.Value().Cmp(common.Big0) < 0 {
		return core.ErrNegativeValue
	}

	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	if currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return core.ErrInsufficientFunds
	}

	intrGas := core.IntrinsicGas(tx.Data(), tx.To() == nil, true) // homestead == true
	if tx.Gas().Cmp(intrGas) < 0 {
		return core.ErrIntrinsicGas
	}

	return nil
}

// format of query data
type jsonRequest struct {
	Method string          `json:"method"`
	Id     json.RawMessage `json:"id,omitempty"`
	Params []interface{}   `json:"params,omitempty"`
}

// rlp decode an etherum transaction
func decodeTx(txBytes []byte) (*ethTypes.Transaction, error) {
	tx := new(ethTypes.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}
