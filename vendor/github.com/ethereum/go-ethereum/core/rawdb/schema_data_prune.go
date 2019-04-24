// Package rawdb contains a collection of low level database accessors.
package rawdb

// The fields below define the low level database schema prefixing for data prune.
var (
	// headDataScanKey tracks the latest know scan header's number.
	headDataScanKey = []byte("LastDataScanHeight")

	// headDataPruneKey tracks the latest know prune header's number.
	headDataPruneKey = []byte("LastDataPruneHeight")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	dataPruneProcessPrefix = []byte("p") // dataPruneProcessPrefix + scan num (uint64 big endian) + prune num (uint64 big endian) + dataPruneProcessSuffix -> trie root hash
	dataPruneProcessSuffix = []byte("n")
)

// dataPruneNumberKey = dataPruneProcessPrefix + scan (uint64 big endian) + prune (uint64 big endian) + dataPruneProcessSuffix
func dataPruneNumberKey(scan, prune uint64) []byte {
	return append(append(append(dataPruneProcessPrefix, encodeBlockNumber(scan)...), encodeBlockNumber(prune)...), dataPruneProcessSuffix...)
}
