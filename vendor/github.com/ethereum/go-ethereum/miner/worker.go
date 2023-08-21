// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package miner

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	tdmTypes "github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	pabi "github.com/pchain/abi"

	"gopkg.in/fatih/set.v0"
)

const (
	resultQueueSize  = 10
	miningLogAtDepth = 5

	// txChanSize is the size of channel listening to TxPreEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
	// chainSideChanSize is the size of channel listening to ChainSideEvent.
	chainSideChanSize = 10
)

// Agent can register themself with the worker
type Agent interface {
	Work() chan<- *Work
	SetReturnCh(chan<- *Result)
	Stop()
	Start()
	GetHashRate() int64
}

// Work is the workers current environment and holds
// all of the current state information
type Work struct {
	signer types.Signer

	state     *state.StateDB // apply state changes here
	ancestors *set.Set       // ancestor set (used for checking uncle parent validity)
	family    *set.Set       // family set (used for checking uncle invalidity)
	uncles    *set.Set       // uncle set
	tcount    int            // tx count in cycle

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	ops *types.PendingOps // pending events here

	createdAt time.Time
	logger    log.Logger
}

type Result struct {
	Work         *Work
	Block        *types.Block
	Intermediate *tdmTypes.IntermediateBlockResult
}

// worker is the main object which takes care of applying messages to the new state
type worker struct {
	config *params.ChainConfig
	engine consensus.Engine
	eth    Backend
	chain  *core.BlockChain

	// Feeds
	pendingLogsFeed event.Feed

	gasFloor uint64
	gasCeil  uint64

	mu sync.Mutex

	// update loop
	mux          *event.TypeMux
	txCh         chan core.TxPreEvent
	txSub        event.Subscription
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	chainSideCh  chan core.ChainSideEvent
	chainSideSub event.Subscription
	wg           sync.WaitGroup

	agents   map[Agent]struct{}
	resultCh chan *Result
	exitCh   chan struct{}

	proc core.Validator

	coinbase common.Address
	extra    []byte

	currentMu sync.Mutex
	current   *Work

	uncleMu        sync.Mutex
	possibleUncles map[common.Hash]*types.Block

	unconfirmed *unconfirmedBlocks // set of locally mined blocks pending canonicalness confirmations

	// atomic status counters
	mining int32
	atWork int32

	logger log.Logger
	cch    core.CrossChainHelper
}

func newWorker(config *params.ChainConfig, engine consensus.Engine, eth Backend, mux *event.TypeMux, gasFloor, gasCeil uint64, cch core.CrossChainHelper) *worker {
	worker := &worker{
		config:         config,
		engine:         engine,
		eth:            eth,
		mux:            mux,
		gasFloor:       gasFloor,
		gasCeil:        gasCeil,
		txCh:           make(chan core.TxPreEvent, txChanSize),
		chainHeadCh:    make(chan core.ChainHeadEvent, chainHeadChanSize),
		chainSideCh:    make(chan core.ChainSideEvent, chainSideChanSize),
		resultCh:       make(chan *Result, resultQueueSize),
		exitCh:         make(chan struct{}),
		chain:          eth.BlockChain(),
		proc:           eth.BlockChain().Validator(),
		possibleUncles: make(map[common.Hash]*types.Block),
		agents:         make(map[Agent]struct{}),
		unconfirmed:    newUnconfirmedBlocks(eth.BlockChain(), miningLogAtDepth, config.ChainLogger),
		logger:         config.ChainLogger,
		cch:            cch,
	}
	// Subscribe TxPreEvent for tx pool
	worker.txSub = eth.TxPool().SubscribeTxPreEvent(worker.txCh)
	// Subscribe events for blockchain
	worker.chainHeadSub = eth.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
	worker.chainSideSub = eth.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)

	go worker.mainLoop()

	go worker.resultLoop()
	//worker.commitNewWork()

	return worker
}

func (self *worker) setEtherbase(addr common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.coinbase = addr
}

func (self *worker) setExtra(extra []byte) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.extra = extra
}

func (self *worker) pending() (*types.Block, *state.StateDB) {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
			new(trie.Trie),
		), self.current.state.Copy()
	}
	return self.current.Block, self.current.state.Copy()
}

func (self *worker) pendingBlock() *types.Block {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
			new(trie.Trie),
		)
	}
	return self.current.Block
}

