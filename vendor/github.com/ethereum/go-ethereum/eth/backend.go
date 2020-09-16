// Copyright 2014 The go-ethereum Authors
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

// Package eth implements the Ethereum protocol.
package eth

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	istanbulBackend "github.com/ethereum/go-ethereum/consensus/istanbul/backend"
	"github.com/ethereum/go-ethereum/consensus/pdbft"
	tendermintBackend "github.com/ethereum/go-ethereum/consensus/pdbft"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/datareduction"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"gopkg.in/urfave/cli.v1"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Ethereum implements the Ethereum full node service.
type Ethereum struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the ethereum

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb ethdb.Database // Block chain database
	pruneDb ethdb.Database // Prune data database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend *EthApiBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etherbase common.Address
	solcPath  string

	networkId     uint64
	netRPCService *ethapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}

func (s *Ethereum) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Ethereum object (including the
// initialisation of the common Ethereum object)
func New(ctx *node.ServiceContext, config *Config, cliCtx *cli.Context,
	cch core.CrossChainHelper, logger log.Logger, isTestnet bool) (*Ethereum, error) {

	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run eth.Ethereum in light sync mode, use les.LightEthereum")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := ctx.OpenDatabase("chaindata", config.DatabaseCache, config.DatabaseHandles, "eth/db/chaindata/")
	if err != nil {
		return nil, err
	}
	pruneDb, err := ctx.OpenDatabase("prunedata", config.DatabaseCache, config.DatabaseHandles, "pchain/db/prune/")
	if err != nil {
		return nil, err
	}

	isMainChain := params.IsMainChain(ctx.ChainId())
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithDefault(chainDb, config.Genesis, isMainChain, isTestnet)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}

	// Update HTLC Hard Fork and Contract if any one blank
	switch ctx.ChainId() {
	case "pchain":
		if chainConfig.OutOfStorageBlock == nil {
			chainConfig.OutOfStorageBlock = params.MainnetChainConfig.OutOfStorageBlock
		}
		chainConfig.ExtractRewardMainBlock = params.MainnetChainConfig.ExtractRewardMainBlock
		chainConfig.Sd2mcV1Block           = params.MainnetChainConfig.Sd2mcV1Block
		chainConfig.ChildSd2mcWhenEpochEndsBlock = params.MainnetChainConfig.ChildSd2mcWhenEpochEndsBlock
		chainConfig.ValidateHTLCBlock = params.MainnetChainConfig.ValidateHTLCBlock

	case "testnet":
		if chainConfig.OutOfStorageBlock == nil {
			chainConfig.OutOfStorageBlock = params.TestnetChainConfig.OutOfStorageBlock
		}
		chainConfig.ExtractRewardMainBlock = params.TestnetChainConfig.ExtractRewardMainBlock
		chainConfig.Sd2mcV1Block           = params.TestnetChainConfig.Sd2mcV1Block
		chainConfig.ChildSd2mcWhenEpochEndsBlock = params.TestnetChainConfig.ChildSd2mcWhenEpochEndsBlock
		chainConfig.ValidateHTLCBlock = params.TestnetChainConfig.ValidateHTLCBlock
	case "child_0":
		if (chainConfig.HashTimeLockContract == common.Address{}) {
			if isTestnet {
				chainConfig.HashTimeLockContract = params.TestnetChainConfig.Child0HashTimeLockContract
			} else {
				chainConfig.HashTimeLockContract = params.MainnetChainConfig.Child0HashTimeLockContract
			}
		}
		if isTestnet {
			chainConfig.OutOfStorageBlock      = params.TestnetChainConfig.Child0OutOfStorageBlock
			chainConfig.ExtractRewardMainBlock = params.TestnetChainConfig.ExtractRewardMainBlock
			chainConfig.Sd2mcV1Block           = params.TestnetChainConfig.Sd2mcV1Block
			chainConfig.ChildSd2mcWhenEpochEndsBlock = params.TestnetChainConfig.ChildSd2mcWhenEpochEndsBlock
			chainConfig.ValidateHTLCBlock = params.TestnetChainConfig.ValidateHTLCBlock
		} else {
			chainConfig.OutOfStorageBlock      = params.MainnetChainConfig.Child0OutOfStorageBlock
			chainConfig.ExtractRewardMainBlock = params.MainnetChainConfig.ExtractRewardMainBlock
			chainConfig.Sd2mcV1Block           = params.MainnetChainConfig.Sd2mcV1Block
			chainConfig.ChildSd2mcWhenEpochEndsBlock = params.MainnetChainConfig.ChildSd2mcWhenEpochEndsBlock
			chainConfig.ValidateHTLCBlock = params.MainnetChainConfig.ValidateHTLCBlock

		}
	default:
		if isTestnet {
			chainConfig.OutOfStorageBlock      = params.TestnetChainConfig.OutOfStorageBlock
			chainConfig.ExtractRewardMainBlock = params.TestnetChainConfig.ExtractRewardMainBlock
			chainConfig.Sd2mcV1Block           = params.TestnetChainConfig.Sd2mcV1Block
			chainConfig.ChildSd2mcWhenEpochEndsBlock = params.TestnetChainConfig.ChildSd2mcWhenEpochEndsBlock
			chainConfig.ValidateHTLCBlock = params.TestnetChainConfig.ValidateHTLCBlock
		} else {
			chainConfig.OutOfStorageBlock      = params.MainnetChainConfig.OutOfStorageBlock
			chainConfig.ExtractRewardMainBlock = params.MainnetChainConfig.ExtractRewardMainBlock
			chainConfig.Sd2mcV1Block           = params.MainnetChainConfig.Sd2mcV1Block
			chainConfig.ChildSd2mcWhenEpochEndsBlock = params.MainnetChainConfig.ChildSd2mcWhenEpochEndsBlock
			chainConfig.ValidateHTLCBlock = params.MainnetChainConfig.ValidateHTLCBlock

		}
	}

	chainConfig.ChainLogger = logger
	logger.Info("Initialised chain configuration", "config", chainConfig)

	eth := &Ethereum{
		config:         config,
		chainDb:        chainDb,
		pruneDb:        pruneDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, config, chainConfig, chainDb, cliCtx, cch),
		shutdownChan:   make(chan bool),
		networkId:      config.NetworkId,
		gasPrice:       config.MinerGasPrice,
		etherbase:      config.Etherbase,
		solcPath:       config.SolcPath,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	// force to set the istanbul etherbase to node key address
	if chainConfig.Istanbul != nil {
		eth.etherbase = crypto.PubkeyToAddress(ctx.NodeKey().PublicKey)
	}

	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	logger.Info("Initialising Ethereum protocol", "versions", eth.engine.Protocol().Versions, "network", config.NetworkId, "dbversion", dbVer)

	if !config.SkipBcVersionCheck {
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Geth %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
		} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
			logger.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
			rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
		}
	}
	var (
		vmConfig    = vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit: config.TrieCleanCache,

			TrieDirtyLimit:    config.TrieDirtyCache,
			TrieDirtyDisabled: config.NoPruning,
			TrieTimeLimit:     config.TrieTimeout,
		}
	)
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, eth.chainConfig, eth.engine, vmConfig, cch)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		logger.Warn("Rewinding chain to upgrade configuration", "err", compat)
		eth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	eth.txPool = core.NewTxPool(config.TxPool, eth.chainConfig, eth.blockchain, cch)

	if eth.protocolManager, err = NewProtocolManager(eth.chainConfig, config.SyncMode, config.NetworkId, eth.eventMux, eth.txPool, eth.engine, eth.blockchain, chainDb, cch); err != nil {
		return nil, err
	}
	eth.miner = miner.New(eth, eth.chainConfig, eth.EventMux(), eth.engine, config.MinerGasFloor, config.MinerGasCeil, cch)
	eth.miner.SetExtra(makeExtraData(config.ExtraData))

	eth.ApiBackend = &EthApiBackend{eth, nil, nil, cch}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	eth.ApiBackend.gpo = gasprice.NewOracle(eth.ApiBackend, gpoParams)

	return eth, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"geth",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service
