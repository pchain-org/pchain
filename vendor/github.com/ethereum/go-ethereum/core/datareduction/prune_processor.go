package datareduction

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"sort"
	"sync/atomic"
)

var (
	// max scan trie height
	max_count_trie uint64 = 100
	// max retain trie height
	max_remain_trie uint64 = 10
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	pruning int32 // indicate pruning is running or not
)

type PruneProcessor struct {
	db      ethdb.Database // Low level persistent database to store prune counting statistics
	prunedb PruneDatabase

	bc      *core.BlockChain
	chainDb ethdb.Database // database instance to delete the state/block data

	pendingDeleteHashList []common.Hash // Temp List for batch delete the node
}

type PruneStatus struct {
	Running           bool   `json:"is_running"`
	LatestBlockNumber uint64 `json:"latest_block_number"`
	LatestScanNumber  uint64 `json:"latest_scan_number"`
	LatestPruneNumber uint64 `json:"latest_prune_number"`
}

type NodeCount map[common.Hash]uint64

type processLeafTrie func(addr common.Address, account state.Account)

func StartPruning() bool {
	return atomic.CompareAndSwapInt32(&pruning, 0, 1)
}

func StopPruning() bool {
	return atomic.CompareAndSwapInt32(&pruning, 1, 0)
}

func NewPruneProcessor(chaindb, prunedb ethdb.Database, bc *core.BlockChain) *PruneProcessor {
	return &PruneProcessor{
		db:                    prunedb,
		prunedb:               NewDatabase(prunedb),
		bc:                    bc,
		chainDb:               chaindb,
		pendingDeleteHashList: nil,
	}
}

func (p *PruneProcessor) Process(blockNumber, scanNumber, pruneNumber uint64) (uint64, uint64) {
	newScanNumber, newPruneNumber := scanNumber, pruneNumber

	// Step 1. determine the scan height
	scanOrNot, scanStart, scanEnd := calculateScan(scanNumber, blockNumber)

	log.Infof("Data Reduction - scan ? %v , %d - %d", scanOrNot, scanStart, scanEnd)

	if scanOrNot {
		// Step 2. Read Latest Node Count
		nodeCount := p.readLatestNodeCount(scanNumber, pruneNumber)

		var stateRoots []common.Hash
		if pruneNumber > 0 {
			// Add previous state root for prune
			for i := pruneNumber + 1; i <= scanNumber; i++ {
				header := p.bc.GetHeaderByNumber(i)
				stateRoots = append(stateRoots, header.Root)
			}
		}

		for i := scanStart; i <= scanEnd; i++ {

			//TODO Cache the header
			header := p.bc.GetHeaderByNumber(i)

			//log.Printf("Block: %v, Root %x", i, header.Root)
			if skip := p.countBlockChainTrie(header.Root, nodeCount); skip {
				newScanNumber++
				newPruneNumber++
				continue
			}

			stateRoots = append(stateRoots, header.Root)

			// Prune Data Process
			if len(nodeCount) > 0 && len(stateRoots) >= int(max_count_trie) {
				newScanNumber = i
				newPruneNumber = p.processScanData(newScanNumber, stateRoots, nodeCount)
				// Remain state root
				stateRoots = stateRoots[len(stateRoots)-int(max_remain_trie):]
			}
		}

		// Process the remaining state root
		if len(nodeCount) > 0 {
			newScanNumber = scanEnd
			newPruneNumber = p.processScanData(newScanNumber, stateRoots, nodeCount)
		} else if newScanNumber > scanNumber || newPruneNumber > pruneNumber {
			// Update the last number if skip case happened
			p.writeLastNumber(newScanNumber, newPruneNumber)
		}
	}
	return newScanNumber, newPruneNumber
}

func calculateScan(scan, latestBlockHeight uint64) (scanOrNot bool, from, to uint64) {

	var unscanHeight uint64
	if scan == 0 {
		unscanHeight = latestBlockHeight
		from = 0
	} else {
		unscanHeight = latestBlockHeight - scan
		from = scan + 1
	}

	if unscanHeight > max_count_trie {
		mul := latestBlockHeight / max_count_trie
		to = mul * max_count_trie
	}

	if to != 0 {
		scanOrNot = true
	}

	return
}