func (self *worker) start() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 1)

	//if istanbul, ok := self.engine.(consensus.Istanbul); ok {
	//	istanbul.Start(self.chain, self.chain.CurrentBlock, self.chain.HasBadBlock)
	//}

	if tendermint, ok := self.engine.(consensus.Tendermint); ok {
		err := tendermint.Start(self.chain, self.chain.CurrentBlock, self.chain.HasBadBlock)
		if err != nil {
			self.logger.Error("Starting Tendermint failed", "err", err)
		}
	}

	// spin up agents
	for agent := range self.agents {
		agent.Start()
	}
}

func (self *worker) stop() {
	self.wg.Wait()

	self.mu.Lock()
	defer self.mu.Unlock()
	if atomic.LoadInt32(&self.mining) == 1 {
		for agent := range self.agents {
			agent.Stop()
		}
	}

	if stoppableEngine, ok := self.engine.(consensus.EngineStartStop); ok {
		engineStopErr := stoppableEngine.Stop()
		if engineStopErr != nil {
			self.logger.Error("Stop Engine failed.", "err", engineStopErr)
		} else {
			self.logger.Info("Stop Engine Success.")
		}
	}

	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.atWork, 0)
}

// isRunning returns an indicator whether worker is running or not.
func (w *worker) isRunning() bool {
	return atomic.LoadInt32(&w.mining) == 1
}

// close terminates all background threads maintained by the worker.
// Note the worker does not support being closed multiple times.
func (w *worker) close() {
	close(w.exitCh)
}

func (self *worker) register(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.agents[agent] = struct{}{}
	agent.SetReturnCh(self.resultCh)
}

func (self *worker) unregister(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	delete(self.agents, agent)
	agent.Stop()
}

func (self *worker) mainLoop() {
	defer self.txSub.Unsubscribe()
	defer self.chainHeadSub.Unsubscribe()
	defer self.chainSideSub.Unsubscribe()

	for {
		// A real event arrived, process interesting content
		select {
		// Handle ChainHeadEvent
		case ev := <-self.chainHeadCh:
			if self.isRunning() {
				if h, ok := self.engine.(consensus.Handler); ok {
					h.NewChainHead(ev.Block)
				}
				self.commitNewWork()
			}

		// Handle ChainSideEvent
		case ev := <-self.chainSideCh:
			self.uncleMu.Lock()
			self.possibleUncles[ev.Block.Hash()] = ev.Block
			self.uncleMu.Unlock()

		// Handle TxPreEvent
		case ev := <-self.txCh:
			// Apply transaction to the pending state if we're not mining
			if !self.isRunning() && self.current != nil {
				self.currentMu.Lock()
				acc, _ := types.Sender(self.current.signer, ev.Tx)
				txs := map[common.Address]types.Transactions{acc: {ev.Tx}}
				txset := types.NewTransactionsByPriceAndNonce(self.current.signer, txs, self.current.header.BaseFee)

				self.commitTransactionsEx(txset, self.coinbase, big.NewInt(0), self.cch)
				self.currentMu.Unlock()
			} else {
				// If we're mining, but nothing is being processed, wake on new transactions
				if self.config.Clique != nil && self.config.Clique.Period == 0 {
					self.commitNewWork()
				}
			}

		// System stopped
		case <-self.exitCh:
			return
		case <-self.txSub.Err():
			return
		case <-self.chainHeadSub.Err():
			return
		case <-self.chainSideSub.Err():
			return
		}
	}
}

