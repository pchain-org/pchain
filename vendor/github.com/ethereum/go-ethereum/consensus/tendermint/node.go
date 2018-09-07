package tendermint

import (
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-p2p"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/consensus/tendermint/consensus"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/pchain/common/plogger"
	"io/ioutil"
	_ "net/http/pprof"
	"time"
	"github.com/ethereum/go-ethereum/params"
)

var logger = plogger.GetLogger("tendermint")

type PChainP2P interface {
	Switch() *p2p.Switch
	AddrBook() *p2p.AddrBook
}

type Node struct {
	cmn.BaseService
	// config
	config cfg.Config // user config
	//genesisDoc    *types.GenesisDoc    // initial validator set
	privValidator *types.PrivValidator // local node's validator key

	epochDB dbm.DB

	sw       *p2p.Switch   // p2p connections
	addrBook *p2p.AddrBook // known peers

	// services
	evsw types.EventSwitch // pub/sub for services
	//blockStore       *bc.BlockStore              // store the blockchain to disk
	consensusState   *consensus.ConsensusState   // latest consensus state
	consensusReactor *consensus.ConsensusReactor // for participating in the consensus
	//txIndexer        txindex.TxIndexer

	backend *backend
	cch     core.CrossChainHelper
}

func NewNodeNotStart(backend *backend, config cfg.Config, chainConfig *params.ChainConfig, sw *p2p.Switch, addrBook *p2p.AddrBook, cch core.CrossChainHelper) *Node {
	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	keydir := config.GetString("keystore")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile, keydir)

	epochDB := dbm.NewDB("epoch", config.GetString("db_backend"), config.GetString("db_dir"))

	// Make ConsensusReactor
	consensusState := consensus.NewConsensusState(backend, config, chainConfig, cch)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := consensus.NewConsensusReactor(consensusState /*, fastSync*/)

	// Add Reactor to P2P Switch
	sw.AddReactor(config.GetString("chain_id"), "CONSENSUS", consensusReactor)

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	// add the event switch to all services
	// they should all satisfy events.Eventable
	SetEventSwitch(eventSwitch, consensusReactor)

	node := &Node{
		config:        config,
		privValidator: privValidator,

		epochDB: epochDB,

		sw:       sw,
		addrBook: addrBook,

		evsw: eventSwitch,

		backend: backend,
		cch:     cch,

		consensusState:   consensusState,
		consensusReactor: consensusReactor,
	}
	node.BaseService = *cmn.NewBaseService(logger, "Node", node)

	consensusState.SetNode(node)

	return node
}

func (n *Node) OnStart() error {

	logger.Infoln("(n *Node) OnStart()")

	/*
		state, epoch := n.consensusState.InitStateAndEpoch()
		n.consensusState.Initialize()
		n.consensusState.UpdateToStateAndEpoch(state, epoch)
	*/
	_, err := n.evsw.Start()
	if err != nil {
		cmn.Exit(cmn.Fmt("Failed to start switch: %v", err))
	}

	// Add ChainID to P2P Node Info
	n.sw.NodeInfo().AddNetwork(n.config.GetString("chain_id"))

	// Start the Reactors for this Chain
	n.sw.StartChainReactor(n.config.GetString("chain_id") /*state.TdmExtra.ChainID*/)

	// run the profile server
	profileHost := n.config.GetString("prof_laddr")
	if profileHost != "" {

		go func() {
			logger.Warn("Profile server", "error", http.ListenAndServe(profileHost, nil))
		}()
	}

	return nil
}

func (n *Node) OnStop() {
	logger.Infoln("(n *Node) OnStop() called")
	n.BaseService.OnStop()

	n.sw.StopChainReactor(n.consensusState.GetState().TdmExtra.ChainID)
	n.evsw.Stop()
	n.consensusReactor.Stop()
}