func CreateConsensusEngine(ctx *node.ServiceContext, config *Config, chainConfig *params.ChainConfig, db ethdb.Database,
	cliCtx *cli.Context, cch core.CrossChainHelper) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// If Istanbul is requested, set it up
	if chainConfig.Istanbul != nil {
		if chainConfig.Istanbul.Epoch != 0 {
			config.Istanbul.Epoch = chainConfig.Istanbul.Epoch
		}
		config.Istanbul.ProposerPolicy = istanbul.ProposerPolicy(chainConfig.Istanbul.ProposerPolicy)
		return istanbulBackend.New(&config.Istanbul, ctx.NodeKey(), db)
	}
	// If Tendermint is requested, set it up
	if chainConfig.Tendermint != nil {
		if chainConfig.Tendermint.Epoch != 0 {
			config.Tendermint.Epoch = chainConfig.Tendermint.Epoch
		}
		config.Tendermint.ProposerPolicy = pdbft.ProposerPolicy(chainConfig.Tendermint.ProposerPolicy)
		return tendermintBackend.New(chainConfig, cliCtx, ctx.NodeKey(), cch)
	}

	// Otherwise assume proof-of-work
	ethConfig := config.Ethash
	switch {
	case ethConfig.PowMode == ethash.ModeFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case ethConfig.PowMode == ethash.ModeTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester()
	case ethConfig.PowMode == ethash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ethash.Config{
			CacheDir:       ctx.ResolvePath(ethConfig.CacheDir),
			CachesInMem:    ethConfig.CachesInMem,
			CachesOnDisk:   ethConfig.CachesOnDisk,
			DatasetDir:     ethConfig.DatasetDir,
			DatasetsInMem:  ethConfig.DatasetsInMem,
			DatasetsOnDisk: ethConfig.DatasetsOnDisk,
		})
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Ethereum) APIs() []rpc.API {

	apis := ethapi.GetAPIs(s.ApiBackend, s.solcPath)
	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)
	// Append all the local APIs and return
	apis = append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(s),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
	return apis
}

