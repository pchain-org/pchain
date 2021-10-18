package rawdb

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	pabi "github.com/pchain/abi"
)

var (
	tx3Prefix       = []byte("t") // tx3Prefix + chainId + txHash -> tx3
	tx3LookupPrefix = []byte("k") // tx3LookupPrefix + chainId + txHash -> tx3 lookup metadata
	Tx3ProofPrefix  = []byte("p") // tx3ProofPrefix + chainId + height -> proof data
)

// TX3LookupEntry is a positional metadata to help looking up the tx3 proof content given only its chainId and hash.
type TX3LookupEntry struct {
	BlockIndex uint64
	TxIndex    uint64
}

func GetTX3(db ethdb.Reader, chainId string, txHash common.Hash) *types.Transaction {
	key := append(tx3Prefix, append([]byte(chainId), txHash.Bytes()...)...)
	bs, err := db.Get(key)
	if len(bs) == 0 || err != nil {
		return nil
	}

	tx, err := decodeTx(bs)
	if err != nil {
		return nil
	}

	return tx
}

func GetTX3ProofData(db ethdb.Reader, chainId string, txHash common.Hash) *types.TX3ProofData {
	// Retrieve the lookup metadata
	hash, blockNumber, txIndex := GetTX3LookupEntry(db, chainId, txHash)
	if hash == (common.Hash{}) {
		return nil
	}

	encNum := encodeBlockNumber(blockNumber)
	key := append(Tx3ProofPrefix, append([]byte(chainId), encNum...)...)
	bs, err := db.Get(key)
	if len(bs) == 0 || err != nil {
		return nil
	}

	var proofData types.TX3ProofData
	err = rlp.DecodeBytes(bs, &proofData)
	if err != nil {
		return nil
	}

	var i int
	for i = 0; i < len(proofData.TxIndexs); i++ {
		if uint64(proofData.TxIndexs[i]) == txIndex {
			break
		}
	}
	if i >= len(proofData.TxIndexs) { // can't find the txIndex
		return nil
	}

	ret := types.TX3ProofData{
		Header:   proofData.Header,
		TxIndexs: make([]uint, 1),
		TxProofs: make([]*types.BSKeyValueSet, 1),
	}
	ret.TxIndexs[0] = proofData.TxIndexs[i]
	ret.TxProofs[0] = proofData.TxProofs[i]

	return &ret
}

func GetTX3LookupEntry(db ethdb.Reader, chainId string, txHash common.Hash) (common.Hash, uint64, uint64) {
	// Load the positional metadata from disk and bail if it fails
	key := append(tx3LookupPrefix, append([]byte(chainId), txHash.Bytes()...)...)
	bs, err := db.Get(key)
	if len(bs) == 0 || err != nil {
		return common.Hash{}, 0, 0
	}

	// Parse and return the contents of the lookup entry
	var entry TX3LookupEntry
	if err := rlp.DecodeBytes(bs, &entry); err != nil {
		return common.Hash{}, 0, 0
	}
	return txHash, entry.BlockIndex, entry.TxIndex
}

func GetAllTX3ProofData(db ethdb.Database) []*types.TX3ProofData {
	var ret []*types.TX3ProofData
	iter := db.NewIteratorWithPrefix(Tx3ProofPrefix)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		if !bytes.HasPrefix(key, Tx3ProofPrefix) {
			break
		}

		var proofData *types.TX3ProofData
		err := rlp.DecodeBytes(value, proofData)
		if err != nil {
			continue
		}
		ret = append(ret, proofData)
	}

	return ret
}

func WriteTX3(db ethdb.Writer, chainId string, header *types.Header, txIndex uint, val []byte) error {

	var tx types.Transaction
	err := rlp.DecodeBytes(val, &tx)
	if err != nil {
		return err
	}

	if pabi.IsPChainContractAddr(tx.To()) {
		data := tx.Data()
		function, err := pabi.FunctionTypeFromId(data[:4])
		if err != nil {
			return err
		}

		if function == pabi.WithdrawFromChildChain {
			txHash := tx.Hash()
			key1 := append(tx3Prefix, append([]byte(chainId), txHash.Bytes()...)...)
			bs, _ := rlp.EncodeToBytes(&tx)
			if err = db.Put(key1, bs); err != nil {
				return err
			}

			entry := TX3LookupEntry{
				BlockIndex: header.Number.Uint64(),
				TxIndex:    uint64(txIndex),
			}
			data, _ := rlp.EncodeToBytes(entry)
			key2 := append(tx3LookupPrefix, append([]byte(chainId), txHash.Bytes()...)...)
			if err := db.Put(key2, data); err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteTX3(db ethdb.Database, chainId string, txHash common.Hash) {
	// Retrieve the lookup metadata
	hash, blockNumber, txIndex := GetTX3LookupEntry(db, chainId, txHash)
	if hash == (common.Hash{}) {
		return
	}

	// delete the tx3 itself
	key1 := append(tx3Prefix, append([]byte(chainId), txHash.Bytes()...)...)
	db.Delete(key1)

	// delete the tx3 lookup metadata
	key2 := append(tx3LookupPrefix, append([]byte(chainId), txHash.Bytes()...)...)
	db.Delete(key2)

	encNum := encodeBlockNumber(blockNumber)
	key3 := append(Tx3ProofPrefix, append([]byte(chainId), encNum...)...)
	bs, err := db.Get(key3)
	if len(bs) == 0 || err != nil {
		return
	}

	var proofData types.TX3ProofData
	err = rlp.DecodeBytes(bs, &proofData)
	if err != nil {
		return
	}

	var i int
	for i = 0; i < len(proofData.TxIndexs); i++ {
		if uint64(proofData.TxIndexs[i]) == txIndex {
			break
		}
	}
	if i >= len(proofData.TxIndexs) { // can't find the txIndex
		return
	}

	proofData.TxIndexs = append(proofData.TxIndexs[:i], proofData.TxIndexs[i+1:]...)
	proofData.TxProofs = append(proofData.TxProofs[:i], proofData.TxProofs[i+1:]...)
	if len(proofData.TxIndexs) == 0 {
		// delete the whole proof data
		db.Delete(key3)
	} else {
		// update the proof data
		bs, _ := rlp.EncodeToBytes(proofData)
		db.Put(key3, bs)
	}
}

func decodeTx(txBytes []byte) (*types.Transaction, error) {

	tx := new(types.Transaction)
	rlpStream := rlp.NewStream(bytes.NewBuffer(txBytes), 0)
	if err := tx.DecodeRLP(rlpStream); err != nil {
		return nil, err
	}
	return tx, nil
}
