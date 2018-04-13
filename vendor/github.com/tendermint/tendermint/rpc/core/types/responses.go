package core_types

import (
	"strings"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-rpc/types"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	ep "github.com/tendermint/tendermint/epoch"
)

type ResultBlockchainInfo struct {
	LastHeight int                `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

type ResultGenesis struct {
	Genesis *types.GenesisDoc `json:"genesis"`
}

type ResultBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

type ResultCommit struct {
	Header          *types.Header `json:"header"`
	Commit          *types.Commit `json:"commit"`
	CanonicalCommit bool          `json:"canonical"`
}

type ResultStatus struct {
	NodeInfo          *p2p.NodeInfo `json:"node_info"`
	PubKey            crypto.PubKey `json:"pub_key"`
	LatestBlockHash   []byte        `json:"latest_block_hash"`
	LatestAppHash     []byte        `json:"latest_app_hash"`
	LatestBlockHeight int           `json:"latest_block_height"`
	LatestBlockTime   int64         `json:"latest_block_time"` // nano
}

func (s *ResultStatus) TxIndexEnabled() bool {
	if s == nil || s.NodeInfo == nil {
		return false
	}
	for _, s := range s.NodeInfo.Other {
		info := strings.Split(s, "=")
		if len(info) == 2 && info[0] == "tx_index" {
			return info[1] == "on"
		}
	}
	return false
}

type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	Peers     []Peer   `json:"peers"`
}

type ResultDialSeeds struct {
	Log string `json:"log"`
}

type Peer struct {
	p2p.NodeInfo     `json:"node_info"`
	IsOutbound       bool                 `json:"is_outbound"`
	ConnectionStatus p2p.ConnectionStatus `json:"connection_status"`
}

type ResultValidators struct {
	BlockHeight int                `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
}

type ResultDumpConsensusState struct {
	RoundState      string   `json:"round_state"`
	PeerRoundStates []string `json:"peer_round_states"`
}

type ResultBroadcastTx struct {
	Code abci.CodeType `json:"code"`
	Data []byte        `json:"data"`
	Log  string        `json:"log"`

	Hash []byte `json:"hash"`
}

type ResultBroadcastTxCommit struct {
	CheckTx   *abci.ResponseCheckTx   `json:"check_tx"`
	DeliverTx *abci.ResponseDeliverTx `json:"deliver_tx"`
	Hash      []byte                  `json:"hash"`
	Height    int                     `json:"height"`
}

type ResultTx struct {
	Height   int                    `json:"height"`
	Index    int                    `json:"index"`
	TxResult abci.ResponseDeliverTx `json:"tx_result"`
	Tx       types.Tx               `json:"tx"`
	Proof    types.TxProof          `json:"proof,omitempty"`
}

type ResultUnconfirmedTxs struct {
	N   int        `json:"n_txs"`
	Txs []types.Tx `json:"txs"`
}

type ResultABCIInfo struct {
	Response abci.ResponseInfo `json:"response"`
}

type ResultABCIQuery struct {
	Response abci.ResponseQuery `json:"response"`
}

type ResultUnsafeFlushMempool struct{}

type ResultUnsafeSetConfig struct{}

type ResultUnsafeProfile struct{}

type ResultSubscribe struct {
}

type ResultUnsubscribe struct {
}

type ResultEvent struct {
	Name string            `json:"name"`
	Data types.TMEventData `json:"data"`
}

//author@liaoyd
type ResultValidatorOperation struct{
	From string	`json:"from"`
	Epoch int	`json:"epoch"`
	Power  uint64   `json:"power"`
	Action string	`json:"action"`
	Target string	`json:"target"`
}

type ResultValidatorOperationSimple struct {
	Epoch        int        `json:"epoch"`
	Operation    string     `json:"operation"`
	Amount       uint64     `json:"amount"`
}

type ResultValidatorEpochValidator struct {
	Validator   *types.Validator	`json"validator"`
	Operation   *ResultValidatorOperationSimple `json:"operation"`
}

type ResultValidatorsOperation struct {
	VOArray []*ep.ValidatorOperation
}

type ResultValidatorEpoch struct{
	//BlockHeight int                `json:"block_height"`
	//Validators  []*ResultValidatorEpochValidator `json:"validators"`
	//Epoch	*epoch.Epoch		`json:"epoch"`
	EpochNumber int
	Validator   *types.GenesisValidator
}

type ResultEpoch struct{
	Epoch	*types.OneEpochDoc		`json:"epoch"`
}

type ResultUint64 struct {
	Value uint64
}

//----------------------------------------
// response & result types