func (self *worker) resultLoop() {
	for {
		mustCommitNewWork := true
		select {
		case result := <-self.resultCh:
			atomic.AddInt32(&self.atWork, -1)

			if !self.isRunning() || result == nil {
				continue
			}

			var block *types.Block
			var receipts types.Receipts
			var state *state.StateDB
			var ops *types.PendingOps

			if result.Work != nil {
				block = result.Block
				hash := block.Hash()
				work := result.Work

				for i, receipt := range work.receipts {
					// add block location fields
					receipt.BlockHash = hash
					receipt.BlockNumber = block.Number()
					receipt.TransactionIndex = uint(i)

					// Update the block hash in all logs since it is now available and not when the
					// receipt/log of individual transactions were created.
					for _, l := range receipt.Logs {
						l.BlockHash = hash
					}
				}
				for _, log := range work.state.Logs() {
					log.BlockHash = hash
				}
				receipts = work.receipts
				state = work.state
				ops = work.ops
			} else if result.Intermediate != nil {
				block = result.Intermediate.Block

				for i, receipt := range result.Intermediate.Receipts {
					// add block location fields
					receipt.BlockHash = block.Hash()
					receipt.BlockNumber = block.Number()
					receipt.TransactionIndex = uint(i)
				}

				receipts = result.Intermediate.Receipts
				state = result.Intermediate.State
				ops = result.Intermediate.Ops
			} else {
				continue
			}

			// execute the pending ops.
			for _, op := range ops.Ops() {
				if err := core.ApplyOp(op, self.chain, self.cch); err != nil {
					log.Error("Failed executing", "op", op, "err", err)
				}
			}

			self.chain.MuLock()

			stat, err := self.chain.WriteBlockWithState(block, receipts, state)
			if err != nil {
				self.logger.Error("Failed writing block to chain", "err", err)
				self.chain.MuUnLock()
				continue
			}
			// check if canon block and write transactions
			if stat == core.CanonStatTy {
				// implicit by posting ChainHeadEvent
				mustCommitNewWork = false
			}
			// Broadcast the block and announce chain insertion event
			self.mux.Post(core.NewMinedBlockEvent{Block: block})
			var (
				events []interface{}
				logs   = state.Logs()
			)
			events = append(events, core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
			if stat == core.CanonStatTy {
				events = append(events, core.ChainHeadEvent{Block: block})
			}

			self.chain.MuUnLock()

			self.chain.PostChainEvents(events, logs)

			// Insert the block into the set of pending ones to wait for confirmations
			self.unconfirmed.Insert(block.NumberU64(), block.Hash())

			if mustCommitNewWork {
				self.commitNewWork()
			}
		case <-self.exitCh:
			return
		}
	}
}

// push sends a new work task to currently live miner agents.
func (self *worker) push(work *Work) {
	if atomic.LoadInt32(&self.mining) != 1 {
		return
	}
	for agent := range self.agents {
		atomic.AddInt32(&self.atWork, 1)
		if ch := agent.Work(); ch != nil {
			ch <- work
		}
	}
}

// makeCurrent creates a new environment for the current cycle.
func (self *worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := self.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	work := &Work{
		signer:    types.MakeSignerWithMainBlock(self.config, header.MainChainNumber),
		state:     state,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		header:    header,
		ops:       new(types.PendingOps),
		createdAt: time.Now(),
		logger:    self.logger,
	}

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			work.family.Add(uncle.Hash())
		}
		work.family.Add(ancestor.Hash())
		work.ancestors.Add(ancestor.Hash())
	}

	// Keep track of transactions which return errors so they can be removed
	work.tcount = 0
	self.current = work
	return nil
}

