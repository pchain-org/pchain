package main

import (
	"bufio"
	"errors"
	"fmt"
	gethmain "github.com/ethereum/go-ethereum/cmd/geth"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pchain/chain"
	"gopkg.in/urfave/cli.v1"
	"math/big"
	"os"
	"strings"
	"time"
)

var chaindataDbName = "chaindata"
var snapshotDbName = "snapshot"
var backupDbName = "chaindata.bk"

var chaindataFullFilename = chaindataDbName
var snapshotFullFilename = snapshotDbName
var backupFullFilename = backupDbName

var (
	snapshotCommand = cli.Command{
		Name:   "snapshot",
		Usage:  "Make blockchain snapshot",
		Action: utils.MigrateFlags(snapshot),
		Flags: []cli.Flag{
			utils.DataDirFlag,
			StringChainIdFlag,
		},
		Category: "CHAIN/DATA COMMANDS",
		Description: `
Make a snapshot of one chain.
the snapshot will contain the latest block, the latest state of this chain;
transactions/receipts will not be contained. 
the tx3/tx4/epoch information will keep the same.
This command is to reduce the data-size of one chain to the minimum,
it will be helpful when one node just wants to run with minimum size
with no need for history data. this could extremely save validator's disk.`,
	}
)

func snapshot(ctx *cli.Context) error {

	//make the log only output the error
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlError, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
	ctx.GlobalSet("verbosity", "1") //0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail

	if ctx == nil {
		log.Errorf("ctx is null, pchain does not work")
		return nil
	}

	chainId := ctx.GlobalString(StringChainIdFlag.Name)
	fmt.Printf("loading chain: %v\n", chainId)
	pchain := chain.LoadChain(ctx, chainId)
	if pchain == nil {
		log.Errorf("Load Chain '%s' failed.", chainId)
		return nil
	}
	fmt.Printf("done.\n")

	var ethereum *eth.Ethereum
	pchain.EthNode.ServiceRegistered(&ethereum)
	if ethereum == nil {
		log.Errorf("copyLastestData(), ethereum is nil")
		return errors.New("ethereum is nil")
	}
	chain := ethereum.BlockChain()

	if !continueWork(chain.CurrentBlock().NumberU64()) {
		os.Exit(0)
	}

	if err := doSnapshot(ctx, chainId, chain); err != nil {
		return err
	}

	return nil
}