const (
	// 0x0 bytes are for the blockchain
	ResultTypeGenesis        = byte(0x01)
	ResultTypeBlockchainInfo = byte(0x02)
	ResultTypeBlock          = byte(0x03)
	ResultTypeCommit         = byte(0x04)

	// 0x2 bytes are for the network
	ResultTypeStatus    = byte(0x20)
	ResultTypeNetInfo   = byte(0x21)
	ResultTypeDialSeeds = byte(0x22)

	// 0x4 bytes are for the consensus
	ResultTypeValidators         = byte(0x40)
	ResultTypeDumpConsensusState = byte(0x41)

	// 0x6 bytes are for txs / the application
	ResultTypeBroadcastTx       = byte(0x60)
	ResultTypeUnconfirmedTxs    = byte(0x61)
	ResultTypeBroadcastTxCommit = byte(0x62)
	ResultTypeTx                = byte(0x63)

	// 0x7 bytes are for querying the application
	ResultTypeABCIQuery = byte(0x70)
	ResultTypeABCIInfo  = byte(0x71)

	// 0x8 bytes are for events
	ResultTypeSubscribe   = byte(0x80)
	ResultTypeUnsubscribe = byte(0x81)
	ResultTypeEvent       = byte(0x82)

	// 0xa bytes for testing
	ResultTypeUnsafeSetConfig        = byte(0xa0)
	ResultTypeUnsafeStartCPUProfiler = byte(0xa1)
	ResultTypeUnsafeStopCPUProfiler  = byte(0xa2)
	ResultTypeUnsafeWriteHeapProfile = byte(0xa3)
	ResultTypeUnsafeFlushMempool     = byte(0xa4)

	// 0xb bytes for extension
	ResultTypeValidatorOperation = byte(0xb0)
	ResultTypeValidatorEpoch = byte(0xb1)
	ResultTypeEpoch = byte(0xb2)
	ResultTypeValidatorsOperation = byte(0xb3)

	//the basic type, from 0xff
	ResultTypeUint64 = byte(0xff)
)

type TMResult interface {
	rpctypes.Result
}

//modified by author@liaoyd
//add ResultTypeTransEx
// for wire.readReflect
var _ = wire.RegisterInterface(
	struct{ TMResult }{},
	wire.ConcreteType{&ResultGenesis{}, ResultTypeGenesis},
	wire.ConcreteType{&ResultBlockchainInfo{}, ResultTypeBlockchainInfo},
	wire.ConcreteType{&ResultBlock{}, ResultTypeBlock},
	wire.ConcreteType{&ResultCommit{}, ResultTypeCommit},
	wire.ConcreteType{&ResultStatus{}, ResultTypeStatus},
	wire.ConcreteType{&ResultNetInfo{}, ResultTypeNetInfo},
	wire.ConcreteType{&ResultDialSeeds{}, ResultTypeDialSeeds},
	wire.ConcreteType{&ResultValidators{}, ResultTypeValidators},
	wire.ConcreteType{&ResultDumpConsensusState{}, ResultTypeDumpConsensusState},
	wire.ConcreteType{&ResultBroadcastTx{}, ResultTypeBroadcastTx},
	wire.ConcreteType{&ResultBroadcastTxCommit{}, ResultTypeBroadcastTxCommit},
	wire.ConcreteType{&ResultTx{}, ResultTypeTx},
	wire.ConcreteType{&ResultUnconfirmedTxs{}, ResultTypeUnconfirmedTxs},
	wire.ConcreteType{&ResultSubscribe{}, ResultTypeSubscribe},
	wire.ConcreteType{&ResultUnsubscribe{}, ResultTypeUnsubscribe},
	wire.ConcreteType{&ResultEvent{}, ResultTypeEvent},
	wire.ConcreteType{&ResultUnsafeSetConfig{}, ResultTypeUnsafeSetConfig},
	wire.ConcreteType{&ResultUnsafeProfile{}, ResultTypeUnsafeStartCPUProfiler},
	wire.ConcreteType{&ResultUnsafeProfile{}, ResultTypeUnsafeStopCPUProfiler},
	wire.ConcreteType{&ResultUnsafeProfile{}, ResultTypeUnsafeWriteHeapProfile},
	wire.ConcreteType{&ResultUnsafeFlushMempool{}, ResultTypeUnsafeFlushMempool},
	wire.ConcreteType{&ResultABCIQuery{}, ResultTypeABCIQuery},
	wire.ConcreteType{&ResultABCIInfo{}, ResultTypeABCIInfo},
	wire.ConcreteType{&ResultValidatorOperation{}, ResultTypeValidatorOperation},
	wire.ConcreteType{&ResultValidatorEpoch{}, ResultTypeValidatorEpoch},
	wire.ConcreteType{&ResultEpoch{}, ResultTypeEpoch},
	wire.ConcreteType{&ResultValidatorsOperation{}, ResultTypeValidatorsOperation},
	wire.ConcreteType{&ResultUint64{}, ResultTypeUint64},
)
