package state

import (
	"bytes"
	"github.com/ethereum/go-ethereum/log"
	"time"

	. "github.com/tendermint/go-common"
	//cfg "github.com/tendermint/go-config"
	//dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	//"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex"
	//"github.com/ethereum/go-ethereum/consensus/tendermint/state/txindex/null"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
	//"fmt"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/pkg/errors"
)

/*
var (
	stateKey         = []byte("stateKey")
)
*/
//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	// mtx for writing to db
	//mtx sync.Mutex
	//db  dbm.DB

	// should not change
	//GenesisDoc *types.GenesisDoc

	/*
		ChainID    string
		Height     uint64 // Genesis state has this set to 0.  So, Block(H=0) does not exist.
		Time       time.Time
		BlockID    types.BlockID
		NeedToSave 	bool //record the number of the block which should be saved to main chain
		EpochNumber	uint64
	*/
	TdmExtra *types.TendermintExtra

	Epoch *epoch.Epoch
	//Validators      *types.ValidatorSet
	//LastValidators  *types.ValidatorSet // block.LastCommit validated against this

	// AppHash is updated after Commit
	//AppHash []byte

	//TxIndexer txindex.TxIndexer `json:"-"` // Transaction indexer.

	// Intermediate results from processing
	// Persisted separately from the state
	//abciResponses *ABCIResponses

	logger log.Logger
}

func NewState(logger log.Logger) *State {
	return &State{logger: logger}
}

/*
func LoadState(stateDB dbm.DB) *State {
	state := loadState(stateDB, stateKey)
	return state
}

func loadState(db dbm.DB, key []byte) *State {
	s := &State{db: db, TxIndexer: &null.TxIndex{}}
	buf := db.Get(key)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&s, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			Exit(Fmt("LoadState: Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}
*/

func (s *State) Copy() *State {
	//fmt.Printf("State.Copy(), s.LastValidators are %v\n",s.LastValidators)
	//debug.PrintStack()

	return &State{
		//db:              s.db,
		//GenesisDoc: s.GenesisDoc,
		/*
			ChainID:         s.ChainID,
			Height:			 s.Height,
			BlockID:         s.BlockID,
			Time: 		     s.Time,
			EpochNumber:     s.EpochNumber,
			NeedToSave:      s.NeedToSave,
		*/
		TdmExtra: s.TdmExtra.Copy(),
		Epoch:    s.Epoch.Copy(),
		//Validators:      s.Validators.Copy(),
		//LastValidators:  s.LastValidators.Copy(),
		//AppHash:         s.AppHash,
		//TxIndexer:       s.TxIndexer, // pointer here, not value
		logger: s.logger,
	}
}

/*
func (s *State) Save() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.db.SetSync(stateKey, s.Bytes())
}
*/
func (s *State) Equals(s2 *State) bool {
	return bytes.Equal(s.Bytes(), s2.Bytes())
}

func (s *State) Bytes() []byte {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(s, buf, n, err)
	if *err != nil {
		PanicCrisis(*err)
	}
	return buf.Bytes()
}

func (s *State) GetValidators() (*types.ValidatorSet, *types.ValidatorSet, error) {

	if s.Epoch == nil {
		return nil, nil, errors.New("epoch does not exist")
	}

	if s.TdmExtra.EpochNumber == uint64(s.Epoch.Number) {
		return s.Epoch.Validators, s.Epoch.Validators, nil
	} else if s.TdmExtra.EpochNumber == uint64(s.Epoch.Number-1) {
		return s.Epoch.GetPreviousEpoch().Validators, s.Epoch.Validators, nil
	}

	return nil, nil, errors.New("epoch information error")
}

//-----------------------------------------------------------------------------
// Genesis

// MakeGenesisState creates state from types.GenesisDoc.
//
// Used in tests.
func MakeGenesisState( /*db dbm.DB,  genDoc *types.GenesisDoc,*/ chainID string, logger log.Logger) *State {
	//if len(genDoc.CurrentEpoch.Validators) == 0 {
	//	Exit(Fmt("The genesis file has no validators"))
	//}
	//
	//if genDoc.GenesisTime.IsZero() {
	//	genDoc.GenesisTime = time.Now()
	//}

	// Make validators slice
	//validators := make([]*types.Validator, len(genDoc.CurrentEpoch.Validators))
	//for i, val := range genDoc.CurrentEpoch.Validators {
	//	pubKey := val.PubKey
	//	address := pubKey.Address()
	//
	//	// Make validator
	//	validators[i] = &types.Validator{
	//		Address:     address,
	//		PubKey:      pubKey,
	//		VotingPower: val.Amount,
	//	}
	//}

	return &State{
		//db:              db,
		//GenesisDoc: genDoc,
		TdmExtra: &types.TendermintExtra{
			ChainID:         chainID,
			Height:          0,
			Time:            time.Now(),
			EpochNumber:     0,
			NeedToSave:      false,
			NeedToBroadcast: false,
		},
		//Validators:      types.NewValidatorSet(validators),
		//LastValidators:  types.NewValidatorSet(nil),
		//AppHash:         genDoc.AppHash,
		//TxIndexer:       &null.TxIndex{}, // we do not need indexer during replay and in tests
		logger: logger,
	}
}