func (p *PruneProcessor) readLatestNodeCount(scanNumber, pruneNumber uint64) NodeCount {
	nodeCount := make(NodeCount)

	lastHash := rawdb.ReadDataPruneTrieRootHash(p.db, scanNumber, pruneNumber)
	if (lastHash != common.Hash{}) {
		lastPruneTrie, openErr := p.prunedb.OpenPruneTrie(lastHash)
		if openErr != nil {
			log.Error("Data Reduction - Unable read the last Prune Trie.", "err", openErr)
		} else {
			it := trie.NewIterator(lastPruneTrie.NodeIterator(nil))
			for it.Next() {
				nodeHash := common.BytesToHash(lastPruneTrie.GetKey(it.Key))
				var nodeHashCount uint64
				rlp.DecodeBytes(it.Value, &nodeHashCount)
				nodeCount[nodeHash] = nodeHashCount
			}
		}
	}
	return nodeCount
}

func (p *PruneProcessor) countBlockChainTrie(root common.Hash, nodeCount NodeCount) (skip bool) {
	t, openErr := p.bc.StateCache().OpenTrie(root)
	if openErr != nil {
		if _, ok := openErr.(*trie.MissingNodeError); ok {
			// Missing Node Error means the root node of the trie has been removed earlier, so skip the trie and return
			skip = true
		} else {
			log.Error("Data Reduction - Error when open the Main Trie", "err", openErr, "stateroot", root)
		}
		return
	}

	countTrie(t, nodeCount, func(addr common.Address, account state.Account) {
		if account.Root != emptyRoot {
			if storageTrie, stErr := p.bc.StateCache().OpenStorageTrie(common.Hash{}, account.Root); stErr == nil {
				countTrie(storageTrie, nodeCount, nil)
			} else {
				log.Error("Data Reduction - Error when open the Storage Trie", "err", stErr, "storageroot", account.Root, "account", addr)
			}
		}

		if account.TX1Root != emptyRoot {
			if tx1Trie, tx1Err := p.bc.StateCache().OpenTX1Trie(common.Hash{}, account.TX1Root); tx1Err == nil {
				countTrie(tx1Trie, nodeCount, nil)
			} else {
				log.Error("Data Reduction - Error when open the TX1 Trie", "err", tx1Err, "tx1root", account.TX1Root, "account", addr)
			}
		}

		if account.TX3Root != emptyRoot {
			if tx3Trie, tx3Err := p.bc.StateCache().OpenTX3Trie(common.Hash{}, account.TX3Root); tx3Err == nil {
				countTrie(tx3Trie, nodeCount, nil)
			} else {
				log.Error("Data Reduction - Error when open the TX3 Trie", "err", tx3Err, "tx3root", account.TX3Root, "account", addr)
			}
		}

		if account.ProxiedRoot != emptyRoot {
			if proxiedTrie, proxiedErr := p.bc.StateCache().OpenProxiedTrie(common.Hash{}, account.ProxiedRoot); proxiedErr == nil {
				countTrie(proxiedTrie, nodeCount, nil)
			} else {
				log.Error("Data Reduction - Error when open the Proxied Trie", "err", proxiedErr, "proxiedroot", account.ProxiedRoot, "account", addr)
			}
		}

		if account.RewardRoot != emptyRoot {
			if rewardTrie, rewardErr := p.bc.StateCache().OpenRewardTrie(common.Hash{}, account.RewardRoot); rewardErr == nil {
				countTrie(rewardTrie, nodeCount, nil)
			} else {
				log.Error("Data Reduction - Error when open the Reward Trie", "err", rewardErr, "rewardroot", account.RewardRoot, "account", addr)
			}
		}
	})
	return
}

func countTrie(t state.Trie, nodeCount NodeCount, processLeaf processLeafTrie) {
	child := true
	for it := t.NodeIterator(nil); it.Next(child); {
		if !it.Leaf() {
			nodeHash := it.Hash()
			if _, exist := nodeCount[nodeHash]; exist {
				nodeCount[nodeHash]++
				child = false
			} else {
				nodeCount[nodeHash] = 1
				child = true
			}
		} else {
			// Process the Account -> Inner Trie
			if processLeaf != nil {
				addr := t.GetKey(it.LeafKey())
				if len(addr) == 20 {
					var data state.Account
					rlp.DecodeBytes(it.LeafBlob(), &data)

					processLeaf(common.BytesToAddress(addr), data)
				}
			}
		}
	}
}