func (s *Ethereum) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Ethereum) Etherbase() (eb common.Address, err error) {
	if tdm, ok := s.engine.(consensus.Tendermint); ok {
		eb = tdm.PrivateValidator()
		if eb != (common.Address{}) {
			return eb, nil
		} else {
			return eb, errors.New("private validator missing")
		}
	} else {
		s.lock.RLock()
		etherbase := s.etherbase
		s.lock.RUnlock()

		if etherbase != (common.Address{}) {
			return etherbase, nil
		}
		if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
			if accounts := wallets[0].Accounts(); len(accounts) > 0 {
				etherbase := accounts[0].Address

				s.lock.Lock()
				s.etherbase = etherbase
				s.lock.Unlock()

				log.Info("Etherbase automatically configured", "address", etherbase)
				return etherbase, nil
			}
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// set in js console via admin interface or wrapper from cli flags
func (self *Ethereum) SetEtherbase(etherbase common.Address) {
	if _, ok := self.engine.(consensus.Istanbul); ok {
		log.Error("Cannot set etherbase in Istanbul consensus")
		return
	}
	if _, ok := self.engine.(consensus.Tendermint); ok {
		log.Error("Cannot set etherbase in Tendermint consensus")
		return
	}

	self.lock.Lock()
	self.etherbase = etherbase
	self.lock.Unlock()

	self.miner.SetEtherbase(etherbase)
}

func (s *Ethereum) StartMining(local bool) error {
	var eb common.Address
	if tdm, ok := s.engine.(consensus.Tendermint); ok {
		eb = tdm.PrivateValidator()
		if (eb == common.Address{}) {
			log.Error("Cannot start mining without private validator")
			return errors.New("private validator missing")
		}
	} else {
		eb, err := s.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignHash)
		}
	}

	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so noone will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	return nil
}

func (s *Ethereum) StopMining()         { s.miner.Stop() }
func (s *Ethereum) IsMining() bool      { return s.miner.Mining() }
func (s *Ethereum) Miner() *miner.Miner { return s.miner }