func (self *worker) commitNewWork() {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	tstart := time.Now()
	parent := self.chain.CurrentBlock()

	tstamp := tstart.Unix()
	if parent.Time() >= uint64(tstamp) {
		tstamp = int64(parent.Time() + 1)
	}

	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); tstamp > now+1 {
		wait := time.Duration(tstamp-now) * time.Second
		self.logger.Info("Mining too far in the future", "suppose but not wait", common.PrettyDuration(wait))
		//In pchain, there is no need to sleep to wait, commit work immediately
		//time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, self.gasFloor, self.gasCeil),
		Extra:      self.extra,
		Time:       big.NewInt(tstamp),
	}
	// Set baseFee and GasLimit if we are on an EIP-1559 chain
	if self.config.IsLondon(header.Number) {
		header.BaseFee = misc.CalcBaseFee(self.config, parent.Header())
		if !self.config.IsLondon(parent.Number()) {
			parentGasLimit := parent.GasLimit() * params.ElasticityMultiplier
			header.GasLimit = core.CalcGasLimit(parent, parentGasLimit, self.gasCeil)
		}
	}
	// Only set the coinbase if our consensus engine is running (avoid spurious block rewards)
	if self.isRunning() {
		if self.coinbase == (common.Address{}) {
			log.Error("Refusing to mine without coinbase")
			return
		}
		header.Coinbase = self.coinbase
	}
	if err := self.engine.Prepare(self.chain, header); err != nil {
		self.logger.Error("Failed to prepare header for mining", "err", err)
		return
	}
	// If we are care about TheDAO hard-fork check whether to override the extra-data or not
	if daoBlock := self.config.DAOForkBlock; daoBlock != nil {
		// Check whether the block is among the fork extra-override range
		limit := new(big.Int).Add(daoBlock, params.DAOForkExtraRange)
		if header.Number.Cmp(daoBlock) >= 0 && header.Number.Cmp(limit) < 0 {
			// Depending whether we support or oppose the fork, override differently
			if self.config.DAOForkSupport {
				header.Extra = common.CopyBytes(params.DAOForkBlockExtra)
			} else if bytes.Equal(header.Extra, params.DAOForkBlockExtra) {
				header.Extra = []byte{} // If miner opposes, don't let it use the reserved extra-data
			}
		}
	}
	// Could potentially happen if starting to mine in an odd state.
	err := self.makeCurrent(parent, header)
	if err != nil {
		self.logger.Error("Failed to create mining context", "err", err)
		return
	}
	// Create the current work task and check any fork transitions needed
	work := self.current
	if self.config.DAOForkSupport && self.config.DAOForkBlock != nil && self.config.DAOForkBlock.Cmp(header.Number) == 0 {
		misc.ApplyDAOHardFork(work.state)
	}

	// Fill the block with all available pending transactions.
	pending, err := self.eth.TxPool().Pending()
	if err != nil {
		self.logger.Error("Failed to fetch pending transactions", "err", err)
		return
	}

	totalUsedMoney := big.NewInt(0)
	txs := types.NewTransactionsByPriceAndNonce(self.current.signer, pending, header.BaseFee)
	//work.commitTransactions(self.mux, txs, self.chain, self.coinbase)
	rmTxs := self.commitTransactionsEx(txs, self.coinbase, totalUsedMoney, self.cch)

	// Remove the Invalid Transactions during tx execution (eg: tx4)
	if len(rmTxs) > 0 {
		self.eth.TxPool().RemoveTxs(rmTxs)
	}

	//collect cross chain txes and process
	cctBlock := self.engine.(consensus.Tendermint).CurrentCCTBlock()
	mainHeight := new(big.Int).Set(header.MainChainNumber)
	if self.config.IsMainChain() {
		mainHeight = mainHeight.Sub(mainHeight, common.Big1) //can't scan current pending block
	}
	if mainHeight != nil {

		if cctBlock == nil { //initialize the cctblocks in db, make it -5 from current height
			cctBlock = new(big.Int).Sub(mainHeight, big.NewInt(int64(types.CCTBatchBlocks)))
			if cctBlock.Sign() < 0 {
				cctBlock = common.Big0
			}
			self.engine.(consensus.Tendermint).WriteCurrentCCTBlock(cctBlock)
		}

		scanStep := new(big.Int).Sub(mainHeight, cctBlock)
		if scanStep.Sign() > 0 { //skip these blocks if there is no ccttx in them
			scanBlocks := scanStep.Uint64()
			if scanBlocks > types.CCTBatchBlocks {
				scanBlocks = types.CCTBatchBlocks
			}

			for i := uint64(0); i < scanBlocks; i++ {
				cctBlock = cctBlock.Add(cctBlock, common.Big1)
				cctTxsInOneBlock, _ := self.cch.GetCCTTxStatusByChainId(cctBlock, self.config.PChainId)
				if len(cctTxsInOneBlock) != 0 {
					self.commitCCTExecTransactionsEx(cctTxsInOneBlock, self.coinbase, totalUsedMoney, self.cch)
				}
			}
			self.engine.(consensus.Tendermint).WriteCurrentCCTBlock(cctBlock)
		} else if scanStep.Sign() < 0 {
			self.engine.(consensus.Tendermint).WriteCurrentCCTBlock(mainHeight)
		}
	}

	// compute uncles for the new block.
	var (
		uncles    []*types.Header
		badUncles []common.Hash
	)
	for hash, uncle := range self.possibleUncles {
		if len(uncles) == 2 {
			break
		}
		if err := self.commitUncle(work, uncle.Header()); err != nil {
			self.logger.Trace("Bad uncle found and will be removed", "hash", hash)
			self.logger.Trace(fmt.Sprint(uncle))

			badUncles = append(badUncles, hash)
		} else {
			self.logger.Debug("Committing new uncle to block", "hash", hash)
			uncles = append(uncles, uncle.Header())
		}
	}
	for _, hash := range badUncles {
		delete(self.possibleUncles, hash)
	}

	// Create the new block to seal with the consensus engine
	if work.Block, err = self.engine.Finalize(self.chain, header, work.state, work.txs, totalUsedMoney, uncles, work.receipts, work.ops); err != nil {
		self.logger.Error("Failed to finalize block for sealing", "err", err)
		return
	}
	// We only care about logging if we're actually mining.
	if self.isRunning() {
		self.logger.Info("Commit new full mining work", "number", work.Block.Number(), "txs", work.tcount, "uncles", len(uncles), "elapsed", common.PrettyDuration(time.Since(tstart)))
		self.unconfirmed.Shift(work.Block.NumberU64() - 1)
	}
	self.push(work)
}

