package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/abci/example/dummy"
	crypto "github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	cfg "github.com/ethereum/go-ethereum/consensus/tendermint/config/tendermint_test"
	"github.com/ethereum/go-ethereum/consensus/tendermint/mempool"
	"github.com/ethereum/go-ethereum/consensus/tendermint/proxy"
	"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	ep "github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
)

var (
	privKey      = crypto.GenPrivKeyEd25519FromSecret([]byte("execution_test"))
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func TestApplyBlock(t *testing.T) {
	cc := proxy.NewLocalClientCreator(dummy.NewDummyApplication())
	config := cfg.ResetConfig("execution_test_")
	proxyApp := proxy.NewAppConns(config, cc, nil)
	_, err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()
	mempool := mempool.NewMempool(config, proxyApp.Mempool())

	state := state()
	indexer := &dummyIndexer{0}
	state.TxIndexer = indexer

	epoch := epoch()
	state.Epoch = epoch

	// make block
	block := makeBlock(1, state)

	err = state.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool)

	require.Nil(t, err)
	assert.Equal(t, nTxsPerBlock, indexer.Indexed) // test indexing works

	// TODO check state and mempool
}

//----------------------------------------------------------------------------

// make some bogus txs
func makeTxs(blockNum int) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(blockNum), byte(i)}))
	}
	return txs
}

func state() *State {
	return MakeGenesisState(dbm.NewMemDB(), &types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			types.GenesisValidator{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	})
}

func epoch() *ep.Epoch {
	return &ep.Epoch{}
}

func makeBlock(num int, state *State) *types.Block {
	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	_,val,_ := state.GetValidators()
	valHash := val.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}
	block, _ := types.MakeBlock(num, chainID, makeTxs(num), new(types.Commit),
		prevBlockID, valHash, state.AppHash, nil, testPartSize)
	return block
}

// dummyIndexer increments counter every time we index transaction.
type dummyIndexer struct {
	Indexed int
}

func (indexer *dummyIndexer) Get(hash []byte) (*types.TxResult, error) {
	return nil, nil
}

func (indexer *dummyIndexer) AddBatch(batch *txindex.Batch) error {
	indexer.Indexed += batch.Size()
	return nil
}
