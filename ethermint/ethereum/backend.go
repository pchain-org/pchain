package ethereum

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	abciTypes "github.com/tendermint/abci/types"
	emtTypes "github.com/pchain/ethermint/types"
	tmTypes "github.com/ethereum/go-ethereum/consensus/tendermint/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/consensus/tendermint"
	"gopkg.in/urfave/cli.v1"
)

const TRANSACTION_NUM_LIMIT = 200000

// Intermediate state of a block, updated with each DeliverTx and reset on Commit
type work struct {
	header *ethTypes.Header
	parent *ethTypes.Block
	state  *state.StateDB
	config *params.ChainConfig
	chainDb ethdb.Database

	txIndex      int
	transactions []*ethTypes.Transaction
	receipts     ethTypes.Receipts
	allLogs      []*ethTypes.Log

	totalUsedGas *uint64
	totalUsedMoney *big.Int
	rewardPerBlock *big.Int
	gp           *core.GasPool
}

type pending struct {
	commitMutex *sync.Mutex
	work        *work
}

// Backend handles the chain database and VM
type Backend struct {
	ethereum *eth.Ethereum
	pending  *pending
	config   *eth.Config
}

const (
	maxWaitForServerRetries = 10
)

// New creates a new Backend
func NewBackend(ctx *node.ServiceContext, config *eth.Config, cliCtx *cli.Context,
                pNode tendermint.PChainP2P, cch core.CrossChainHelper) (*Backend, error) {

	//p := &pending{commitMutex: &sync.Mutex{}}
	var p *pending = nil
	ethereum, err := eth.New(ctx, config, p, cliCtx, pNode, cch)
	if err != nil {
		return nil, err
	}
	ethereum.BlockChain().SetValidator(NullBlockProcessor{})
	ethBackend := &Backend{
		ethereum: ethereum,
		pending:  p,
		config:   config,
	}

	return ethBackend, nil
}

//----------------------------------------------------------------------

// we must implement our own net service since we don't have access to `internal/ethapi`
type NetRPCService struct {
	networkVersion int
}

func (n *NetRPCService) Version() string {
	return fmt.Sprintf("%d", n.networkVersion)
}

// Listening returns an indication if the node is listening for network connections.
func (s *NetRPCService) Listening() bool {
	return true // always listening
}

// PeerCount returns the number of connected peers
func (s *NetRPCService) PeerCount() hexutil.Uint {
	return 0
}

type MinerRPCService struct {
	networkVersion int
}