func (self *worker) commitUncle(work *Work, uncle *types.Header) error {
	hash := uncle.Hash()
	if work.uncles.Has(hash) {
		return fmt.Errorf("uncle not unique")
	}
	if !work.ancestors.Has(uncle.ParentHash) {
		return fmt.Errorf("uncle's parent unknown (%x)", uncle.ParentHash[0:4])
	}
	if work.family.Has(hash) {
		return fmt.Errorf("uncle already in family (%x)", hash)
	}
	work.uncles.Add(uncle.Hash())
	return nil
}

func (w *worker) commitTransactionsEx(txs *types.TransactionsByPriceAndNonce, coinbase common.Address, totalUsedMoney *big.Int, cch core.CrossChainHelper) (rmTxs types.Transactions) {

	gp := new(core.GasPool).AddGas(w.current.header.GasLimit)

	var coalescedLogs []*types.Log

	for {
		// If we don't have enough gas for any further transactions then we're done
		if gp.Gas() < params.TxGas {
			w.logger.Trace("Not enough gas for further transactions", "have", gp, "want", params.TxGas)
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(w.current.signer, tx)
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !w.config.IsEIP155(w.current.header.Number) {
			w.logger.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", w.config.EIP155Block)

			txs.Pop()
			continue
		}

		// Start executing the transaction
		w.current.state.Prepare(tx.Hash(), w.current.tcount)

		logs, err := w.commitTransactionEx(tx, coinbase, gp, totalUsedMoney, cch)
		switch err {
		case core.ErrGasLimitReached:
			// Pop the current out-of-gas transaction without shifting in the next from the account
			w.logger.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case core.ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			w.logger.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case core.ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			w.logger.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case core.ErrInvalidTx4:
			// Remove the tx4
			rmTxs = append(rmTxs, tx)
			w.logger.Trace("Invalid Tx4, this tx will be removed", "hash", tx.Hash())
			txs.Shift()

		case nil:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			w.current.tcount++
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			w.logger.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}

	if len(coalescedLogs) > 0 || w.current.tcount > 0 {
		// make a copy, the state caches the logs and these logs get "upgraded" from pending to mined
		// logs by filling in the block hash when the block was mined by the local miner. This can
		// cause a race condition if a log was "upgraded" before the PendingLogsEvent is processed.
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go func(logs []*types.Log, tcount int) {
			if len(logs) > 0 {
				w.mux.Post(core.PendingLogsEvent{Logs: logs})
			}
			if tcount > 0 {
				w.mux.Post(core.PendingStateEvent{})
			}
		}(cpy, w.current.tcount)
	}
	return
}

func (w *worker) commitTransactionEx(tx *types.Transaction, coinbase common.Address, gp *core.GasPool, totalUsedMoney *big.Int, cch core.CrossChainHelper) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	receipt, err := core.ApplyTransaction(w.config, w.chain, nil, gp, w.current.state, w.current.ops, w.current.header, tx, &w.current.header.GasUsed, totalUsedMoney, vm.Config{}, cch, true)
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}

	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)

	return receipt.Logs, nil
}

func (w *worker) NewChildCCTTx(cts *types.CCTTxStatus, nonce uint64, addrSig []byte) (*types.Transaction, error) {

	input, err := pabi.ChainABI.Pack(pabi.CrossChainTransferExec.String(), cts.MainBlockNumber, cts.TxHash, cts.Owner,
		cts.FromChainId, cts.ToChainId, cts.Amount, cts.Status, types.ReceiptStatusSuccessful, addrSig)
	if err != nil {
		return nil, err
	}

	tx := types.NewTransaction(nonce, pabi.ChainContractMagicAddr, nil, 0, common.Big256, input)
	return w.engine.(consensus.Tendermint).SignTx(tx)
}