//update the state with new insert block information
func (n *Node) SaveState(block *ethTypes.Block) {

	epoch := n.consensusState.Epoch
	state := n.consensusState.GetState()

	fmt.Printf("(n *Node) SaveState(block *ethTypes.Block) with state.height = %v, block.height = %v\n",
		uint64(state.TdmExtra.Height), block.NumberU64())

	if uint64(state.TdmExtra.Height) != block.NumberU64() {
		fmt.Printf("(n *Node) SaveState(block *ethTypes.Block)， block height not equal\n")
	}

	epoch.Save()
	//state.Save()

	n.consensusState.StartNewHeight()
}

func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}

// Add the event switch to reactors, mempool, etc.
func SetEventSwitch(evsw types.EventSwitch, eventables ...types.Eventable) {
	for _, e := range eventables {
		e.SetEventSwitch(evsw)
	}
}

// Add a Listener to accept inbound peer connections.
// Add listeners before starting the Node.
// The first listener is the primary listener (in NodeInfo)
func (n *Node) AddListener(l p2p.Listener) {
	n.sw.AddListener(l)
}

func (n *Node) ConsensusState() *consensus.ConsensusState {
	return n.consensusState
}

func (n *Node) ConsensusReactor() *consensus.ConsensusReactor {
	return n.consensusReactor
}

func (n *Node) EventSwitch() types.EventSwitch {
	return n.evsw
}

// XXX: for convenience
func (n *Node) PrivValidator() *types.PrivValidator {
	return n.privValidator
}

func (n *Node) Config() cfg.Config {
	return n.config
}

func (n *Node) EpochDB() dbm.DB {
	return n.epochDB
}

/*
func (n *Node) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}
*/

//------------------------------------------------------------------------------
// Users wishing to:
//	* use an external signer for their validators
//	* supply an in-proc abci app
// should fork tendermint/tendermint and implement RunNode to
// call NewNode with their custom priv validator and/or custom
// proxy.ClientCreator interface
/*
func RunNode(config cfg.Config, app *app.EthermintApplication) {
	// Wait until the genesis doc becomes available
	genDocFile := config.GetString("genesis_file")
	if !cmn.FileExists(genDocFile) {
		log.Notice(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !cmn.FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				cmn.Exit(cmn.Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc, err := types.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				cmn.PanicSanity(cmn.Fmt("Genesis doc parse json error: %v", err))
			}
			if genDoc.ChainID == "" {
				cmn.PanicSanity(cmn.Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	// Create & start node
	n := NewNodeDefault(config, nil)

	//protocol, address := ProtocolAndAddress(config.GetString("node_laddr"))
	//l := p2p.NewDefaultListener(protocol, address, config.GetBool("skip_upnp"))
	//n.AddListener(l)
	err := n.OnStart()
	if err != nil {
		cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
	}

	//log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())
	// If seedNode is provided by config, dial out.
	if config.GetString("seeds") != "" {
		seeds := strings.Split(config.GetString("seeds"), ",")
		n.DialSeeds(seeds)
	}

	// Run the RPC server.
	if config.GetString("rpc_laddr") != "" {
		_, err := n.StartRPC()
		if err != nil {
			cmn.PanicCrisis(err)
		}
	}
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}
*/

//func (n *Node) NodeInfo() *p2p.NodeInfo {
//	return n.sw.NodeInfo()
//}
//
//func (n *Node) DialSeeds(seeds []string) error {
//	return n.sw.DialSeeds(n.addrBook, seeds)
//}

// Defaults to tcp
func ProtocolAndAddress(listenAddr string) (string, string) {
	protocol, address := "tcp", listenAddr
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	}
	return protocol, address
}

func MakeTendermintNode(backend *backend, config cfg.Config, chainConfig *params.ChainConfig, pNode PChainP2P, cch core.CrossChainHelper) *Node {

	genDocFile := config.GetString("genesis_file")
	if !cmn.FileExists(genDocFile) {
		//log.Notice(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		fmt.Printf(cmn.Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !cmn.FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				cmn.Exit(cmn.Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc, err := types.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				cmn.PanicSanity(cmn.Fmt("Genesis doc parse json error: %v", err))
			}
			if genDoc.ChainID == "" {
				cmn.PanicSanity(cmn.Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	return NewNodeNotStart(backend, config, chainConfig, pNode.Switch(), pNode.AddrBook(), cch)
}
