package rawdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"encoding/binary"
	"github.com/ethereum/go-ethereum/log"
	)

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadPruneRoot(db ethdb.Reader, s, d uint64) common.Hash {
	data, _ := db.Get(pruneKey(s, d))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

func WritePruneRoot(db ethdb.Writer, hash common.Hash, s, d uint64) {
	if err := db.Put(pruneKey(s, d), hash.Bytes()); err != nil {
		log.Crit("Failed to store last scan hash", "err", err)
	}
}
//
//func WriteHeadScanHash(db ethdb.Writer, hash common.Hash, s, d uint64) {
//	if err := db.Put(pruneKey(s, d), hash.Bytes()); err != nil {
//		log.Crit("Failed to store last scan hash", "err", err)
//	}
//}

func ReadScanHeight(db ethdb.Reader) *uint64 {
	data, _ := db.Get(scanKey)
	if len(data) == 0 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

func WriteScanHeight(db ethdb.Writer, s uint64) {
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], s)
	if err := db.Put(scanKey, count[:]); err != nil {
		log.Crit("Failed to store last scan hash", "err", err)
	}
}

func ReadDeleteHeight(db ethdb.Reader) *uint64 {
	data, _ := db.Get(deleteKey)
	if len(data) == 0 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

func WriteDeleteHeight(db ethdb.Writer, d uint64) {
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], d)
	if err := db.Put(deleteKey, count[:]); err != nil {
		log.Crit("Failed to store last scan hash", "err", err)
	}
}