func (s *Ethereum) ChainConfig() *params.ChainConfig   { return s.chainConfig }
func (s *Ethereum) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *Ethereum) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *Ethereum) TxPool() *core.TxPool               { return s.txPool }
func (s *Ethereum) EventMux() *event.TypeMux           { return s.eventMux }
func (s *Ethereum) Engine() consensus.Engine           { return s.engine }
func (s *Ethereum) ChainDb() ethdb.Database            { return s.chainDb }
func (s *Ethereum) IsListening() bool                  { return true } // Always listening
func (s *Ethereum) EthVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Ethereum) NetVersion() uint64                 { return s.networkId }
func (s *Ethereum) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Ethereum) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ethereum protocol implementation.
func (s *Ethereum) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers()

	// Start the RPC service
	s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}

	// Start the Auto Mining Loop
	go s.loopForMiningEvent()

	// Start the Data Reduction
	if s.config.PruneStateData && s.chainConfig.PChainId == "child_0"{
		go s.StartScanAndPrune(0)
	}

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (s *Ethereum) Stop() error {
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.engine.Close()
	s.miner.Close()
	s.eventMux.Stop()

	s.chainDb.Close()
	s.pruneDb.Close()
	close(s.shutdownChan)

	return nil
}

func (s *Ethereum) loopForMiningEvent() {
	// Start/Stop mining Feed
	startMiningCh := make(chan core.StartMiningEvent, 1)
	startMiningSub := s.blockchain.SubscribeStartMiningEvent(startMiningCh)

	stopMiningCh := make(chan core.StopMiningEvent, 1)
	stopMiningSub := s.blockchain.SubscribeStopMiningEvent(stopMiningCh)

	defer startMiningSub.Unsubscribe()
	defer stopMiningSub.Unsubscribe()

	for {
		select {
		case <-startMiningCh:
			if !s.IsMining() {
				s.lock.RLock()
				price := s.gasPrice
				s.lock.RUnlock()
				s.txPool.SetGasPrice(price)
				s.chainConfig.ChainLogger.Info("PDBFT Consensus Engine will be start shortly")
				s.engine.(consensus.Tendermint).ForceStart()
				s.StartMining(true)
			} else {
				s.chainConfig.ChainLogger.Info("PDBFT Consensus Engine already started")
			}
		case <-stopMiningCh:
			if s.IsMining() {
				s.chainConfig.ChainLogger.Info("PDBFT Consensus Engine will be stop shortly")
				s.StopMining()
			} else {
				s.chainConfig.ChainLogger.Info("PDBFT Consensus Engine already stopped")
			}
		case <-startMiningSub.Err():
			return
		case <-stopMiningSub.Err():
			return
		}
	}
}

func (s *Ethereum) StartScanAndPrune(blockNumber uint64) {

	if datareduction.StartPruning() {
		log.Info("Data Reduction - Start")
	} else {
		log.Info("Data Reduction - Pruning is already running")
		return
	}

	latestBlockNumber := s.blockchain.CurrentHeader().Number.Uint64()
	if blockNumber == 0 || blockNumber >= latestBlockNumber {
		blockNumber = latestBlockNumber
		log.Infof("Data Reduction - Last block number %v", blockNumber)
	} else {
		log.Infof("Data Reduction - User defined Last block number %v", blockNumber)
	}

	ps := rawdb.ReadHeadScanNumber(s.pruneDb)
	var scanNumber uint64
	if ps != nil {
		scanNumber = *ps
	}

	pp := rawdb.ReadHeadPruneNumber(s.pruneDb)
	var pruneNumber uint64
	if pp != nil {
		pruneNumber = *pp
	}
	log.Infof("Data Reduction - Last scan number %v, prune number %v", scanNumber, pruneNumber)

	pruneProcessor := datareduction.NewPruneProcessor(s.chainDb, s.pruneDb, s.blockchain, s.config.PruneBlockData)
	//pruneProcessor := datareduction.NewPruneProcessor(s.chainDb, s.pruneDb, s.blockchain)

	lastScanNumber, lastPruneNumber := pruneProcessor.Process(blockNumber, scanNumber, pruneNumber)
	log.Infof("Data Reduction Completed - After prune, last number scan %v, prune number %v", lastScanNumber, lastPruneNumber)

	datareduction.StopPruning()
}