func (w *worker) needHandle(chainId string, cts *types.CCTTxStatus, latestCCTES *types.CCTTxExecStatus) bool {

	if chainId == cts.FromChainId {
		if cts.Status == types.CCTRECEIVED && latestCCTES == nil {
			return true //need handle
		} else if cts.Status == types.CCTFAILED && !cts.LastOperationDone &&
			latestCCTES != nil && latestCCTES.Status == types.CCTRECEIVED && latestCCTES.LocalStatus == types.ReceiptStatusSuccessful {
			return true //need revert
		}
	} else if chainId == cts.ToChainId {
		if cts.Status == types.CCTFROMSUCCEEDED && latestCCTES == nil {
			return true //need send succeed signal
		} else if cts.Status == types.CCTSUCCEEDED && !cts.LastOperationDone &&
			latestCCTES != nil && latestCCTES.Status == types.CCTFROMSUCCEEDED && latestCCTES.LocalStatus == types.ReceiptStatusSuccessful {
			return true //need do real transfer
		}
	}

	return false
}

func (w *worker) commitCCTExecTransactionsEx(cctTxsInOneBlock []*types.CCTTxStatus, coinbase common.Address, totalUsedMoney *big.Int, cch core.CrossChainHelper) {

	gp := new(core.GasPool).AddGas(w.current.header.GasLimit)

	tdmEngine := w.engine.(consensus.Tendermint)
	addr, addrSig := tdmEngine.ConsensusAddressSignature()

	for _, cts := range cctTxsInOneBlock {
		latestCCTES := tdmEngine.GetLatestCCTExecStatus(cts.TxHash)
		if w.needHandle(w.config.PChainId, cts, latestCCTES) {

			nonce := w.current.state.GetNonce(addr)
			tx, err := w.NewChildCCTTx(cts, nonce, addrSig)
			if err != nil {
				w.logger.Trace("called NewChildCCTTx", "error", err)
				continue
			}
			if tx.Protected() && !w.config.IsEIP155(w.current.header.Number) {
				w.logger.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", w.config.EIP155Block)
				continue
			}

			// Start executing the transaction
			w.current.state.Prepare(tx.Hash(), w.current.tcount)

			_, err = w.commitCCTExecTransactionEx(tx, w.coinbase, gp, totalUsedMoney, w.cch)
			if err != nil {
				w.logger.Trace("cross chain transfer", "error", err)
			} else {
				nonce++
			}
		}
	}
}

func (w *worker) commitCCTExecTransactionEx(tx *types.Transaction, coinbase common.Address, gp *core.GasPool, totalUsedMoney *big.Int, cch core.CrossChainHelper) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	receipt, err := core.ApplyTransaction(w.config, w.chain, nil, gp, w.current.state, w.current.ops, w.current.header, tx, &w.current.header.GasUsed, totalUsedMoney, vm.Config{}, cch, true)
	if err == core.ErrTryCCTTxExec {
		w.current.state.RevertToSnapshot(snap)

		args := pabi.CrossChainTransferExecArgs{}
		pabi.ChainABI.UnpackMethodInputs(&args, pabi.CrossChainTransferExec.String(), tx.Data())
		input, err := pabi.ChainABI.Pack(pabi.CrossChainTransferExec.String(), args.MainBlockNumber, args.TxHash, args.Owner,
			args.FromChainId, args.ToChainId, args.Amount, args.Status, types.ReceiptStatusFailed)
		if err != nil {
			return nil, err
		}

		newTx := types.NewTransaction(tx.Nonce(), pabi.ChainContractMagicAddr, nil, 0, common.Big256, input)
		newTx, _ = w.engine.(consensus.Tendermint).SignTx(newTx)
		receipt.TxHash = newTx.Hash()
		receipt.Status = types.ReceiptStatusFailed //should be this value, just re-assign

		w.current.txs = append(w.current.txs, newTx)
		w.current.receipts = append(w.current.receipts, receipt)

	} else if err != nil {
		return nil, err
	} else {
		w.current.txs = append(w.current.txs, tx)
		w.current.receipts = append(w.current.receipts, receipt)

	}

	return receipt.Logs, nil
}
