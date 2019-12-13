package state

import (
	"bytes"
	"github.com/ethereum/go-ethereum/log"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/pkg/errors"
)

// NOTE: not goroutine-safe.
type State struct {
	TdmExtra *types.TendermintExtra
	Epoch *epoch.Epoch
	logger log.Logger
}

func NewState(logger log.Logger) *State {
	return &State{logger: logger}
}

func (s *State) Copy() *State {

	return &State{
		TdmExtra: s.TdmExtra.Copy(),
		Epoch:    s.Epoch.Copy(),
		logger: s.logger,
	}
}

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
func MakeGenesisState(chainID string, logger log.Logger) *State {

	return &State{
		TdmExtra: &types.TendermintExtra{
			ChainID:         chainID,
			Height:          0,
			Time:            time.Now(),
			EpochNumber:     0,
			NeedToSave:      false,
			NeedToBroadcast: false,
		},
		logger: logger,
	}
}
