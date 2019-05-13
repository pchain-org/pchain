package datareduction

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"log"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/rlp"
	"time"
	"github.com/ethereum/go-ethereum/ethdb"
	"sort"
	"bytes"
	"fmt"
)

var max_count_trie uint64 = 100

type PruneDB struct {
	prunerawdb            ethdb.Database
	db                    PruneDatabase
	pchaindb              ethdb.Database
	statedb               state.Database
	pendingDeleteHashList []common.Hash
}

type NodeCount map[common.Hash]uint64

func New(pchaindb ethdb.Database, prunerawdb ethdb.Database) *PruneDB {
	statedb := state.NewDatabase(pchaindb)
	prunedb := NewDatabase(prunerawdb)
	return &PruneDB{
		prunerawdb:            prunerawdb,
		db:                    prunedb,
		pchaindb:              pchaindb,
		statedb:               statedb,
		pendingDeleteHashList: nil,
	}
}

func (p *PruneDB) ScanAndPrune(blockNumber *uint64, scanNumber, pruneNumber uint64) (uint64, uint64){
	lastScanNumber, nodeCount := p.ScanTrie(blockNumber, scanNumber, pruneNumber)
	//lastScanNumber, _ := prune.ScanTrie(blockNumber, scanNumber, pruneNumber)

	log.Printf("after scan, before prune, last number scan %v, prune %v", lastScanNumber, pruneNumber)
	lastPruneNumber := p.PruneTrie(nodeCount, lastScanNumber, pruneNumber)
	return lastScanNumber, lastPruneNumber
}

func (p *PruneDB) ScanTrie(blockNumber *uint64, scanNumber, pruneNumber uint64) (uint64, NodeCount) {
	return p.scanTrie(blockNumber, scanNumber, pruneNumber)
}

func (p *PruneDB) PruneTrie(count NodeCount, scanNumber, pruneNumber uint64) uint64 {
	return p.pruneTrie(count, scanNumber, pruneNumber)
}

func (p *PruneDB) scanTrie(blockNumber *uint64, scanNumber, pruneNumber uint64) (uint64, NodeCount) {
	pchaindb := p.pchaindb
	//pchaindb, _ := rawdb.NewLevelDBDatabase("C:\\Users\\liaoyd\\AppData\\Roaming\\Pchain\\testnet\\geth\\chaindata", 0, 0, "eth/db/chaindata/")
	newScanNumber := scanNumber
	nodeCount := p.readLatestNodeCount(scanNumber, pruneNumber)
	log.Println("Data Reduction - Scan: fetch the latest node count, total nodes ", len(nodeCount))
	log.Println("Data Reduction - Scan: try to scan the block")
	scanOrNot, scanStart, scanEnd := calculateScan(scanNumber, *blockNumber)
	if scanOrNot {
		//scanEnd = 199

		//top10value(nodeCount)

		// scan the block
		b := scanEnd - scanStart + 1
		mul := b / max_count_trie
		for i := uint64(0); i < mul; i++ {
			round_f := scanStart + i*max_count_trie
			round_t := scanStart + (i+1)*max_count_trie - 1

			//PrintMemUsage()
			start := time.Now()
			for j := round_f; j <= round_t; j++ {
				blockHash := rawdb.ReadCanonicalHash(pchaindb, j)
				header := rawdb.ReadHeader(pchaindb, blockHash, j)

				//log.Printf("Block: %v, Root %x", i, header.Root)
				p.countBlockChainTrie(header.Root, nodeCount)
				//log.Println(nodeCount)
			}
			//top10value(nodeCount)
			//PrintMemUsage()

			p.commitDataPruneTrie(nodeCount, round_t, pruneNumber)
			newScanNumber = round_t

			log.Println(time.Since(start).String())
			log.Println("Total Nodes: ", len(nodeCount))
			log.Println("Scan for trie", round_t, pruneNumber)
		}
		log.Println("Data Reduction - Scan: Scan Block completed")
	} else {
		log.Println("Data Reduction - Scan: Scan Block not required")
	}
	return newScanNumber, nodeCount
}