func (p *PruneProcessor) processScanData(latestScanNumber uint64, stateRoots []common.Hash, nodeCount NodeCount) uint64 {
	log.Infof("Data Reduction - After Scan, Total Nodes: %d, %d", latestScanNumber, len(nodeCount))

	// Prune State Data
	p.pruneData(stateRoots[:len(stateRoots)-int(max_remain_trie)], nodeCount)

	newPruneNumber := latestScanNumber - max_remain_trie

	// Commit the new scaned/pruned node count to trie
	p.commitDataPruneTrie(nodeCount, latestScanNumber, newPruneNumber)

	log.Infof("Data Reduction - After Prune, Total Nodes: %d", len(nodeCount))
	log.Infof("Data Reduction - Scan/Prune Completed for trie %d %d", latestScanNumber, newPruneNumber)
	return newPruneNumber
}

func (p *PruneProcessor) pruneData(stateRoots []common.Hash, nodeCount NodeCount) {
	for _, root := range stateRoots {
		p.pruneBlockChainTrie(root, nodeCount)
	}

	if len(p.pendingDeleteHashList) > 0 {
		log.Infof("Data Reduction - %d hashes will be deleted from chaindb", len(p.pendingDeleteHashList))
		batch := p.chainDb.NewBatch()
		for _, hash := range p.pendingDeleteHashList {
			if batchDeleteError := batch.Delete(hash.Bytes()); batchDeleteError != nil {
				log.Error("Data Reduction - Error when delete the hash from chaindb", "err", batchDeleteError, "hash", hash)
			}
		}
		if writeErr := batch.Write(); writeErr != nil {
			log.Error("Data Reduction - Error when write the deletion batch", "err", writeErr)
		} else {
			log.Info("Data Reduction - write the deletion batch success")
		}
		p.pendingDeleteHashList = nil
	} else {
		log.Info("Data Reduction - no hashes will be deleted from chaindb")
	}
}

func (p *PruneProcessor) pruneBlockChainTrie(root common.Hash, nodeCount NodeCount) {
	t, openErr := p.bc.StateCache().OpenTrie(root)
	if openErr != nil {
		log.Error("Data Reduction - Error when open the Main Trie", "err", openErr, "stateroot", root)
		return
	}

	pruneTrie(t, nodeCount, &p.pendingDeleteHashList, func(addr common.Address, account state.Account) {
		if account.Root != emptyRoot {
			if storageTrie, stErr := p.bc.StateCache().OpenStorageTrie(common.Hash{}, account.Root); stErr == nil {
				pruneTrie(storageTrie, nodeCount, &p.pendingDeleteHashList, nil)
			} else {
				log.Error("Data Reduction - Error when open the Storage Trie", "err", stErr, "storageroot", account.Root, "account", addr)
			}
		}

		if account.TX1Root != emptyRoot {
			if tx1Trie, tx1Err := p.bc.StateCache().OpenTX1Trie(common.Hash{}, account.TX1Root); tx1Err == nil {
				pruneTrie(tx1Trie, nodeCount, &p.pendingDeleteHashList, nil)
			} else {
				log.Error("Data Reduction - Error when open the TX1 Trie", "err", tx1Err, "tx1root", account.TX1Root, "account", addr)
			}
		}

		if account.TX3Root != emptyRoot {
			if tx3Trie, tx3Err := p.bc.StateCache().OpenTX3Trie(common.Hash{}, account.TX3Root); tx3Err == nil {
				pruneTrie(tx3Trie, nodeCount, &p.pendingDeleteHashList, nil)
			} else {
				log.Error("Data Reduction - Error when open the TX3 Trie", "err", tx3Err, "tx3root", account.TX3Root, "account", addr)
			}
		}

		if account.ProxiedRoot != emptyRoot {
			if proxiedTrie, proxiedErr := p.bc.StateCache().OpenProxiedTrie(common.Hash{}, account.ProxiedRoot); proxiedErr == nil {
				pruneTrie(proxiedTrie, nodeCount, &p.pendingDeleteHashList, nil)
			} else {
				log.Error("Data Reduction - Error when open the Proxied Trie", "err", proxiedErr, "proxiedroot", account.ProxiedRoot, "account", addr)
			}
		}

		if account.RewardRoot != emptyRoot {
			if rewardTrie, rewardErr := p.bc.StateCache().OpenRewardTrie(common.Hash{}, account.RewardRoot); rewardErr == nil {
				pruneTrie(rewardTrie, nodeCount, &p.pendingDeleteHashList, nil)
			} else {
				log.Error("Data Reduction - Error when open the Reward Trie", "err", rewardErr, "rewardroot", account.RewardRoot, "account", addr)
			}
		}
	})

}

