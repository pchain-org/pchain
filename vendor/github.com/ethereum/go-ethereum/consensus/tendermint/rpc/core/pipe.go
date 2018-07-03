package core

import (
	cfg "github.com/tendermint/go-config"

	crypto "github.com/tendermint/go-crypto"
	p2p "github.com/tendermint/go-p2p"
	"github.com/ethereum/go-ethereum/consensus/tendermint/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/proxy"
	"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

//----------------------------------------------
// These interfaces are used by RPC and must be thread safe

type Consensus interface {
	GetValidators() (int, []*types.Validator)
	GetRoundState() *consensus.RoundState
}

type P2P interface {
	Listeners() []p2p.Listener
	Peers() p2p.IPeerSet
	NumPeers() (outbound, inbound, dialig int)
	NodeInfo() *p2p.NodeInfo
	IsListening() bool
	DialSeeds(*p2p.AddrBook, []string) error
}

//----------------------------------------
type RPCDataContext struct {
	// external, thread safe interfaces
	eventSwitch   types.EventSwitch
	proxyAppQuery proxy.AppConnQuery
	config        cfg.Config

	// interfaces defined in types and above
	blockStore     types.BlockStore
	mempool        types.Mempool
	consensusState Consensus
	p2pSwitch      P2P

	// objects
	pubKey    crypto.PubKey
	genDoc    *types.GenesisDoc // cache the genesis structure
	addrBook  *p2p.AddrBook
	txIndexer txindex.TxIndexer
}

func (r *RPCDataContext) SetConfig(c cfg.Config) {
	r.config = c
}

func (r *RPCDataContext) SetEventSwitch(evsw types.EventSwitch) {
	r.eventSwitch = evsw
}

func (r *RPCDataContext) SetBlockStore(bs types.BlockStore) {
	r.blockStore = bs
}

func (r *RPCDataContext) SetMempool(mem types.Mempool) {
	r.mempool = mem
}

func (r *RPCDataContext) SetConsensusState(cs Consensus) {
	r.consensusState = cs
}

func (r *RPCDataContext) SetSwitch(sw P2P) {
	r.p2pSwitch = sw
}

func (r *RPCDataContext) SetPubKey(pk crypto.PubKey) {
	r.pubKey = pk
}

func (r *RPCDataContext) SetGenesisDoc(doc *types.GenesisDoc) {
	r.genDoc = doc
}

func (r *RPCDataContext) SetAddrBook(book *p2p.AddrBook) {
	r.addrBook = book
}

func (r *RPCDataContext) SetProxyAppQuery(appConn proxy.AppConnQuery) {
	r.proxyAppQuery = appConn
}

func (r *RPCDataContext) SetTxIndexer(indexer txindex.TxIndexer) {
	r.txIndexer = indexer
}