func doSnapshot(ctx *cli.Context, chainId string, chain *core.BlockChain) error {

	dstDiskDb, dstStateDb, err := prepareDestinationDb(ctx, chainId)
	if err != nil {
		log.Errorf("err :%v", err)
		return err
	}

	//1. copy the start-block and end-block of all epoch
	fmt.Printf("start copying epoch blocks... ")
	if err := copyEpochEndpointBlock(ctx, chain, dstDiskDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//2. copy the lastest block
	fmt.Println("start copying last block")
	if err := copyLastBlock(ctx, chain, dstDiskDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//3. copy out_of_storage rewards
	fmt.Println("start copying out of storage rewards")
	if err := copyOutOfStorage(ctx, chain, dstDiskDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//4. copy preimages which map hash to address
	fmt.Printf("start copying preimages... ")
	if err := copyPreimage(ctx, chain, dstDiskDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//5. copy diverse properties
	fmt.Println("start copying diverse properties")
	if err := copyDiverseProperties(ctx, chain, dstDiskDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//6. copy the lastest state
	//to snap the state, close source diskdb and reload root to make snapshot
	stateDb, _ := chain.State()
	srcDiskDb := stateDb.Database().TrieDB().DiskDB().(ethdb.Database)
	srcDiskDb.Close()

	fmt.Printf("start copying last state... ")
	if err := copyLastState(ctx, chainId, chain.CurrentBlock().Root(), dstDiskDb, dstStateDb); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")

	//backup old db and move snapshot db as current db
	dstDiskDb.Close()

	//7. finally, backup original data and turn snapshot on
	fmt.Println("start backup and replace database directory")
	if err := switchDirectory(ctx, chainId); err != nil {
		log.Errorf("err :%v", err)
		return err
	}
	fmt.Println("done.")
	fmt.Println("")

	fmt.Printf("now the new snapshoted database is under %v,\n"+
		"the old database before snapshot is backuped under %v,\n"+
		"make sure the snapshoted database do work before deleting old database,\n"+
		"enjoy the new journey!\n",
		chaindataFullFilename,
		backupFullFilename)

	return nil
}

//copy start-block and end-block(headers) of each epoch, not include the state
func copyEpochEndpointBlock(ctx *cli.Context, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	tdm := bc.Engine().(consensus.Tendermint)
	currentEpoch := tdm.GetEpoch()

	epochCount := 0
	backLen := 0
	content := fmt.Sprintf("%v epochs copied", epochCount)
	replacePrint(backLen, content)
	backLen = len(content)

	epoch := currentEpoch
	for epoch != nil {

		if epoch != currentEpoch {
			copyCommonBlock(ctx, epoch.EndBlock, bc, dstDiskDb)
		}

		copyCommonBlock(ctx, epoch.StartBlock, bc, dstDiskDb)

		epoch = epoch.GetPreviousEpoch()
		epochCount++
		content = fmt.Sprintf("%v epochs copied", epochCount)
		replacePrint(backLen, content)
		backLen = len(content)
	}

	fmt.Println("")

	return nil
}

//copy previous block(headers), not include the state
func copyCommonBlock(ctx *cli.Context, number uint64, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	//1. write block
	block := bc.GetBlockByNumber(number)
	td := bc.GetTd(block.Hash(), number)
	if td == nil {
		return consensus.ErrUnknownAncestor
	}

	hash := block.Hash()
	rawdb.WriteTd(dstDiskDb, hash, number, td)
	rawdb.WriteBlock(dstDiskDb, block)
	rawdb.WriteReceipts(dstDiskDb, hash, number, nil)
	rawdb.WriteCanonicalHash(dstDiskDb, hash, number)
	//rawdb.WriteHeadBlockHash(dstDiskDb, block.Hash())
	//rawdb.WriteHeadHeaderHash(dstDiskDb, block.Hash())

	if number == uint64(0) {
		srcStateDb, err := bc.State()
		if err != nil {
			return err
		}
		//configPrefix   = []byte("ethereum-config-") // config prefix for the db
		srcDiskDb := srcStateDb.Database().TrieDB().DiskDB().(ethdb.Database)
		stored := rawdb.ReadCanonicalHash(srcDiskDb, number)
		storedCfg := rawdb.ReadChainConfig(srcDiskDb, stored)
		rawdb.WriteChainConfig(dstDiskDb, block.Hash(), storedCfg)
	}

	return nil
}

//copy last block(headers/tx/receips, not include the state
func copyLastBlock(ctx *cli.Context, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	//1. write block
	block := bc.CurrentBlock()
	//localTd := bc.GetTd(block.Hash(), block.NumberU64())
	ptd := bc.GetTd(block.ParentHash(), block.NumberU64()-1)
	if ptd == nil {
		return consensus.ErrUnknownAncestor
	}
	externTd := new(big.Int).Add(block.Difficulty(), ptd)

	rawdb.WriteTd(dstDiskDb, block.Hash(), block.NumberU64(), externTd)
	rawdb.WriteBlock(dstDiskDb, block)
	rawdb.WriteCanonicalHash(dstDiskDb, block.Hash(), block.NumberU64())
	/*TODO*/
	//rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
	rawdb.WriteHeadBlockHash(dstDiskDb, block.Hash())
	rawdb.WriteHeadHeaderHash(dstDiskDb, block.Header().Hash())
	rawdb.WriteHeadFastBlockHash(dstDiskDb, block.Hash())

	/*TODO*/
	//rawdb.WriteTxLookupEntries(batch, block) //!! txes in latest block also needed to snapshot

	return nil
}

func copyOutOfStorage(ctx *cli.Context, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	srcDiskDb, _, err := prepareSourceDb(ctx, bc)
	if err != nil {
		return err
	}

	//RewardPrefix = []byte("w") // rewardPrefix + address + num (uint64 big endian) -> reward value
	it0 := srcDiskDb.NewIteratorWithPrefix(rawdb.RewardPrefix)
	for it0.Next() {
		if len(it0.Key()) < len(rawdb.RewardPrefix)+common.AddressLength+rawdb.Uint64Len {
			return errors.New("RewardExtractPrefix key length is shorter than 21, no address included")
		}
		if err := dstDiskDb.Put(it0.Key(), it0.Value()); err != nil {
			return err
		}
	}
	it0.Release()

	//RewardExtractPrefix = []byte("extrRwd-epoch-")
	it1 := srcDiskDb.NewIteratorWithPrefix(rawdb.RewardExtractPrefix)
	for it1.Next() {
		if len(it1.Key()) < len(rawdb.RewardExtractPrefix)+common.AddressLength {
			return errors.New("RewardExtractPrefix key length is shorter than 21, no address included")
		}
		if err := dstDiskDb.Put(it1.Key(), it1.Value()); err != nil {
			return err
		}
	}
	it1.Release()

	//OosLastBlockKey = []byte("oos-last-block")
	blockBytes, err := srcDiskDb.Get(rawdb.OosLastBlockKey)
	if err != nil {
		return err
	}
	if err := dstDiskDb.Put(rawdb.OosLastBlockKey, blockBytes); err != nil {
		return nil
	}

	/*TODO*/
	//ProposedInEpochPrefix          = []byte("proposed-in-epoch-")
	//StartMarkProposalInEpochPrefix = []byte("sp-in-epoch-")

	return nil
}

func copyPreimage(ctx *cli.Context, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	srcDiskDb, _, err := prepareSourceDb(ctx, bc)
	if err != nil {
		return err
	}

	//preimagePrefix = []byte("secure-key-")      // preimagePrefix + hash -> preimage
	it := srcDiskDb.NewIteratorWithPrefix(rawdb.PreimagePrefix)
	defer it.Release()

	preimageCount := 0
	backLen := 0
	content := fmt.Sprintf("%v preimges copied", preimageCount)
	replacePrint(backLen, content)
	backLen = len(content)

	for it.Next() {
		if len(it.Key()) < len(rawdb.PreimagePrefix)+common.HashLength {
			return errors.New("RewardExtractPrefix key length is shorter than 21, no address included")
		}
		if err := dstDiskDb.Put(it.Key(), it.Value()); err != nil {
			return err
		}
		preimageCount++
		content = fmt.Sprintf("%v preimges copied", preimageCount)
		replacePrint(backLen, content)
		backLen = len(content)
	}

	fmt.Println("")

	return nil
}

func copyDiverseProperties(ctx *cli.Context, bc *core.BlockChain, dstDiskDb ethdb.Database) error {

	srcDiskDb, _, err := prepareSourceDb(ctx, bc)
	if err != nil {
		return err
	}

	//databaseVerisionKey = []byte("DatabaseVersion") // databaseVerisionKey tracks the current database version.
	dbVersion := rawdb.ReadDatabaseVersion(srcDiskDb)
	rawdb.WriteDatabaseVersion(dstDiskDb, *dbVersion)

	//fastTrieProgressKey = []byte("TrieSync")  // fastTrieProgressKey tracks the number of trie entries imported during fast sync.
	count := rawdb.ReadFastTrieProgress(srcDiskDb)
	rawdb.WriteFastTrieProgress(dstDiskDb, count)

	return nil
}

type Snapshot struct {
	diskDb  ethdb.Database
	stateDb *state.StateDB
}

func (sn *Snapshot) Handle(key, value []byte) {

	sn.diskDb.Put(key, value)
}

func copyLastState(ctx *cli.Context, chainId string, root common.Hash, dstDiskDb ethdb.Database, dstStateDb *state.StateDB) error {

	sn := &Snapshot{
		diskDb:  dstDiskDb,
		stateDb: dstStateDb,
	}

	//new snapshot db
	_, cfg := gethmain.MakeConfigNode(ctx, chainId)
	nodeConfig := cfg.Node
	ethConfig := &cfg.Eth
	diskDb, err := rawdb.NewLevelDBDatabase(nodeConfig.ResolvePath(chaindataDbName), ethConfig.DatabaseCache, ethConfig.DatabaseHandles, "eth/db/chaindata/")
	if err != nil {
		return err
	}

	cacheConfig := &core.CacheConfig{
		TrieCleanLimit: 256,
		TrieDirtyLimit: 256,
		TrieTimeLimit:  5 * time.Minute,
	}

	stateCache := state.NewDatabaseWithCacheSnapshot(diskDb, cacheConfig.TrieCleanLimit, sn)
	stateDb, err := state.New(root, stateCache)
	if err != nil {
		return err
	}

	if err := stateDb.DoSnapshot(sn); err != nil {
		return err
	}

	dstStateCache := state.NewDatabaseWithCache(dstDiskDb, cacheConfig.TrieCleanLimit)
	_, err = state.New(root, dstStateCache)
	if err != nil {
		return err
	}

	diskDb.Close()

	return nil
}

func prepareSourceDb(ctx *cli.Context, bc *core.BlockChain) (ethdb.Database, *state.StateDB, error) {

	stateDb, err := bc.State()
	if err != nil {
		return nil, nil, err
	}

	diskDb := stateDb.Database().TrieDB().DiskDB().(ethdb.Database)

	return diskDb, stateDb, nil
}

func prepareDestinationDb(ctx *cli.Context, chainId string) (ethdb.Database, *state.StateDB, error) {

	//new snapshot db
	_, cfg := gethmain.MakeConfigNode(ctx, chainId)
	nodeConfig := cfg.Node
	ethConfig := &cfg.Eth
	diskDb, err := rawdb.NewLevelDBDatabase(nodeConfig.ResolvePath(snapshotDbName), ethConfig.DatabaseCache, ethConfig.DatabaseHandles, "eth/db/chaindata/")
	if err != nil {
		return nil, nil, err
	}

	cacheConfig := &core.CacheConfig{
		TrieCleanLimit: 256,
		TrieDirtyLimit: 256,
		TrieTimeLimit:  5 * time.Minute,
	}

	stateCache := state.NewDatabaseWithCache(diskDb, cacheConfig.TrieCleanLimit)
	stateDb, err := state.New(common.Hash{}, stateCache)
	if err != nil {
		return nil, nil, err
	}

	return diskDb, stateDb, nil
}

func switchDirectory(ctx *cli.Context, chainId string) error {

	_, cfg := gethmain.MakeConfigNode(ctx, chainId)
	nodeConfig := cfg.Node
	chaindataFullFilename = nodeConfig.ResolvePath(chaindataDbName)
	snapshotFullFilename = nodeConfig.ResolvePath(snapshotDbName)
	backupFullFilename = nodeConfig.ResolvePath(backupDbName)

	if err := os.Rename(chaindataFullFilename, backupFullFilename); err != nil {
		return err
	}

	if err := os.Rename(snapshotFullFilename, chaindataFullFilename); err != nil {
		return err
	}

	return nil
}

func continueWork(block uint64) bool {
	fmt.Printf("snapshot the data with block: %v, continue?(Y/n)", block)
	input := bufio.NewScanner(os.Stdin)
	if input.Scan() {
		text := input.Text()
		if strings.EqualFold(text, "y") || strings.EqualFold(text, "yes") {
			return true
		} else {
			return false
		}
	}
	return false
}

func replacePrint(backlen int, content string) {

	backString := ""
	for i := 0; i < backlen; i++ {
		backString += "\b"
	}
	fmt.Print(backString)
	fmt.Print(content)
}