func pruneTrie(t state.Trie, nodeCount NodeCount, pendingDeleteHashList *[]common.Hash, processLeaf processLeafTrie) {
	child := true
	for it := t.NodeIterator(nil); it.Next(child); {
		if !it.Leaf() {
			nodeHash := it.Hash()
			if nodeCount[nodeHash] > 0 {
				nodeCount[nodeHash]--
			}

			if nodeCount[nodeHash] == 0 {
				child = true
				*pendingDeleteHashList = append(*pendingDeleteHashList, nodeHash)
				delete(nodeCount, nodeHash)
			} else {
				child = false
			}
		} else {
			// Process the Account -> Inner Trie
			if processLeaf != nil {
				addr := t.GetKey(it.LeafKey())
				if len(addr) == 20 {
					var data state.Account
					rlp.DecodeBytes(it.LeafBlob(), &data)

					processLeaf(common.BytesToAddress(addr), data)
				}
			}
		}
	}
}

func (p *PruneProcessor) commitDataPruneTrie(nodeCount NodeCount, lastScanNumber, lastPruneNumber uint64) {
	// Store the Node Count into data prune trie
	// Commit the Prune Trie
	pruneTrie, _ := p.prunedb.OpenPruneTrie(common.Hash{})

	for key, count := range nodeCount {
		value, _ := rlp.EncodeToBytes(count)
		pruneTrie.TryUpdate(key[:], value)
	}
	pruneTrieRoot, commit_err := pruneTrie.Commit(nil)
	log.Info("Data Reduction - Commit Prune Trie", "hash", pruneTrieRoot.Hex(), "err", commit_err)
	// Commit to Prune DB
	db_commit_err := p.prunedb.TrieDB().Commit(pruneTrieRoot, true)
	log.Info("Data Reduction - Write to Prune DB", "err", db_commit_err)

	// Write the Root Hash of Prune Trie
	rawdb.WriteDataPruneTrieRootHash(p.db, pruneTrieRoot, lastScanNumber, lastPruneNumber)
	// Write the last number
	p.writeLastNumber(lastScanNumber, lastPruneNumber)
}

func (p *PruneProcessor) writeLastNumber(lastScanNumber, lastPruneNumber uint64) {
	rawdb.WriteHeadScanNumber(p.db, lastScanNumber)
	rawdb.WriteHeadPruneNumber(p.db, lastPruneNumber)
}

func (nc NodeCount) String() string {
	list := make([]common.Hash, 0, len(nc))
	for key := range nc {
		list = append(list, key)
	}
	sort.Slice(list, func(i, j int) bool {
		return bytes.Compare(list[i].Bytes(), list[j].Bytes()) == 1
	})

	result := ""
	for _, key := range list {
		result += fmt.Sprintf("%v: %d \n", key.Hex(), nc[key])
	}
	return result
}

func GetLatestStatus(prunedb ethdb.Database) *PruneStatus {
	var scanNo, pruneNo uint64
	if ps := rawdb.ReadHeadScanNumber(prunedb); ps != nil {
		scanNo = *ps
	}
	if pp := rawdb.ReadHeadPruneNumber(prunedb); pp != nil {
		pruneNo = *pp
	}

	return &PruneStatus{
		Running:           atomic.LoadInt32(&pruning) == 1,
		LatestScanNumber:  scanNo,
		LatestPruneNumber: pruneNo,
	}
}