func (p *PruneDB) readLatestNodeCount(scanNumber, pruneNumber uint64) NodeCount {
	prunedb := p.db
	prunerawdb := p.prunerawdb

	nodeCount := make(NodeCount)

	lastHash := rawdb.ReadDataPruneTrieRootHash(prunerawdb, scanNumber, pruneNumber)
	log.Println("Last Hash: ", lastHash.Hex())
	if (lastHash != common.Hash{}) {
		lastPruneTrie, openErr := prunedb.OpenPruneTrie(lastHash)
		if openErr != nil {
			log.Println("Unable read the last Prune Trie. Error ", openErr)
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


func (p *PruneDB) pruneTrie(count NodeCount, scanNumber, pruneNumber uint64) uint64 {
	pchaindb := p.pchaindb

	newPruneNumber := pruneNumber
	pruneOrNot, pruneStart, pruneEnd := calculatePrune(pruneNumber, scanNumber)
	if pruneOrNot {
		log.Println("Data Reduction - Prune: try to prune the Block")
		b := pruneEnd - pruneStart + 1
		mul := b / max_count_trie
		for i := uint64(0); i < mul; i++ {
			round_f := pruneStart + i*max_count_trie
			round_t := pruneStart + (i+1)*max_count_trie - 1

			p.pendingDeleteHashList = nil

			start := time.Now()
			for j := round_f; j <= round_t; j++ {
				blockHash := rawdb.ReadCanonicalHash(pchaindb, j)
				header := rawdb.ReadHeader(pchaindb, blockHash, j)

				//log.Printf("Block: %v, Root %x", i, header.Root)
				p.pruneBlockChainTrie(header.Root, count)
				//log.Println(nodeCount)
			}
			log.Println(time.Since(start).String())
			log.Println("Total Nodes: ", len(count))
			log.Println("Total Pending Delete Hash List: ", len(p.pendingDeleteHashList))

			batch := pchaindb.NewBatch()
			for _, hash := range p.pendingDeleteHashList {
				batch.Delete(hash.Bytes())
			}
			writeErr := batch.Write()
			log.Println(writeErr)

			p.commitDataPruneTrie(count, scanNumber, round_t)
			newPruneNumber = round_t

			log.Println("Prune for trie", scanNumber, round_t)
		}
		log.Println("Data Reduction - Prune: Prune Block completed")
	} else {
		log.Println("Data Reduction - Prune: Prune Block not required")
	}
	return newPruneNumber
}

func (p *PruneDB) pruneBlockChainTrie(root common.Hash, nodeCount NodeCount) {
	statedb := p.statedb
	t, openErr := statedb.OpenTrie(root)
	if openErr != nil {
		log.Println(openErr)
		return
	}

	child := true

	it := t.NodeIterator(nil)
	for i := 0; it.Next(child); i++ {
		if !it.Leaf() {
			nodeHash := it.Hash()
			if nodeCount[nodeHash] > 0 {
				nodeCount[nodeHash]--
			}

			if nodeCount[nodeHash] == 0 {
				child = true
				p.pendingDeleteHashList = append(p.pendingDeleteHashList, nodeHash)
				delete(nodeCount, nodeHash)
			} else {
				child = false
			}
		}
	}

	//log.Println(pchaindb.Has(pendingDeleteHashList[0][:]))

}

func (p *PruneDB) commitDataPruneTrie(nodeCount NodeCount, lastScanNumber, lastPruneNumber uint64) {
	// Store the Node Count into data prune trie
	// Commit the Prune Trie
	prunedb := p.db
	prunerawdb := p.prunerawdb

	pruneTrie, _ := prunedb.OpenPruneTrie(common.Hash{})

	for key, count := range nodeCount {
		value, _ := rlp.EncodeToBytes(count)
		pruneTrie.TryUpdate(key[:], value)
	}
	pruneTrieRoot, commit_err := pruneTrie.Commit(nil)
	log.Println("Commit Hash", pruneTrieRoot.Hex(), commit_err)
	// Commit to Prune DB
	db_commit_err := prunedb.TrieDB().Commit(pruneTrieRoot, true)
	log.Println(db_commit_err)

	// Write the Root Hash of Prune Trie
	rawdb.WriteDataPruneTrieRootHash(prunerawdb, pruneTrieRoot, lastScanNumber, lastPruneNumber)
	rawdb.WriteHeadScanNumber(prunerawdb, lastScanNumber)
	rawdb.WriteHeadPruneNumber(prunerawdb, lastPruneNumber)
}

func calculatePrune(prune, scan uint64) (pruneOrNot bool, from, to uint64) {
	if scan >= prune+max_count_trie {
		pruneOrNot = true
		if prune == 0 {
			from = 0
		} else {
			from = prune + 1
		}
		to = scan - max_count_trie
	}
	return
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
		to = mul*max_count_trie - 1
	}

	if to != 0 {
		scanOrNot = true
	}

	return
}

func (p *PruneDB) countBlockChainTrie(root common.Hash, nodeCount NodeCount) {
	statedb := p.statedb
	t, openErr := statedb.OpenTrie(root)
	if openErr != nil {
		log.Println(openErr)
		return
	}

	child := true

	it := t.NodeIterator(nil)
	for i := 0; it.Next(child); i++ {
		if !it.Leaf() {
			//log.Println("node ", i, "hash - ", it.Hash().Hex())
			nodeHash := it.Hash()
			if _, exist := nodeCount[nodeHash]; exist {
				nodeCount[nodeHash]++
				child = false
			} else {
				nodeCount[nodeHash] = 1
				child = true
			}
		}
		//} else {
		//
		//	var data state.Account
		//	rlp.DecodeBytes(it.LeafBlob(), &data)
		//
		//}
	}
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