// APIs returns the collection of RPC services the ethereum package offers.
func (s *Backend) APIs() []rpc.API {
	apis := s.Ethereum().APIs()
	retApis := []rpc.API{}
	for _, v := range apis {

		if v.Namespace == "net" {
			networkVersion := 1
			v.Service = &NetRPCService{networkVersion}
		}

		retApis = append(retApis, v)
	}

	apis = retApis

	return retApis
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ethereum protocol implementation.
func (s *Backend) Start(srvr *p2p.Server) error {
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (s *Backend) Stop() error {
	s.ethereum.Stop()
	return nil
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Backend) Protocols() []p2p.Protocol {
	return nil
}

// Ethereum returns the underlying the ethereum object
func (s *Backend) Ethereum() *eth.Ethereum {
	return s.ethereum
}

// Config returns the eth.Config
func (s *Backend) Config() *eth.Config {
	return s.config
}

//----------------------------------------------------------------------

func (s *pending) Pending() (*ethTypes.Block, *state.StateDB) {
	s.commitMutex.Lock()
	defer s.commitMutex.Unlock()

	return ethTypes.NewBlock(
		s.work.header,
		s.work.transactions,
		nil,
		s.work.receipts,
	), s.work.state.Copy()
}

func (s *pending) PendingBlock() *ethTypes.Block {
	s.commitMutex.Lock()
	defer s.commitMutex.Unlock()

	return ethTypes.NewBlock(
		s.work.header,
		s.work.transactions,
		nil,
		s.work.receipts,
	)
}


func (b *Backend) DeliverTx(tx *ethTypes.Transaction) error {
	return b.pending.deliverTx(b.ethereum.BlockChain(), b.config,
				tx, b.Ethereum().ApiBackend.GetCrossChainHelper())
}

func (p *pending) deliverTx(blockchain *core.BlockChain, config *eth.Config,
				tx *ethTypes.Transaction, cch core.CrossChainHelper) error {
	p.commitMutex.Lock()
	defer p.commitMutex.Unlock()

	blockHash := common.Hash{}
	return p.work.deliverTx(blockchain, config, blockHash, tx, cch)
}

func (w *work) deliverTx(blockchain *core.BlockChain, config *eth.Config, blockHash common.Hash,
				tx *ethTypes.Transaction, cch core.CrossChainHelper) error {

	w.state.Prepare(tx.Hash(), blockHash, w.txIndex)
	fmt.Printf("(w *work) deliverTx(); before apply transaction, w.gp is %v\n", w.gp)
	receipt, _, err := core.ApplyTransactionEx(
		w.config,
		blockchain,
		nil,
		w.gp,
		w.state,
		w.header,
		tx,
		w.totalUsedGas,
		w.totalUsedMoney,
		vm.Config{EnablePreimageRecording: config.EnablePreimageRecording},
		cch,
	)
	if err != nil {
		return err
		glog.V(logger.Debug).Infof("DeliverTx error: %v", err)
		return abciTypes.ErrInternalError
	}
	fmt.Printf("(w *work) deliverTx(); after apply transaction, w.gp is %v\n", w.gp)
	logs := w.state.GetLogs(tx.Hash())

	w.txIndex += 1

	w.transactions = append(w.transactions, tx)
	w.receipts = append(w.receipts, receipt)
	w.allLogs = append(w.allLogs, logs...)

	return err
}

//----------------------------------------------------------------------

func (b *Backend) AccumulateRewards(strategy emtTypes.Strategy, rewardPerBlock *big.Int) {
	b.pending.accumulateRewards(strategy, rewardPerBlock)
}

func (p *pending) accumulateRewards(strategy emtTypes.Strategy, rewardPerBlock *big.Int) {
	p.commitMutex.Lock()
	defer p.commitMutex.Unlock()
	// set the epoch reward per block
	p.work.rewardPerBlock = rewardPerBlock
	p.work.accumulateRewards(strategy)
}

func (w *work) accumulateRewards(strategy emtTypes.Strategy) {

	glog.V(logger.Debug).Infof("(w *work) accumulateRewards(), w.header.GasUsed is %v, w.totalUsedGas is %v, w.totalUsedMoney is %v, validators are: %v",
		w.header.GasUsed, w.totalUsedGas, w.totalUsedMoney, tmTypes.GenesisValidatorsString(strategy.GetUpdatedValidators()))
	w.header.GasUsed = *w.totalUsedGas
	strategy.AccumulateRewards(w.state, w.header, []*ethTypes.Header{}, w.totalUsedMoney, w.rewardPerBlock)
	//core.AccumulateRewards(w.state, w.header, []*ethTypes.Header{})
	glog.V(logger.Debug).Infof("(w *work) accumulateRewards() end")
}

//----------------------------------------------------------------------

func (b *Backend) Commit(receiver common.Address) (common.Hash, error) {
	return b.pending.commit(b.ethereum.BlockChain(), b.ethereum.ChainDb(), receiver)
}

func (p *pending) commit(blockchain *core.BlockChain, chainDb ethdb.Database, receiver common.Address) (common.Hash, error) {
	p.commitMutex.Lock()
	defer p.commitMutex.Unlock()

	blockHash, err := p.work.commit(blockchain)
	if err != nil {
		return common.Hash{}, err
	}

	work, err := p.resetWork(blockchain, chainDb, receiver)
	if err != nil {
		return common.Hash{}, err
	}

	p.work = work
	return blockHash, err
}

func (w *work) commit(blockchain *core.BlockChain) (common.Hash, error) {
	// commit ethereum state and update the header
	/*
	hashArray, err := w.state.Commit(false) // XXX: ugh hardforks
	if err != nil {
		return common.Hash{}, err
	}
	w.header.Root = hashArray

	// tag logs with state root
	// NOTE: BlockHash ?
	for _, log := range w.allLogs {
		log.BlockHash = hashArray
	}

	// save the block to disk
	glog.V(logger.Debug).Infof("Committing block with state hash %X and root hash %X", hashArray, blockHash)
	_, err = blockchain.InsertChain([]*ethTypes.Block{block})
	if err != nil {
		glog.V(logger.Debug).Infof("Error inserting ethereum block in chain: %v", err)
		return common.Hash{}, err
	}
	*/

	block := ethTypes.NewBlock(w.header, w.transactions, nil, w.receipts)
	blockHash := block.Hash()

	// Update the block hash in all logs since it is now available and not when the
	// receipt/log of individual transactions were created.
	for _, r := range w.receipts {
		for _, l := range r.Logs {
			l.BlockHash = block.Hash()
		}
	}
	for _, log := range w.state.Logs() {
		log.BlockHash = block.Hash()
	}
	_, err := blockchain.WriteBlockWithState(block, w.receipts, w.state)
	if err != nil {
		log.Error("Failed writing block to chain", "err", err)
		return common.Hash{}, err
	}
	// check if canon block and write transactions
	//if stat == core.CanonStatTy {
		// implicit by posting ChainHeadEvent
		//mustCommitNewWork = false
	//}
	// Broadcast the block and announce chain insertion event
	/*
	self.mux.Post(core.NewMinedBlockEvent{Block: block})
	var (
		events []interface{}
		logs   = work.state.Logs()
	)
	events = append(events, core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
	if stat == core.CanonStatTy {
		events = append(events, core.ChainHeadEvent{Block: block})
	}
	self.chain.PostChainEvents(events, logs)
	*/

	return blockHash, err
}

//----------------------------------------------------------------------

func (b *Backend) ResetWork(receiver common.Address) error {
	work, err := b.pending.resetWork(b.ethereum.BlockChain(), b.ethereum.ChainDb(), receiver)
	b.pending.work = work
	return err
}

func (p *pending) resetWork(blockchain *core.BlockChain, chainDb ethdb.Database, receiver common.Address) (*work, error) {
	state, err := blockchain.State()
	if err != nil {
		return nil, err
	}

	currentBlock := blockchain.CurrentBlock()
	ethHeader := newBlockHeader(receiver, currentBlock)

	usedGas := new(uint64)
	*usedGas = 0

	return &work{
		header:       ethHeader,
		parent:       currentBlock,
		state:        state,
		config:	      blockchain.Config(),
		chainDb:      chainDb,
		txIndex:      0,
		totalUsedGas: usedGas,
		totalUsedMoney: big.NewInt(0),
		gp:           new(core.GasPool).AddGas(ethHeader.GasLimit),
	}, nil
}

//----------------------------------------------------------------------

func (b *Backend) UpdateHeaderWithTimeInfo(tmHeader *abciTypes.Header) {
	b.pending.updateHeaderWithTimeInfo(b.ethereum.ApiBackend.ChainConfig(), tmHeader.Time)
}

func (p *pending) updateHeaderWithTimeInfo(config *params.ChainConfig, parentTime uint64) {
	p.commitMutex.Lock()
	defer p.commitMutex.Unlock()

	p.work.updateHeaderWithTimeInfo(config, parentTime)
}

func (w *work) updateHeaderWithTimeInfo(config *params.ChainConfig, parentTime uint64) {
	//lastBlock := w.parent
	w.header.Time = new(big.Int).SetUint64(parentTime)
	//w.header.Difficulty = core.CalcDifficulty(config, parentTime, lastBlock.Time().Uint64(), lastBlock.Number(), lastBlock.Difficulty())
	//no need for Difficult, set a specific number
	w.header.Difficulty = new(big.Int).SetUint64(0xabcdabcd)
}

//----------------------------------------------------------------------

func newBlockHeader(receiver common.Address, prevBlock *ethTypes.Block) *ethTypes.Header {
	return &ethTypes.Header{
		Number:     prevBlock.Number().Add(prevBlock.Number(), big.NewInt(1)),
		ParentHash: prevBlock.Hash(),
		GasLimit:   core.CalcGasLimit(prevBlock),
		Coinbase:   receiver,
	}
}
