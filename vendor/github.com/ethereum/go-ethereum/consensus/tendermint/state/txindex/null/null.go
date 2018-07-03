package null

import (
	"errors"

	"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

// TxIndex acts as a /dev/null.
type TxIndex struct{}

// Tx panics.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	return nil, errors.New(`Indexing is disabled (set 'tx_index = "kv"' in config)`)
}

// Batch returns nil.
func (txi *TxIndex) AddBatch(batch *txindex.Batch) error {
	return nil
}
